// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package firmware

import (
	"bytes"
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"time"

	gossh "golang.org/x/crypto/ssh"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"chromiumos/tast/common/servo"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/dut"
	"chromiumos/tast/errors"
	"chromiumos/tast/lsbrelease"
	"chromiumos/tast/remote/firmware/reporters"
	"chromiumos/tast/remote/firmware/rpm"
	"chromiumos/tast/rpc"
	fwpb "chromiumos/tast/services/cros/firmware"
	"chromiumos/tast/shutil"
	"chromiumos/tast/ssh/linuxssh"
	"chromiumos/tast/testing"
)

// Helper tracks several firmware-related objects. The recommended way to initialize the helper is to use firmware.fixture:
//
// import (
//	...
//	"chromiumos/tast/remote/firmware/fixture"
// )
//
// func init() {
//	testing.AddTest(&testing.Test{
//		...
//              Fixture: fixture.NormalMode,
//	})
// }
//
// func MyTest(ctx context.Context, s *testing.State) {
// 	h := s.FixtValue().(*fixture.Value).Helper
//
// 	if err := h.RequireServo(ctx); err != nil {
// 		s.Fatal("Failed to init servo: ", err)
// 	}
// ...
// }
type Helper struct {
	// BiosServiceClient provides bios related services such as GBBFlags manipulation.
	BiosServiceClient fwpb.BiosServiceClient

	// Board contains the DUT's board, as reported by the Platform RPC.
	// Currently, this is based on /etc/lsb-release's CHROMEOS_RELEASE_BOARD.
	Board string

	// Config contains a variety of platform-specific attributes.
	Config *Config

	// cfgFilepath is the full path to the data directory containing fw-testing-configs JSON files.
	// Any tests requiring a Config should set cfgFilepath to s.DataPath(firmware.ConfigFile) during NewHelper.
	cfgFilepath string

	// These vars track whether the DUT's on-board image, and the USB images are known to have up-to-date Tast host files.
	dutInternalStorageHasTastFiles bool
	dutUsbHasTastFiles             bool

	// DUT is used for communicating with the device under test.
	DUT *dut.DUT

	// hostFilesTmpDir is a temporary directory on the test server holding a copy of Tast host files.
	hostFilesTmpDir string

	// Model contains the DUT's model, as reported by the Platform RPC.
	// Currently, this is based on cros_config / name.
	Model string

	// Reporter reports various info from the DUT.
	Reporter *reporters.Reporter

	// RPCClient is a direct client connection to the Tast gRPC server hosted on the DUT.
	RPCClient *rpc.Client

	// disallowServices prevents RequireRPCClient from working if set.
	disallowServices bool

	// rpcHint is needed in order to create an RPC client connection.
	rpcHint *testing.RPCHint

	// RPCUtils allows the Helper to call the firmware utils RPC service.
	RPCUtils fwpb.UtilsServiceClient

	// Servo allows us to send commands to a servo device.
	Servo *servo.Servo

	// servoHostPort is the address and port of the machine acting as the servo host, normally provided via the "servo" command-line variable.
	servoHostPort string
	keyFile       string
	keyDir        string

	// ServoProxy wraps the Servo object, and communicates with the servod instance.
	ServoProxy *servo.Proxy

	// RPM is a remote power management client. Only valid in the test lab.
	RPM *rpm.RPM

	// dutHostname is the real name of the dut, even if tast is connected to a forwarded port.
	dutHostname string

	// powerunitHostname, powerunitOutlet, hydraHostname identify the managed power outlet for the DUT.
	powerunitHostname, powerunitOutlet, hydraHostname string
}

// WaitConnectOption includes situations to wait to connect from.
type WaitConnectOption string

const (
	// FromHibernation alerts WaitConnect to skip
	// on setting servo control while DUT is still
	// in the process of waking up from hibernation.
	FromHibernation WaitConnectOption = "hibernation"
)

// NewHelper creates a new Helper object with info from testing.State.
// For tests that do not use a certain Helper aspect (e.g. RPC or Servo), it is OK to pass null-values (nil or "").
func NewHelper(d *dut.DUT, rpcHint *testing.RPCHint, cfgFilepath, servoHostPort, dutHostname, powerunitHostname, powerunitOutlet, hydraHostname string) *Helper {
	return &Helper{
		cfgFilepath:       cfgFilepath,
		DUT:               d,
		keyFile:           d.KeyFile(),
		keyDir:            d.KeyDir(),
		Reporter:          reporters.New(d),
		rpcHint:           rpcHint,
		servoHostPort:     servoHostPort,
		dutHostname:       dutHostname,
		powerunitHostname: powerunitHostname,
		powerunitOutlet:   powerunitOutlet,
		hydraHostname:     hydraHostname,
	}
}

// NewHelperWithoutDUT creates a new Helper object with info from testing.State. The resulting Helper will be unable to ssh to the DUT.
func NewHelperWithoutDUT(cfgFilepath, servoHostPort, keyFile, keyDir string) *Helper {
	return &Helper{
		cfgFilepath:   cfgFilepath,
		keyFile:       keyFile,
		keyDir:        keyDir,
		servoHostPort: servoHostPort,
	}
}

// Close shuts down any firmware objects associated with the Helper.
// Generally, tests should defer Close() immediately after initializing a Helper.
func (h *Helper) Close(ctx context.Context) error {
	var allErrors []error
	if h.hostFilesTmpDir != "" {
		if err := os.RemoveAll(h.hostFilesTmpDir); err != nil {
			allErrors = append(allErrors, errors.Wrap(err, "removing server's copy of Tast host files"))
		}
		h.hostFilesTmpDir = ""
	}
	if err := h.CloseRPCConnection(ctx); err != nil {
		isIgnorable := false
		for rootErr := err; rootErr != nil && !isIgnorable; rootErr = errors.Unwrap(rootErr) {
			// The gRPC Canceled error just means the connection is already closed.
			if st, ok := status.FromError(rootErr); ok && st.Code() == codes.Canceled {
				isIgnorable = true
			}
		}
		if !isIgnorable {
			allErrors = append(allErrors, errors.Wrap(err, "closing rpc connection"))
		}
	}
	if err := h.CloseServo(ctx); err != nil {
		allErrors = append(allErrors, errors.Wrap(err, "closing servo"))
	}
	if len(allErrors) > 0 {
		for err := range allErrors[1:] {
			testing.ContextLog(ctx, "Suppressed error: ", err)
		}
		return allErrors[0]
	}
	return nil
}

// EnsureDUTBooted checks the power state, and attempts to boot the DUT if it is off.
func (h *Helper) EnsureDUTBooted(ctx context.Context) error {
	if h.DUT != nil && h.DUT.Connected(ctx) {
		return nil
	}
	if err := h.RequireServo(ctx); err != nil {
		return errors.Wrap(err, "could not connect to servo")
	}
	if hasEC, err := h.Servo.HasControl(ctx, string(servo.ECSystemPowerState)); err != nil {
		testing.ContextLog(ctx, "Error checking for chrome ec: ", err)
	} else if hasEC {
		state, err := h.Servo.GetECSystemPowerState(ctx)
		if err != nil {
			testing.ContextLog(ctx, "Error getting power state: ", err)
		}
		if state == "S0" {
			testing.ContextLog(ctx, "Waiting for DUT to finish booting")
			// The machine is up, just wait for it to finish booting.
			h.CloseRPCConnection(ctx)
			waitBootCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
			defer cancel()
			if err = h.WaitConnect(waitBootCtx); err == nil {
				return nil
			}
			// If WaitConnect didn't work, let it reset.
		}
	}
	testing.ContextLog(ctx, "Connecting power")
	if err := h.SetDUTPower(ctx, true); err != nil {
		testing.ContextLog(ctx, "Failed to connect charger: ", err)
	}

	// Cr50 goes to sleep during hibernation and battery cutoff, and when DUT wakes, CCD state might be locked.
	if val, err := h.Servo.GetString(ctx, servo.GSCCCDLevel); err != nil {
		testing.ContextLog(ctx, "Failed to get gsc_ccd_level: ", err)
	} else if val != servo.Open {
		testing.ContextLogf(ctx, "CCD is not open, got %q. Attempting to unlock", val)
		if err := h.Servo.SetString(ctx, servo.CR50Testlab, servo.Open); err != nil {
			testing.ContextLog(ctx, "Failed to unlock CCD: ", err)
		}
	}

	testing.ContextLog(ctx, "Resetting DUT")
	if err := h.Servo.SetPowerState(ctx, servo.PowerStateReset); err != nil {
		testing.ContextLog(ctx, "Failed to reset DUT: ", err)
	}
	if err := h.DisconnectDUT(ctx); err != nil {
		testing.ContextLog(ctx, "Error closing connections to DUT: ", err)
	}
	h.CloseRPCConnection(ctx)
	return h.WaitConnect(ctx)
}

// DisallowServices prevents RequireRPCClient from being used for the lifetime of this Helper.
func (h *Helper) DisallowServices() {
	h.disallowServices = true
}

// RequireRPCClient creates a client connection to the DUT's gRPC server, unless a connection already exists.
func (h *Helper) RequireRPCClient(ctx context.Context) error {
	if h.disallowServices {
		return errors.New("RPC services disabled by fixture")
	}
	if h.RPCClient != nil {
		return nil
	}
	// rpcHint comes from testing.State, so it needs to be manually set in advance.
	if h.rpcHint == nil {
		return errors.New("cannot create RPC client connection without rpcHint")
	}
	testing.ContextLog(ctx, "Opening RPCClient connection")
	var cl *rpc.Client
	const rpcConnectTimeout = 5 * time.Minute
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		if !h.DUT.Connected(ctx) {
			if err := h.DUT.Connect(ctx); err != nil {
				return err
			}
		}
		var err error
		cl, err = rpc.Dial(ctx, h.DUT, h.rpcHint)
		return err
	}, &testing.PollOptions{Timeout: rpcConnectTimeout}); err != nil {
		return errors.Wrap(err, "dialing RPC connection")
	}
	h.RPCClient = cl
	return nil
}

// RequireRPCUtils creates a firmware.UtilsServiceClient, unless one already exists.
func (h *Helper) RequireRPCUtils(ctx context.Context) error {
	if h.RPCUtils != nil {
		return nil
	}
	if err := h.RequireRPCClient(ctx); err != nil {
		return errors.Wrap(err, "requiring RPC client")
	}
	h.RPCUtils = fwpb.NewUtilsServiceClient(h.RPCClient.Conn)
	return nil
}

// RequireBiosServiceClient creates a firmware.BiosServiceClient, unless one already exists.
// You must add `SoftwareDeps: []string{"flashrom"},` to your `testing.Test` to use this.
func (h *Helper) RequireBiosServiceClient(ctx context.Context) error {
	if h.BiosServiceClient != nil {
		return nil
	}
	if err := h.RequireRPCClient(ctx); err != nil {
		return errors.Wrap(err, "requiring RPC client")
	}
	h.BiosServiceClient = fwpb.NewBiosServiceClient(h.RPCClient.Conn)
	return nil
}

// CloseRPCConnection shuts down the RPC client (if present), and removes any RPC clients that the Helper was tracking.
func (h *Helper) CloseRPCConnection(ctx context.Context) error {
	defer func() {
		h.RPCClient, h.RPCUtils, h.BiosServiceClient = nil, nil, nil
	}()
	if h.RPCClient != nil {
		testing.ContextLog(ctx, "Closing RPCClient connection")
		if err := h.RPCClient.Close(ctx); err != nil {
			return errors.Wrap(err, "closing rpc client")
		}
	}
	return nil
}

// DisconnectDUT shuts down all connections to the DUT. Call this after you have powered down the DUT.
func (h *Helper) DisconnectDUT(ctx context.Context) error {
	rpcerr := h.CloseRPCConnection(ctx)
	// Disconnect the dut even if the rpc connection failed to close.
	if h.DUT != nil {
		duterr := h.DUT.Disconnect(ctx)
		if duterr != nil {
			return duterr
		}
	}
	return rpcerr
}

// RequirePlatform fetches the DUT's board and model and caches them, unless they have already been cached.
func (h *Helper) RequirePlatform(ctx context.Context) error {
	if h.Board == "" {
		board, err := h.Reporter.Board(ctx)
		if err != nil {
			return errors.Wrap(err, "getting DUT board")
		}
		h.Board = strings.ToLower(board)
	}
	if h.Model == "" {
		model, err := h.Reporter.Model(ctx)
		// Ignore error, as not all boards have a model.
		if err == nil {
			h.Model = strings.ToLower(model)
		} else {
			testing.ContextLogf(ctx, "Failed to get DUT model for board %s: %+v", h.Board, err)
		}
	}
	return nil
}

// OverridePlatform sets board and model if the passed in params are not blank.
func (h *Helper) OverridePlatform(ctx context.Context, board, model string) {
	if board != "" {
		h.Board = strings.ToLower(board)
	}
	if model != "" {
		h.Model = strings.ToLower(model)
	}
}

// RequireConfig creates a firmware.Config, unless one already exists.
// This requires your test to have `Data: []string{firmware.ConfigFile},` in its `testing.Test` block.
func (h *Helper) RequireConfig(ctx context.Context) error {
	if h.Config != nil {
		return nil
	}
	if err := h.RequirePlatform(ctx); err != nil {
		return errors.Wrap(err, "requiring DUT platform")
	}
	// cfgFilepath comes from testing.State, so it needs to be passed during NewHelper.
	if h.cfgFilepath == "" {
		return errors.New("cannot create firmware Config with a null Helper.cfgFilepath")
	}
	cfg, err := NewConfig(h.cfgFilepath, h.Board, h.Model)
	if err != nil {
		return errors.Wrapf(err, "during NewConfig with board=%s, model=%s", h.Board, h.Model)
	}
	h.Config = cfg
	return nil
}

// RequireServo creates a servo.Servo, unless one already exists.
func (h *Helper) RequireServo(ctx context.Context) error {
	if h.Servo != nil {
		return nil
	}
	pxy, err := servo.NewProxy(ctx, h.servoHostPort, h.keyFile, h.keyDir)
	if err != nil {
		return errors.Wrap(err, "connecting to servo")
	}
	h.ServoProxy = pxy
	h.Servo = pxy.Servo()
	return nil
}

// CloseServo closes the connection to the servo, use RequireServo to open it again.
func (h *Helper) CloseServo(ctx context.Context) error {
	defer func() {
		h.ServoProxy = nil
		h.Servo = nil
		h.RPM = nil
	}()
	var err error
	if h.RPM != nil {
		if err = h.RPM.Close(ctx); err != nil {
			err = errors.Wrap(err, "failed to close rpm client")
		}
	}
	if h.ServoProxy != nil {
		h.ServoProxy.Close(ctx)
	}
	return err
}

const (
	// dutLocalRunner, dutLocalBundleDir, and dutLocalDataDir are paths on the DUT containing Tast host files.
	dutLocalRunner    = "/usr/local/bin/local_test_runner"
	dutLocalBundleDir = "/usr/local/libexec/tast/"
	dutLocalDataDir   = "/usr/local/share/tast/"

	// tmpLocalRunner, tmpLocalBundleDir, and tmpLocalDataDir are relative paths, within a tempdir on the server, to copies of Tast host files.
	tmpLocalRunner    = "local-runner"
	tmpLocalBundleDir = "local-bundle-dir/"
	tmpLocalDataDir   = "local-data-dir/"
)

// DoesServerHaveTastHostFiles determines whether the test server has a copy of Tast host files.
func (h *Helper) DoesServerHaveTastHostFiles() bool {
	return h.hostFilesTmpDir != ""
}

// CopyTastFilesFromDUT retrieves Tast host files from the DUT and stores them locally for later use.
// This allows the test server to re-push Tast files to the DUT if a different OS image is booted mid-test.
func (h *Helper) CopyTastFilesFromDUT(ctx context.Context) error {
	if h.DoesServerHaveTastHostFiles() {
		return errors.New("cannot copy Tast files from DUT twice")
	}

	// Create temp dir to hold copied Tast files.
	tmpDir, err := ioutil.TempDir("", "tast-host-files-copy")
	if err != nil {
		return err
	}
	h.hostFilesTmpDir = tmpDir

	// Copy files from DUT onto test server.
	testing.ContextLogf(ctx, "Copying Tast host files to test server at %s", tmpDir)
	for dutSrc, serverDst := range map[string]string{
		dutLocalRunner:    filepath.Join(tmpDir, tmpLocalRunner),
		dutLocalBundleDir: filepath.Join(tmpDir, tmpLocalBundleDir),
		dutLocalDataDir:   filepath.Join(tmpDir, tmpLocalDataDir),
	} {
		// Only copy the file if it exists.
		if err = h.DUT.Conn().CommandContext(ctx, "test", "-x", dutSrc).Run(); err == nil {
			if err = linuxssh.GetFile(ctx, h.DUT.Conn(), dutSrc, serverDst, linuxssh.PreserveSymlinks); err != nil {
				return errors.Wrapf(err, "copying local Tast file %s from DUT", dutSrc)
			}
		} else if _, ok := err.(*gossh.ExitError); !ok {
			return errors.Wrapf(err, "checking for existence of %s", dutSrc)
		}
	}
	return nil
}

// SyncTastFilesToDUT copies the test server's copy of Tast host files back onto the DUT. This is only necessary if you want to use gRPC services.
// TODO(gredelston): When Autotest SSP tarballs contain local Tast test bundles, refactor this code
// so that it pushes Tast files to the DUT via the same means as the upstream Tast framework.
// As of the time of this writing, that is not possible; see http://g/tast-owners/sBhC1w-ET8g.
func (h *Helper) SyncTastFilesToDUT(ctx context.Context) error {
	if !h.DoesServerHaveTastHostFiles() {
		return errors.New("must copy Tast files from DUT before syncing back onto DUT")
	}
	fileMap := map[string]string{
		filepath.Join(h.hostFilesTmpDir, tmpLocalRunner):    dutLocalRunner,
		filepath.Join(h.hostFilesTmpDir, tmpLocalBundleDir): dutLocalBundleDir,
		filepath.Join(h.hostFilesTmpDir, tmpLocalDataDir):   dutLocalDataDir,
	}
	for key := range fileMap {
		if _, err := os.Stat(key); os.IsNotExist(err) {
			delete(fileMap, key)
		}
	}

	testing.ContextLog(ctx, "Syncing Tast files from test server onto DUT: ", fileMap)
	if _, err := linuxssh.PutFiles(ctx, h.DUT.Conn(), fileMap, linuxssh.DereferenceSymlinks); err != nil {
		return errors.Wrap(err, "failed syncing Tast files from test server onto DUT")
	}
	return nil
}

// SetupUSBKey prepares the USB disk for a test. (Borrowed from Tauto's firmware_test.py)
// It checks the setup of USB disk and a valid ChromeOS test image inside.
// Downloads the test image if the image isn't the right version.
// Will break the DUT if it is currently booted off the USB drive in recovery mode.
func (h *Helper) SetupUSBKey(ctx context.Context, cloudStorage *testing.CloudStorage) (retErr error) {
	testing.ContextLog(ctx, "Validating image usbkey on servo")
	// Power cycling the USB key helps to make it visible to the host.
	if err := h.Servo.SetUSBMuxState(ctx, servo.USBMuxOff); err != nil {
		return errors.Wrap(err, "failed to power off usbkey")
	}
	// This call is super slow.
	usbdev, err := h.Servo.GetStringTimeout(ctx, servo.ImageUSBKeyDev, time.Second*90)
	if err != nil {
		return errors.Wrap(err, "servo call image_usbkey_dev failed")
	}
	if usbdev == "" {
		return errors.New("no USB key detected")
	}
	var fdiskOutput []byte
	// Verify that the device really exists on the servo host.
	err = testing.Poll(ctx, func(ctx context.Context) error {
		fdiskOutput, err = h.ServoProxy.OutputCommand(ctx, true, "fdisk", "-l", usbdev)
		return err
	}, &testing.PollOptions{
		Timeout:  10 * time.Second,
		Interval: 1 * time.Second,
	})
	if err != nil {
		return errors.Wrapf(err, "validate usb key at %q", usbdev)
	}
	testing.ContextLogf(ctx, "Output from fdisk -l %q: %s", usbdev, fdiskOutput)
	testing.ContextLog(ctx, "Checking ChromeOS image name on usbkey")
	mountPath := fmt.Sprintf("/media/servo_usb/%d", h.ServoProxy.GetPort())
	// Unmount whatever might be mounted.
	h.ServoProxy.RunCommandQuiet(ctx, true, "umount", "-q", mountPath)

	// ChromeOS root fs is in /dev/sdx3.
	mountSrc := usbdev + "3"
	if err = h.ServoProxy.RunCommand(ctx, true, "mkdir", "-p", mountPath); err != nil {
		return errors.Wrapf(err, "mkdir failed at %q", mountPath)
	}
	var lsb map[string]string
	// Failures here are a bad USB image, so don't fail, just write the new image.
	err = func() error {
		if err = h.ServoProxy.RunCommand(ctx, true, "mount", "-o", "ro", mountSrc, mountPath); err != nil {
			if cloudStorage == nil {
				return errors.Errorf("Mount of %q failed at %q", mountSrc, mountPath)
			}
			testing.ContextLogf(ctx, "Mount of %q failed at %q", mountSrc, mountPath)
		}
		defer h.ServoProxy.RunCommand(ctx, true, "umount", mountPath)
		output, err := h.ServoProxy.OutputCommand(ctx, true, "cat", fmt.Sprintf("%s/etc/lsb-release", mountPath))
		if err != nil {
			if cloudStorage == nil {
				return errors.Errorf("Failed to read lsb-release")
			}
			testing.ContextLog(ctx, "Failed to read lsb-release")
		}
		lsb, err = lsbrelease.Parse(bytes.NewReader(output))
		if err != nil {
			if cloudStorage == nil {
				return errors.Errorf("Failed to parse lsb-release")
			}
			testing.ContextLog(ctx, "Failed to parse lsb-release")
		}
		return nil
	}()
	if err != nil {
		return errors.Wrap(err, "bad USB image, and requested no USB image download")
	}
	releaseBuilderPath := lsb[lsbrelease.BuilderPath]
	dutBuilderPath, err := h.Reporter.BuilderPath(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to get DUT builder path")
	}
	if strings.Contains(dutBuilderPath, "-postsubmit") {
		testing.ContextLogf(ctx, "Current build on DUT (%s) is not a release image, using %s from USB stick", dutBuilderPath, releaseBuilderPath)
		return nil
	}

	if !strings.Contains(lsb[lsbrelease.ReleaseTrack], "test") {
		if cloudStorage == nil {
			return errors.Errorf("The image on usbkey is not a test image")
		}
		testing.ContextLog(ctx, "The image on usbkey is not a test image")
		releaseBuilderPath = ""
	}

	if releaseBuilderPath == dutBuilderPath {
		return nil
	}

	if cloudStorage == nil {
		testing.ContextLogf(ctx, "User requested no USB image download, using %s even though it differs from DUT %s", releaseBuilderPath, dutBuilderPath)
		return nil
	}
	testing.ContextLogf(ctx, "Current build on USB (%s) differs from DUT (%s), proceed with download", releaseBuilderPath, dutBuilderPath)

	// Copying the behavior from src/third_party/hdctools/servo/drv/usb_downloader.py.
	// Write the chromiumos_test_image.bin straight over /dev/sdx (usbdev).
	// That code expects a url to a unpacked chromiumos_test_image.bin, but cloudStorage.Open doesn't handle devserver artifacts like `test_image`,
	// so we need to manually untar the file and write it over the usb device.

	// TODO if needed, recovery images are at .../recovery_image.tar.xz.
	testImageURL := "build-artifact:///chromiumos_test_image.tar.xz"
	// TODO(b/217635723): Revisit later when we have a solution for accessing dev servers on non-DUT machines.
	dataURL, err := cloudStorage.Stage(ctx, testImageURL)
	if err != nil {
		return errors.Wrapf(err, "failed to download test image %s", dutBuilderPath)
	}
	if dataURL.Scheme != "http" && dataURL.Scheme != "https" {
		return errors.Errorf("CloudStorage url is not http(s): %q", dataURL)
	}

	testing.ContextLog(ctx, "Flashing test OS image to USB")
	// Make sure the device is synced whether or not the command succeeds.
	defer func(ctx context.Context) {
		if err = h.ServoProxy.RunCommand(ctx, true, "sync", usbdev); err != nil {
			if retErr == nil {
				retErr = errors.Wrap(err, "sync failed")
			} else {
				testing.ContextLogf(ctx, "Sync failed: %s", err)
			}
		}
		if err = h.ServoProxy.RunCommand(ctx, true, "blockdev", "--rereadpt", usbdev); err != nil && retErr == nil {
			if retErr == nil {
				retErr = errors.Wrap(err, "blockdev failed")
			} else {
				testing.ContextLogf(ctx, "blockdev failed: %s", err)
			}
		}
	}(ctx)

	// Reduce the context deadline to let the deferred calls succeed.
	ctx, cancel := ctxutil.Shorten(ctx, 30*time.Second)
	defer cancel()
	// If it did have tast files, it won't shortly.
	h.dutUsbHasTastFiles = false
	// On my computer with a servo v4, this takes 7 minutes.
	if err = h.ServoProxy.RunCommand(ctx, true, "sh", "-c", fmt.Sprintf("wget -nv -O - %s | tar -JxOf - | dd of=%s bs=1M iflag=fullblock conv=nocreat,fsync", shutil.Escape(dataURL.String()), shutil.Escape(usbdev))); err != nil {
		return errors.Wrapf(err, "failed to flash os image %q to USB %q", testImageURL, usbdev)
	}
	testing.ContextLogf(ctx, "Successfully flashed %q from %q", usbdev, testImageURL)
	return nil
}

// WaitForPowerStates polls for DUT to get to a specific powerstate
func (h *Helper) WaitForPowerStates(ctx context.Context, interval, timeout time.Duration, powerStates ...string) error {
	// Try reading the power state from the EC.
	err := testing.Poll(ctx, func(c context.Context) error {
		currPowerState, err := h.Servo.GetECSystemPowerState(ctx)
		if err != nil {
			return errors.Wrap(err, "failed to check powerstate")
		}
		if !comparePowerStates(currPowerState, powerStates...) {
			return errors.Errorf("Power state = %s", currPowerState)
		}
		return nil
	}, &testing.PollOptions{Timeout: timeout, Interval: interval})
	if err != nil {
		return errors.Errorf("failed to get one of %v power state: %s", powerStates, err)
	}
	return nil
}

func comparePowerStates(currState string, expectedStates ...string) bool {
	for _, state := range expectedStates {
		if currState == state {
			return true
		}
	}
	return false
}

func (h *Helper) waitDutS0(ctx context.Context) error {
	const reconnectRetryDelay = time.Second
	testing.ContextLog(ctx, "Waiting DUT to reach S0")
	if err := h.WaitForPowerStates(ctx, reconnectRetryDelay, 1*time.Minute, "S0"); err != nil {
		return errors.Wrap(err, "wait for S0")
	}
	testing.ContextLog(ctx, "Sleeping 20s for boot to finish")
	if err := testing.Sleep(ctx, 20*time.Second); err != nil {
		return errors.Wrap(err, "sleep 20s")
	}
	return nil
}

// wcOptsContain determines whether a slice of WaitConnectOption contains a specific Option.
func wcOptsContain(opts []WaitConnectOption, contained WaitConnectOption) bool {
	for _, v := range opts {
		if v == contained {
			return true
		}
	}
	return false
}

// WaitConnect is similar to DUT.WaitConnect, except that it works with RO EC firmware.
// Pass a context with a deadline if you don't want to wait forever.
// If --var noSSH=true is set, this degrades to waiting for S0 + a sleep.
func (h *Helper) WaitConnect(ctx context.Context, opts ...WaitConnectOption) error {
	const reconnectRetryDelay = time.Second

	if err := h.RequireServo(ctx); err != nil {
		return errors.Wrap(err, "failed to connect to servo")
	}
	if h.DUT == nil {
		return h.waitDutS0(ctx)
	}
	testing.ContextLogf(ctx, "Waiting for %s to connect", h.DUT.HostName())
	for {
		// SetDUTPDDataRole would fail when DUT is still in the process
		// of waking up from hibernation.
		if !wcOptsContain(opts, FromHibernation) {
			if err := h.Servo.SetDUTPDDataRole(ctx, servo.DFP); err != nil {
				testing.ContextLogf(ctx, "Failed to set pd data role to DFP: %s", err)
			}
		}
		err := h.DUT.Connect(ctx)
		if err == nil {
			return nil
		}

		select {
		case <-time.After(reconnectRetryDelay):
			break
		case <-ctx.Done():
			if err.Error() == ctx.Err().Error() {
				return err
			}
			return errors.Wrapf(err, "context error = %v", ctx.Err())
		}
	}
}

// RequireRPM creates the RPM client in h.RPM.
func (h *Helper) RequireRPM(ctx context.Context) error {
	if h.RPM != nil {
		return nil
	}
	if err := h.RequireServo(ctx); err != nil {
		return err
	}
	var err error
	if h.ServoProxy.Proxied() {
		h.RPM, err = rpm.NewLabRPM(ctx, h.ServoProxy, h.dutHostname, h.powerunitHostname, h.powerunitOutlet, h.hydraHostname)
	} else {
		h.RPM, err = rpm.NewLabRPM(ctx, nil, h.dutHostname, h.powerunitHostname, h.powerunitOutlet, h.hydraHostname)
	}
	if err != nil {
		return errors.Wrap(err, "new rpm client")
	}
	return nil
}

// SetDUTPower turns the DUT's power on or off. Uses servo v4 pd role if possible, and falls back to RPM.
// To use RPM the command line vars `powerunitHostname` and `powerunitOutlet` must be set.
// `dutHostname` can be used to override the DUT's hostname, if ssh and rpm have different names.
// For plugs attached to hyrda, also set var `hydraHostname`.
func (h *Helper) SetDUTPower(ctx context.Context, powerOn bool) error {
	// Try servo SetPDRole (servo v4 type C). The servo is slightly evil though, and will report that it has this control even for Type-A.
	connectionType := ""
	hasControl, err := h.Servo.HasControl(ctx, string(servo.PDRole))
	if err != nil {
		return errors.Wrap(err, "checking for control")
	}
	if hasControl {
		connectionType, err = h.Servo.GetString(ctx, "root.dut_connection_type")
		if err != nil {
			return errors.Wrap(err, "getting connection type")
		}
	}
	if connectionType == "type-c" {
		role := servo.PDRoleSnk
		if powerOn {
			role = servo.PDRoleSrc
		}
		if err := h.Servo.SetPDRole(ctx, role); err != nil {
			return errors.Wrap(err, "set pd role")
		}
		testing.ContextLogf(ctx, "SetPDRole: %q", role)
		return nil
	}
	// Try rpm client
	if h.powerunitHostname != "" {
		if err := h.RequireRPM(ctx); err != nil {
			return err
		}
		powerState := rpm.Off
		if powerOn {
			powerState = rpm.On
		}
		if ok, err := h.RPM.SetPower(ctx, powerState); err != nil {
			return errors.Wrap(err, "set power via rpm")
		} else if !ok {
			return errors.Errorf("rpm client did not set power state to %s", powerState)
		}
		return nil
	}
	return errors.New("servo does not support pd role and no rpm vars provided")
}

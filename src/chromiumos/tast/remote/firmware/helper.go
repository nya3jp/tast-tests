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

	"chromiumos/tast/dut"
	"chromiumos/tast/errors"
	"chromiumos/tast/lsbrelease"
	"chromiumos/tast/remote/firmware/reporters"
	"chromiumos/tast/remote/servo"
	"chromiumos/tast/rpc"
	fwpb "chromiumos/tast/services/cros/firmware"
	"chromiumos/tast/ssh/linuxssh"
	"chromiumos/tast/testing"
)

// Helper tracks several firmware-related objects. The recommended way to initialize the helper is to use firmware.Pre:
//
// import (
//	...
//	"chromiumos/tast/remote/firmware/pre"
// )
//
// func init() {
//	testing.AddTest(&testing.Test{
//		...
//              Data:         []string{firmware.ConfigFile},
//              Pre:          pre.NormalMode(),
//              ServiceDeps:  []string{"tast.cros.firmware.BiosService", "tast.cros.firmware.UtilsService"},
//              SoftwareDeps: []string{"crossystem", "flashrom"},
//              Vars:         []string{"servo"},
//	})
// }
//
// func MyTest(ctx context.Context, s *testing.State) {
// 	h := s.PreValue().(*pre.Value).Helper
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

	// doesDUTImageHaveTastFiles and doesRecHaveTastFiles track whether the DUT's on-board image
	// and the USB recovery image are known to have up-to-date Tast host files.
	doesDUTImageHaveTastFiles bool
	doesRecHaveTastFiles      bool

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

	// rpcHint is needed in order to create an RPC client connection.
	rpcHint *testing.RPCHint

	// RPCUtils allows the Helper to call the firmware utils RPC service.
	RPCUtils fwpb.UtilsServiceClient

	// Servo allows us to send commands to a servo device.
	Servo *servo.Servo

	// servoHostPort is the address and port of the machine acting as the servo host, normally provided via the "servo" command-line variable.
	servoHostPort string

	// ServoProxy wraps the Servo object, and communicates with the servod instance.
	ServoProxy *servo.Proxy
}

// NewHelper creates a new Helper object with info from testing.State.
// For tests that do not use a certain Helper aspect (e.g. RPC or Servo), it is OK to pass null-values (nil or "").
func NewHelper(d *dut.DUT, rpcHint *testing.RPCHint, cfgFilepath, servoHostPort string) *Helper {
	return &Helper{
		cfgFilepath:   cfgFilepath,
		DUT:           d,
		Reporter:      reporters.New(d),
		rpcHint:       rpcHint,
		servoHostPort: servoHostPort,
	}
}

// Close shuts down any firmware objects associated with the Helper.
// Generally, tests should defer Close() immediately after initializing a Helper.
func (h *Helper) Close(ctx context.Context) error {
	var firstErr error
	if h.ServoProxy != nil {
		h.ServoProxy.Close(ctx)
	}
	if h.hostFilesTmpDir != "" {
		if err := os.RemoveAll(h.hostFilesTmpDir); err != nil {
			firstErr = errors.Wrap(err, "removing server's copy of Tast host files")
		}
		h.hostFilesTmpDir = ""
	}
	if err := h.CloseRPCConnection(ctx); err != nil && firstErr == nil {
		isIgnorable := false
		for rootErr := err; rootErr != nil && !isIgnorable; rootErr = errors.Unwrap(rootErr) {
			// The gRPC Canceled error just means the connection is already closed.
			if st, ok := status.FromError(rootErr); ok && st.Code() == codes.Canceled {
				isIgnorable = true
			}
		}
		if !isIgnorable {
			firstErr = errors.Wrap(err, "closing rpc connection")
		}
	}
	return firstErr
}

// EnsureDUTBooted checks the power state, and attempts to boot the DUT if it is off.
func (h *Helper) EnsureDUTBooted(ctx context.Context) error {
	if h.DUT.Connected(ctx) {
		return nil
	}
	if err := h.RequireServo(ctx); err != nil {
		return nil
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
			// The machine is up, just wait for it to finish booting
			h.CloseRPCConnection(ctx)
			if err = h.DUT.WaitConnect(ctx); err == nil {
				return nil
			}
			// If WaitConnect didn't work, let it reset.
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
	return h.DUT.WaitConnect(ctx)
}

// RequireRPCClient creates a client connection to the DUT's gRPC server, unless a connection already exists.
func (h *Helper) RequireRPCClient(ctx context.Context) error {
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
		var err error
		cl, err = rpc.Dial(ctx, h.DUT, h.rpcHint, "cros")
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
	duterr := h.DUT.Disconnect(ctx)
	if duterr != nil {
		return duterr
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
		// Ignore error, as not all boards have a model
		if err == nil {
			h.Model = strings.ToLower(model)
		} else {
			testing.ContextLogf(ctx, "Failed to get DUT model for board %s: %+v", h.Board, err)
		}
	}
	return nil
}

// RequireConfig creates a firmware.Config, unless one already exists.
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
	pxy, err := servo.NewProxy(ctx, h.servoHostPort, h.DUT.KeyFile(), h.DUT.KeyDir())
	if err != nil {
		return errors.Wrap(err, "connecting to servo")
	}
	h.ServoProxy = pxy
	h.Servo = pxy.Servo()
	return nil
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
		// Only copy the file if it exists
		if err = h.DUT.Conn().Command("test", "-x", dutSrc).Run(ctx); err == nil {
			if err = linuxssh.GetFile(ctx, h.DUT.Conn(), dutSrc, serverDst, linuxssh.PreserveSymlinks); err != nil {
				return errors.Wrapf(err, "copying local Tast file %s from DUT", dutSrc)
			}
		} else if _, ok := err.(*gossh.ExitError); !ok {
			return errors.Wrapf(err, "checking for existence of %s: %T", dutSrc, err)
		}
	}
	return nil
}

// SyncTastFilesToDUT copies the test server's copy of Tast host files back onto the DUT via rsync.
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
func (h *Helper) SetupUSBKey(ctx context.Context, cloudStorage *testing.CloudStorage) error {
	//     self.stage_build_to_usbkey()
	testing.ContextLog(ctx, "Validating image usbkey on servo")
	// This call is super slow.
	usbdev, err := h.Servo.GetStringTimeout(ctx, servo.ImageUSBKeyDev, time.Second*90)
	if err != nil {
		return errors.Wrap(err, "servo call image_usbkey_dev failed")
	}
	if usbdev == "" {
		return errors.New("no USB key detected")
	}
	// Verify that the device really exists on the servo host
	if err = h.ServoProxy.RunCommand(ctx, true, "fdisk", "-l", usbdev); err != nil {
		return errors.Wrapf(err, "validate usb key at %q", usbdev)
	}

	testing.ContextLog(ctx, "Checking ChromeOS image name on usbkey")
	mountPath := fmt.Sprintf("/media/servo_usb/%d", h.ServoProxy.GetPort())
	// Unmount whatever might be mounted
	h.ServoProxy.RunCommand(ctx, true, "umount", mountPath)

	// ChromeOS root fs is in /dev/sdx3
	mountSrc := usbdev + "3"
	if err = h.ServoProxy.RunCommand(ctx, true, "mkdir", "-p", mountPath); err != nil {
		return errors.Wrapf(err, "mkdir failed at %q", mountPath)
	}
	var lsb map[string]string
	if err = func() error {
		if err = h.ServoProxy.RunCommand(ctx, true, "mount", "-o", "ro", mountSrc, mountPath); err != nil {
			return errors.Wrapf(err, "mount of %q failed at %q", mountSrc, mountPath)
		}
		defer h.ServoProxy.RunCommand(ctx, true, "umount", mountPath)
		output, err := h.ServoProxy.OutputCommand(ctx, true, "cat", fmt.Sprintf("%s/etc/lsb-release", mountPath))
		if err != nil {
			return errors.Wrap(err, "failed to read lsb-release")
		}
		lsb, err = lsbrelease.Parse(bytes.NewReader(output))
		if err != nil {
			return errors.Wrap(err, "failed to parse lsb-release")
		}
		return nil
	}(); err != nil {
		return err
	}
	releaseBuilderPath := lsb[lsbrelease.BuilderPath]
	if !strings.Contains(lsb[lsbrelease.ReleaseTrack], "test") {
		testing.ContextLog(ctx, "The image on usbkey is not a test image")
		releaseBuilderPath = ""
	}

	dutBuilderPath, err := h.Reporter.BuilderPath(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to get DUT builder path")
	}
	if releaseBuilderPath == dutBuilderPath {
		return nil
	}
	testing.ContextLogf(ctx, "Current build on USB (%s) differs from DUT (%s), proceed with download", releaseBuilderPath, dutBuilderPath)
	// TODO if needed, recovery images are at .../recovery_image.tar.xz
	// TODO, change to Config.BuildArtifactsURL if that becomes accessible
	testImageURL := "gs://chromeos-image-archive/" + dutBuilderPath + "/chromiumos_test_image.tar.xz"
	reader, err := cloudStorage.Open(ctx, testImageURL)
	if err != nil {
		return errors.Wrapf(err, "failed to download test image %s", dutBuilderPath)
	}
	defer reader.Close()
	tempname, err := h.ServoProxy.OutputCommand(ctx, false, "tempfile", "-m", "0644")
	if err != nil {
		return errors.Wrap(err, "failed to create tmp file on servo host")
	}
	defer h.ServoProxy.RunCommand(ctx, false, "rm", string(tempname))
	// Copy to the servo host and untar
	if err = h.ServoProxy.InputCommand(ctx, false, reader, "tar", "-Jxvf", "-",
		"-C", filepath.Dir(string(tempname)),
		fmt.Sprintf("--transform=s/chromiumos_test_image.bin/%s/", filepath.Base(string(tempname))),
		"chromiumos_test_image.bin"); err != nil {
		return errors.Wrap(err, "failed to copy os image to servo host")
	}
	testing.ContextLog(ctx, "Flashing test OS image to USB")
	if err = h.ServoProxy.RunCommand(ctx, false, "cros", "flash", "usb://"+usbdev, string(tempname)); err != nil {
		return errors.Wrap(err, "failed to flash os image to usb")
	}

	return nil
}

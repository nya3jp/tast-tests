// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package firmware

import (
	"archive/zip"
	"context"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"chromiumos/tast/dut"
	"chromiumos/tast/errors"
	"chromiumos/tast/remote/firmware/reporters"
	"chromiumos/tast/remote/servo"
	"chromiumos/tast/rpc"
	fwpb "chromiumos/tast/services/cros/firmware"
	"chromiumos/tast/ssh/linuxssh"
	"chromiumos/tast/testing"
)

// Helper tracks several firmware-related objects.
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

	// DUT is used for communicating with the device under test.
	DUT *dut.DUT

	// dutTastFilesTmpDir is a temporary directory on the test server holding a copy of the DUT's Tast files.
	dutTastFilesTmpDir string

	// dutTastFilesInstallTime is the time when Tast files were installed onto the DUT, in seconds since the epoch.
	dutTastFilesInstallTime int

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
	if h.dutTastFilesTmpDir != "" {
		if err := os.RemoveAll(h.dutTastFilesTmpDir); err != nil {
			firstErr = errors.Wrap(err, "removing remote copy of local Tast files")
		}
		h.dutTastFilesTmpDir = ""
	}
	if err := h.CloseRPCConnection(ctx); err != nil && firstErr == nil {
		firstErr = errors.Wrap(err, "closing rpc connection")
	}
	return firstErr
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
		if err := h.RPCClient.Close(ctx); err != nil {
			return errors.Wrap(err, "closing rpc client")
		}
	}
	return nil
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
	if h.servoHostPort == "" {
		return errors.New(`got empty servoHostPort; want s.RequiredVar("servo")`)
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
	// dutLocalRunner, dutLocalBundleDir, and dutLocalDataDir are paths to local Tast files and directories on the DUT.
	dutLocalRunner    = "/usr/local/bin/local_test_runner"
	dutLocalBundleDir = "/usr/local/libexec/tast/"
	dutLocalDataDir   = "/usr/local/share/tast/"

	// tmpLocalRunner, tmpLocalBundleDir, and tmpLocalDataDir are relative paths, within a tempdir on the server, to copies of the DUT's Tast files.
	tmpLocalRunner    = "local-runner"
	tmpLocalBundleDir = "local-bundle-dir"
	tmpLocalDataDir   = "local-data-dir"

	// tastDataZipName and tastBundleZipName are basenames for zipfiles containing the DUT's tast directories.
	tastDataZipName   = "tast-bundles.zip"
	tastBundleZipName = "tast-data.zip"
)

// AreDUTTastFilesOnServer determines whether the test server has a copy of the DUT's local Tast files.
func (h *Helper) AreDUTTastFilesOnServer() bool {
	return h.dutTastFilesTmpDir != ""
}

// CopyTastFilesFromDUT retrieves Tast files from the DUT and stores them locally for later use.
// This allows the remote host to re-push Tast files to the DUT if a different OS is booted mid-test.
func (h *Helper) CopyTastFilesFromDUT(ctx context.Context) error {
	if h.AreDUTTastFilesOnServer() {
		return errors.New("cannot copy Tast files from DUT twice")
	}

	// Create temp dir to hold copied Tast files.
	tmpDir, err := ioutil.TempDir("", "dut-tast-files-copy")
	if err != nil {
		return err
	}
	h.dutTastFilesTmpDir = tmpDir

	// Copy files from DUT onto test server.
	testing.ContextLogf(ctx, "Copying DUT Tast files to test server at %s", tmpDir)
	for dutSrc, serverDst := range map[string]string{
		dutLocalRunner:    filepath.Join(tmpDir, tmpLocalRunner),
		dutLocalBundleDir: filepath.Join(tmpDir, tmpLocalBundleDir),
		dutLocalDataDir:   filepath.Join(tmpDir, tmpLocalDataDir),
	} {
		if err = linuxssh.GetFile(ctx, h.DUT.Conn(), dutSrc, serverDst); err != nil {
			return errors.Wrap(err, "copying local Tast files from DUT")
		}
	}

	// Zip local files into tmpDir.
	for srcBasename, zipBasename := range map[string]string{
		tmpLocalDataDir:   tastDataZipName,
		tmpLocalBundleDir: tastBundleZipName,
	} {
		srcDir := filepath.Join(tmpDir, srcBasename)
		dstZip := filepath.Join(tmpDir, zipBasename)
		if err := zipDir(srcDir, dstZip); err != nil {
			return errors.Wrapf(err, "zipping %q into %q", srcDir, dstZip)
		}
	}

	// Record the last modified time of the DUT's Tast files.
	out, err := h.Reporter.CommandOutputLines(ctx, "stat", "-c", "%Y", dutLocalRunner, dutLocalBundleDir, dutLocalDataDir)
	if err != nil {
		return err
	}
	for _, line := range out {
		mtime, err := strconv.Atoi(strings.TrimSpace(line))
		if err != nil {
			return errors.Wrapf(err, "converting `stat` output line to int: %q", line)
		}
		if h.dutTastFilesInstallTime == 0 || mtime < h.dutTastFilesInstallTime {
			h.dutTastFilesInstallTime = mtime
		}
	}
	return nil
}

// CopyTastFilesToDUT copies the test server's copy of local Tast files back onto the DUT.
func (h *Helper) CopyTastFilesToDUT(ctx context.Context) error {
	if !h.AreDUTTastFilesOnServer() {
		return errors.New("must copy Tast files from DUT before copying back onto DUT")
	}
	testing.ContextLog(ctx, "Copying DUT Tast files back onto DUT")
	srcDstMap := map[string]string{
		filepath.Join(h.dutTastFilesTmpDir, tmpLocalRunner):    dutLocalRunner,
		filepath.Join(h.dutTastFilesTmpDir, tastBundleZipName): filepath.Join("/tmp", tastBundleZipName),
		filepath.Join(h.dutTastFilesTmpDir, tastDataZipName):   filepath.Join("/tmp", tastDataZipName),
	}
	if _, err := linuxssh.PutFiles(ctx, h.DUT.Conn(), srcDstMap, linuxssh.PreserveSymlinks); err != nil {
		return err
	}

	// Unzip Tast bundles and data files to their expected locations on the DUT.
	for zipPath, dstPath := range map[string]string{
		filepath.Join("/tmp", tastBundleZipName): dutLocalBundleDir,
		filepath.Join("/tmp", tastDataZipName):   dutLocalDataDir,
	} {
		if err := h.DUT.Command("rm", "-rf", dstPath).Run(ctx); err != nil {
			return errors.Wrapf(err, "removing pre-existing files on DUT at %s", dstPath)
		}
		if err := h.DUT.Command("unzip", zipPath, "-d", dstPath).Run(ctx); err != nil {
			return errors.Wrapf(err, "unzipping zipfile %s on DUT to %s", zipPath, dstPath)
		}
		if err := h.DUT.Command("rm", "-rf", zipPath).Run(ctx); err != nil {
			return errors.Wrapf(err, "cleaning up zipfile on DUT at %s", zipPath)
		}
	}

	// Set file permissions on the DUT.
	for _, fp := range []string{
		dutLocalRunner,
		filepath.Join(dutLocalBundleDir, "bin_pushed", "local_test_runner"),
		filepath.Join(dutLocalBundleDir, "bundles", "local", "cros"),
		filepath.Join(dutLocalBundleDir, "bundles", "local_pushed", "cros"),
	} {
		if err := h.DUT.Command("chmod", "755", fp).Run(ctx); err != nil {
			return err
		}
	}

	return nil
}

// DoesDUTNeedTastFiles determines whether the DUT's Tast files are either missing or outdated.
func (h *Helper) DoesDUTNeedTastFiles(ctx context.Context) (bool, error) {
	// If the files are not present on the DUT, then installation is needed.
	if ok, err := h.Reporter.DoAllPathsExist(ctx, []string{dutLocalRunner, dutLocalBundleDir, dutLocalDataDir}); err != nil {
		return false, errors.Wrap(err, "checking whether Tast files are present on DUT")
	} else if !ok {
		return true, nil
	}

	// If the files are present but older than the previously recorded install time, then installation is needed.
	out, err := h.Reporter.CommandOutputLines(ctx, "stat", "-c", "%Y", dutLocalRunner, dutLocalBundleDir, dutLocalDataDir)
	if err != nil {
		return false, errors.Wrap(err, "checking last-modified-time of DUT's Tast files")
	}
	for _, line := range out {
		mtime, err := strconv.Atoi(strings.TrimSpace(line))
		if err != nil {
			return false, errors.Wrapf(err, "converting `stat` output line to int: %q", line)
		}
		if mtime < h.dutTastFilesInstallTime {
			return true, nil
		}
	}
	return false, nil
}

// zipDir compresses a directory's contents on the test server into a zipfile.
func zipDir(srcDir, zipPath string) error {
	zipFile, err := os.Create(zipPath)
	if err != nil {
		return errors.Wrapf(err, "creating zipfile at %s", zipPath)
	}
	defer zipFile.Close()

	w := zip.NewWriter(zipFile)
	defer w.Close()

	if err = filepath.Walk(srcDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return errors.Wrapf(err, "crawling path %s", path)
		}

		// No need to add directories directly to the zipfile.
		// filepath.Walk will crawl that directory.
		if info.IsDir() {
			return nil
		}

		// Open the file within srcDir so we can write it to the zipfile.
		file, err := os.Open(path)
		if err != nil {
			return errors.Wrapf(err, "opening path %s", path)
		}
		defer file.Close()

		// Add a new file to the zip file with the same name.
		// The argument to w.Create must be a relative path: it must not begin with '/'.
		relPath, err := filepath.Rel(srcDir, path)
		if err != nil {
			return errors.Wrapf(err, "file %s did not contain prefix %s", path, srcDir)
		}
		f, err := w.Create(relPath)
		if err != nil {
			return errors.Wrapf(err, "creating new file %s in zip writer", relPath)
		}

		// Copy the contents of the src file into the zip's new file.
		if _, err = io.Copy(f, file); err != nil {
			return errors.Wrapf(err, "copying file %s into zip file", path)
		}

		return nil
	}); err != nil {
		return errors.Wrap(err, "adding source files to zipfile")
	}
	return nil
}

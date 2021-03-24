// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package firmware

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"time"

	"chromiumos/tast/common/testexec"
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

	// doesNormalHaveTastFiles and doesRecHaveTastFiles track whether the DUT's normal image, and recovery image, each are known to have up-to-date Tast host files.
	doesNormalHaveTastFiles bool
	doesRecHaveTastFiles    bool

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
		if err = linuxssh.GetFile(ctx, h.DUT.Conn(), dutSrc, serverDst); err != nil {
			return errors.Wrapf(err, "copying local Tast file %s from DUT", dutSrc)
		}
	}
	return nil
}

// SyncTastFilesToDUT copies the test server's copy of Tast host files back onto the DUT via rsync.
func (h *Helper) SyncTastFilesToDUT(ctx context.Context) error {
	if !h.DoesServerHaveTastHostFiles() {
		return errors.New("must copy Tast files from DUT before syncing back onto DUT")
	}
	testing.ContextLog(ctx, "Syncing Tast files from test server onto DUT")
	dutHost := strings.Split(h.DUT.HostName(), ":")[0] // HostName == host::port

	// Ensure that SSH KeyFile has appropriate permissions
	if fi, err := os.Stat(h.DUT.KeyFile()); err != nil {
		return errors.Wrap(err, "getting file info for SSH key file")
	} else if fi.Mode() != 0600 {
		if err := os.Chmod(h.DUT.KeyFile(), 0600); err != nil {
			return errors.Wrap(err, "setting permission for SSH key file: ")
		}
	}

	for relSrc, dst := range map[string]string{
		tmpLocalRunner:    dutLocalRunner,
		tmpLocalBundleDir: dutLocalBundleDir,
		tmpLocalDataDir:   dutLocalDataDir,
	} {
		absSrc := filepath.Join(h.hostFilesTmpDir, relSrc)
		// Trailing slashes are meaningful for rsync, but filepath.Join trims trailing suffixes.
		if strings.HasSuffix(relSrc, "/") && !strings.HasSuffix(absSrc, "/") {
			absSrc += "/"
		}
		remoteDst := fmt.Sprintf("%s:%s", dutHost, dst)

		// Call rsync.
		// -a = archive mode. Includes recursion, maintaining links, file permissions/executability, modified times, owners, groups, device/special files.
		// --rsh = specify remote command. This allows us to use SSH with -i, to pass in the key file for authentication.
		if err := testexec.CommandContext(ctx, "rsync", "-a", "--rsh", fmt.Sprintf("ssh -i %s", h.DUT.KeyFile()), absSrc, remoteDst).Run(testexec.DumpLogOnError); err != nil {
			return errors.Wrapf(err, "syncing %s to %s", absSrc, remoteDst)
		}
	}

	// Set file permissions on the DUT.
	if err := h.DUT.Command("chmod", "755",
		dutLocalRunner,
		filepath.Join(dutLocalBundleDir, "bin_pushed", "local_test_runner"),
		filepath.Join(dutLocalBundleDir, "bundles", "local", "cros"),
		filepath.Join(dutLocalBundleDir, "bundles", "local_pushed", "cros"),
	).Run(ctx); err != nil {
		return errors.Wrap(err, "changing file permissions on DUT")
	}
	return nil
}

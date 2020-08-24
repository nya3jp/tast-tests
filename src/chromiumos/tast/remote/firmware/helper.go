// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package firmware

import (
	"context"
	"strings"
	"time"

	"chromiumos/tast/dut"
	"chromiumos/tast/errors"
	"chromiumos/tast/remote/firmware/reporters"
	"chromiumos/tast/remote/servo"
	"chromiumos/tast/rpc"
	fwpb "chromiumos/tast/services/cros/firmware"
	"chromiumos/tast/testing"
)

// Helper tracks several firmware-related objects.
type Helper struct {
	// Board contains the DUT's board, as reported by the Platform RPC.
	// Currently, this is based on /etc/lsb-release's CHROMEOS_RELEASE_BOARD.
	Board string

	// Config contains a variety of platform-specific attributes.
	Config *Config

	// configDataDir is the full path to the data directory containing fw-testing-configs JSON files.
	// Any tests requiring a Config should set configDataDir to s.DataPath(firmware.ConfigDir) during NewHelper.
	configDataDir string

	// DUT is used for communicating with the device under test.
	DUT *dut.DUT

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
func NewHelper(d *dut.DUT, rpcHint *testing.RPCHint, configDataDir, servoHostPort string) *Helper {
	return &Helper{
		configDataDir: configDataDir,
		DUT:           d,
		Reporter:      reporters.New(d),
		rpcHint:       rpcHint,
		servoHostPort: servoHostPort,
	}
}

// Close shuts down any firmware objects associated with the Helper.
// Generally, tests should defer Close() immediately after initializing a Helper.
func (h *Helper) Close(ctx context.Context) error {
	if h.ServoProxy != nil {
		h.ServoProxy.Close(ctx)
	}
	if err := h.CloseRPCConnection(ctx); err != nil {
		return errors.Wrap(err, "closing rpc connection")
	}
	return nil
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

// CloseRPCConnection shuts down the RPC client (if present), and removes any RPC clients that the Helper was tracking.
func (h *Helper) CloseRPCConnection(ctx context.Context) error {
	if h.RPCClient != nil {
		if err := h.RPCClient.Close(ctx); err != nil {
			h.RPCClient, h.RPCUtils = nil, nil
			return errors.Wrap(err, "closing rpc client")
		}
	}
	h.RPCClient, h.RPCUtils = nil, nil
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
		if err != nil {
			return errors.Wrap(err, "getting DUT model")
		}
		h.Model = strings.ToLower(model)
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
	// configDataDir comes from testing.State, so it needs to be passed during NewHelper.
	if h.configDataDir == "" {
		return errors.New("cannot create firmware Config with a null Helper.configDataDir")
	}
	cfg, err := NewConfig(h.configDataDir, h.Board, h.Model)
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

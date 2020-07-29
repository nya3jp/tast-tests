// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package firmware

import (
	"context"
	"strings"

	"github.com/golang/protobuf/ptypes/empty"

	"chromiumos/tast/dut"
	"chromiumos/tast/errors"
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

	// ConfigDataDir is the full path to the data directory containing fw-testing-configs JSON files.
	// Any tests using a Config should set ConfigDataDir to s.DataPath(firmware.ConfigDir).
	ConfigDataDir string

	// DUT is used for communicating with the device under test.
	DUT *dut.DUT

	// Model contains the DUT's model, as reported by the Platform RPC.
	// Currently, this is based on cros_config / name.
	Model string

	// RPCClient is a direct client connection to the Tast gRPC server hosted on the DUT.
	RPCClient *rpc.Client

	// rpcHint is needed in order to create an RPC client connection.
	rpcHint *testing.RPCHint

	// RPCUtils allows the Helper to call the firmware utils RPC service.
	RPCUtils fwpb.UtilsServiceClient
}

// NewHelper creates a new Helper object with info from testing.State.
func NewHelper(d *dut.DUT, rpcHint *testing.RPCHint) *Helper {
	return &Helper{
		DUT:     d,
		rpcHint: rpcHint,
	}
}

// Close shuts down any firmware objects associated with the Helper.
// Generally, tests should defer Close() immediately after initializing a Helper.
func (h *Helper) Close(ctx context.Context) error {
	return h.CloseRPCConnection(ctx)
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
	cl, err := rpc.Dial(ctx, h.DUT, h.rpcHint, "cros")
	if err != nil {
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
func (h *Helper) CloseRPCConnection(ctx context.Context) (err error) {
	if h.RPCClient != nil {
		if err = h.RPCClient.Close(ctx); err != nil {
			err = errors.Wrap(err, "closing rpc client")
		}
	}
	h.RPCClient = nil
	h.RPCUtils = nil
	return
}

// RequirePlatform fetches the DUT's board and model from RPC and caches them, unless they have already been cached.
func (h *Helper) RequirePlatform(ctx context.Context) error {
	if h.Board != "" {
		return nil
	}
	if err := h.RequireRPCUtils(ctx); err != nil {
		return errors.Wrap(err, "requiring RPC utils")
	}
	platformResponse, err := h.RPCUtils.Platform(ctx, &empty.Empty{})
	if err != nil {
		return errors.Wrap(err, "during Platform rpc")
	}
	h.Board = strings.ToLower(platformResponse.Board)
	h.Model = strings.ToLower(platformResponse.Model)
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
	// ConfigDataDir comes from testing.State, so it needs to be manually set in advance.
	if h.ConfigDataDir == "" {
		return errors.New("cannot create firmware Config without first setting Helper's ConfigDataDir")
	}
	cfg, err := NewConfig(h.ConfigDataDir, h.Board, h.Model)
	if err != nil {
		return errors.Wrapf(err, "during NewConfig with board=%s, model=%s", h.Board, h.Model)
	}
	h.Config = cfg
	return nil
}

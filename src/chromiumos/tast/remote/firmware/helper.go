// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package firmware

import (
	"context"

	"chromiumos/tast/dut"
	"chromiumos/tast/errors"
	"chromiumos/tast/rpc"
	fwpb "chromiumos/tast/services/cros/firmware"
	"chromiumos/tast/testing"
)

// Helper tracks several firmware-related objects.
type Helper struct {
	// DUT is used for communicating with the device under test.
	DUT *dut.DUT

	// RPCClient is a direct client connection to the Tast gRPC server hosted on the DUT.
	RPCClient *rpc.Client

	// RPCHint is needed in order to create an RPC client connection.
	// Any tests using an RPC client should set RPCHint to s.RPCHint().
	RPCHint *testing.RPCHint

	// RPCUtils allows the Helper to call the firmware utils RPC service.
	RPCUtils fwpb.UtilsServiceClient
}

// NewHelper creates a new Helper object with minimal attributes initialized.
func NewHelper(d *dut.DUT) *Helper {
	return &Helper{DUT: d}
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
	// RPCHint comes from testing.State, so it needs to be manually set in advance.
	if h.RPCHint == nil {
		return errors.New("cannot create RPC client connection without first setting Helper's RPCHint")
	}
	cl, err := rpc.Dial(ctx, h.DUT, h.RPCHint, "cros")
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
func (h *Helper) CloseRPCConnection(ctx context.Context) error {
	if h.RPCClient != nil {
		if err := h.RPCClient.Close(ctx); err != nil {
			return errors.Wrap(err, "closing rpc client")
		}
	}
	h.RPCClient = nil
	h.RPCUtils = nil
	return nil
}

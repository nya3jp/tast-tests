// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package rpcdut

import (
	"context"
	"errors"

	"chromiumos/tast/dut"
	"chromiumos/tast/rpc"
	"chromiumos/tast/testing"
)

var (
	errNotImplemented = errors.New("Not implemented")
)

// RPCDUT extends dut.DUT to maintail an additional RPC connection.
//
// Note that this implementation is not thread safe, yet.
//
// In order for this to work correctly, all other test code must use this DUT's
// methods. This is because this sets up hooks for Reboot that allow us to
// reconnect the RPC connection.
type RPCDUT struct {
	*dut.DUT
	h          *testing.RPCHint
	bundleName string
	cl         *rpc.Client
}

// NewRPCDUT creates a new RPCDUT with a dialed gRPC connection.
func NewRPCDUT(ctx context.Context, d *dut.DUT, h *testing.RPCHint, bundleName string) (*RPCDUT, error) {
	rd := &RPCDUT{DUT: d, h: h, bundleName: bundleName}

	if err := rd.rpcDial(ctx); err != nil {
		return nil, err
	}

	return rd, nil
}

func (rd *RPCDUT) rpcDial(ctx context.Context) error {
	testing.ContextLog(ctx, "Dialing RPC connection")
	cl, err := rpc.Dial(ctx, rd.DUT, rd.h, rd.bundleName)
	if err == nil {
		testing.ContextLog(ctx, "Connection is ", cl.Conn.GetState())
		rd.cl = cl
	}
	return err
}

func (rd *RPCDUT) rpcClose(ctx context.Context) error {
	testing.ContextLog(ctx, "Closing RPC connection")
	testing.ContextLog(ctx, "Connection is ", rd.cl.Conn.GetState())
	defer func() { rd.cl = nil }()

	if rd.cl != nil {
		return rd.cl.Close(ctx)
	}
	return nil
}

// RPCHint return the saved RPC hint.
func (rd *RPCDUT) RPCHint() *testing.RPCHint {
	return rd.h
}

// ConnRPC returns the current gRPC client connection.
func (rd *RPCDUT) ConnRPC() *rpc.Client {

	return rd.cl
}

// CloseRPC closes the gRPC connection.
func (rd *RPCDUT) CloseRPC(ctx context.Context) error {
	return rd.rpcClose(ctx)
}

// Reboot the dut, but reestablish the gRPC connection.
//
// The tast gRPC connection relies on the dut.DUT's existing ssh connection.
// When a Dial is initiated, it starts the remote bundle on the dut with the
// "-rpc" argument and sets up pipes for stdin/stdout.
// The returned gRPC client proxies gRPC requests to and from the remote bundle
// over the stdin/stdout provided by the long running ssh command/connection.
// So, if the dut is going to reboot, the remote bundle will be exiting and the
// ssh connection will be closing. When the ssh conneciton is reconnected,
// we can restart the remote bundle rpc server.
func (rd *RPCDUT) Reboot(ctx context.Context) error {
	if err := rd.rpcClose(ctx); err != nil {
		testing.ContextLog(ctx, "Failed to close gRPC connection before dut reboot")
	}
	err := rd.DUT.Reboot(ctx)
	if err := rd.rpcDial(ctx); err != nil {
		testing.ContextLog(ctx, "Failed to open gRPC connection after dut reboot")
	}
	return err
}

// WaitUnreachable errors out, since we have not implemented this yet.
func (d *RPCDUT) WaitUnreachable(ctx context.Context) error {
	return errNotImplemented
}

// WaitConnect errors out, since we have not implemented this yet.
func (d *RPCDUT) WaitConnect(ctx context.Context) error {
	return errNotImplemented
}

// Connect errors out, since we have not implemented this yet.
func (d *RPCDUT) Connect(ctx context.Context) error {
	return errNotImplemented
}

// Disconnect errors out, since we have not implemented this yet.
func (d *RPCDUT) Disconnect(ctx context.Context) error {
	return errNotImplemented
}

// Close errors out, since we have not implemented this yet.
func (d *RPCDUT) Close(ctx context.Context) error {
	return errNotImplemented
}

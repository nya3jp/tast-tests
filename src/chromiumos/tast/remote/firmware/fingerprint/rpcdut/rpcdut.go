// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package rpcdut provides a dut.DUT override that adds an additional
// managed RPC client connection.
package rpcdut

import (
	"context"

	"chromiumos/tast/dut"
	"chromiumos/tast/rpc"
	"chromiumos/tast/ssh"
	"chromiumos/tast/testing"
)

// RPCDUT extends dut.DUT to maintain an additional RPC connection.
//
// In order for this to work correctly, test code must use this DUT's
// methods and always reconstruct rpc clients using RPC() before use.
// This is because RPCDUT sets up hooks that intercept dut.DUT's methods
// to enable us to reconnect the RPC connection, when the underlying dut
// connection is changed using Connect, DIsconnect, Reboot, WaitUnreachable,
// or WaitConnect. Upon reconnect, any saved rpc clients from RPC()
// will be invalidated.
//
// Note that this implementation is not thread safe.
type RPCDUT struct {
	d          *dut.DUT
	h          *testing.RPCHint
	bundleName string
	cl         *rpc.Client
}

// NewRPCDUT creates a new RPCDUT with a dialed rpc connection.
func NewRPCDUT(ctx context.Context, d *dut.DUT, h *testing.RPCHint, bundleName string) (*RPCDUT, error) {
	rd := &RPCDUT{d: d, h: h, bundleName: bundleName}

	if err := rd.rpcDial(ctx); err != nil {
		return nil, err
	}

	return rd, nil
}

// rpcDial opens the rpc connection.
func (rd *RPCDUT) rpcDial(ctx context.Context) error {
	testing.ContextLog(ctx, "Dialing RPC connection")
	cl, err := rpc.Dial(ctx, rd.d, rd.h, rd.bundleName)
	if err != nil {
		return err
	}
	rd.cl = cl
	return nil
}

// rpcClose closes the rpc connection if one existed.
func (rd *RPCDUT) rpcClose(ctx context.Context) error {
	if rd == nil {
		return nil
	}
	testing.ContextLog(ctx, "Closing RPC connection")
	err := rd.cl.Close(ctx)
	rd.cl = nil
	return err
}

// RPCHint return the saved RPC hint.
func (rd *RPCDUT) RPCHint() *testing.RPCHint {
	return rd.h
}

// RPC returns the current rpc client or nil if a reconnection (Reboot)
// previously failed.
//
// The client returned will be nullified if a reconnection was necessary.
// This can happen if a Reboot was issued.
func (rd *RPCDUT) RPC() *rpc.Client {
	return rd.cl
}

// DUT returns the underlying DUT that does not manager the rpc connection.
func (rd *RPCDUT) DUT() *dut.DUT {
	return rd.d
}

// Close closes the rpc connection without disconnecting the ssh connection.
func (rd *RPCDUT) Close(ctx context.Context) error {
	return rd.Close(ctx)
}

// Reboot the dut and then reestablish the rpc connection.
//
// The tast gRPC connection relies on the dut.DUT's existing ssh connection.
// When a Dial is initiated, it starts the remote bundle on the dut with the
// "-rpc" argument and sets up pipes for stdin/stdout.
// The returned gRPC client proxies gRPC requests to and from the remote bundle
// over the stdin/stdout provided by the long running ssh command/connection.
// So, if the dut is going to reboot, the remote bundle will be exiting and the
// ssh connection will be closing. When the ssh connection is reconnected,
// we can restart the remote bundle rpc server.
func (rd *RPCDUT) Reboot(ctx context.Context) error {
	if err := rd.Close(ctx); err != nil {
		testing.ContextLog(ctx, "Failed to close rpc connection before reboot: ", err)
	}
	if err := rd.d.Reboot(ctx); err != nil {
		return err
	}
	if err := rd.rpcDial(ctx); err != nil {
		testing.ContextLog(ctx, "Failed to reopen rpc connection after reboot: ", err)
		return err
	}
	return nil
}

// WaitUnreachable errors out, since we have not implemented this yet.
func (rd *RPCDUT) WaitUnreachable(ctx context.Context) error {
	if err := rd.Close(ctx); err != nil {
		testing.ContextLog(ctx, "Failed to close rpc connection before disconnect: ", err)
	}
	if err := rd.d.WaitUnreachable(ctx); err != nil {
		return err
	}
	return nil
}

// WaitConnect errors out, since we have not implemented this yet.
// func (rd *RPC) WaitConnect(ctx context.Context) error {
// 	return errNotImplemented
// }

// Connect the dut and rpc connection.
func (rd *RPCDUT) Connect(ctx context.Context) error {
	if err := rd.Close(ctx); err != nil {
		testing.ContextLog(ctx, "Failed to close rpc connection before connect: ", err)
	}
	if err := rd.d.Connect(ctx); err != nil {
		return err
	}
	if err := rd.rpcDial(ctx); err != nil {
		testing.ContextLog(ctx, "Failed to dial rpc connection after connect: ", err)
		return err
	}
	return nil
}

// Disconnect the dut and rpc connection.
func (rd *RPCDUT) Disconnect(ctx context.Context) error {
	if err := rd.Close(ctx); err != nil {
		testing.ContextLog(ctx, "Failed to close rpc connection before disconnect: ", err)
	}
	if err := rd.d.Disconnect(ctx); err != nil {
		return err
	}
	return nil
}

// Conn returns the underlying DUT Conn().
func (rd *RPCDUT) Conn() *ssh.Conn {
	return rd.d.Conn()
}

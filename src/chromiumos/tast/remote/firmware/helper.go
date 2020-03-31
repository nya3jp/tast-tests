// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package firmware

/*
This file implements firmware.Helper, a struct which provides access to several firmware features.
*/

import (
	"context"

	"chromiumos/tast/dut"
	"chromiumos/tast/errors"
	"chromiumos/tast/remote/servo"
	"chromiumos/tast/rpc"
	fwpb "chromiumos/tast/services/cros/firmware"
	"chromiumos/tast/testing"
)

// Helper contains pointers to several features that are widely used by firmware tests.
type Helper struct {
	Config    *Config
	hasRPC    bool
	hasServo  bool
	DUT       *dut.DUT
	Proxy     *servo.Proxy
	rpcClient *rpc.Client
	Servo     *servo.Servo
	Utils     fwpb.UtilsServiceClient
}

// NewHelper creates a Helper object, without any of its features registered.
// A typical firmware test might begin like this:
//     h := firmware.NewHelper(s.DUT())
//     defer h.Close(ctx)
//     if err := h.AddConfig(ctx); err != nil {...} // if config is needed
//     (ditto h.AddRPC, etc.)
func NewHelper(d *dut.DUT) *Helper {
	return &Helper{DUT: d}
}

// AddConfig creates a Config object, and registers it to the Helper.
func (h *Helper) AddConfig(ctx context.Context) error {
	cfg, err := NewConfig()
	if err != nil {
		return errors.Wrap(err, "creating config")
	}
	h.Config = cfg
	return nil
}

// AddRPC connects to gRPC and registers the connection to the Helper, along with all firmware-related service clients.
func (h *Helper) AddRPC(ctx context.Context, rpcHint *testing.RPCHint) error {
	cl, err := rpc.Dial(ctx, h.DUT, rpcHint, "cros")
	if err != nil {
		return errors.Wrap(err, "establishing rpc connection")
	}
	h.rpcClient = cl
	h.Utils = fwpb.NewUtilsServiceClient(cl.Conn)
	h.hasRPC = true
	return nil
}

// AddServo registers a servo/proxy to the Helper.
func (h *Helper) AddServo(ctx context.Context, servoSpec string) error {
	pxy, err := servo.NewProxy(ctx, servoSpec, h.DUT.KeyFile(), h.DUT.KeyDir())
	if err != nil {
		return errors.Wrap(err, "connecting to servo")
	}
	h.Proxy = pxy
	h.Servo = h.Proxy.Servo()
	h.hasServo = true
	return nil
}

// Close initiates the shutdown of all processes tracked by Helper.
func (h *Helper) Close(ctx context.Context) {
	if h.hasRPC {
		h.rpcClient.Close(ctx)
	}
	if h.hasServo {
		h.Proxy.Close(ctx)
	}
}

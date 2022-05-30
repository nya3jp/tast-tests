// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package policyutil

import (
	"context"

	"github.com/golang/protobuf/ptypes/empty"

	"go.chromium.org/chromiumos/tast/dut"
	"go.chromium.org/chromiumos/tast/errors"
	"go.chromium.org/chromiumos/tast-tests/remote/hwsec"
	"go.chromium.org/chromiumos/tast/rpc"
	oc "go.chromium.org/chromiumos/tast-tests/services/cros/hwsec"
	"go.chromium.org/chromiumos/tast/testing"
)

// EnsureTPMAndSystemStateAreResetLocal initialises the required helpers and calls the EnsureTPMAndSystemStateAreReset locally.
func EnsureTPMAndSystemStateAreResetLocal(ctx context.Context, dut *dut.DUT, hint *testing.RPCHint) error {
	cl, err := rpc.Dial(ctx, dut, hint)
	if err != nil {
		return errors.Wrap(err, "failed to connect to the RPC service on the DUT")
	}
	defer cl.Close(ctx)

	pc := oc.NewOwnershipServiceClient(cl.Conn)

	if _, err := pc.EnsureTPMAndSystemStateAreReset(ctx, &empty.Empty{}); err != nil {
		return errors.Wrap(err, "failed to reset the TPM locally")
	}

	return nil
}

// EnsureTPMAndSystemStateAreResetRemote initialises the required helpers and calls EnsureTPMAndSystemStateAreReset remotely.
func EnsureTPMAndSystemStateAreResetRemote(ctx context.Context, d *dut.DUT) error {
	r := hwsec.NewCmdRunner(d)

	helper, err := hwsec.NewHelper(r, d)
	if err != nil {
		return errors.Wrap(err, "helper creation error")
	}

	if err := helper.EnsureTPMAndSystemStateAreReset(ctx); err != nil {
		return errors.Wrap(err, "failed to reset system")
	}

	return nil
}

// EnsureTPMAndSystemStateAreReset calls EnsureTPMAndSystemStateAreResetLocal and if that fails, EnsureTPMAndSystemStateAreResetRemote.
// This avoids reboots as much as possible.
func EnsureTPMAndSystemStateAreReset(ctx context.Context, d *dut.DUT, hint *testing.RPCHint) error {
	if err := EnsureTPMAndSystemStateAreResetLocal(ctx, d, hint); err != nil {
		testing.ContextLog(ctx, "Local reset failed: ", err)

		return EnsureTPMAndSystemStateAreResetRemote(ctx, d)
	}

	return nil
}

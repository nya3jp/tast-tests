// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package firmware

/*
This file implements functions to check or switch the DUT's boot mode.
*/

import (
	"context"
	"time"

	"github.com/golang/protobuf/ptypes/empty"

	fwCommon "chromiumos/tast/common/firmware"
	"chromiumos/tast/dut"
	"chromiumos/tast/errors"
	fwpb "chromiumos/tast/services/cros/firmware"
	"chromiumos/tast/testing"
)

// CheckBootMode forwards to the CheckBootMode RPC to check whether the DUT is in a specified boot mode.
func CheckBootMode(ctx context.Context, utils fwpb.UtilsServiceClient, bootMode fwCommon.BootMode) (bool, error) {
	res, err := utils.CurrentBootMode(ctx, &empty.Empty{})
	if err != nil {
		return false, errors.Wrap(err, "calling CurrentBootMode rpc")
	}
	return bootMode == fwCommon.BootModeFromProto[res.BootMode], nil
}

// RebootToMode reboots the DUT into the specified boot mode.
// This has the side-effect of disconnecting the RPC client from the DUT's RPC server.
func RebootToMode(ctx context.Context, d *dut.DUT, utils fwpb.UtilsServiceClient, toMode fwCommon.BootMode) error {
	res, err := utils.CurrentBootMode(ctx, &empty.Empty{})
	if err != nil {
		return errors.Wrap(err, "calling CurrentBootMode rpc")
	}
	fromMode := fwCommon.BootModeFromProto[res.BootMode]

	switch toMode {
	case fwCommon.BootModeNormal:
		switch fromMode {
		case fwCommon.BootModeNormal:
			if err := d.Reboot(ctx); err != nil {
				return errors.Wrap(err, "rebooting DUT")
			}
			return nil
		default:
			return errors.Errorf("unsupported firmware boot mode transition %s>%s", fromMode, toMode)
		}
	default:
		return errors.Errorf("unsupported firmware boot mode transition to %s", toMode)
	}
}

// poweroff safely powers off the DUT with the "poweroff" command, then waits for the DUT to be unreachable.
func poweroff(ctx context.Context, d *dut.DUT) error {
	testing.ContextLog(ctx, "Powering off DUT")
	poweroffCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	d.Command("poweroff").Run(poweroffCtx) // ignore the error

	offCtx, cancel := context.WithTimeout(ctx, 3*time.Minute)
	defer cancel()
	if err := d.WaitUnreachable(offCtx); err != nil {
		return errors.Wrap(err, "waiting for dut to be unreachable after sending poweroff command")
	}
	return nil
}

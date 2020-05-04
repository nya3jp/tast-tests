// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package firmware

/*
This file implements functions to check or switch the DUT's boot mode.
*/

import (
	"context"

	"github.com/golang/protobuf/ptypes/empty"

	fwCommon "chromiumos/tast/common/firmware"
	"chromiumos/tast/dut"
	"chromiumos/tast/errors"
	"chromiumos/tast/remote/servo"
	fwpb "chromiumos/tast/services/cros/firmware"
)

// CheckBootMode forwards to the CheckBootMode RPC to check whether the DUT is in a specified boot mode.
func CheckBootMode(ctx context.Context, utils fwpb.UtilsServiceClient, bootMode fwCommon.BootMode) (bool, error) {
	res, err := utils.CurrentBootMode(ctx, &empty.Empty{})
	if err != nil {
		return false, errors.Wrap(err, "calling CurrentBootMode rpc")
	}
	return bootMode == fwCommon.BootModeFromProto[res.BootMode], nil
}

// RebootToMode reboots the DUT and switches to the specified boot mode.
func RebootToMode(ctx context.Context, d *dut.DUT, sv *servo.Servo, utils fwpb.UtilsServiceClient, cfg *Config, toMode fwCommon.BootMode, log func(...interface{})) error {
	res, err := utils.CurrentBootMode(ctx, &empty.Empty{})
	if err != nil {
		return errors.Wrap(err, "calling CurrentBootMode rpc")
	}
	fromMode := fwCommon.BootModeFromProto[res.BootMode]

	switch toMode {
	case fwCommon.BootModeNormal:
		switch fromMode {
		case fwCommon.BootModeNormal:
			log("Setting PowerState to Off")
			go sv.SetPowerState(ctx, servo.PowerStateOff)
			log("Waiting until client unreachable")
			if err := d.WaitUnreachable(ctx); err != nil {
				return errors.Wrapf(err, "waiting for DUT to be unreachable after setting %s to %s", servo.PowerState, servo.PowerStateOff)
			}
			log("Setting PowerState to On")
			go sv.SetPowerState(ctx, servo.PowerStateOn)
			log("Waiting until client connected")
			if err := d.WaitConnect(ctx); err != nil {
				return errors.Wrapf(err, "waiting to connect to DUT after setting %s to %s", servo.PowerState, servo.PowerStateOn)
			}
			return nil
		default:
			return errors.Errorf("unsupported firmware boot mode transition %s>%s", fromMode, toMode)
		}
	default:
		return errors.Errorf("unsupported firmware boot mode transition to %s", toMode)
	}
}

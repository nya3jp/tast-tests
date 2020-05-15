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
	"chromiumos/tast/remote/servo"
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
func RebootToMode(ctx context.Context, d *dut.DUT, svo *servo.Servo, utils fwpb.UtilsServiceClient, toMode fwCommon.BootMode) error {
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
		case fwCommon.BootModeRecovery:
			if err := cyclePowerState(ctx, d, svo, servo.PowerStateOn); err != nil {
				return errors.Wrapf(err, "cycling dut power state to %s", servo.PowerStateOn)
			}
			return nil
		default:
			return errors.Errorf("unsupported firmware boot mode transition %s>%s", fromMode, toMode)
		}
	case fwCommon.BootModeRecovery:
		if err := cyclePowerState(ctx, d, svo, servo.PowerStateRec); err != nil {
			return errors.Wrapf(err, "cycling dut power state to %s", servo.PowerStateRec)
		}
		return nil
	default:
		return errors.Errorf("unsupported firmware boot mode transition to %s", toMode)
	}
}

// shutdown safely shuts down the DUT with the "shutdown" command, then waits for the DUT to be unreachable before returning.
func shutdown(ctx context.Context, d *dut.DUT) error {
	shutdownCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	d.Command("shutdown").Run(shutdownCtx) // ignore the error

	offCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	if err := d.WaitUnreachable(offCtx); err != nil {
		return errors.Wrap(err, "waiting for dut to be unreachable after sending shutdown command")
	}
	return nil
}

// cyclePowerState sets the PowerState control to Off, then sets the PowerState to a specified value, and reconnects to the DUT.
func cyclePowerState(ctx context.Context, d *dut.DUT, s *servo.Servo, ps servo.PowerStateValue) error {
	if err := shutdown(ctx, d); err != nil {
		return errors.Wrapf(err, "shutting down dut to reboot to %s", ps)
	}
	s.SetPowerState(ctx, ps)
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
		defer cancel()
		if err := d.WaitConnect(ctx); err != nil {
			return errors.Wrap(err, "failed to connect to DUT")
		}
		return nil
	}, &testing.PollOptions{Timeout: 3 * time.Minute}); err != nil {
		return errors.Wrap(err, "failed to reconnect to DUT")
	}
	return nil
}

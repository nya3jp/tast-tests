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

const (
	// cmdTimeout is a short duration used for sending commands.
	cmdTimeout = 3 * time.Second

	// offTimeout is the timeout to wait for the DUT to be unreachable after powering off.
	offTimeout = 3 * time.Minute
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
func RebootToMode(ctx context.Context, d *dut.DUT, svo *servo.Servo, cfg *Config, utils fwpb.UtilsServiceClient, toMode fwCommon.BootMode) error {
	if d == nil {
		return errors.New("invalid nil pointer for DUT")
	}
	if svo == nil {
		return errors.New("invalid nil pointer for servo")
	}
	if _, err := utils.BlockingSync(ctx, &empty.Empty{}); err != nil {
		return errors.Wrap(err, "syncing dut before reboot")
	}

	// Determine current boot mode
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
			if err := cyclePowerState(ctx, d, svo, cfg, servo.PowerStateOn); err != nil {
				return errors.Wrapf(err, "cycling dut power state to %s", servo.PowerStateOn)
			}
			return nil
		default:
		}
	case fwCommon.BootModeRecovery:
		// Setup USBKey
		testing.ContextLog(ctx, "Setting USB Mux state to host")
		if err := svo.SetUSBMuxState(ctx, servo.USBMuxHost); err != nil {
			return errors.Wrap(err, "setting usb mux state to host before powering off")
		}
		if err := svo.SetV4Role(ctx, servo.V4RoleSnk); err != nil {
			return err
		}
		// Reboot to Mode
		if _, err := utils.BlockingSync(ctx, &empty.Empty{}); err != nil {
			return errors.Wrap(err, "syncing dut before reboot")
		}
		if err := poweroff(ctx, d, svo); err != nil {
			return errors.Wrap(err, "powering off dut")
		}
		testing.ContextLog(ctx, "Setting USB Mux state to DUT")
		if err := svo.SetUSBMuxState(ctx, servo.USBMuxDUT); err != nil {
			return errors.Wrap(err, "setting usb mux state to dut while dut is off")
		}
		if err := svo.SetPowerState(ctx, servo.PowerStateRec); err != nil {
			return errors.Wrapf(err, "setting power state to %s", servo.PowerStateRec)
		}
		testing.ContextLog(ctx, "Attempting to reconnect to DUT")
		if err := testing.Poll(ctx, func(ctx context.Context) error {
			return d.WaitConnect(ctx)
		}, &testing.PollOptions{Timeout: 3 * time.Minute}); err != nil {
			return errors.Wrap(err, "failed to reconnect to DUT after booting to recovery")
		}
		if err := testing.Sleep(ctx, 2*time.Minute); err != nil {
			return errors.Wrap(err, "failed to sleep?")
		}
		return nil
	default:
	}
	return errors.Errorf("unsupported firmware boot mode transition: %s to %s", fromMode, toMode)
}

// poweroff safely powers off the DUT with the "poweroff" command, then waits for the DUT to be unreachable.
func poweroff(ctx context.Context, d *dut.DUT, svo *servo.Servo) error {
	testing.ContextLog(ctx, "Powering off DUT")
	poweroffCtx, cancel := context.WithTimeout(ctx, cmdTimeout)
	defer cancel()
	d.Command("poweroff").Run(poweroffCtx) // ignore the error

	offCtx, cancel := context.WithTimeout(ctx, offTimeout)
	defer cancel()
	if err := d.WaitUnreachable(offCtx); err != nil {
		return errors.Wrap(err, "waiting for dut to be unreachable after sending poweroff command")
	}
	// Show servod that the power state has changed
	svo.SetPowerState(ctx, servo.PowerStateOff)
	return nil
}

// cyclePowerState safely powers off the DUT, then sets the power state to a specified value.
func cyclePowerState(ctx context.Context, d *dut.DUT, svo *servo.Servo, cfg *Config, ps servo.PowerStateValue) error {
	testing.ContextLog(ctx, "Powering off DUT")
	if err := poweroff(ctx, d, svo); err != nil {
		return errors.Wrap(err, "powering off dut")
	}
	testing.ContextLogf(ctx, "Setting power state to %s", ps)
	if err := svo.SetPowerState(ctx, ps); err != nil {
		return err
	}
	testing.ContextLogf(ctx, "Trying to reconnect (timeout=%d seconds)", cfg.DelayRebootToPing)
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		testing.ContextLog(ctx, "Polling")
		// Set a short timeout to the iteration in case of any SSH operations
		// blocking for a long time.
		ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
		defer cancel()
		if err := d.WaitConnect(ctx); err != nil {
			return errors.Wrap(err, "failed to connect to DUT")
		}
		return nil
	}, &testing.PollOptions{Timeout: 3 * time.Minute}); err != nil {
		return errors.Wrap(err, "failed to wait to reconnect to DUT")
	}
	testing.ContextLog(ctx, "Done waiting. Good job")
	return nil
}

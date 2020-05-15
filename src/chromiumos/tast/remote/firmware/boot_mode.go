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
	"chromiumos/tast/rpc"
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
// This has the side-effect of disconnecting the RPC client.
func RebootToMode(ctx context.Context, d *dut.DUT, svo *servo.Servo, cfg *Config, cl *rpc.Client, toMode fwCommon.BootMode) error {
	if d == nil {
		return errors.New("invalid nil pointer for DUT")
	}
	if svo == nil {
		return errors.New("invalid nil pointer for servo")
	}

	utils := fwpb.NewUtilsServiceClient(cl.Conn)
	if _, err := utils.BlockingSync(ctx, &empty.Empty{}); err != nil {
		return errors.Wrap(err, "syncing dut before reboot")
	}

	// Determine current boot mode
	res, err := utils.CurrentBootMode(ctx, &empty.Empty{})
	if err != nil {
		return errors.Wrap(err, "calling CurrentBootMode rpc")
	}
	fromMode := fwCommon.BootModeFromProto[res.BootMode]

	// Kill RPC client connection before rebooting
	cl.Close(ctx)

	switch toMode {
	case fwCommon.BootModeNormal:
		if err := d.Reboot(ctx); err != nil {
			return errors.Wrap(err, "rebooting DUT")
		}
		return nil
	case fwCommon.BootModeRecovery:
		// In recovery boot, the locked EC RO doesn't support PD for most CrOS devices.
		// The default servo v4 power role is SRC, making the DUT a SNK.
		// Lack of PD makes CrOS unable to do the data role swap from UFP to DFP.
		// As a result, the DUT can't see the USB disk and Ethernet dongle on Servo v4.
		// This is a workaround to set Servo v4 as a SNK when using the USB disk for recovery boot.
		if err := svo.SetV4Role(ctx, servo.V4RoleSnk); err != nil {
			return errors.Wrap(err, "setting servo_v4 role to snk before powering off")
		}
		if err := poweroff(ctx, d, svo); err != nil {
			return errors.Wrap(err, "powering off dut")
		}
		// Servo must reveal the USB key to the DUT in order for the DUT to boot from USB.
		if err := svo.SetUSBMuxState(ctx, servo.USBMuxDUT); err != nil {
			return errors.Wrap(err, "setting usb mux state to dut while dut is off")
		}
		if err := svo.SetPowerState(ctx, servo.PowerStateRec); err != nil {
			return errors.Wrapf(err, "setting power state to %s", servo.PowerStateRec)
		}
		testing.ContextLog(ctx, "Reestablishing connection to DUT")
		if err := testing.Poll(ctx, func(ctx context.Context) error {
			return d.WaitConnect(ctx)
		}, &testing.PollOptions{Timeout: 3 * time.Minute}); err != nil {
			return errors.Wrap(err, "failed to reconnect to DUT after booting to recovery")
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

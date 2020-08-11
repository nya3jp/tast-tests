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
	"chromiumos/tast/errors"
	"chromiumos/tast/remote/servo"
	"chromiumos/tast/testing"
)

const (
	// cmdTimeout is a short duration used for sending commands.
	cmdTimeout = 3 * time.Second

	// offTimeout is the timeout to wait for the DUT to be unreachable after powering off.
	offTimeout = 3 * time.Minute

	// reconnectTimeout is the timeout to wait to reconnect to the DUT after rebooting.
	reconnectTimeout = 3 * time.Minute
)

// ModeSwitcher enables booting the DUT into different firmware boot modes (normal, dev, rec).
type ModeSwitcher struct {
	Helper *Helper
}

// NewModeSwitcher creates a new ModeSwitcher. It relies on a firmware Helper to track dependent objects, such as servo and RPC client.
func NewModeSwitcher(h *Helper) *ModeSwitcher {
	return &ModeSwitcher{
		Helper: h,
	}
}

// CheckBootMode forwards to the CheckBootMode RPC to check whether the DUT is in a specified boot mode.
func (ms *ModeSwitcher) CheckBootMode(ctx context.Context, bootMode fwCommon.BootMode) (bool, error) {
	h := ms.Helper
	if err := h.RequireRPCUtils(ctx); err != nil {
		return false, errors.Wrap(err, "requiring RPC utils")
	}
	res, err := h.RPCUtils.CurrentBootMode(ctx, &empty.Empty{})
	if err != nil {
		return false, errors.Wrap(err, "calling CurrentBootMode rpc")
	}
	return bootMode == fwCommon.BootModeFromProto[res.BootMode], nil
}

// RebootToMode reboots the DUT into the specified boot mode.
// This has the side-effect of disconnecting the RPC client.
func (ms ModeSwitcher) RebootToMode(ctx context.Context, toMode fwCommon.BootMode) error {
	h := ms.Helper
	if err := h.RequireRPCUtils(ctx); err != nil {
		return errors.Wrap(err, "requiring RPC utils")
	}
	if _, err := h.RPCUtils.BlockingSync(ctx, &empty.Empty{}); err != nil {
		return errors.Wrap(err, "syncing DUT before reboot")
	}

	res, err := h.RPCUtils.CurrentBootMode(ctx, &empty.Empty{})
	if err != nil {
		return errors.Wrap(err, "calling CurrentBootMode rpc")
	}
	fromMode := fwCommon.BootModeFromProto[res.BootMode]

	// Kill RPC client connection before rebooting
	h.CloseRPCConnection(ctx)

	switch toMode {
	case fwCommon.BootModeNormal:
		if err := h.DUT.Reboot(ctx); err != nil {
			return errors.Wrap(err, "rebooting DUT")
		}
	case fwCommon.BootModeRecovery:
		if err := h.RequireServo(ctx); err != nil {
			return errors.Wrap(err, "requiring servo")
		}
		// In recovery boot, the locked EC RO doesn't support PD for most CrOS devices.
		// The default servo v4 power role is SRC, making the DUT a SNK.
		// Lack of PD makes CrOS unable to do the data role swap from UFP to DFP.
		// As a result, the DUT can't see the USB disk and Ethernet dongle on Servo v4.
		// This is a workaround to set Servo v4 as a SNK when using the USB disk for recovery boot.
		if err := h.Servo.SetV4Role(ctx, servo.V4RoleSnk); err != nil {
			return errors.Wrap(err, "setting servo_v4 role to snk before powering off")
		}
		if err := ms.poweroff(ctx); err != nil {
			return errors.Wrap(err, "powering off DUT")
		}
		// Servo must show the USB key to the DUT in order for the DUT to boot from USB.
		if err := h.Servo.SetUSBMuxState(ctx, servo.USBMuxDUT); err != nil {
			return errors.Wrap(err, "setting usb mux state to DUT while DUT is off")
		}
		if err := h.Servo.SetPowerState(ctx, servo.PowerStateRec); err != nil {
			return errors.Wrapf(err, "setting power state to %s", servo.PowerStateRec)
		}
		testing.ContextLog(ctx, "Reestablishing connection to DUT")
		if err := testing.Poll(ctx, func(ctx context.Context) error {
			return h.DUT.WaitConnect(ctx)
		}, &testing.PollOptions{Timeout: reconnectTimeout}); err != nil {
			return errors.Wrap(err, "failed to reconnect to DUT after booting to recovery")
		}
	default:
		return errors.Errorf("unsupported firmware boot mode transition: %s to %s", fromMode, toMode)
	}
	if ok, err := ms.CheckBootMode(ctx, toMode); err != nil {
		return errors.Wrapf(err, "checking boot mode after reboot to %s", toMode)
	} else if !ok {
		return errors.Errorf("DUT was not in %s after RebootToMode", toMode)
	}
	testing.ContextLogf(ctx, "DUT is now in %s mode", toMode)
	return nil
}

// poweroff safely powers off the DUT with the "poweroff" command, then waits for the DUT to be unreachable.
func (ms *ModeSwitcher) poweroff(ctx context.Context) error {
	h := ms.Helper
	if err := h.RequireServo(ctx); err != nil {
		return errors.Wrap(err, "requiring servo")
	}
	testing.ContextLog(ctx, "Powering off DUT")
	poweroffCtx, cancel := context.WithTimeout(ctx, cmdTimeout)
	defer cancel()
	h.DUT.Command("poweroff").Run(poweroffCtx) // ignore the error

	offCtx, cancel := context.WithTimeout(ctx, offTimeout)
	defer cancel()
	if err := h.DUT.WaitUnreachable(offCtx); err != nil {
		return errors.Wrap(err, "waiting for DUT to be unreachable after sending poweroff command")
	}
	// Show servod that the power state has changed
	h.Servo.SetPowerState(ctx, servo.PowerStateOff)
	return nil
}

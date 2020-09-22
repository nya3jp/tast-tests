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
func NewModeSwitcher(ctx context.Context, h *Helper) (*ModeSwitcher, error) {
	if err := h.RequireConfig(ctx); err != nil {
		return nil, errors.Wrap(err, "requiring firmware config")
	}
	return &ModeSwitcher{
		Helper: h,
	}, nil
}

// RebootToMode reboots the DUT into the specified boot mode.
// This has the side-effect of disconnecting the RPC client.
func (ms ModeSwitcher) RebootToMode(ctx context.Context, toMode fwCommon.BootMode) error {
	h := ms.Helper
	if err := h.RequireServo(ctx); err != nil {
		return errors.Wrap(err, "requiring servo")
	}

	fromMode, err := h.Reporter.CurrentBootMode(ctx)
	if err != nil {
		return errors.Wrap(err, "determining boot mode at the start of RebootToMode")
	}

	// Perform blocking sync prior to reboot, then close the RPC connection.
	if err := h.RequireRPCUtils(ctx); err != nil {
		return errors.Wrap(err, "requiring RPC utils")
	}
	if _, err := h.RPCUtils.BlockingSync(ctx, &empty.Empty{}); err != nil {
		return errors.Wrap(err, "syncing DUT before reboot")
	}
	h.CloseRPCConnection(ctx)

	switch toMode {
	case fwCommon.BootModeNormal:
		// 1. Set DUT power_state to "off".
		// 2. Wait ECBootToPwrButton.
		// 3. Ensure that we cannot reach DUT.
		// 4. Set DUT power_state to "on".
		// 5. If booting from Dev Mode, deactivate firmware screen.
		h.Servo.SetPowerState(ctx, servo.PowerStateOff)
		if err := testing.Sleep(ctx, h.Config.ECBootToPwrButton); err != nil {
			return errors.Wrapf(err, "waiting %s (ECBootToPwrButton) while booting DUT into normal mode", h.Config.ECBootToPwrButton)
		}
		offCtx, cancel := context.WithTimeout(ctx, offTimeout)
		defer cancel()
		if err := h.DUT.WaitUnreachable(offCtx); err != nil {
			return errors.Wrap(err, "waiting for DUT to be unreachable after powering off")
		}
		if err := h.Servo.SetPowerState(ctx, servo.PowerStateOn); err != nil {
			return err
		}
		if fromMode == fwCommon.BootModeDev {
			if err := ms.fwScreenToNormalMode(ctx); err != nil {
				return errors.Wrap(err, "moving from firmware screen to normal mode")
			}
		}
	case fwCommon.BootModeRecovery:
		// Recovery mode requires the DUT to boot the image on the USB.
		// Thus, the servo must show the USB to the DUT.
		if err := ms.enableRecMode(ctx, servo.USBMuxDUT); err != nil {
			return err
		}
	case fwCommon.BootModeDev:
		// 1. Set power_state to 'rec', but don't show the DUT a USB image to boot from.
		// 2. From the firmware screen that appears, press keys to transition to dev mode.
		//    The specific keypresses will depend on the DUT's ModeSwitcherType.
		if err := ms.enableRecMode(ctx, servo.USBMuxHost); err != nil {
			return err
		}
		if err := ms.fwScreenToDevMode(ctx); err != nil {
			return errors.Wrap(err, "moving from firmware screen to dev mode")
		}
	default:
		return errors.Errorf("unsupported firmware boot mode: %s", toMode)
	}

	// Reconnect to the DUT.
	testing.ContextLog(ctx, "Reestablishing connection to DUT")
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		return h.DUT.WaitConnect(ctx)
	}, &testing.PollOptions{Timeout: reconnectTimeout}); err != nil {
		return errors.Wrapf(err, "failed to reconnect to DUT after booting to %s", toMode)
	}

	// Verify successful reboot.
	if ok, err := h.Reporter.CheckBootMode(ctx, toMode); err != nil {
		return errors.Wrapf(err, "checking boot mode after reboot to %s", toMode)
	} else if !ok {
		return errors.Errorf("DUT was not in %s after RebootToMode", toMode)
	}
	testing.ContextLogf(ctx, "DUT is now in %s mode", toMode)
	return nil
}

// fwScreenToNormalMode moves the DUT from the firmware bootup screen to Normal mode.
// This should be called immediately after powering on.
// The actual behavior depends on the ModeSwitcherType.
func (ms *ModeSwitcher) fwScreenToNormalMode(ctx context.Context) error {
	h := ms.Helper
	if err := h.RequireServo(ctx); err != nil {
		return errors.Wrap(err, "requiring servo")
	}
	switch h.Config.ModeSwitcherType {
	case KeyboardDevSwitcher:
		// 1. Sleep for [FirmwareScreen] seconds.
		// 2. Press enter.
		// 3. Sleep for [ConfirmScreen] seconds.
		// 4. Press enter.
		if err := testing.Sleep(ctx, h.Config.FirmwareScreen); err != nil {
			return errors.Wrapf(err, "sleeping for %s (FirmwareScreen) while disabling dev mode", h.Config.FirmwareScreen)
		}
		if err := h.Servo.KeypressWithDuration(ctx, servo.Enter, servo.DurPress); err != nil {
			return errors.Wrap(err, "pressing Enter on firmware screen while disabling dev mode")
		}
		if err := testing.Sleep(ctx, h.Config.ConfirmScreen); err != nil {
			return errors.Wrapf(err, "sleeping for %s (ConfirmScreen) while disabling dev mode", h.Config.ConfirmScreen)
		}
		if err := h.Servo.KeypressWithDuration(ctx, servo.Enter, servo.DurPress); err != nil {
			return errors.Wrap(err, "pressing Enter on confirm screen while disabling dev mode")
		}
	default:
		return errors.Errorf("unsupported ModeSwitcherType %s for fwScreenToNormalMode", h.Config.ModeSwitcherType)
	}
	return nil
}

// fwScreenToDevMode moves the DUT from the firmware bootup screen to Dev mode.
// This should be called immediately after powering on.
// The actual behvior depends on the ModeSwitcherType.
func (ms *ModeSwitcher) fwScreenToDevMode(ctx context.Context) error {
	h := ms.Helper
	if err := h.RequireServo(ctx); err != nil {
		return errors.Wrap(err, "requiring servo")
	}

	switch h.Config.ModeSwitcherType {
	case KeyboardDevSwitcher:
		// 1. Wait until the firmware screen appears.
		// 2. Press Ctrl-D to move to the confirm screen.
		// 3. Wait until the confirm screen appears.
		// 4. Push some button depending on the DUT's config: toggle the rec button, press power, or press enter.
		if err := testing.Sleep(ctx, h.Config.FirmwareScreen); err != nil {
			return err
		}
		if err := h.Servo.KeypressWithDuration(ctx, servo.CtrlD, servo.DurPress); err != nil {
			return err
		}
		if err := testing.Sleep(ctx, h.Config.ConfirmScreen); err != nil {
			return err
		}
		if h.Config.RecButtonDevSwitch {
			if err := h.Servo.ToggleOffOn(ctx, servo.RecMode); err != nil {
				return err
			}
		} else if h.Config.PowerButtonDevSwitch {
			if err := h.Servo.KeypressWithDuration(ctx, servo.PowerKey, servo.DurPress); err != nil {
				return err
			}
		} else {
			if err := h.Servo.KeypressWithDuration(ctx, servo.Enter, servo.DurPress); err != nil {
				return err
			}
		}
	default:
		return errors.Errorf("booting to dev mode: unsupported ModeSwitcherType: %s", h.Config.ModeSwitcherType)
	}
	return nil
}

// enableRecMode powers the DUT into the "rec" state, but does not wait to reconnect to the DUT.
// If booting into rec mode, usbMux should point to the DUT, so that the DUT can finish booting into recovery mode.
// Otherwise, usbMux should point to the Host. This will prevent the DUT from transitioning to rec mode, so other operations can be performed (such as bypassing to dev mode).
func (ms *ModeSwitcher) enableRecMode(ctx context.Context, usbMux servo.USBMuxState) error {
	h := ms.Helper
	if err := h.RequireServo(ctx); err != nil {
		return errors.Wrap(err, "requiring servo")
	}
	if usbMux == servo.USBMuxDUT {
		// In recovery boot, the locked EC RO doesn't support PD for most CrOS devices.
		// The default servo v4 power role is SRC, making the DUT a SNK.
		// Lack of PD makes CrOS unable to do the data role swap from UFP to DFP.
		// As a result, the DUT can't see the USB disk and Ethernet dongle on Servo v4.
		// This is a workaround to set Servo v4 as a SNK when using the USB disk for recovery boot.
		if err := h.Servo.SetV4Role(ctx, servo.V4RoleSnk); err != nil {
			return errors.Wrap(err, "setting servo_v4 role to snk before powering off")
		}
	}
	if err := ms.poweroff(ctx); err != nil {
		return errors.Wrap(err, "powering off DUT")
	}
	if err := h.Servo.SetUSBMuxState(ctx, usbMux); err != nil {
		return errors.Wrap(err, "setting usb mux state to DUT while DUT is off")
	}
	if err := h.Servo.SetPowerState(ctx, servo.PowerStateRec); err != nil {
		return errors.Wrapf(err, "setting power state to %s", servo.PowerStateRec)
	}
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

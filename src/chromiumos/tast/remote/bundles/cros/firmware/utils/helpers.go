// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package utils contains functionality shared by tests that
// exercise firmware.
package utils

import (
	"context"
	"strconv"
	"time"

	fwCommon "chromiumos/tast/common/firmware"
	"chromiumos/tast/common/servo"
	"chromiumos/tast/errors"
	"chromiumos/tast/remote/firmware"
	"chromiumos/tast/remote/firmware/reporters"
	"chromiumos/tast/testing"
)

// ChangeFWVariant checks if current FW variant (A/B) is equal to the fwVar, if not it switches to the fwVar
func ChangeFWVariant(ctx context.Context, h *firmware.Helper, ms *firmware.ModeSwitcher, fwVar fwCommon.RWSection) error {
	testing.ContextLogf(ctx, "Check the firmware version, looking for %q", fwVar)
	if isFWVerCorrect, err := h.Reporter.CheckFWVersion(ctx, string(fwVar)); err != nil {
		return errors.Wrap(err, "failed to check a firmware version")
	} else if !isFWVerCorrect {
		testing.ContextLogf(ctx, "Set FW tries to %q", fwVar)
		if err := firmware.SetFWTries(ctx, h.DUT, fwVar, 0); err != nil {
			return errors.Wrapf(err, "failed to set FW tries to %q", fwVar)
		}

		if err := ms.ModeAwareReboot(ctx, firmware.WarmReset); err != nil {
			return errors.Wrap(err, "failed to perform mode aware reboot")
		}

		testing.ContextLog(ctx, "Check the firmware version after reboot")
		if isFWVerCorrect, err := h.Reporter.CheckFWVersion(ctx, string(fwVar)); err != nil {
			return errors.Wrap(err, "failed to check a firmware version")
		} else if !isFWVerCorrect {
			return errors.New("failed to boot into the expected firmware version")
		}

		// Require again here since reboots in test cause nil pointer errors otherwise.
		if err := h.RequireBiosServiceClient(ctx); err != nil {
			return errors.Wrap(err, "requiring BiosServiceClient")
		}
	}
	return nil
}

// CheckRecReason checks if recovery reason occures in the expReason slice
func CheckRecReason(ctx context.Context, h *firmware.Helper, ms *firmware.ModeSwitcher, expReasons []reporters.RecoveryReason) error {
	if err := h.Servo.SetUSBMuxState(ctx, servo.USBMuxHost); err != nil {
		errors.Wrap(err, "failed to set the USB Mux direction to the Host")
	}

	// Test element required if rebooting from recovery to anything
	if err := h.Servo.WatchdogRemove(ctx, servo.WatchdogCCD); err != nil {
		errors.Wrap(err, "failed to remove watchdog for ccd")
	}

	if err := h.Servo.SetPowerState(ctx, servo.PowerStateWarmReset); err != nil {
		errors.Wrap(err, "failed to warm reset DUT")
	}

	if err := h.RequireServo(ctx); err != nil {
		errors.Wrap(err, "failed to init servo")
	}

	if err := h.CloseRPCConnection(ctx); err != nil {
		errors.Wrap(err, "failed to close RPC connection")
	}

	// Recovery mode requires the DUT to boot the image on the USB.
	// Thus, the servo must show the USB to the DUT.
	if err := ms.EnableRecMode(ctx, servo.USBMuxDUT); err != nil {
		errors.Wrap(err, "failed to enable recovery mode")
	}

	connectCtx, cancel := context.WithTimeout(ctx, 10*time.Minute)
	defer cancel()
	if err := h.WaitConnect(connectCtx); err != nil {
		errors.Wrap(err, "failed to reconnect to DUT after booting to recovery mode")
	}

	if isRecovery, err := h.Reporter.CheckBootMode(ctx, fwCommon.BootModeRecovery); err != nil {
		errors.Wrap(err, "failed to check a boot mode")
	} else if !isRecovery {
		errors.New("failed to boot into the recovery mode")
	}

	if containsRecReason, err := h.Reporter.ContainsRecoveryReason(ctx, expReasons); err != nil || !containsRecReason {
		errors.Wrap(err, "failed to get expected recovery reason")
	}

	return nil
}

// CheckCrossystemWPSW returns an error if crossystem wpsw_cur value does not match expectedWPSW
func CheckCrossystemWPSW(ctx context.Context, h *firmware.Helper, expectedWPSW int) error {
	r := reporters.New(h.DUT)
	testing.ContextLog(ctx, "Check crossystem for write protect state param")
	strWPSW, err := r.CrossystemParam(ctx, reporters.CrossystemParamWpswCur)
	if err != nil {
		return errors.Wrapf(err, "failed to get crossystem %v value", reporters.CrossystemParamWpswCur)
	}
	currWPSW, err := strconv.Atoi(strWPSW)
	if err != nil {
		return errors.Wrap(err, "failed to convert crossystem wpsw value to integer value")
	}
	testing.ContextLogf(ctx, "Current write protect state: %v, Expected state: %v", currWPSW, expectedWPSW)
	if currWPSW != expectedWPSW {
		return errors.Errorf("expected WP state to %v, is actually %v", expectedWPSW, currWPSW)
	}
	return nil
}

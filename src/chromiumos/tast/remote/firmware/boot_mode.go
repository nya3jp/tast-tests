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

	fwCommon "chromiumos/tast/common/firmware"
	"chromiumos/tast/common/servo"
	"chromiumos/tast/errors"
	fwpb "chromiumos/tast/services/cros/firmware"
	"chromiumos/tast/ssh"
	"chromiumos/tast/testing"
)

const (
	// cmdTimeout is a short duration used for sending commands.
	cmdTimeout = 6 * time.Second

	// offTimeout is the timeout to wait for the DUT to be unreachable after powering off.
	offTimeout = 3 * time.Minute

	// PowerStateTimeout is the timeout to wait for the DUT reach a powerstate.
	PowerStateTimeout = 20 * time.Second

	// PowerStateInterval is the interval to wait before polling DUT powerstate.
	PowerStateInterval = 250 * time.Millisecond

	// reconnectTimeout is the timeout to wait to reconnect to the DUT after rebooting.
	reconnectTimeout = 3 * time.Minute

	// usbVisibleTime is the time to wait after making the USB stick visible to DUT
	usbVisibleTime = 5 * time.Second
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

// ModeSwitchOption allows mode-switching methods to exhibit different behaviors.
type ModeSwitchOption int

const (
	// AllowGBBForce allows the DUT to force rebooting into dev mode via GBB flags.
	// This way of switching is more reliable, but is not appropriate for all tests.
	AllowGBBForce ModeSwitchOption = iota

	// AssumeGBBFlagsCorrect skips setting the GBB flags when switching modes.
	// This can save some time if the GBB flags are known to be in the desired state.
	AssumeGBBFlagsCorrect ModeSwitchOption = iota

	// CopyTastFiles copies the Tast files from the DUT before rebooting, and writes them back to the DUT afterwards.
	// This is necessary if you want to use any gRPC services.
	CopyTastFiles ModeSwitchOption = iota

	// SkipModeCheckAfterReboot can be passed in as an option to ModeAwareReboot, skipping
	// boot mode check after resetting DUT. One instance where this can be useful is
	// when verifying that FWMP prevents DUT from booting into dev mode.
	SkipModeCheckAfterReboot ModeSwitchOption = iota

	// PressEnterAtToNorm presses ENTER to allow DUT to continue to boot when dev mode disabled by FWMP.
	PressEnterAtToNorm ModeSwitchOption = iota
)

// msOptsContain determines whether a slice of ModeSwitchOptions contains a specific Option.
func msOptsContain(opts []ModeSwitchOption, want ModeSwitchOption) bool {
	for _, o := range opts {
		if o == want {
			return true
		}
	}
	return false
}

// RebootToMode reboots the DUT into the specified boot mode.
// This has the side-effect of disconnecting the RPC client.
// Requires `SoftwareDeps: []string{"crossystem", "flashrom"},`.
func (ms ModeSwitcher) RebootToMode(ctx context.Context, toMode fwCommon.BootMode, opts ...ModeSwitchOption) error {
	h := ms.Helper
	if err := h.RequireServo(ctx); err != nil {
		return errors.Wrap(err, "requiring servo")
	}

	fromMode, err := h.Reporter.CurrentBootMode(ctx)
	if err != nil {
		return errors.Wrap(err, "determining boot mode at the start of RebootToMode")
	}

	// Unless AssumeGBBFlagsCorrect is passed, fix the GBB flags for the desired boot mode.
	if !msOptsContain(opts, AssumeGBBFlagsCorrect) {
		flags := fwpb.GBBFlagsState{}
		if msOptsContain(opts, AllowGBBForce) {
			switch toMode {
			case fwCommon.BootModeDev:
				flags.Clear = append(flags.Clear, fwpb.GBBFlag_FORCE_DEV_BOOT_USB)
				flags.Set = append(flags.Set, fwpb.GBBFlag_FORCE_DEV_SWITCH_ON, fwpb.GBBFlag_DEV_SCREEN_SHORT_DELAY)
			case fwCommon.BootModeUSBDev:
				flags.Set = append(flags.Set, fwpb.GBBFlag_FORCE_DEV_BOOT_USB, fwpb.GBBFlag_FORCE_DEV_SWITCH_ON, fwpb.GBBFlag_DEV_SCREEN_SHORT_DELAY)
			default:
				flags.Clear = append(flags.Clear, fwpb.GBBFlag_FORCE_DEV_SWITCH_ON, fwpb.GBBFlag_DEV_SCREEN_SHORT_DELAY, fwpb.GBBFlag_FORCE_DEV_BOOT_USB)
			}
		} else {
			flags.Clear = append(flags.Clear, fwpb.GBBFlag_FORCE_DEV_SWITCH_ON, fwpb.GBBFlag_DEV_SCREEN_SHORT_DELAY, fwpb.GBBFlag_FORCE_DEV_BOOT_USB)
		}
		if err := fwCommon.ClearAndSetGBBFlags(ctx, h.DUT, &flags); err != nil {
			return errors.Wrap(err, "setting GBB flags")
		}
	}

	// When booting to a different image, such as normal vs. recovery, the new image might
	// not have Tast host files installed. So, store those files on the test server and reinstall later.
	fromModeUsb := false
	toModeUsb := false
	if fromMode == fwCommon.BootModeRecovery || fromMode == fwCommon.BootModeUSBDev {
		fromModeUsb = true
	}
	if toMode == fwCommon.BootModeRecovery || toMode == fwCommon.BootModeUSBDev {
		toModeUsb = true
	}

	if fromModeUsb != toModeUsb && !h.DoesServerHaveTastHostFiles() && msOptsContain(opts, CopyTastFiles) {
		if err := h.CopyTastFilesFromDUT(ctx); err != nil {
			return errors.Wrap(err, "copying Tast files from DUT to test server")
		}
		// Remember which image the Tast files came from.
		if fromModeUsb {
			h.dutUsbHasTastFiles = true
		} else {
			h.dutInternalStorageHasTastFiles = true
		}
	}

	// Perform sync prior to reboot, then close the RPC connection.
	if err := h.DUT.Conn().CommandContext(ctx, "sync").Run(ssh.DumpLogOnError); err != nil {
		testing.ContextLogf(ctx, "Failed to sync DUT: %s", err)
	}
	h.CloseRPCConnection(ctx)

	// Booting from rec to anything else will cause EC to restart, potentally breaking the servo watchdog.
	if fromMode == fwCommon.BootModeRecovery {
		if err := h.Servo.WatchdogRemove(ctx, servo.WatchdogCCD); err != nil {
			return errors.Wrap(err, "failed to remove watchdog for ccd")
		}
	}

	switch toMode {
	case fwCommon.BootModeNormal:
		hasSerialAP := false
		if fromMode != fwCommon.BootModeNormal {
			hasSerialAP = ms.hasSerialAPFirmware(ctx)
		}
		if err := ms.PowerOff(ctx); err != nil {
			return errors.Wrap(err, "powering off DUT")
		}
		if err := h.Servo.SetUSBMuxState(ctx, servo.USBMuxOff); err != nil {
			return errors.Wrap(err, "disable usb for normal")
		}
		if err := h.Servo.SetPowerState(ctx, servo.PowerStateOn); err != nil {
			return err
		}
		if fromMode != fwCommon.BootModeNormal {
			if err := ms.FwScreenToNormalMode(ctx, hasSerialAP, true); err != nil {
				return errors.Wrap(err, "moving from firmware screen to normal mode")
			}
		}
		// Reconnect to the DUT.
		testing.ContextLog(ctx, "Reestablishing connection to DUT")
		connectCtx, cancel := context.WithTimeout(ctx, reconnectTimeout)
		defer cancel()
		if err := h.WaitConnect(connectCtx); err != nil {
			return errors.Wrapf(err, "failed to reconnect to DUT after booting to %s", toMode)
		}
	case fwCommon.BootModeRecovery:
		// Recovery mode requires the DUT to boot the image on the USB.
		// Thus, the servo must show the USB to the DUT.
		if err := ms.enableRecMode(ctx, servo.USBMuxDUT); err != nil {
			return err
		}
		// Reconnect to the DUT.
		testing.ContextLog(ctx, "Reestablishing connection to DUT")
		connectCtx, cancel := context.WithTimeout(ctx, reconnectTimeout)
		defer cancel()
		if err := h.WaitConnect(connectCtx); err != nil {
			return errors.Wrapf(err, "failed to reconnect to DUT after booting to %s", toMode)
		}
	case fwCommon.BootModeDev:
		testing.ContextLog(ctx, "Disabling dev_boot_usb, disabling dev_boot_signed_only")
		if err := h.DUT.Conn().CommandContext(ctx, "crossystem", "dev_boot_usb=0", "dev_boot_signed_only=0", "dev_default_boot=disk").Run(ssh.DumpLogOnError); err != nil {
			return errors.Wrap(err, "disabling dev_boot_usb")
		}
		if msOptsContain(opts, AllowGBBForce) {
			// 1. Set the GBB flag which forces dev mode upon reboot.
			//    This was handled earlier in this function, prior to terminating the RPC connection.
			// 2. Reboot the DUT.
			if err := h.DUT.Reboot(ctx); err != nil {
				return errors.Wrap(err, "rebooting DUT to force dev mode via GBB")
			}
			break
		}
		hasSerialAP := ms.hasSerialAPFirmware(ctx)
		transitionToDev := true
		// Recovery -> Dev sometimes gets stuck on the recovery screen. Try a normal reboot first.
		// Even if it doesn't get us back to Dev, rebooting from Normal -> Dev is less flaky.
		if fromMode == fwCommon.BootModeRecovery || fromMode == fwCommon.BootModeUSBDev {
			if err := h.Servo.SetPowerState(ctx, servo.PowerStateWarmReset); err != nil {
				return err
			}
			// Depending on how we got to to dev mode, we might end up in normal mode or the recovery
			// menu, so navigate to dev mode, but it that fails, fall through to the next attempt below.
			if err := ms.FwScreenToDevMode(ctx, hasSerialAP, true); err == nil {
				newMode, err := h.Reporter.CurrentBootMode(ctx)
				if err != nil {
					return errors.Wrap(err, "determining boot mode after simple reboot")
				}
				testing.ContextLogf(ctx, "Warm reset finished, DUT in %s", newMode)
				transitionToDev = newMode != fwCommon.BootModeDev
			}
		}
		if transitionToDev {
			// 1. Set power_state to 'rec', but don't show the DUT a USB image to boot from.
			// 2. From the firmware screen that appears, press keys to transition to dev mode.
			//    The specific keypresses will depend on the DUT's ModeSwitcherType.
			if err := ms.enableRecMode(ctx, servo.USBMuxOff); err != nil {
				return err
			}
			if err := ms.FwScreenToDevMode(ctx, hasSerialAP, true); err != nil {
				return errors.Wrap(err, "moving from firmware screen to dev mode")
			}
		} else {
			// Reconnect to the DUT.
			testing.ContextLog(ctx, "Reestablishing connection to DUT")
			connectCtx, cancel := context.WithTimeout(ctx, reconnectTimeout)
			defer cancel()
			if err := h.WaitConnect(connectCtx); err != nil {
				return errors.Wrapf(err, "failed to reconnect to DUT after booting to %s", toMode)
			}
		}
	case fwCommon.BootModeUSBDev:
		transitionToDev := true
		transitionToDevUsb := true
		if msOptsContain(opts, AllowGBBForce) {
			transitionToDev = false
		}
		hasSerialAP := ms.hasSerialAPFirmware(ctx)
		// Recovery -> Dev sometimes gets stuck on the recovery screen. Try a normal reboot first.
		// Even if it doesn't get us back to Dev, rebooting from Normal -> Dev is less flaky.
		if fromMode == fwCommon.BootModeRecovery {
			testing.ContextLog(ctx, "Rebooting to leave recovery mode")
			if err := h.Servo.SetPowerState(ctx, servo.PowerStateWarmReset); err != nil {
				return err
			}
			// Depending on how we got to to rec mode, we might end up in normal mode or the recovery
			// menu, so navigate to dev mode, but it that fails, fall through to the next attempt below.
			if err := ms.FwScreenToDevMode(ctx, hasSerialAP, true); err == nil {
				newMode, err := h.Reporter.CurrentBootMode(ctx)
				if err != nil {
					return errors.Wrap(err, "determining boot mode after simple reboot")
				}
				testing.ContextLogf(ctx, "Warm reset finished, DUT in %s", newMode)
				switch newMode {
				case fwCommon.BootModeDev:
					transitionToDev = false
				case fwCommon.BootModeUSBDev:
					transitionToDev = false
					transitionToDevUsb = false
				}
			}
		}
		if transitionToDev {
			// 1. Set power_state to 'rec', but don't show the DUT a USB image to boot from.
			// 2. From the firmware screen that appears, press keys to transition to dev mode.
			//    The specific keypresses will depend on the DUT's ModeSwitcherType.
			testing.ContextLog(ctx, "Rebooting to enter dev mode first")
			if err := ms.enableRecMode(ctx, servo.USBMuxOff); err != nil {
				return err
			}
			if err := ms.FwScreenToDevMode(ctx, hasSerialAP, true); err != nil {
				return errors.Wrap(err, "moving from firmware screen to dev mode")
			}
			newMode, err := h.Reporter.CurrentBootMode(ctx)
			if err != nil {
				return errors.Wrap(err, "determining boot mode after reboot to dev")
			}
			testing.ContextLogf(ctx, "Reboot to dev finished, DUT in %s", newMode)

		}
		if transitionToDevUsb {
			// 1. Set power_state to 'rec', but don't show the DUT a USB image to boot from.
			// 2. From the firmware screen that appears, press keys to transition to dev mode.
			//    The specific keypresses will depend on the DUT's ModeSwitcherType.
			testing.ContextLog(ctx, "Enabling dev_boot_usb, disabling dev_boot_signed_only")
			if err := h.DUT.Conn().CommandContext(ctx, "crossystem", "dev_boot_usb=1", "dev_boot_signed_only=0", "dev_default_boot=usb").Run(ssh.DumpLogOnError); err != nil {
				return errors.Wrap(err, "enabling dev_boot_usb")
			}
			testing.ContextLog(ctx, "Enabling USB")
			if err := h.Servo.SetUSBMuxState(ctx, servo.USBMuxDUT); err != nil {
				return err
			}
			testing.ContextLogf(ctx, "Sleeping %s to let USB become visible to DUT", usbVisibleTime)
			if err := testing.Sleep(ctx, usbVisibleTime); err != nil {
				return err
			}
			testing.ContextLog(ctx, "Rebooting")
			powerOffCtx, cancel := context.WithTimeout(ctx, cmdTimeout)
			defer cancel()
			// Since the DUT will power off, deadline exceeded is expected here.
			if err := h.DUT.Conn().CommandContext(powerOffCtx, "reboot").Run(); err != nil && !errors.Is(err, context.DeadlineExceeded) {
				return errors.Wrapf(err, "DUT poweroff %T", err)
			}

			offCtx, cancel := context.WithTimeout(ctx, offTimeout)
			defer cancel()
			if err := ms.Helper.DUT.WaitUnreachable(offCtx); err != nil {
				return errors.Wrap(err, "waiting for DUT to be unreachable after reboot")
			}
			if err := ms.fwScreenToUSBDevMode(ctx); err != nil {
				return errors.Wrap(err, "moving from firmware screen to usb dev mode")
			}
		}
		// Reconnect to the DUT.
		testing.ContextLog(ctx, "Reestablishing connection to DUT")
		connectCtx, cancel := context.WithTimeout(ctx, reconnectTimeout)
		defer cancel()
		if err := h.WaitConnect(connectCtx); err != nil {
			return errors.Wrapf(err, "failed to reconnect to DUT after booting to %s", toMode)
		}
	default:
		return errors.Errorf("unsupported firmware boot mode: %s", toMode)
	}

	// Send Tast files back to DUT.
	needSync := (toModeUsb != fromModeUsb) && msOptsContain(opts, CopyTastFiles)
	if toModeUsb {
		needSync = needSync && !h.dutUsbHasTastFiles
	} else {
		needSync = needSync && !h.dutInternalStorageHasTastFiles
	}
	if needSync {
		if err := h.SyncTastFilesToDUT(ctx); err != nil {
			return errors.Wrapf(err, "syncing Tast files to DUT after booting to %s", toMode)
		}
		if toModeUsb {
			h.dutUsbHasTastFiles = true
		} else {
			h.dutInternalStorageHasTastFiles = true
		}
	}

	// Verify successful reboot.
	if curr, err := h.Reporter.CurrentBootMode(ctx); err != nil {
		return errors.Wrapf(err, "checking boot mode after reboot to %s", toMode)
	} else if curr != toMode {
		return errors.Errorf("incorrect boot mode after RebootToMode: got %s; want %s", curr, toMode)
	}
	testing.ContextLogf(ctx, "DUT is now in %s mode", toMode)
	return nil
}

// ResetType is an enum of ways to reset a DUT: warm and cold.
type ResetType string

// There are two ResetTypes: warm and cold.
const (
	// WarmReset uses the Servo control power_state=warm_reset.
	WarmReset ResetType = "warm"

	// ColdReset uses the Servo control power_state=reset.
	// It is identical to setting the power_state to off, then on.
	// It also resets the EC, as by the 'cold_reset' signal.
	ColdReset ResetType = "cold"
)

// Each ResetType is associated with a particular servo.PowerStateValue.
var resetTypePowerState = map[ResetType]servo.PowerStateValue{
	WarmReset: servo.PowerStateWarmReset,
	ColdReset: servo.PowerStateReset,
}

// ModeAwareReboot resets the DUT with awareness of the DUT boot mode.
// Dev mode will be retained, but rec mode will default back to normal mode.
// This has the side-effect of disconnecting the RPC connection.
func (ms *ModeSwitcher) ModeAwareReboot(ctx context.Context, resetType ResetType, opts ...ModeSwitchOption) error {
	h := ms.Helper
	if err := h.RequireServo(ctx); err != nil {
		return errors.Wrap(err, "requiring servo")
	}

	fromMode, err := h.Reporter.CurrentBootMode(ctx)
	if err != nil {
		return errors.Wrap(err, "determining boot mode at the start of ModeAwareReboot")
	}

	// Memorize the boot ID, so that we can compare later.
	origBootID, err := h.Reporter.BootID(ctx)
	if err != nil {
		return errors.Wrap(err, "determining boot ID before reboot")
	}

	hasSerialAP := false
	if fromMode == fwCommon.BootModeDev {
		hasSerialAP = ms.hasSerialAPFirmware(ctx)
	}
	// Perform sync prior to reboot, then close the RPC connection.
	if err := h.DUT.Conn().CommandContext(ctx, "sync").Run(ssh.DumpLogOnError); err != nil {
		testing.ContextLogf(ctx, "Failed to sync DUT: %s", err)
	}
	h.CloseRPCConnection(ctx)

	if fromMode == fwCommon.BootModeUSBDev {
		// The USB stick should already be visible, but set the direction just to be sure.
		testing.ContextLog(ctx, "Enabling USB")
		if err := h.Servo.SetUSBMuxState(ctx, servo.USBMuxDUT); err != nil {
			return err
		}
		testing.ContextLogf(ctx, "Sleeping %s to let USB become visible to DUT", usbVisibleTime)
		if err := testing.Sleep(ctx, usbVisibleTime); err != nil {
			return err
		}
	}

	// Reset DUT, and wait for it to be unreachable.
	powerState, ok := resetTypePowerState[resetType]
	if !ok {
		return errors.Errorf("no power state associated with resetType %v", resetType)
	}
	if err := h.Servo.SetPowerState(ctx, powerState); err != nil {
		return err
	}

	// If in dev mode, bypass the TO_DEV screen.
	if fromMode == fwCommon.BootModeDev {
		if err := ms.FwScreenToDevMode(ctx, hasSerialAP, true, opts...); err != nil {
			return errors.Wrap(err, "bypassing fw screen")
		}
	} else if fromMode == fwCommon.BootModeUSBDev {
		if err := ms.fwScreenToUSBDevMode(ctx); err != nil {
			return errors.Wrap(err, "bypassing fw screen")
		}
	} else {
		testing.ContextLog(ctx, "Reestablishing connection to DUT")
		connectCtx, cancel := context.WithTimeout(ctx, reconnectTimeout)
		defer cancel()
		if err := h.WaitConnect(connectCtx); err != nil {
			return errors.Wrap(err, "failed to connect to DUT")
		}
	}
	if bootID, err := h.Reporter.BootID(ctx); err != nil {
		return errors.Wrap(err, "reporting boot ID")
	} else if bootID == origBootID {
		return errors.Errorf("new boot ID == old boot ID: %s", bootID)
	}

	// Verify successful reboot.
	// Dev mode should be preserved, but recovery mode will be lost in the reset.
	var expectMode fwCommon.BootMode
	if fromMode == fwCommon.BootModeRecovery {
		expectMode = fwCommon.BootModeNormal
	} else {
		expectMode = fromMode
	}
	if curr, err := h.Reporter.CurrentBootMode(ctx); err != nil {
		return errors.Wrapf(err, "checking boot mode after resetting from %s", fromMode)
	} else if curr != expectMode && !msOptsContain(opts, SkipModeCheckAfterReboot) {
		return errors.Errorf("incorrect boot mode after resetting DUT: got %s; want %s", curr, expectMode)
	}
	return nil
}

// FwScreenToNormalMode moves the DUT from the firmware bootup screen to Normal mode.
// This should be called immediately after powering on.
// The actual behavior depends on the ModeSwitcherType.
func (ms *ModeSwitcher) FwScreenToNormalMode(ctx context.Context, hasSerialAP, waitForFwScreen bool) error {
	h := ms.Helper
	if err := h.RequireServo(ctx); err != nil {
		return errors.Wrap(err, "requiring servo")
	}
	if waitForFwScreen {
		testing.ContextLogf(ctx, "Sleeping %s (FirmwareScreen)", h.Config.FirmwareScreen)
		if err := testing.Sleep(ctx, h.Config.FirmwareScreen); err != nil {
			return errors.Wrapf(err, "sleeping for %s (FirmwareScreen) to wait for INSERT screen", h.Config.FirmwareScreen)
		}
	}
	if hasSerialAP {
		testing.ContextLogf(ctx, "Sleeping %s (SerialFirmwareBootDelay)", h.Config.SerialFirmwareBootDelay)
		if err := testing.Sleep(ctx, h.Config.SerialFirmwareBootDelay); err != nil {
			return errors.Wrapf(err, "sleeping for %s (SerialFirmwareBootDelay) while enabling dev mode", h.Config.SerialFirmwareBootDelay)
		}
	}
	switch h.Config.ModeSwitcherType {
	case KeyboardDevSwitcher:
		// 1. Sleep for [FirmwareScreen] seconds.
		// 2. Press enter.
		// 3. Sleep for [KeypressDelay] seconds.
		// 4. Press enter.
		testing.ContextLog(ctx, "Pressing ENTER")
		if err := h.Servo.KeypressWithDuration(ctx, servo.Enter, servo.DurTab); err != nil {
			return errors.Wrap(err, "pressing Enter on firmware screen while disabling dev mode")
		}
		testing.ContextLogf(ctx, "Sleeping %s (KeypressDelay)", h.Config.KeypressDelay)
		if err := testing.Sleep(ctx, h.Config.KeypressDelay); err != nil {
			return errors.Wrapf(err, "sleeping for %s (KeypressDelay) while disabling dev mode", h.Config.KeypressDelay)
		}
		testing.ContextLog(ctx, "Pressing ENTER")
		if err := h.Servo.KeypressWithDuration(ctx, servo.Enter, servo.DurTab); err != nil {
			return errors.Wrap(err, "pressing Enter on confirm screen while disabling dev mode")
		}
	case MenuSwitcher:
		// 1. Sleep for [FirmwareScreen] seconds.
		// 2. Press Ctrl+S.
		// 3. Sleep for [KeypressDelay] seconds.
		// 4. Press enter.
		testing.ContextLog(ctx, "Pressing Ctrl-S")
		if err := h.Servo.KeypressWithDuration(ctx, servo.CtrlS, servo.DurTab); err != nil {
			return errors.Wrap(err, "pressing Ctrl-S on firmware screen while disabling dev mode")
		}
		testing.ContextLogf(ctx, "Sleeping %s (KeypressDelay)", h.Config.KeypressDelay)
		if err := testing.Sleep(ctx, h.Config.KeypressDelay); err != nil {
			return errors.Wrapf(err, "sleeping for %s (KeypressDelay) while disabling dev mode", h.Config.KeypressDelay)
		}
		testing.ContextLog(ctx, "Pressing ENTER")
		if err := h.Servo.KeypressWithDuration(ctx, servo.Enter, servo.DurTab); err != nil {
			return errors.Wrap(err, "pressing Enter on confirm screen while disabling dev mode")
		}
	case TabletDetachableSwitcher:
		// 1. Wait until the firmware screen appears.
		// 2. Hold volume_up for 100ms to highlight the previous menu item (Enable Root Verification).
		// 3. Sleep for [KeypressDelay] seconds to confirm keypress.
		// 4. Press power to select Enable Root Verification.
		// 5. Sleep for [KeypressDelay] seconds to confirm keypress.
		// 6. Wait until the TO_NORM screen appears.
		// 7. Press power to select Confirm Enabling Verified Boot.
		if err := h.Servo.SetInt(ctx, servo.VolumeUpHold, 100); err != nil {
			return errors.Wrap(err, "changing menu selection to 'Enable Root Verification'")
		}
		if err := testing.Sleep(ctx, h.Config.KeypressDelay); err != nil {
			return errors.Wrapf(err, "sleeping for %s (KeypressDelay) while disabling dev mode", h.Config.KeypressDelay)
		}
		if err := h.Servo.KeypressWithDuration(ctx, servo.PowerKey, servo.DurTab); err != nil {
			return errors.Wrap(err, "selecting menu option 'Enable Root Verification'")
		}
		if err := testing.Sleep(ctx, h.Config.KeypressDelay); err != nil {
			return errors.Wrapf(err, "sleeping for %s (KeypressDelay) while disabling dev mode", h.Config.KeypressDelay)
		}
		if err := h.Servo.KeypressWithDuration(ctx, servo.PowerKey, servo.DurTab); err != nil {
			return errors.Wrap(err, "selecting menu option 'Confirm Enabling Verified Boot'")
		}
	default:
		return errors.Errorf("unsupported ModeSwitcherType %s for FwScreenToNormalMode", h.Config.ModeSwitcherType)
	}
	return nil
}

// FwScreenToDevMode moves the DUT from the firmware bootup screen to Dev mode.
// This should be called immediately after powering on.
// The actual behavior depends on the ModeSwitcherType.
func (ms *ModeSwitcher) FwScreenToDevMode(ctx context.Context, hasSerialAP, waitForFwScreen bool, opts ...ModeSwitchOption) error {
	h := ms.Helper
	if err := h.RequireServo(ctx); err != nil {
		return errors.Wrap(err, "requiring servo")
	}

	if waitForFwScreen {
		if err := testing.Sleep(ctx, h.Config.FirmwareScreen); err != nil {
			return errors.Wrapf(err, "sleeping for %s (FirmwareScreen) to wait for INSERT screen", h.Config.FirmwareScreen)
		}
	}
	if hasSerialAP {
		testing.ContextLogf(ctx, "Sleeping %s (SerialFirmwareBootDelay)", h.Config.SerialFirmwareBootDelay)
		if err := testing.Sleep(ctx, h.Config.SerialFirmwareBootDelay); err != nil {
			return errors.Wrapf(err, "sleeping for %s (SerialFirmwareBootDelay) while enabling dev mode", h.Config.SerialFirmwareBootDelay)
		}
	}
	switch h.Config.ModeSwitcherType {
	case MenuSwitcher:
		// Same as KeyboardDevSwitcher.
		fallthrough
	case KeyboardDevSwitcher:
		// 1. Wait until the firmware screen appears.
		// 2. Press Ctrl-D to move to the confirm screen.
		// 3. Wait until the confirm screen appears.
		// 4. Push some button depending on the DUT's config: toggle the rec button, press power, or press enter.
		testing.ContextLog(ctx, "Pressing Ctrl-D")
		if err := h.Servo.KeypressWithDuration(ctx, servo.CtrlD, servo.DurTab); err != nil {
			return err
		}
		testing.ContextLogf(ctx, "Sleeping %s (KeypressDelay)", h.Config.KeypressDelay)
		if err := testing.Sleep(ctx, h.Config.KeypressDelay); err != nil {
			return err
		}
		if h.Config.RecButtonDevSwitch {
			testing.ContextLog(ctx, "Toggling RecMode")
			if err := h.Servo.ToggleOnOff(ctx, servo.RecMode); err != nil {
				return err
			}
		} else if h.Config.PowerButtonDevSwitch {
			testing.ContextLog(ctx, "Pressing power key")
			if err := h.Servo.KeypressWithDuration(ctx, servo.PowerKey, servo.DurPress); err != nil {
				return err
			}
		} else {
			testing.ContextLog(ctx, "Pressing enter key")
			if err := h.Servo.KeypressWithDuration(ctx, servo.Enter, servo.DurTab); err != nil {
				return err
			}
		}
		testing.ContextLogf(ctx, "Sleeping %s", 2*time.Second)
		if err := testing.Sleep(ctx, 2*time.Second); err != nil {
			return err
		}
		testing.ContextLog(ctx, "Set DFP mode")
		if err := h.Servo.SetDUTPDDataRole(ctx, servo.DFP); err != nil {
			testing.ContextLogf(ctx, "Failed to set pd data role to DFP: %s", err)
		}
		pressingKeyTillConnected := func(key string, connectTimeout time.Duration) error {
			testing.ContextLogf(ctx, "Pressing %s", key)
			switch key {
			case "CTRL-D":
				if err := h.Servo.KeypressWithDuration(ctx, servo.CtrlD, servo.DurTab); err != nil {
					return err
				}
			case "ENTER":
				if err := h.Servo.KeypressWithDuration(ctx, servo.Enter, servo.DurTab); err != nil {
					return err
				}
			}
			ctx, cancel := context.WithTimeout(ctx, connectTimeout)
			defer cancel()
			connectTimeout += time.Second
			return h.DUT.WaitConnect(ctx)
		}
		connectTimeout := 2 * time.Second
		if err := testing.Poll(ctx, func(ctx context.Context) error {
			// Keep pressing CTRL-D until connected, but wait a little longer for the connect each time.
			if err := pressingKeyTillConnected("CTRL-D", connectTimeout); err != nil {
				return err
			}
			return nil
		}, &testing.PollOptions{Timeout: reconnectTimeout}); err != nil && !msOptsContain(opts, PressEnterAtToNorm) {
			return errors.Wrap(err, "failed to reconnect to DUT")
		} else if err != nil && msOptsContain(opts, PressEnterAtToNorm) {
			// DUTs would boot into the to_norm screen if dev mode was disabled by FWMP.
			// Keep pressing ENTER to bypass the to_norm screen, until connection is established.
			if err := testing.Poll(ctx, func(ctx context.Context) error {
				if err := pressingKeyTillConnected("ENTER", connectTimeout); err != nil {
					return err
				}
				return nil
			}, &testing.PollOptions{Timeout: reconnectTimeout}); err != nil {
				return errors.Wrap(err, "failed to reconnect to DUT")
			}
		}
	case TabletDetachableSwitcher:
		// 1. Wait [FirmwareScreen] seconds for the INSERT screen to appear.
		// 2. Hold both VolumeUp and VolumeDown for 100ms to trigger TO_DEV screen.
		// 3. Wait [KeypressDelay] seconds to confirm keypress.
		// 4. Hold VolumeUp for 100ms to change menu selection to 'Confirm enabling developer mode'.
		// 5. Wait [KeypressDelay] seconds to confirm keypress.
		// 6. Press PowerKey to select menu item.
		// 7. Wait [KeypressDelay] seconds to confirm keypress.
		// 8. Wait [FirmwareScreen] seconds to transition screens.
		if err := h.Servo.SetInt(ctx, servo.VolumeUpDownHold, 100); err != nil {
			return errors.Wrap(err, "triggering TO_DEV screen")
		}
		if err := testing.Sleep(ctx, h.Config.KeypressDelay); err != nil {
			return errors.Wrapf(err, "sleeping for %s (KeypressDelay) to confirm triggering TO_DEV screen", h.Config.KeypressDelay)
		}
		if err := h.Servo.SetInt(ctx, servo.VolumeUpHold, 100); err != nil {
			return errors.Wrap(err, "changing menu selection to 'Confirm enabling developer mode' on TO_DEV screen")
		}
		if err := testing.Sleep(ctx, h.Config.KeypressDelay); err != nil {
			return errors.Wrapf(err, "sleeping for %s (KeypressDelay) to confirm changing menu selection on TO_DEV screen", h.Config.KeypressDelay)
		}
		if err := h.Servo.KeypressWithDuration(ctx, servo.PowerKey, servo.DurTab); err != nil {
			return errors.Wrap(err, "selecting menu item 'Confirm enabling developer mode' on TO_DEV screen")
		}
		// Reconnect to the DUT.
		connectCtx, cancel := context.WithTimeout(ctx, reconnectTimeout)
		defer cancel()
		if err := h.WaitConnect(connectCtx); err != nil {
			return errors.Wrap(err, "failed to reconnect to DUT")
		}
	default:
		return errors.Errorf("booting to dev mode: unsupported ModeSwitcherType: %s", h.Config.ModeSwitcherType)
	}
	return nil
}

// fwScreenToUSBDevMode moves the DUT from the firmware bootup screen to USB Dev mode.
// This should be called immediately after powering on.
// The actual behavior depends on the ModeSwitcherType.
func (ms *ModeSwitcher) fwScreenToUSBDevMode(ctx context.Context) error {
	h := ms.Helper
	if err := h.RequireServo(ctx); err != nil {
		return errors.Wrap(err, "requiring servo")
	}

	switch h.Config.ModeSwitcherType {
	case MenuSwitcher:
		// Same as KeyboardDevSwitcher.
		fallthrough
	case KeyboardDevSwitcher:
		// 1. Wait until the firmware screen appears.
		// 2. Press Ctrl-U to move to the confirm screen.
		// 3. Wait until the confirm screen appears.
		// 4. Push some button depending on the DUT's config: toggle the rec button, press power, or press enter.
		testing.ContextLog(ctx, "Set DFP mode")
		if err := h.Servo.SetDUTPDDataRole(ctx, servo.DFP); err != nil {
			testing.ContextLogf(ctx, "Failed to set pd data role to DFP: %s", err)
		}
		// Keep pressing CTRL-U until connected, but wait a little longer for the connect each time.
		connectTimeout := 2 * time.Second
		if err := testing.Poll(ctx, func(ctx context.Context) error {
			testing.ContextLog(ctx, "Pressing CTRL-U")
			if err := h.Servo.KeypressWithDuration(ctx, servo.CtrlU, servo.DurTab); err != nil {
				return err
			}
			ctx, cancel := context.WithTimeout(ctx, connectTimeout)
			defer cancel()
			connectTimeout += time.Second
			return h.DUT.WaitConnect(ctx)
		}, &testing.PollOptions{Timeout: reconnectTimeout}); err != nil {
			return errors.Wrap(err, "failed to reconnect to DUT")
		}
	default:
		return errors.Errorf("booting to dev mode: unsupported ModeSwitcherType: %s", h.Config.ModeSwitcherType)
	}

	return nil
}

// enableRecMode powers the DUT into the "rec" state, but does not wait to reconnect to the DUT.
// If booting into rec mode, usbMux should point to the DUT, so that the DUT can finish booting into recovery mode.
// Otherwise, usbMux should be off. This will prevent the DUT from transitioning to rec mode, so other operations can be performed (such as bypassing to dev mode).
func (ms *ModeSwitcher) enableRecMode(ctx context.Context, usbMux servo.USBMuxState) error {
	h := ms.Helper
	if err := h.RequireServo(ctx); err != nil {
		return errors.Wrap(err, "requiring servo")
	}
	if err := ms.PowerOff(ctx); err != nil {
		return errors.Wrap(err, "powering off DUT")
	}
	if err := h.Servo.SetUSBMuxState(ctx, usbMux); err != nil {
		return errors.Wrapf(err, "setting usb mux state to %s while DUT is off", usbMux)
	}
	if err := h.Servo.SetPowerState(ctx, servo.PowerStateRec); err != nil {
		return errors.Wrapf(err, "setting power state to %s", servo.PowerStateRec)
	}
	return nil
}

// PowerOff safely powers off the DUT with the "poweroff" command, then waits for the DUT to be unreachable.
func (ms *ModeSwitcher) PowerOff(ctx context.Context) error {
	h := ms.Helper
	if err := h.RequireServo(ctx); err != nil {
		return errors.Wrap(err, "requiring servo")
	}
	testing.ContextLog(ctx, "Powering off DUT")
	powerOffCtx, cancel := context.WithTimeout(ctx, cmdTimeout)
	defer cancel()
	// Since the DUT will power off, deadline exceeded is expected here.
	if err := h.DUT.Conn().CommandContext(powerOffCtx, "poweroff").Run(); err != nil && !errors.Is(err, context.DeadlineExceeded) {
		return errors.Wrapf(err, "DUT poweroff %T", err)
	}

	// Try reading the power state from the EC.
	err := h.WaitForPowerStates(ctx, PowerStateInterval, PowerStateTimeout, "G3", "S5")
	if err == nil {
		return nil
	}
	testing.ContextLogf(ctx, "Failed to get G3 or S5 power state: %s", err)

	// We didn't reach G3/S5 so try having servo power off instead.
	if err := h.Servo.SetPowerState(ctx, servo.PowerStateOff); err != nil {
		return errors.Wrap(err, "set power_state:off")
	}

	// If the EC didn't return a power state, try wait unreachable instead.
	if err := ms.waitUnreachable(ctx); err != nil {
		return errors.Wrap(err, "waiting for DUT to be unreachable after sending poweroff command")
	}
	return nil
}

func (ms *ModeSwitcher) waitUnreachable(ctx context.Context) error {
	offCtx, cancel := context.WithTimeout(ctx, offTimeout)
	defer cancel()
	if err := ms.Helper.DUT.WaitUnreachable(offCtx); err != nil {
		return errors.Wrap(err, "waiting for DUT to be unreachable after powering off")
	}
	return nil
}

func (ms *ModeSwitcher) hasSerialAPFirmware(ctx context.Context) bool {
	// TODO(b/206004543): Get this working. Reading CONFIG_CONSOLE_SERIAL doesn't work.
	return false
}

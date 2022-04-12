// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package firmware

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"time"

	"github.com/golang/protobuf/ptypes/empty"

	"chromiumos/tast/common/servo"
	"chromiumos/tast/errors"
	"chromiumos/tast/remote/firmware"
	"chromiumos/tast/remote/firmware/fixture"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

// Create enum to specify which tests need to be run
type wakeMethod int

const (
	wakeByPowerBtn wakeMethod = iota
	wakeByKeyboard
	wakeByLid
	wakeByUSBKeyboard
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         ECWakeSource,
		Desc:         "Test that DUT goes to G3 powerstate on shutdown",
		Contacts:     []string{"tij@google.com", "cros-fw-engprod@google.com"},
		Attr:         []string{"group:firmware", "firmware_unstable"},
		Fixture:      fixture.NormalMode,
		HardwareDeps: hwdep.D(hwdep.ChromeEC()),
		SoftwareDeps: []string{"chrome"},
		LacrosStatus: testing.LacrosVariantUnneeded,
		ServiceDeps:  []string{"tast.cros.firmware.UtilsService"},
		Timeout:      5 * time.Minute,
		Params: []testing.Param{
			{
				Name:              "power_btn",
				ExtraHardwareDeps: hwdep.D(hwdep.InternalDisplay()),
				Val:               wakeByPowerBtn,
			},
			{
				Name:              "keypress",
				ExtraHardwareDeps: hwdep.D(hwdep.Keyboard()),
				Val:               wakeByKeyboard,
			},
			{
				Name:              "lid",
				ExtraHardwareDeps: hwdep.D(hwdep.Lid()),
				Val:               wakeByLid,
			},
			{
				Name: "usb_keyboard",
				Val:  wakeByUSBKeyboard,
			},
		},
	})
}

// time constants
const (
	ecSuspendDelay     time.Duration = 5 * time.Second
	dutWakeDelay       time.Duration = 5 * time.Second
	lidSwitchDelay     time.Duration = 2 * time.Second
	powerdSuspendDelay time.Duration = 3 * time.Second
)

// regular expressions
const (
	reGetTabletMode          string = `\[\S+ tablet mode (enabled|disabled)\]`
	reTabletModeNotAvailable string = `Command 'tabletmode' not found or ambiguous`
	reCheckTabletMode        string = `(` + reGetTabletMode + `|` + reTabletModeNotAvailable + `)`
	reHasKsstate             string = `Keyboard scan disable mask: (0x[0-9a-fA-F]{8})`
	reNoKsstate              string = `Command \'ksstate\' not found or ambiguous\.`
	reGetKsstate             string = `(` + reHasKsstate + `|` + reNoKsstate + `)`
)

func ECWakeSource(ctx context.Context, s *testing.State) {
	h := s.FixtValue().(*fixture.Value).Helper
	testType := s.Param().(wakeMethod)
	if err := h.RequireServo(ctx); err != nil {
		s.Fatal("Failed to connect to servo: ", err)
	}
	if err := h.RequireRPCUtils(ctx); err != nil {
		s.Fatal("Requiring RPC utils: ", err)
	}

	// Create instance of chrome for login so that DUT suspends mode instead of shutting down.
	s.Log("New Chrome service")
	if _, err := h.RPCUtils.NewChrome(ctx, &empty.Empty{}); err != nil {
		s.Fatal("Failed to create instance of chrome: ", err)
	}

	switch testType {
	case wakeByPowerBtn:
		// If the DUT has no internal display, pressing the power button in a suspended state
		// would cause it to shut down instead of wake.
		s.Log("Suspend DUT and wake using power key")
		if err := testWakeWithPowerKey(ctx, h); err != nil {
			s.Fatal("Failed to suspend and wake with power key: ", err)
		}
	case wakeByKeyboard:
		s.Log("Suspend DUT and try to wake with keypress")
		if err := testSuspendAndWakeWithInternalKeypress(ctx, h); err != nil {
			s.Fatal("Failed to test waking with internal keypress: ", err)
		}

	case wakeByLid:
		s.Log("Suspend DUT by closing lid and wake by opening lid")
		if err := testSuspendAndWakeWithLid(ctx, h); err != nil {
			s.Fatal("Failed to suspend and wake with lid: ", err)
		}

		s.Log("Suspend DUT then close lid, wake by opening lid")
		if err := testWakeWithLid(ctx, h); err != nil {
			s.Fatal("Failed to suspend and wake with lid: ", err)
		}
	case wakeByUSBKeyboard:
		s.Log("Suspend DUT and wake using keypress from USB keyboard")
		if err := testWakeWithUSBKeyboard(ctx, h); err != nil {
			s.Fatal("Failed to suspend and wake with enter key on usb keyboard: ", err)
		}
	}
}

func testSuspendAndWakeWithLid(ctx context.Context, h *firmware.Helper) error {
	bootID, err := h.Reporter.BootID(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to get boot id")
	}

	// Delay by `dutWakeDelay` like with other suspend and wake tests.
	if err := closeAndOpenLid(ctx, h, dutWakeDelay); err != nil {
		return errors.Wrap(err, "failed to suspend and wake using lid")
	}

	newBootID, err := h.Reporter.BootID(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to get boot id")
	}
	if newBootID != bootID {
		return errors.New("suspend and wake test unexpectedly resulted in a reboot")
	}
	return nil
}

func testWakeWithLid(ctx context.Context, h *firmware.Helper) error {
	bootID, err := h.Reporter.BootID(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to get boot id")
	}

	testing.ContextLog(ctx, "Suspending DUT")
	cmd := h.DUT.Conn().CommandContext(ctx, "powerd_dbus_suspend", fmt.Sprintf("--delay=%d", int(powerdSuspendDelay.Seconds())))
	if err := cmd.Start(); err != nil {
		return errors.Wrap(err, "failed to suspend DUT")
	}

	testing.ContextLogf(ctx, "Sleeping for %s seconds", ecSuspendDelay+powerdSuspendDelay)
	if err := testing.Sleep(ctx, ecSuspendDelay+powerdSuspendDelay); err != nil {
		return errors.Wrap(err, "failed to sleep waiting for suspend")
	}
	testing.ContextLog(ctx, "Checking for S0ix or S3 powerstate")
	if err := h.WaitForPowerStates(ctx, 1*time.Second, 30*time.Second, "S0ix", "S3"); err != nil {
		return errors.Wrap(err, "failed to get S0ix or S3 powerstate")
	}

	testing.ContextLogf(ctx, "Sleeping for %s", dutWakeDelay)
	if err := testing.Sleep(ctx, dutWakeDelay); err != nil {
		return err
	}

	testing.ContextLog(ctx, "Checking it remains suspended for S0ix or S3 powerstate")
	if err := h.WaitForPowerStates(ctx, 1*time.Second, 30*time.Second, "S0ix", "S3"); err != nil {
		return errors.Wrap(err, "failed to get S0ix or S3 powerstate")
	}

	// Delay by `lidSwitchDelay` to ensure lid is detected as closed.
	// Additionally checks that closing lid keeps suspended power state.
	if err := closeAndOpenLid(ctx, h, lidSwitchDelay); err != nil {
		return errors.Wrap(err, "failed to wake from suspended by opening lid")
	}

	newBootID, err := h.Reporter.BootID(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to get boot id")
	}
	if newBootID != bootID {
		return errors.New("suspend and wake test unexpectedly resulted in a reboot")
	}
	return nil
}

func closeAndOpenLid(ctx context.Context, h *firmware.Helper, delay time.Duration) error {
	if err := h.Servo.CloseLid(ctx); err != nil {
		return err
	}

	testing.ContextLogf(ctx, "waiting %s for DUT to settle into suspend state", ecSuspendDelay)
	if err := testing.Sleep(ctx, ecSuspendDelay); err != nil {
		return err
	}

	testing.ContextLog(ctx, "Checking for S0ix or S3 powerstate")
	if err := h.WaitForPowerStates(ctx, 1*time.Second, 60*time.Second, "S0ix", "S3"); err != nil {
		return errors.Wrap(err, "failed to get S0ix or S3 powerstate")
	}

	testing.ContextLogf(ctx, "Sleeping for %s", delay)
	if err := testing.Sleep(ctx, delay); err != nil {
		return err
	}

	if err := h.Servo.OpenLid(ctx); err != nil {
		return err
	}

	testing.ContextLog(ctx, "Waiting for S0 powerstate")
	if err := h.WaitForPowerStates(ctx, 1*time.Second, 30*time.Second, "S0"); err != nil {
		return errors.Wrap(err, "failed to get S0 powerstate")
	}

	return nil
}

func testWakeWithUSBKeyboard(ctx context.Context, h *firmware.Helper) error {
	bootID, err := h.Reporter.BootID(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to get boot id")
	}

	testing.ContextLog(ctx, "Enabling USB keyboard")
	if err := h.Servo.SetOnOff(ctx, servo.USBKeyboard, servo.On); err != nil {
		return errors.Wrapf(err, "failed to set %q to %q with servo", servo.USBKeyboard, servo.On)
	}

	testing.ContextLog(ctx, "Pressing enter key with USB keyboard")
	if err := suspendDUTAndWakeWithKey(ctx, h, servo.USBEnter, true); err != nil {
		return errors.Wrap(err, "failed to suspend and wake dut with enter key on usb keyboard")
	}

	testing.ContextLog(ctx, "Disabling USB keyboard")
	if err := h.Servo.SetOnOff(ctx, servo.USBKeyboard, servo.Off); err != nil {
		return errors.Wrapf(err, "failed to set %q to %q with servo", servo.USBKeyboard, servo.Off)
	}

	newBootID, err := h.Reporter.BootID(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to get boot id")
	}
	if newBootID != bootID {
		return errors.New("suspend and wake test unexpectedly resulted in a reboot")
	}
	return nil
}

func testSuspendAndWakeWithInternalKeypress(ctx context.Context, h *firmware.Helper) error {
	var match []string
	out, err := h.Servo.RunECCommandGetOutput(ctx, "tabletmode off", []string{reCheckTabletMode})
	if err != nil {
		// Not all devices support this cmd.
		testing.ContextLog(ctx, "Failed to disable tabletmode: ", err)
	} else if out != nil {
		tabletModeAvailable := regexp.MustCompile(reGetTabletMode)
		match = tabletModeAvailable.FindStringSubmatch(out[0][0])
	}

	testing.ContextLog(ctx, "Checking ksstate")
	shouldWake := false
	ksstateout, err := h.Servo.RunECCommandGetOutput(ctx, "ksstate", []string{reGetKsstate})
	if err != nil {
		return errors.Wrap(err, "failed to check for presence of ksstate")
	} else if ksstateout != nil {
		ksstatematch := regexp.MustCompile(reHasKsstate).FindStringSubmatch(ksstateout[0][0])
		parsedMask, err := strconv.ParseInt(string(ksstatematch[1]), 0, 0)
		testing.ContextLog(ctx, "keyboard scan disable mask: ", parsedMask)
		if err != nil {
			testing.ContextLog(ctx, "Failed to parse kb disable mask: ", ksstatematch)
		} else {
			// If parsedMask = 0, expect internal keyboard to not be disabled.
			shouldWake = parsedMask == 0
		}
	}

	if match == nil {
		testing.ContextLog(ctx, "DUT does not support tablet mode")
		// Whether or not it wakes on keypress depends on keyboard being enabled/disabled.
		testing.ContextLog(ctx, "Suspend DUT and wake with emulated keypress")
		if err := testWakeWithKeyboard(ctx, h, shouldWake); err != nil {
			return errors.Wrap(err, "failed suspend and wake with keypress")
		}
	} else {
		testing.ContextLog(ctx, "DUT supports tablet mode, disabling tablet mode")
		out, err = h.Servo.RunECCommandGetOutput(ctx, "tabletmode off", []string{reGetTabletMode})
		if err == nil {
			testing.ContextLog(ctx, "current tabletmode state: ", out[0][1])
			// If tabletmode is off, waking on keypress depends on keyboard being enabled/disabled.
			testing.ContextLog(ctx, "Suspend DUT and wake with emulated keypress")
			if err := testWakeWithKeyboard(ctx, h, shouldWake); err != nil {
				return errors.Wrap(err, "failed suspend and wake with keypress with tablet mode disabled")
			}
		} else {
			testing.ContextLog(ctx, "Failed to disable tablet mode")
		}

		testing.ContextLog(ctx, "Enabling tablet mode")
		out, err = h.Servo.RunECCommandGetOutput(ctx, "tabletmode on", []string{reGetTabletMode})
		if err == nil {
			testing.ContextLog(ctx, "current tabletmode state: ", out[0][1])
			// Device is in tablet mode, expect that keyboard is disabled and keypress does not wake device.
			testing.ContextLog(ctx, "Suspend DUT and stay suspended after emulated keypress")
			if err := testWakeWithKeyboard(ctx, h, false); err != nil {
				return errors.Wrap(err, "failed suspend and stay suspended with keypress with tablet mode enabled")
			}
		} else {
			testing.ContextLog(ctx, "Failed to enable tablet mode: ", err)
		}

		testing.ContextLog(ctx, "Resetting tablet mode to initial value")
		if err = h.Servo.RunECCommand(ctx, "tabletmode reset"); err != nil {
			testing.ContextLog(ctx, "Failed to run reset tablet mode: ", err)
		}
	}

	return nil
}

func testWakeWithKeyboard(ctx context.Context, h *firmware.Helper, shouldWake bool) error {
	bootID, err := h.Reporter.BootID(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to get boot id")
	}

	// Disable USB keyboard to ensure internal keyboard is used.
	testing.ContextLog(ctx, "Disabling USB keyboard")
	if err := h.Servo.SetOnOff(ctx, servo.USBKeyboard, servo.Off); err != nil {
		return errors.Wrapf(err, "failed to set %q to %q with servo", servo.USBKeyboard, servo.Off)
	}

	if err = suspendDUTAndWakeWithKey(ctx, h, servo.Enter, shouldWake); err != nil {
		return errors.Wrap(err, "failed to suspend dut then wake/stay suspended")
	}

	newBootID, err := h.Reporter.BootID(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to get boot id")
	}
	if newBootID != bootID {
		return errors.New("suspend and wake test unexpectedly resulted in a reboot")
	}
	return nil
}

func testWakeWithPowerKey(ctx context.Context, h *firmware.Helper) error {
	bootID, err := h.Reporter.BootID(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to get boot id")
	}

	// Power button should always wake DUT.
	if err := suspendDUTAndWakeWithKey(ctx, h, servo.PowerKey, true); err != nil {
		return errors.Wrap(err, "failed to suspend and wake dut with power key")
	}

	testing.ContextLog(ctx, "Getting new boot ID")
	newBootID, err := h.Reporter.BootID(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to get boot id")
	}
	if newBootID != bootID {
		return errors.New("suspend and wake test unexpectedly resulted in a reboot")
	}
	return nil
}

func suspendDUTAndWakeWithKey(ctx context.Context, h *firmware.Helper, wakeKey servo.KeypressControl, shouldWake bool) error {
	testing.ContextLog(ctx, "Suspending DUT")
	cmd := h.DUT.Conn().CommandContext(ctx, "powerd_dbus_suspend", fmt.Sprintf("--delay=%d", int(powerdSuspendDelay.Seconds())))
	if err := cmd.Start(); err != nil {
		return errors.Wrap(err, "failed to suspend DUT")
	}

	testing.ContextLogf(ctx, "Sleeping for %s seconds", ecSuspendDelay+powerdSuspendDelay)
	if err := testing.Sleep(ctx, ecSuspendDelay+powerdSuspendDelay); err != nil {
		return errors.Wrap(err, "failed to sleep waiting for suspend")
	}

	testing.ContextLog(ctx, "Checking for S0ix or S3 powerstate")
	if err := h.WaitForPowerStates(ctx, 1*time.Second, 30*time.Second, "S0ix", "S3"); err != nil {
		return errors.Wrap(err, "failed to get S0ix or S3 powerstate")
	}

	testing.ContextLogf(ctx, "Pressing %v key", string(wakeKey))
	if err := h.Servo.KeypressWithDuration(ctx, wakeKey, servo.DurTab); err != nil {
		return errors.Wrapf(err, "failed to press %v key on DUT", string(wakeKey))
	}

	// If shouldn't wake, DUT should remain suspended, and wake from suspended when power button is pressed.
	testing.ContextLog(ctx, "Should device be awake after keypress: ", shouldWake)
	if !shouldWake {
		testing.ContextLogf(ctx, "Sleeping for %s", ecSuspendDelay)
		if err := testing.Sleep(ctx, ecSuspendDelay); err != nil {
			return err
		}

		testing.ContextLog(ctx, "Expect DUT to remain suspended, wait for S0ix or S3 powerstates")
		if err := h.WaitForPowerStates(ctx, 2*time.Second, 90*time.Second, "S0ix", "S3"); err != nil {
			return errors.Wrap(err, "failed to get one of S0ix or S3 powerstates")
		}

		// Wake DUT again since it remained suspended.
		testing.ContextLog(ctx, "Pressing power key")
		if err := h.Servo.KeypressWithDuration(ctx, servo.PowerKey, servo.DurPress); err != nil {
			return errors.Wrap(err, "failed to wake DUT with power button")
		}
	}

	testing.ContextLogf(ctx, "Sleeping for %s to wait for DUT to wake", ecSuspendDelay)
	if err := testing.Sleep(ctx, ecSuspendDelay); err != nil {
		return errors.Wrapf(err, "failed to sleep for %s waiting for power on", ecSuspendDelay)
	}

	// Both cases expect DUT to have woken.
	testing.ContextLog(ctx, "Waiting for S0 powerstate")
	if err := h.WaitForPowerStates(ctx, 1*time.Second, 30*time.Second, "S0"); err != nil {
		return errors.Wrap(err, "failed to get S0 powerstate")
	}

	if err := h.WaitConnect(ctx); err != nil {
		return errors.Wrap(err, "failed to reconnect to DUT")
	}
	return nil
}

// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package firmware

import (
	"context"
	"regexp"
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
		ServiceDeps:  []string{"tast.cros.firmware.UtilsService"},
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
	ecSuspendDelay time.Duration = 5 * time.Second
	dutWakeDelay   time.Duration = 5 * time.Second
	lidSwitchDelay time.Duration = 2 * time.Second
)

// keypress constants
const (
	testWakeKey servo.KeypressControl = servo.Enter
)

// regular expressions
const (
	reGetTabletMode          string = `\[\S+ tablet mode (enabled|disabled)\]`
	reTabletModeNotAvailable string = `Command 'tabletmode' not found or ambiguous`
	reCheckTabletMode        string = `(` + reGetTabletMode + `|` + reTabletModeNotAvailable + `)`
)

type wakeSourceArgs struct {
	ctx context.Context
	h   *firmware.Helper
	ms  *firmware.ModeSwitcher
}

func ECWakeSource(ctx context.Context, s *testing.State) {
	h := s.FixtValue().(*fixture.Value).Helper
	testType := s.Param().(wakeMethod)
	if err := h.RequireServo(ctx); err != nil {
		s.Fatal("Failed to connect to servo: ", err)
	}
	if err := h.RequireRPCUtils(ctx); err != nil {
		s.Fatal("Requiring RPC utils: ", err)
	}
	ms, err := firmware.NewModeSwitcher(ctx, h)
	if err != nil {
		s.Fatal("Failed to create mode switcher: ", err)
	}

	// Create instance of chrome for login so that DUT suspends mode instead of shutting down.
	s.Log("New Chrome service")
	if _, err := h.RPCUtils.NewChrome(ctx, &empty.Empty{}); err != nil {
		s.Fatal("Failed to create instance of chrome: ", err)
	}

	args := &wakeSourceArgs{
		ctx: ctx,
		h:   h,
		ms:  ms,
	}

	switch testType {
	case wakeByPowerBtn:
		// If the DUT has no internal display, pressing the power button in a suspended state
		// would cause it to shut down instead of wake.
		s.Log("Suspend DUT and wake using power key")
		if err := testWakeWithPowerKey(args); err != nil {
			s.Fatal("Failed to suspend and wake with power key: ", err)
		}
	case wakeByKeyboard:
		s.Log("Check if tabletmode cmd is availabe on DUT")
		out, err := h.Servo.RunECCommandGetOutput(ctx, "tabletmode off", []string{reCheckTabletMode})
		if err != nil {
			s.Fatal("Failed to run tabletmode command: ", err)
		}
		tabletModeNotAvailable := regexp.MustCompile(reTabletModeNotAvailable)
		if match := tabletModeNotAvailable.FindStringSubmatch(out[0].([]interface{})[0].(string)); match != nil {
			s.Log("DUT does not support tablet mode: ", match)
			s.Log("Suspend DUT and wake with emulated keypress")
			if err := testWakeWithKeyboard(args, true); err != nil {
				s.Fatal("Failed suspend and wake with keypress: ", err)
			}
		} else {
			s.Log("DUT supports tablet mode")
			s.Log("Disabling tablet mode")
			out, err = h.Servo.RunECCommandGetOutput(ctx, "tabletmode off", []string{reGetTabletMode})
			if err == nil {
				s.Log("current tabletmode state: ", out[0].([]interface{})[1].(string))
				s.Log("Suspend DUT and wake with emulated keypress")
				if err := testWakeWithKeyboard(args, true); err != nil {
					s.Log("Failed suspend and wake with keypress with tablet mode disabled: ", err)
				}
			} else {
				s.Log("Failed to disable tablet mode: ", err)
			}

			s.Log("Enabling tablet mode")
			out, err = h.Servo.RunECCommandGetOutput(ctx, "tabletmode on", []string{reGetTabletMode})
			if err == nil {
				s.Log("current tabletmode state: ", out[0].([]interface{})[1].(string))
				// Device is in tablet mode, expect that keypress does not wake device.
				s.Log("Suspend DUT and stay suspended after emulated keypress")
				if err := testWakeWithKeyboard(args, false); err != nil {
					s.Log("Failed suspend and stay suspended with keypress with tablet mode enabled: ", err)
				}
			} else {
				s.Log("Failed to enable tablet mode: ", err)
			}

			s.Log("Resetting tablet mode to initial value")
			if err = h.Servo.RunECCommand(ctx, "tabletmode reset"); err != nil {
				s.Log("Failed to run reset tablet mode: ", err)
			}
		}
	case wakeByLid:
		s.Log("Suspend DUT by closing lid and wake by opening lid")
		if err := testSuspendAndWakeWithLid(args); err != nil {
			s.Fatal("Failed to suspend and wake with lid: ", err)
		}

		s.Log("Suspend DUT then close lid, wake by opening lid")
		if err := testWakeWithLid(args); err != nil {
			s.Fatal("Failed to suspend and wake with lid: ", err)
		}
	case wakeByUSBKeyboard:
		s.Log("Suspend DUT and wake using keypress from USB keyboard")
		if err := testWakeWithUSBKeyboard(args); err != nil {
			s.Fatal("Failed to suspend and wake with enter key on usb keyboard: ", err)
		}
	}
}

func testSuspendAndWakeWithLid(args *wakeSourceArgs) error {
	bootID, err := args.h.Reporter.BootID(args.ctx)
	if err != nil {
		return errors.Wrap(err, "failed to get boot id")
	}

	// Delay by `dutWakeDelay` like with other suspend and wake tests.
	if err := closeAndOpenLid(args, dutWakeDelay); err != nil {
		return errors.Wrap(err, "failed to suspend and wake using lid")
	}

	newBootID, err := args.h.Reporter.BootID(args.ctx)
	if err != nil {
		return errors.Wrap(err, "failed to get boot id")
	}
	if newBootID != bootID {
		return errors.New("suspend and wake test unexpectedly resulted in a reboot")
	}
	return nil
}

func testWakeWithLid(args *wakeSourceArgs) error {
	bootID, err := args.h.Reporter.BootID(args.ctx)
	if err != nil {
		return errors.Wrap(err, "failed to get boot id")
	}

	testing.ContextLog(args.ctx, "Suspending DUT")
	cmd := args.h.DUT.Conn().CommandContext(args.ctx, "powerd_dbus_suspend")
	if err := cmd.Start(); err != nil {
		return errors.Wrap(err, "failed to suspend DUT")
	}

	testing.ContextLog(args.ctx, "Checking for S0ix or S3 powerstate")
	if err := args.ms.WaitForPowerStates(args.ctx, firmware.PowerStateInterval, firmware.PowerStateTimeout, "S0ix", "S3"); err != nil {
		return errors.Wrap(err, "failed to get S0ix or S3 powerstate")
	}

	testing.ContextLogf(args.ctx, "Sleeping for %s", dutWakeDelay)
	if err := testing.Sleep(args.ctx, dutWakeDelay); err != nil {
		return err
	}

	// Delay by `lidSwitchDelay` to ensure lid is detected as closed.
	// Additionally checks that closing lid keeps suspended power state.
	if err := closeAndOpenLid(args, lidSwitchDelay); err != nil {
		return errors.Wrap(err, "failed to wake from suspended by opening lid")
	}

	newBootID, err := args.h.Reporter.BootID(args.ctx)
	if err != nil {
		return errors.Wrap(err, "failed to get boot id")
	}
	if newBootID != bootID {
		return errors.New("suspend and wake test unexpectedly resulted in a reboot")
	}
	return nil
}

func closeAndOpenLid(args *wakeSourceArgs, delay time.Duration) error {
	if err := args.h.Servo.CloseLid(args.ctx); err != nil {
		return err
	}

	testing.ContextLog(args.ctx, "Checking for S0ix or S3 powerstate")
	if err := args.ms.WaitForPowerStates(args.ctx, firmware.PowerStateInterval, firmware.PowerStateTimeout, "S0ix", "S3"); err != nil {
		return errors.Wrap(err, "failed to get S0ix or S3 powerstate")
	}

	testing.ContextLogf(args.ctx, "Sleeping for %s", delay)
	if err := testing.Sleep(args.ctx, delay); err != nil {
		return err
	}

	if err := args.h.Servo.OpenLid(args.ctx); err != nil {
		return err
	}

	testing.ContextLog(args.ctx, "Waiting for S0 powerstate")
	if err := args.ms.WaitForPowerStates(args.ctx, firmware.PowerStateInterval, firmware.PowerStateTimeout, "S0"); err != nil {
		return errors.Wrap(err, "failed to get S0 powerstate")
	}

	if err := args.h.WaitConnect(args.ctx); err != nil {
		return errors.Wrap(err, "failed to reconnect to DUT")
	}
	return nil
}

func testWakeWithUSBKeyboard(args *wakeSourceArgs) error {
	bootID, err := args.h.Reporter.BootID(args.ctx)
	if err != nil {
		return errors.Wrap(err, "failed to get boot id")
	}

	testing.ContextLog(args.ctx, "Enabling USB keyboard")
	if err := args.h.Servo.SetOnOff(args.ctx, servo.USBKeyboard, servo.On); err != nil {
		return errors.Wrapf(err, "failed to set %q to %q with servo", servo.USBKeyboard, servo.On)
	}

	testing.ContextLog(args.ctx, "Pressing enter key with USB keyboard")
	if err := suspendDUTAndWakeWithKey(args, servo.USBEnter, true); err != nil {
		return errors.Wrap(err, "failed to suspend and wake dut with enter key on usb keyboard")
	}

	testing.ContextLog(args.ctx, "Disabling USB keyboard")
	if err := args.h.Servo.SetOnOff(args.ctx, servo.USBKeyboard, servo.Off); err != nil {
		return errors.Wrapf(err, "failed to set %q to %q with servo", servo.USBKeyboard, servo.Off)
	}

	newBootID, err := args.h.Reporter.BootID(args.ctx)
	if err != nil {
		return errors.Wrap(err, "failed to get boot id")
	}
	if newBootID != bootID {
		return errors.New("suspend and wake test unexpectedly resulted in a reboot")
	}
	return nil
}

func testWakeWithKeyboard(args *wakeSourceArgs, shouldWake bool) error {
	bootID, err := args.h.Reporter.BootID(args.ctx)
	if err != nil {
		return errors.Wrap(err, "failed to get boot id")
	}

	// Disable USB keyboard to ensure internal keyboard is used.
	testing.ContextLog(args.ctx, "Disabling USB keyboard")
	if err := args.h.Servo.SetOnOff(args.ctx, servo.USBKeyboard, servo.Off); err != nil {
		return errors.Wrapf(err, "failed to set %q to %q with servo", servo.USBKeyboard, servo.Off)
	}

	if err = suspendDUTAndWakeWithKey(args, testWakeKey, shouldWake); err != nil {
		return errors.Wrap(err, "failed to suspend dut then wake/stay suspended")
	}

	newBootID, err := args.h.Reporter.BootID(args.ctx)
	if err != nil {
		return errors.Wrap(err, "failed to get boot id")
	}
	if newBootID != bootID {
		return errors.New("suspend and wake test unexpectedly resulted in a reboot")
	}
	return nil
}

func testWakeWithPowerKey(args *wakeSourceArgs) error {
	bootID, err := args.h.Reporter.BootID(args.ctx)
	if err != nil {
		return errors.Wrap(err, "failed to get boot id")
	}

	// Power button should always wake DUT.
	if err := suspendDUTAndWakeWithKey(args, servo.PowerKey, true); err != nil {
		return errors.Wrap(err, "failed to suspend and wake dut with power key")
	}

	testing.ContextLog(args.ctx, "Getting new boot ID")
	newBootID, err := args.h.Reporter.BootID(args.ctx)
	if err != nil {
		return errors.Wrap(err, "failed to get boot id")
	}
	if newBootID != bootID {
		return errors.New("suspend and wake test unexpectedly resulted in a reboot")
	}
	return nil
}

func suspendDUTAndWakeWithKey(args *wakeSourceArgs, wakeKey servo.KeypressControl, shouldWake bool) error {
	testing.ContextLog(args.ctx, "Suspending DUT")
	cmd := args.h.DUT.Conn().CommandContext(args.ctx, "powerd_dbus_suspend")
	if err := cmd.Start(); err != nil {
		return errors.Wrap(err, "failed to suspend DUT")
	}

	testing.ContextLogf(args.ctx, "Sleeping for %s", ecSuspendDelay)
	if err := testing.Sleep(args.ctx, ecSuspendDelay); err != nil {
		return err
	}

	testing.ContextLog(args.ctx, "Checking for S0ix or S3 powerstate")
	if err := args.ms.WaitForPowerStates(args.ctx, firmware.PowerStateInterval, firmware.PowerStateTimeout, "S0ix", "S3"); err != nil {
		return errors.Wrap(err, "failed to get S0ix or S3 powerstate")
	}

	testing.ContextLogf(args.ctx, "Pressing %v key", string(wakeKey))
	if err := args.h.Servo.KeypressWithDuration(args.ctx, wakeKey, servo.DurPress); err != nil {
		return errors.Wrapf(err, "failed to press %v key on DUT", string(wakeKey))
	}

	testing.ContextLogf(args.ctx, "Sleeping for %s", ecSuspendDelay)
	if err := testing.Sleep(args.ctx, ecSuspendDelay); err != nil {
		return err
	}

	// If shouldn't wake, DUT should remain suspended, and wake from suspended when power button is pressed.
	if !shouldWake {
		testing.ContextLog(args.ctx, "Waiting for one of S0ix or S3 powerstates")
		if err := args.ms.WaitForPowerStates(args.ctx, firmware.PowerStateInterval, firmware.PowerStateTimeout, "S0ix", "S3"); err != nil {
			return errors.Wrap(err, "failed to get one of S0ix or S3 powerstates")
		}

		// Wake DUT again since it remained suspended.
		if err := args.h.Servo.KeypressWithDuration(args.ctx, servo.PowerKey, servo.DurPress); err != nil {
			return errors.Wrap(err, "failed to wake DUT with power button")
		}
	}

	// Both cases expect DUT to have woken.
	testing.ContextLog(args.ctx, "Waiting for S0 powerstate")
	if err := args.ms.WaitForPowerStates(args.ctx, firmware.PowerStateInterval, firmware.PowerStateTimeout, "S0"); err != nil {
		return errors.Wrap(err, "failed to get S0 powerstate")
	}

	if err := args.h.WaitConnect(args.ctx); err != nil {
		return errors.Wrap(err, "failed to reconnect to DUT")
	}
	return nil
}

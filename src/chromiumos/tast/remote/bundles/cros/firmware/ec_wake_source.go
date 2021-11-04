// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package firmware

import (
	"context"
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

type hardwareParams struct {
	hasInternalDisplay bool
	hasKeyboard        bool
	hasLid             bool
	hasECHibernate     bool
}

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
				Val: hardwareParams{
					hasInternalDisplay: true,
					hasKeyboard:        false,
					hasLid:             false,
					hasECHibernate:     false,
				},
			},
			{
				Name:              "keypress",
				ExtraHardwareDeps: hwdep.D(hwdep.Keyboard()),
				Val: hardwareParams{
					hasInternalDisplay: false,
					hasKeyboard:        true,
					hasLid:             false,
					hasECHibernate:     false,
				},
			},
			{
				Name:              "lid",
				ExtraHardwareDeps: hwdep.D(hwdep.Lid()),
				Val: hardwareParams{
					hasInternalDisplay: false,
					hasKeyboard:        false,
					hasLid:             true,
					hasECHibernate:     false,
				},
			},
			{
				Name:              "hibernate",
				ExtraHardwareDeps: hwdep.D(hwdep.ECHibernate()),
				Val: hardwareParams{
					hasInternalDisplay: false,
					hasKeyboard:        false,
					hasLid:             false,
					hasECHibernate:     true,
				},
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
	reKsstate string = `Keyboard scan disable mask: 0x([0-9a-fA-F]{8})`
)

type wakeSourceArgs struct {
	ctx context.Context
	h   *firmware.Helper
	ms  *firmware.ModeSwitcher
}

func ECWakeSource(ctx context.Context, s *testing.State) {
	h := s.FixtValue().(*fixture.Value).Helper
	hwParams := s.Param().(hardwareParams)
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

	if hwParams.hasInternalDisplay {
		s.Log("Suspend DUT and wake using power key")
		// If the DUT has no internal display, pressing the power button in a suspended state
		// would cause it to shut down instead of wake.
		if err := suspendAndWakeWithPowerKey(args); err != nil {
			s.Fatal("Failed to suspend and wake with power key: ", err)
		}
	}

	if hwParams.hasKeyboard {
		// If DUT does not support ksstate cmd, skip wake by keyboard test.
		out, err := h.Servo.RunECCommandGetOutput(ctx, "ksstate", []string{reKsstate})
		if err != nil {
			s.Fatal("Failed to run ksstate command")
		}
		kbScanDisableMask, err := strconv.ParseInt(out[0].([]interface{})[1].(string), 16, 0)
		if err != nil {
			s.Fatalf("Error converting %q into a base 16 integer: %v", out, err)
		} else if kbScanDisableMask == 0 {
			s.Log("Suspend DUT and wake with emulated keypress")
			if err := wakeWithKeyboard(args, true); err != nil {
				s.Fatal("Failed suspend and wake with keypress: ", err)
			}
		} else {
			// Device is in tablet mode, expect that keypress does not wake device.
			s.Log("Suspend DUT and stay suspended after emulated keypress")
			if err := wakeWithKeyboard(args, false); err != nil {
				s.Fatal("Failed suspend and stay suspended with keypress: ", err)
			}
		}

		s.Log("Suspend DUT and wake using keypress from USB keyboard")
		if err := wakeWithUSBKeyboard(args); err != nil {
			s.Fatal("Failed to suspend and wake with enter key on usb keyboard: ", err)
		}
	}

	if hwParams.hasLid {
		s.Log("Suspend DUT and wake by opening lid")
		if err := wakeWithLid(args); err != nil {
			s.Fatal("Failed to suspend and wake with lid: ", err)
		}
		s.Log("Suspend DUT by closing lid and wake by opening lid")
		if err := suspendAndWakeWithLid(args); err != nil {
			s.Fatal("Failed to suspend and wake with lid: ", err)
		}
	}

	if hwParams.hasECHibernate {
		s.Log("Suspend DUT by shutting down and hibernating EC, wake by power key")
		if sType, err := h.Servo.GetServoType(ctx); (err == nil) || (sType != "ccd_cr50") {
			if err := hibernateAndWake(args); err != nil {
				s.Fatal("Failed to suspend and wake with lid: ", err)
			}
		} else {
			s.Log("Cannot wake DUT with power button with CCD, skipping hibernate and wake test")
		}
	}
}

func hibernateAndWake(args *wakeSourceArgs) error {
	defer args.h.WaitConnect(args.ctx)
	chargerAttached, err := args.h.Servo.GetChargerAttached(args.ctx)
	if err != nil {
		return errors.Wrap(err, "failed to check if charger is attached")
	}

	testing.ContextLog(args.ctx, "Shutting down DUT")
	cmd := args.h.DUT.Conn().CommandContext(args.ctx, "/sbin/shutdown", "-P", "now")
	if err := cmd.Start(); err != nil {
		return errors.Wrap(err, "failed to shut down DUT")
	}
	testing.ContextLog(args.ctx, "Checking for G3 or S5 powerstate")
	err = args.ms.WaitForPowerStates(args.ctx, firmware.PowerStateInterval, firmware.PowerStateTimeout, "G3", "S5")
	if err != nil {
		return errors.Wrap(err, "failed to get G3 or S5 powerstate")
	}

	testing.ContextLog(args.ctx, "Hibernate EC")
	if err := args.h.Servo.ECHibernate(args.ctx); err != nil {
		return errors.Wrap(err, "failed to hibernate EC")
	}

	testing.ContextLogf(args.ctx, "Sleeping for %v seconds", int(dutWakeDelay/time.Second))
	if err := testing.Sleep(args.ctx, dutWakeDelay); err != nil {
		return err
	}

	if _, err := args.h.Servo.RunECCommandGetOutput(args.ctx, "help", []string{".*>"}); !chargerAttached && err == nil {
		return errors.New("DUT is not in hibernate")
	}

	testing.ContextLog(args.ctx, "Powering DUT back on with short press of the power button")
	if err := args.h.Servo.KeypressWithDuration(args.ctx, servo.PowerKey, servo.DurTab); err != nil {
		return errors.Wrap(err, "failed to power on DUT with short press of the power button")
	}

	testing.ContextLog(args.ctx, "Waiting for S0 powerstate")
	return args.ms.WaitForPowerStates(args.ctx, firmware.PowerStateInterval, firmware.PowerStateTimeout, "S0")
}

func suspendAndWakeWithLid(args *wakeSourceArgs) error {
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

func wakeWithLid(args *wakeSourceArgs) error {
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

	testing.ContextLogf(args.ctx, "Sleeping for %v seconds", int(dutWakeDelay/time.Second))
	if err := testing.Sleep(args.ctx, dutWakeDelay); err != nil {
		return err
	}

	// Delay by `lidSwitchDelay` to ensure lid is detected as closed.
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
	defer args.h.WaitConnect(args.ctx)
	testing.ContextLog(args.ctx, "Closing DUT Lid")
	if err := args.h.Servo.CloseLid(args.ctx); err != nil {
		return err
	}

	testing.ContextLog(args.ctx, "Checking for S0ix or S3 powerstate")
	if err := args.ms.WaitForPowerStates(args.ctx, firmware.PowerStateInterval, firmware.PowerStateTimeout, "S0ix", "S3"); err != nil {
		return errors.Wrap(err, "failed to get S0ix or S3 powerstate")
	}

	testing.ContextLogf(args.ctx, "Sleeping for %v seconds", int(delay/time.Second))
	if err := testing.Sleep(args.ctx, delay); err != nil {
		return err
	}

	testing.ContextLog(args.ctx, "Opening DUT Lid")
	if err := args.h.Servo.OpenLid(args.ctx); err != nil {
		return err
	}
	testing.ContextLog(args.ctx, "Waiting for S0 powerstate")
	if err := args.ms.WaitForPowerStates(args.ctx, firmware.PowerStateInterval, firmware.PowerStateTimeout, "S0"); err != nil {
		return errors.Wrap(err, "failed to get S0 powerstate")
	}
	return nil
}

func wakeWithUSBKeyboard(args *wakeSourceArgs) error {
	testing.ContextLog(args.ctx, "Enabling USB keyboard")
	if err := args.h.Servo.SetOnOff(args.ctx, servo.USBKeyboard, servo.On); err != nil {
		return errors.Wrapf(err, "failed to set %q to %q with servo", servo.USBKeyboard, servo.On)
	}

	bootID, err := args.h.Reporter.BootID(args.ctx)
	if err != nil {
		return errors.Wrap(err, "failed to get boot id")
	}

	testing.ContextLog(args.ctx, "Pressing enter key with USB keyboard")
	if err := suspendAndWakeWithKey(args, servo.USBEnter); err != nil {
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

func wakeWithKeyboard(args *wakeSourceArgs, shouldWake bool) error {
	defer args.h.WaitConnect(args.ctx)
	bootID, err := args.h.Reporter.BootID(args.ctx)
	if err != nil {
		return errors.Wrap(err, "failed to get boot id")
	}

	err = suspendAndWakeWithKey(args, testWakeKey)
	if err != nil && shouldWake {
		return errors.Wrap(err, "failed to get S0 powerstate")
	} else if err == nil && !shouldWake {
		return errors.Wrap(err, "dut unexpectedly reached S0 powerstate")
	} else if err != nil { // if successfully stays suspended
		if err := args.h.Servo.KeypressWithDuration(args.ctx, servo.PowerKey, servo.DurPress); err != nil {
			return errors.Wrap(err, "failed to wake DUT with power button")
		}
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

func suspendAndWakeWithPowerKey(args *wakeSourceArgs) error {
	defer args.h.WaitConnect(args.ctx)
	bootID, err := args.h.Reporter.BootID(args.ctx)
	if err != nil {
		return errors.Wrap(err, "failed to get boot id")
	}

	if err := suspendAndWakeWithKey(args, servo.PowerKey); err != nil {
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

func suspendAndWakeWithKey(args *wakeSourceArgs, wakeKey servo.KeypressControl) error {
	testing.ContextLog(args.ctx, "Suspending DUT")
	cmd := args.h.DUT.Conn().CommandContext(args.ctx, "powerd_dbus_suspend")
	if err := cmd.Start(); err != nil {
		return errors.Wrap(err, "failed to suspend DUT")
	}

	testing.ContextLogf(args.ctx, "Sleeping for %v seconds", int(ecSuspendDelay/time.Second))
	if err := testing.Sleep(args.ctx, ecSuspendDelay); err != nil {
		return err
	}

	testing.ContextLog(args.ctx, "Checking for S0ix or S3 powerstate")
	if err := args.ms.WaitForPowerStates(args.ctx, firmware.PowerStateInterval, firmware.PowerStateTimeout, "S0ix", "S3"); err != nil {
		return errors.Wrap(err, "failed to get S0ix or S3 powerstate")
	}

	testing.ContextLogf(args.ctx, "Sleeping for %v seconds", int(ecSuspendDelay/time.Second))
	if err := testing.Sleep(args.ctx, ecSuspendDelay); err != nil {
		return err
	}

	testing.ContextLogf(args.ctx, "Pressing %v key", string(wakeKey))
	if err := args.h.Servo.KeypressWithDuration(args.ctx, wakeKey, servo.DurPress); err != nil {
		errors.Wrapf(err, "failed to press %v key on DUT", string(wakeKey))
	}

	testing.ContextLog(args.ctx, "Waiting for S0 powerstate")
	return args.ms.WaitForPowerStates(args.ctx, firmware.PowerStateInterval, firmware.PowerStateTimeout, "S0")
}

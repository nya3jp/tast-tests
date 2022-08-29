// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package firmware

import (
	"context"
	"time"

	fwCommon "chromiumos/tast/common/firmware"
	"chromiumos/tast/common/servo"
	"chromiumos/tast/errors"
	"chromiumos/tast/remote/firmware"
	"chromiumos/tast/remote/firmware/fixture"
	"chromiumos/tast/ssh"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

type devFwParam struct {
	devFwScreenName string
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         DetachableDevScreen,
		Desc:         "Confirms basic dev screen behaviors for detachables",
		Contacts:     []string{"cienet-firmware@cienet.corp-partner.google.com", "chromeos-firmware@google.com"},
		Attr:         []string{"group:firmware", "firmware_unstable", "firmware_usb"},
		SoftwareDeps: []string{"crossystem"},
		Fixture:      fixture.DevMode,
		HardwareDeps: hwdep.D(hwdep.ChromeEC(), hwdep.FormFactor(hwdep.Detachable)),
		Params: []testing.Param{{
			Timeout: 60 * time.Minute,
			Val: devFwParam{
				devFwScreenName: "devWarningScreen",
			},
		}, {
			Name:    "dev_options",
			Timeout: 60 * time.Minute,
			Val: devFwParam{
				devFwScreenName: "devOptions",
			},
		}},
	})
}

func DetachableDevScreen(ctx context.Context, s *testing.State) {
	h := s.FixtValue().(*fixture.Value).Helper

	if err := h.RequireServo(ctx); err != nil {
		s.Fatal("Failed to init servo: ", err)
	}

	if err := h.RequireConfig(ctx); err != nil {
		s.Fatal("Requiring config")
	}

	cs := s.CloudStorage()
	if err := h.SetupUSBKey(ctx, cs); err != nil {
		s.Fatal("USBKey not working: ", err)
	}

	type testCase struct {
		trigger     string
		bootFromUSB bool
		bootMode    fwCommon.BootMode
		powerState  string
	}

	devWarningScreen := []testCase{
		{"warningScreenPowerOff", false, fwCommon.BootModeDev, "G3"},
		{"volumeUp", true, fwCommon.BootModeUSBDev, "S0"},
		{"volumeDown", false, fwCommon.BootModeDev, "S0"},
		{"powerButtonLong", false, fwCommon.BootModeDev, "G3"},
		{"spaceEnter", false, fwCommon.BootModeDev, "G3"},
		{"ctrlD", false, fwCommon.BootModeDev, "S0"},
		{"ctrlU", true, fwCommon.BootModeUSBDev, "S0"},
	}
	devOptions := []testCase{
		{"bootFromUSB", true, fwCommon.BootModeUSBDev, "S0"},
		{"bootFromInternal", false, fwCommon.BootModeDev, "S0"},
		{"devOptionsPowerOff", false, fwCommon.BootModeDev, "G3"},
	}

	args := s.Param().(devFwParam)
	var testSteps []testCase
	switch args.devFwScreenName {
	case "devWarningScreen":
		testSteps = devWarningScreen
	case "devOptions":
		testSteps = devOptions
	}

	for _, step := range testSteps {
		s.Log("Enabling dev_boot_usb")
		if err := h.DUT.Conn().CommandContext(ctx, "crossystem", "dev_boot_usb=1").Run(ssh.DumpLogOnError); err != nil {
			s.Fatal("Failed to enable dev_boot_usb: ", err)
		}
		s.Log("Enabling USB connection to DUT")
		if err := h.Servo.SetUSBMuxState(ctx, servo.USBMuxDUT); err != nil {
			s.Fatal("Failed to enable USB: ", err)
		}
		// Reboot DUT to the developer firmware screen.
		if err := h.Servo.SetPowerState(ctx, servo.PowerStateWarmReset); err != nil {
			s.Fatal("Failed to reset DUT: ", err)
		}
		s.Logf("Sleeping for %s (FirmwareScreen) ", h.Config.FirmwareScreen)
		if err := testing.Sleep(ctx, h.Config.FirmwareScreen); err != nil {
			s.Fatalf("Failed to sleep for %s: %v", h.Config.FirmwareScreen, err)
		}
		if args.devFwScreenName == "devOptions" {
			if err := nTimesTraverseSelect(ctx, h, servo.VolumeUpHold, 3, "Developer Options"); err != nil {
				s.Fatal("Failed to enter dev screen: ", err)
			}
		}
		if err := testTrigger(ctx, h, step.trigger); err != nil {
			s.Fatalf("Failed to press %s: ", step.trigger)
		}
		if err := h.WaitForPowerStates(ctx, firmware.PowerStateInterval, 2*time.Minute, step.powerState); err != nil {
			s.Fatalf("Failed to get powerState:%s", step.powerState)
		}
		if step.powerState == "G3" {
			s.Log("Setting dut's power on")
			if err := h.Servo.SetPowerState(ctx, servo.PowerStateOn); err != nil {
				s.Fatal("Failed to set dut's power on: ", err)
			}
		}
		if err := func() error {
			s.Log("Waiting for DUT to power ON")
			waitConnectCtx, cancelWaitConnect := context.WithTimeout(ctx, 8*time.Minute)
			defer cancelWaitConnect()
			if err := h.WaitConnect(waitConnectCtx); err != nil {
				return err
			}
			return nil
		}(); err != nil {
			s.Fatal("Failed to reconnect to DUT: ", err)
		}
		s.Log("Checking for boot device type")
		bootFromUSB, err := h.Reporter.BootedFromRemovableDevice(ctx)
		if err != nil {
			s.Fatal("Failed to check boot device type: ", err)
		}
		if bootFromUSB != step.bootFromUSB {
			s.Fatalf("Expected boot from device: %s, but got: %s", bootedDeviceType(step.bootFromUSB), bootedDeviceType(bootFromUSB))
		}
		s.Log("Checking for DUT's boot mode")
		currentMode, err := h.Reporter.CurrentBootMode(ctx)
		if err != nil {
			s.Fatal("Failed to check for boot mode: ", err)
		}
		if currentMode != step.bootMode {
			s.Fatalf("Expected boot mode: %q, but got: %q", step.bootMode, currentMode)
		}
		s.Logf("Current boot mode: %q", currentMode)
	}
}

func bootedDeviceType(bootFromUSB bool) string {
	if bootFromUSB {
		return "USB"
	}
	return "Internal Disk"
}

func testTrigger(ctx context.Context, h *firmware.Helper, trigger string) error {
	var err error
	testing.ContextLogf(ctx, "Testing trigger %s", trigger)
	switch trigger {
	case "ctrlD":
		err = h.Servo.KeypressWithDuration(ctx, servo.CtrlD, servo.DurTab)
	case "ctrlU":
		err = h.Servo.KeypressWithDuration(ctx, servo.CtrlU, servo.DurTab)
	case "volumeUp":
		err = h.Servo.SetInt(ctx, servo.VolumeUpHold, 3000)
	case "volumeDown":
		err = h.Servo.SetInt(ctx, servo.VolumeDownHold, 3000)
	case "powerButtonLong":
		testing.ContextLogf(ctx, "Pressing power button for %s", h.Config.HoldPwrButtonNoPowerdShutdown)
		err = h.Servo.KeypressWithDuration(ctx, servo.PowerKey, servo.Dur(h.Config.HoldPwrButtonNoPowerdShutdown))
	case "warningScreenPowerOff":
		err = h.Servo.KeypressWithDuration(ctx, servo.PowerKey, servo.DurTab)
	case "spaceEnter":
		// Pressing space & enter boots the dut to normal mode if its
		// firmware screen uses KeyboardDevSwitcher. On detachables,
		// this combination would power the dut off because pressing
		// space triggers nothing, and pressing enter selects the default
		// menu option 'power off'.
		testing.ContextLog(ctx, "Pressing SPACE")
		err = h.Servo.PressKey(ctx, " ", servo.DurTab)
		testing.ContextLogf(ctx, "Sleeping %s (KeypressDelay)", h.Config.KeypressDelay)
		err = testing.Sleep(ctx, h.Config.KeypressDelay)
		testing.ContextLog(ctx, "Pressing ENTER")
		err = h.Servo.KeypressWithDuration(ctx, servo.Enter, servo.DurTab)
	case "bootFromUSB":
		err = nTimesTraverseSelect(ctx, h, servo.VolumeUpHold, 1, "Boot From USB")
	case "bootFromInternal":
		err = nTimesTraverseSelect(ctx, h, servo.VolumeUpHold, 0, "Boot From Internal Disk")
	case "devOptionsPowerOff":
		err = nTimesTraverseSelect(ctx, h, servo.VolumeDownHold, 2, "Power Off")
	}
	if err != nil {
		return errors.Wrapf(err, "failed to press %s", trigger)
	}
	return nil
}

func nTimesTraverseSelect(ctx context.Context, h *firmware.Helper, upOrDown servo.IntControl, times int, menuOpt string) error {
	for ; times > 0; times-- {
		testing.ContextLogf(ctx, "Pressing %s", upOrDown)
		if err := h.Servo.SetInt(ctx, upOrDown, 100); err != nil {
			return errors.Wrapf(err, "pressing %s", upOrDown)
		}
		if err := testing.Sleep(ctx, 3*time.Second); err != nil {
			return errors.Wrap(err, "sleeping for 3s")
		}
	}
	testing.ContextLogf(ctx, "Selecting %q", menuOpt)
	if err := h.Servo.KeypressWithDuration(ctx, servo.PowerKey, servo.DurTab); err != nil {
		return errors.Wrapf(err, "selecting %s", menuOpt)
	}
	if err := testing.Sleep(ctx, 3*time.Second); err != nil {
		return errors.Wrap(err, "sleeping for 3s")
	}
	return nil
}

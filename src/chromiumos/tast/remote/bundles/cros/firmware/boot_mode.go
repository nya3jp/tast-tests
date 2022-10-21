// Copyright 2019 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package firmware

import (
	"context"
	"strconv"
	"time"

	fwCommon "chromiumos/tast/common/firmware"
	"chromiumos/tast/common/servo"
	"chromiumos/tast/remote/firmware"
	"chromiumos/tast/remote/firmware/fixture"
	"chromiumos/tast/remote/firmware/reporters"
	"chromiumos/tast/testing"
)

// bootModeTestParams defines the params for a single test-case.
// bootToMode defines which boot-mode to switch the DUT into.
// allowGBBForce defines whether to force dev mode via GBB flags.
// resetAfterBoot defines whether to perform a ModeAwareReboot after switching to bootToMode.
// resetType defines whether ModeAwareReboot should use a warm or a cold reset.
// checkBootFromMain checks whether device boots from the main storage when a memory device is attached.
type bootModeTestParams struct {
	bootToMode          fwCommon.BootMode
	allowGBBForce       bool
	resetAfterBoot      bool
	resetType           firmware.ResetType
	checkBootFromMain   bool
	checkToNoGoodScreen bool
	checkToBrokenScreen bool
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         BootMode,
		Desc:         "Verifies that remote tests can boot the DUT into, and confirm that the DUT is in, the different firmware modes (normal, dev, and recovery)",
		Contacts:     []string{"cros-fw-engprod@google.com"},
		Attr:         []string{"group:firmware"},
		SoftwareDeps: []string{"crossystem", "flashrom"},
		Vars:         []string{"firmware.skipFlashUSB"},
		Params: []testing.Param{{
			Name:    "normal_warm",
			Fixture: fixture.NormalMode,
			Val: bootModeTestParams{
				resetAfterBoot: true,
				resetType:      firmware.WarmReset,
			},
			ExtraAttr: []string{"firmware_smoke"},
			Timeout:   15 * time.Minute,
		}, {
			Name:    "normal_cold",
			Fixture: fixture.NormalMode,
			Val: bootModeTestParams{
				resetAfterBoot: true,
				resetType:      firmware.ColdReset,
			},
			ExtraAttr: []string{"firmware_smoke"},
			Timeout:   15 * time.Minute,
		}, {
			Name:    "rec_warm",
			Fixture: fixture.NormalMode,
			Val: bootModeTestParams{
				bootToMode:     fwCommon.BootModeRecovery,
				resetAfterBoot: true,
				resetType:      firmware.WarmReset,
			},
			ExtraAttr: []string{"firmware_smoke", "firmware_usb"},
			Timeout:   60 * time.Minute,
		}, {
			Name:    "rec_cold",
			Fixture: fixture.NormalMode,
			Val: bootModeTestParams{
				bootToMode:     fwCommon.BootModeRecovery,
				resetAfterBoot: true,
				resetType:      firmware.ColdReset,
			},
			ExtraAttr: []string{"firmware_smoke", "firmware_usb"},
			Timeout:   60 * time.Minute,
		}, {
			Name:    "dev_usb_cold",
			Fixture: fixture.USBDevModeNoServices,
			Val: bootModeTestParams{
				resetAfterBoot: true,
				resetType:      firmware.ColdReset,
			},
			ExtraAttr: []string{"firmware_usb", "firmware_unstable"},
			Timeout:   60 * time.Minute,
		}, {
			Name:    "dev_warm",
			Fixture: fixture.DevMode,
			Val: bootModeTestParams{
				resetAfterBoot:    true,
				resetType:         firmware.WarmReset,
				checkBootFromMain: true,
			},
			ExtraAttr: []string{"firmware_smoke", "firmware_usb"},
			Timeout:   60 * time.Minute,
		}, {
			Name:    "dev_cold",
			Fixture: fixture.DevMode,
			Val: bootModeTestParams{
				resetAfterBoot:    true,
				resetType:         firmware.ColdReset,
				checkBootFromMain: true,
			},
			ExtraAttr: []string{"firmware_smoke", "firmware_usb"},
			Timeout:   60 * time.Minute,
		}, {
			Name:    "dev_to_rec",
			Fixture: fixture.DevMode,
			Val: bootModeTestParams{
				bootToMode: fwCommon.BootModeRecovery,
			},
			ExtraAttr: []string{"firmware_smoke", "firmware_usb"},
			Timeout:   60 * time.Minute,
		}, {
			Name:    "rec_to_dev",
			Fixture: fixture.RecModeNoServices,
			Val: bootModeTestParams{
				bootToMode: fwCommon.BootModeDev,
			},
			ExtraAttr: []string{"firmware_unstable", "firmware_usb"},
			Timeout:   60 * time.Minute,
		}, {
			Name:    "dev_gbb_to_rec",
			Fixture: fixture.DevModeGBB,
			Val: bootModeTestParams{
				bootToMode: fwCommon.BootModeRecovery,
			},
			ExtraAttr: []string{"firmware_unstable", "firmware_usb"},
			Timeout:   60 * time.Minute,
		}, {
			Name:    "rec_to_dev_gbb",
			Fixture: fixture.RecModeNoServices,
			Val: bootModeTestParams{
				bootToMode:    fwCommon.BootModeDev,
				allowGBBForce: true,
			},
			ExtraAttr: []string{"firmware_unstable", "firmware_usb"},
			Timeout:   60 * time.Minute,
		}, {
			// Verifies that we can go from normal -> dev -> normal without GBB flags.
			Name:    "normal_dev",
			Fixture: fixture.NormalMode,
			Val: bootModeTestParams{
				bootToMode:     fwCommon.BootModeDev,
				allowGBBForce:  false,
				resetAfterBoot: false,
			},
			ExtraAttr: []string{"firmware_bios", "firmware_level2"},
			Timeout:   15 * time.Minute,
		}, {
			Name:    "nogood_screen",
			Fixture: fixture.NormalMode,
			Val: bootModeTestParams{
				bootToMode:          fwCommon.BootModeDev,
				checkToNoGoodScreen: true,
			},
			ExtraAttr: []string{"firmware_unstable", "firmware_usb"},
			Timeout:   30 * time.Minute,
		}, {
			Name:    "broken_screen",
			Fixture: fixture.NormalMode,
			Val: bootModeTestParams{
				bootToMode:          fwCommon.BootModeRecovery,
				checkToBrokenScreen: true,
				resetType:           firmware.ColdReset,
				resetAfterBoot:      true,
			},
			ExtraAttr: []string{"firmware_unstable", "firmware_usb"},
			Timeout:   60 * time.Minute,
		}},
	})
}

func BootMode(ctx context.Context, s *testing.State) {
	tc := s.Param().(bootModeTestParams)
	pv := s.FixtValue().(*fixture.Value)
	h := pv.Helper

	if err := h.RequireServo(ctx); err != nil {
		s.Fatal("Failed to init servo: ", err)
	}

	ms, err := firmware.NewModeSwitcher(ctx, h)
	if err != nil {
		s.Fatal("Creating mode switcher: ", err)
	}

	// Report ModeSwitcherType, for debugging.
	if err := h.RequireConfig(ctx); err != nil {
		s.Fatal("Requiring config")
	}
	s.Log("Mode switcher type: ", h.Config.ModeSwitcherType)

	if err := h.RequireServo(ctx); err != nil {
		s.Fatal("Error opening servo: ", err)
	}
	if tc.bootToMode == fwCommon.BootModeRecovery || tc.checkBootFromMain {
		skipFlashUSB := false
		if skipFlashUSBStr, ok := s.Var("firmware.skipFlashUSB"); ok {
			skipFlashUSB, err = strconv.ParseBool(skipFlashUSBStr)
			if err != nil {
				s.Fatalf("Invalid value for var firmware.skipFlashUSB: got %q, want true/false", skipFlashUSBStr)
			}
		}
		cs := s.CloudStorage()
		if skipFlashUSB {
			cs = nil
		}
		if err := h.SetupUSBKey(ctx, cs); err != nil {
			s.Fatal("USBKey not working: ", err)
		}
	}

	// Double-check that DUT starts in the right mode.
	if curr, err := h.Reporter.CurrentBootMode(ctx); err != nil {
		s.Fatal("Checking boot mode at beginning of test: ", err)
	} else if curr != pv.BootMode {
		s.Logf("DUT started in boot mode %s. Setting up %s", curr, pv.BootMode)
		if err = ms.RebootToMode(ctx, pv.BootMode); err != nil {
			s.Fatalf("Failed to set up %s mode: %s", pv.BootMode, err)
		}
	}
	var usbdev string
	if tc.bootToMode != "" {
		// Switch to tc.bootToMode.
		// RebootToMode ensures that the DUT winds up in the expected boot mode afterward.
		var opts []firmware.ModeSwitchOption
		if tc.allowGBBForce {
			opts = append(opts, firmware.AllowGBBForce)
		} else if !pv.ForcesDevMode {
			// Don't check the dev-force GBB flag if there's no reason for it to have been set.
			opts = append(opts, firmware.AssumeGBBFlagsCorrect)
		}

		if tc.checkToBrokenScreen || tc.checkToNoGoodScreen {
			// Call h.CheckUSBOnServoHost before booting from usb device again
			// in order to get the usbdev and pass it to CheckBrokenScreen.
			usbdev, err = h.CheckUSBOnServoHost(ctx)
			if err != nil {
				s.Fatal("Failed to check the usb key: ", err)
			}
			s.Log("USB path: ", usbdev)
			if tc.checkToNoGoodScreen {
				opts = append(opts, firmware.CheckToNoGoodScreen)
				// An invalid USB is required to check for the NOGOOD screen.
				if err := h.FormatUSB(ctx, usbdev); err != nil {
					s.Fatal("Failed to format the USB: ", err)
				}
			}
		}
		s.Logf("Transitioning to %s mode with options %+v", tc.bootToMode, opts)
		if err = ms.RebootToMode(ctx, tc.bootToMode, opts...); err != nil {
			s.Fatalf("Error during transition from %s to %s: %v", pv.BootMode, tc.bootToMode, err)
		}
		s.Log("Transition completed successfully")
	}

	if tc.checkToBrokenScreen {
		// Verify DUT reaches 'Broken Screen' with broken_screen test.
		if err := h.CheckBrokenScreen(ctx, usbdev); err != nil {
			s.Fatal("Failed to check Broken Screen: ", err)
		}
		s.Log("Disabling USB connection to DUT")
		if err := h.Servo.SetUSBMuxState(ctx, servo.USBMuxOff); err != nil {
			s.Fatal("Failed to disable 'usb3_mux_sel:dut_sees_usbkey': ", err)
		}
		s.Log("Rebooting the DUT with hard reset")
		if err := h.Servo.SetPowerState(ctx, servo.PowerStateReset); err != nil {
			s.Fatal("Failed to reboot the DUT with hard reset: ", err)
		}
		s.Log("Waiting for connection to DUT")
		reconnectTimeout := 8 * time.Minute
		connectCtx, cancel := context.WithTimeout(ctx, reconnectTimeout)
		defer cancel()
		if err := h.WaitConnect(connectCtx); err != nil {
			s.Fatal("Failed to connect to DUT: ", err)
		}
	}

	// Reset the DUT, if the test case calls for it.
	// ModeAwareReboot ensures the DUT winds up in the expected boot mode afterward.
	if tc.resetAfterBoot {
		s.Logf("Resetting DUT (resetType=%v)", tc.resetType)
		if err := ms.ModeAwareReboot(ctx, tc.resetType); err != nil {
			s.Fatal("Error resetting DUT: ", err)
		}
		// See the doc for ModeAwareReboot, the boot mode should be unchanged except that Recovery goes to normal.
		if curr, err := h.Reporter.CurrentBootMode(ctx); err != nil {
			s.Fatal("Failed to determine DUT boot mode: ", err)
		} else if curr != pv.BootMode && pv.BootMode != fwCommon.BootModeRecovery {
			s.Fatalf("Wrong boot mode: got %q, want %q", curr, pv.BootMode)
		} else if curr != fwCommon.BootModeNormal && pv.BootMode == fwCommon.BootModeRecovery {
			s.Fatalf("Wrong boot mode: got %q, want %q", curr, fwCommon.BootModeNormal)
		}
		s.Log("Reset completed successfully")
	} else {
		// Verify the boot mode and then reboot to normal.
		if curr, err := h.Reporter.CurrentBootMode(ctx); err != nil {
			s.Fatal("Failed to determine DUT boot mode: ", err)
		} else if curr != tc.bootToMode {
			s.Fatalf("Wrong boot mode: got %q, want %q", curr, pv.BootMode)
		} else if curr != fwCommon.BootModeNormal {
			s.Logf("Transitioning back from %s to normal mode", curr)
			if err = ms.RebootToMode(ctx, fwCommon.BootModeNormal); err != nil {
				s.Fatalf("Error returning from %s to %s: %+v", curr, fwCommon.BootModeNormal, err)
			}
			s.Log("Transition completed successfully")
		}
	}

	// Check that DUT can boot from the main storage despite USB device attached.
	if tc.checkBootFromMain {
		// Ensure that mainfw_act returns A, and if not set up crossystem param
		// for the device to boot from firmware A next time.
		mainfwAct, err := h.Reporter.CrossystemParam(ctx, reporters.CrossystemParamMainfwAct)
		if err != nil {
			s.Fatal("Failed to get crossystem mainfw_act: ", err)
		}
		if mainfwAct != "A" {
			s.Log("Current mainfw_act not set to A. Attempting to set the device to boot from A during next reboot")
			if err := h.DUT.Conn().CommandContext(ctx, "crossystem", "fw_try_next=A").Run(); err != nil {
				s.Fatal("Failed to set 'crossystem fw_try_next=A': ", err)
			}
		}

		s.Log("Enabling USB connection to DUT")
		if err := h.Servo.SetUSBMuxState(ctx, servo.USBMuxDUT); err != nil {
			s.Fatal("Failed to set 'usb3_mux_sel:dut_sees_usbkey': ", err)
		}

		s.Log("Power-cycling DUT with a warm reset")
		if err := h.Servo.SetPowerState(ctx, servo.PowerStateWarmReset); err != nil {
			s.Fatal("Failed to reboot DUT by servo: ", err)
		}

		s.Log("Waiting for DUT to power ON")
		waitConnectCtx, cancelWaitConnect := context.WithTimeout(ctx, 5*time.Minute)
		defer cancelWaitConnect()

		if err := s.DUT().WaitConnect(waitConnectCtx); err != nil {
			s.Fatal("Failed to reconnect to DUT: ", err)
		}

		s.Log("Checking that DUT has booted from main")
		bootedFromRemovableDevice, err := h.Reporter.BootedFromRemovableDevice(ctx)
		if err != nil {
			s.Fatal("Could not determine boot device type: ", err)
		}
		if bootedFromRemovableDevice {
			s.Fatalf("DUT did not boot from the internal device: got %v, want false", bootedFromRemovableDevice)
		}

		s.Log("Checking the value of mainfw_act after reboot")
		mainfwAct, err = h.Reporter.CrossystemParam(ctx, reporters.CrossystemParamMainfwAct)
		if err != nil {
			s.Fatal("Failed to get crossystem mainfw_act: ", err)
		}
		if mainfwAct != "A" {
			s.Fatalf("Expected mainfw_act:A but got mainfw_act:%s", mainfwAct)
		}
	}
}

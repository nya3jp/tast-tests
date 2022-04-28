// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package power

import (
	"context"
	"time"

	"chromiumos/tast/common/servo"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/remote/bundles/cros/power/powerutils"
	"chromiumos/tast/remote/powercontrol"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

type usbDeviceTestParam struct {
	iter                int
	usbSpeed            string
	noOfConnectedDevice int
	usbDeviceClassName  string
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         USBDeviceFunctionality,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Verifies USB device functionality before and after cold boot",
		Contacts:     []string{"ambalavanan.m.m@intel.com", "intel-chrome-system-automation-team@intel.com"},
		ServiceDeps:  []string{"tast.cros.security.BootLockboxService"},
		SoftwareDeps: []string{"chrome", "reboot"},
		VarDeps:      []string{"servo"},
		HardwareDeps: hwdep.D(hwdep.ChromeEC()),
		Params: []testing.Param{{
			Name: "cold_boot",
			Val: usbDeviceTestParam{
				iter:                1,
				usbSpeed:            "1.5M",
				noOfConnectedDevice: 1,
				usbDeviceClassName:  "Human Interface Device",
			},
			Timeout: 5 * time.Minute,
		}, {
			Name: "cold_boot_stress",
			Val: usbDeviceTestParam{
				iter:                10,
				usbSpeed:            "1.5M",
				noOfConnectedDevice: 2,
				usbDeviceClassName:  "Human Interface Device",
			},
			Timeout: 20 * time.Minute,
		},
		}})
}

func USBDeviceFunctionality(ctx context.Context, s *testing.State) {
	ctxForCleanUp := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 2*time.Minute)
	defer cancel()

	dut := s.DUT()
	testParam := s.Param().(usbDeviceTestParam)

	servoSpec := s.RequiredVar("servo")
	pxy, err := servo.NewProxy(ctx, servoSpec, dut.KeyFile(), dut.KeyDir())
	if err != nil {
		s.Fatal("Failed to connect to servo: ", err)
	}
	defer pxy.Close(ctxForCleanUp)

	defer func(ctx context.Context) {
		testing.ContextLog(ctx, "Performing cleanup")
		if !dut.Connected(ctx) {
			if err := powercontrol.PowerOntoDUT(ctx, pxy, dut); err != nil {
				s.Fatal("Failed to power-on DUT at cleanup: ", err)
			}
		}
	}(ctxForCleanUp)

	// Perform a Chrome login.
	testing.ContextLog(ctx, "Login to Chrome")
	if err := powercontrol.ChromeOSLogin(ctx, dut, s.RPCHint()); err != nil {
		s.Fatal("Failed to login to Chrome: ", err)
	}

	iter := testParam.iter
	for i := 1; i <= iter; i++ {
		testing.ContextLogf(ctx, "Iteration: %d/%d", i, iter)
		// Check for USB Keyboard and/or Mouse detection before cold boot.
		if err := powerutils.USBDeviceDetection(ctx, dut, testParam.usbDeviceClassName, testParam.usbSpeed, testParam.noOfConnectedDevice); err != nil {
			s.Fatal("Failed to detect connected HID device after suspend-resume: ", err)
		}

		powerState := "S5"
		if err := powercontrol.ShutdownAndWaitForPowerState(ctx, pxy, dut, powerState); err != nil {
			s.Fatalf("Failed to shutdown and wait for %q powerstate: %v", powerState, err)
		}

		if err := powercontrol.PowerOntoDUT(ctx, pxy, dut); err != nil {
			s.Fatal("Failed to power on DUT: ", err)
		}

		// Performing chrome login after powering on DUT from coldboot/warmboot.
		if err := powercontrol.ChromeOSLogin(ctx, dut, s.RPCHint()); err != nil {
			s.Fatal("Failed to login to Chrome after power-on from shutdown: ", err)
		}

		// Check for USB Keyboard and/or Mouse detection after cold boot.
		if err := powerutils.USBDeviceDetection(ctx, dut, testParam.usbDeviceClassName, testParam.usbSpeed, testParam.noOfConnectedDevice); err != nil {
			s.Fatal("Failed to detect connected HID device after cold boot: ", err)
		}

		// Perfoming prev_sleep_state check.
		expectedPrevSleepState := 5
		if err := powercontrol.ValidatePrevSleepState(ctx, dut, expectedPrevSleepState); err != nil {
			s.Fatal("Failed to validate previous sleep state: ", err)
		}
	}
}

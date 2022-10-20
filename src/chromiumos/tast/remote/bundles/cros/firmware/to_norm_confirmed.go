// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package firmware

import (
	"context"
	"time"

	fwCommon "chromiumos/tast/common/firmware"
	"chromiumos/tast/common/servo"
	"chromiumos/tast/remote/firmware"
	"chromiumos/tast/remote/firmware/fixture"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         ToNormConfirmed,
		Desc:         "Check that while TO_NORM_CONFIRMED is displayed, ctrl+u, volume and power buttons have no effect",
		Contacts:     []string{"cienet-firmware@cienet.corp-partner.google.com", "chromeos-firmware@google.com"},
		Attr:         []string{"group:firmware", "firmware_unstable", "firmware_usb"},
		SoftwareDeps: []string{"crossystem", "flashrom"},
		// To-do: Some DUTs (i.e. Strongbad) showed behavior of KeyboardDevSwitcher at
		// boot up, even though they are detachables. We're checking for params that
		// could potentially serve as a filter to identify DUT's mode switcher type.
		HardwareDeps: hwdep.D(hwdep.ChromeEC(), hwdep.FormFactor(hwdep.Detachable)),
		Fixture:      fixture.DevMode,
		Timeout:      30 * time.Minute,
	})
}

func ToNormConfirmed(ctx context.Context, s *testing.State) {
	h := s.FixtValue().(*fixture.Value).Helper

	if err := h.RequireServo(ctx); err != nil {
		s.Fatal("Failed to init servo: ", err)
	}

	ms, err := firmware.NewModeSwitcher(ctx, h)
	if err != nil {
		s.Fatal("Creating mode switcher: ", err)
	}

	cs := s.CloudStorage()
	if err := h.SetupUSBKey(ctx, cs); err != nil {
		s.Fatal("USBKey not working: ", err)
	}

	// Enable USB connection to DUT for testing Ctrl+U.
	if err := h.Servo.SetUSBMuxState(ctx, servo.USBMuxDUT); err != nil {
		s.Fatal("Failed to set USB enable: ", err)
	}

	// If any of the buttons tested below are effective, they would prevent
	// DUT from booting into normal mode, and from the main storage.
	testTrigger := []string{"ctrlU", "volumeUp", "volumeDown", "powerButton"}
	for _, trigger := range testTrigger {
		opts := []firmware.ModeSwitchOption{firmware.CheckToNormConfirmed}
		if err := ms.RebootToMode(ctx, fwCommon.BootModeNormal, opts...); err != nil {
			s.Fatal("Failed to boot to normal mode: ", err)
		}
		// Add a short delay to ensure power button released from RebootToMode.
		if err := testing.Sleep(ctx, time.Second); err != nil {
			s.Fatalf("Failed to sleep before testing trigger %s: %v", trigger, err)
		}
		switch trigger {
		case "ctrlU":
			s.Log("Verifying Ctrl+U doesn't trigger a usb boot")
			if err := h.Servo.KeypressWithDuration(ctx, servo.CtrlU, servo.DurTab); err != nil {
				s.Fatal("Failed to make Ctrl+U press: ", err)
			}
		case "volumeUp":
			// In other dev screens, long pressing the volume up button for 3s:
			// DUT boots from USB.
			s.Log("Verifying VolumeUp doesn't trigger a usb boot")
			if err := h.Servo.SetInt(ctx, servo.VolumeUpHold, 3000); err != nil {
				s.Fatal("Failed to make VolumeUp press: ", err)
			}
		case "volumeDown":
			// In other dev screens, long pressing volume down button for 3s:
			// DUT boots into developer mode and from internal disk.
			s.Log("Verifying VolumeDown doesn't trigger boot from main storage in dev mode")
			if err := h.Servo.SetInt(ctx, servo.VolumeDownHold, 3000); err != nil {
				s.Fatal("Failed to make VolumeDown press: ", err)
			}
		case "powerButton":
			// In other dev screens, power button serves as an ENTER key,
			// and pressing it might lead to other screens.
			s.Log("Verifying Power Button doesn't trigger any action")
			if err := h.Servo.KeypressWithDuration(ctx, servo.PowerKey, servo.DurTab); err != nil {
				s.Fatal("Failed to make power button press: ", err)
			}
		}

		waitConnectCtx, cancelWaitConnect := context.WithTimeout(ctx, 2*time.Minute)
		defer cancelWaitConnect()
		if err := h.WaitConnect(waitConnectCtx); err != nil {
			s.Fatal("Failed to reconnect to DUT: ", err)
		}

		// Check for DUT's current boot mode.
		curr, err := h.Reporter.CurrentBootMode(ctx)
		s.Logf("Current Boot Mode: %s", curr)
		if err != nil {
			s.Fatal("Failed to check for boot mode: ", err)
		}
		if curr != fwCommon.BootModeNormal {
			s.Fatalf("Expected boot mode: %s, but got: %s", fwCommon.BootModeNormal, curr)
		}

		s.Log("Checking that DUT has booted from the main storage")
		bootedDeviceType, err := h.Reporter.BootedFromRemovableDevice(ctx)
		if err != nil {
			s.Fatal("Failed to check boot device type: ", err)
		}
		if bootedDeviceType {
			s.Fatal("DUT booted from USB unexpectedly")
		}

		s.Log("Rebooting to dev mode before testing the next trigger")
		if err := ms.RebootToMode(ctx, fwCommon.BootModeDev); err != nil {
			s.Fatal("Failed to boot to dev mode: ", err)
		}
	}
}

// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package firmware

import (
	"context"
	"fmt"
	"strings"
	"time"

	fwCommon "chromiumos/tast/common/firmware"
	"chromiumos/tast/common/servo"
	"chromiumos/tast/common/testexec"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/remote/firmware"
	"chromiumos/tast/remote/firmware/fixture"
	"chromiumos/tast/remote/firmware/reporters"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         DevModeFwScreen,
		Desc:         "Verify the functionality of Ctrl+D while on the dev screen",
		Contacts:     []string{"cienet-firmware@cienet.corp-partner.google.com", "chromeos-firmware@google.com"},
		Attr:         []string{"group:firmware", "firmware_usb"},
		SoftwareDeps: []string{"crossystem"},
		Fixture:      fixture.DevMode,
		Timeout:      20 * time.Minute,
	})
}

func DevModeFwScreen(ctx context.Context, s *testing.State) {
	h := s.FixtValue().(*fixture.Value).Helper

	if err := h.RequireServo(ctx); err != nil {
		s.Fatal("Failed to init servo: ", err)
	}

	if err := h.RequireConfig(ctx); err != nil {
		s.Fatal("Failed to create config: ", err)
	}

	// Save the initial dev_boot_usb value for restoration at the end of test.
	initDevBootUsb, err := h.Reporter.CrossystemParam(ctx, reporters.CrossystemParamDevBootUsb)
	if err != nil {
		s.Fatal("Failed to get crossystem dev_boot_usb: ", err)
	}

	// Save the initial USB mux state for restoration at the end of test.
	usbConnection, err := h.Servo.GetUSBMuxState(ctx)
	if err != nil {
		s.Fatal("Failed to get current USB mux state: ", err)
	}

	// For DUTs using MenuSwitcher, or TabletDetachableSwitcher, we would
	// use a goroutine to keep pressing the <up> key in the background,
	// to prevent exit from firmware screen because of timeout.
	var goRoutineRequired bool
	if h.Config.ModeSwitcherType == firmware.MenuSwitcher || h.Config.ModeSwitcherType == firmware.TabletDetachableSwitcher {
		goRoutineRequired = true
	}

	var holdUp, releaseUp, pressedKey string
	if goRoutineRequired {
		pressedKey = "<up>"
		row, col, err := h.Servo.GetKeyRowCol(pressedKey)
		if err != nil {
			s.Fatalf("Failed to get key column and row of %s: %v", pressedKey, err)
		}

		holdUp = fmt.Sprintf("kbpress %d %d 1", col, row)
		releaseUp = fmt.Sprintf("kbpress %d %d 0", col, row)
	}

	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()
	/*
		Cleanups are done in a deferred call for the following purposes:
		1. Restore dev_boot_usb value.
		2. Restore USB mux state.
		3. Release key used to extend staying at the dev fw screen.
	*/
	defer func(ctx context.Context, resetUSBMUX servo.USBMuxState, resetDevUSBBoot, pressedKey, releaseKey string) {
		if err := h.DUT.Conn().CommandContext(ctx, "crossystem", fmt.Sprintf("dev_boot_usb=%s", resetDevUSBBoot)).Run(testexec.DumpLogOnError); err != nil {
			s.Fatalf("Failed to reset crossystem dev_boot_usb back to %s: %v", resetDevUSBBoot, err)
		}

		if err := h.Servo.SetUSBMuxState(ctx, resetUSBMUX); err != nil {
			s.Fatalf("Failed to reset 'usb3_mux_sel:%s': %v", resetUSBMUX, err)
		}
		if pressedKey != "" {
			if err := h.Servo.RunECCommand(ctx, releaseKey); err != nil {
				s.Fatalf("Failed to release %s key: %v", releaseUp, err)
			}
		}
	}(cleanupCtx, usbConnection, initDevBootUsb, pressedKey, releaseUp)

	// Disable usb connection.
	if err := h.Servo.SetUSBMuxState(ctx, servo.USBMuxHost); err != nil {
		s.Fatalf("Failed to set 'usb3_mux_sel:%s': %v", servo.USBMuxHost, err)
	}

	/*
		Ctrl+D functionality is tested for scenarios as follows:
		1. Set 'crossystem dev_boot_usb=0', reboot, press ctrl+D, and
			expect boot success into dev mode and from main storage.
		2. Set 'crossystem dev_boot_usb=1', reboot, press ctrl+D, and
			expect boot success into dev mode and from main storage.
		3. Set 'crossystem dev_boot_usb=1', reboot, press ctrl+U,
			expect boot to fail. But, following up with ctrl+D would
			allow boot to continue into dev mode and from main storage.
	*/
	done := make(chan bool, 1)
	defer func() {
		close(done)
	}()
	for _, steps := range []struct {
		devBootUSB      string
		testedShortCuts []servo.KeypressControl
	}{
		{"0", []servo.KeypressControl{servo.CtrlD}},
		{"1", []servo.KeypressControl{servo.CtrlD}},
		{"1", []servo.KeypressControl{servo.CtrlU, servo.CtrlD}},
	} {
		s.Logf("Setting dev boot usb value to %s", steps.devBootUSB)
		if err := h.DUT.Conn().CommandContext(ctx, "crossystem", fmt.Sprintf("dev_boot_usb=%s", steps.devBootUSB)).Run(testexec.DumpLogOnError); err != nil {
			s.Fatalf("Failed to set crossystem dev_boot_usb to %s", steps.devBootUSB)
		}

		s.Log("Power-cycling DUT with a warm reset")
		if err := h.Servo.SetPowerState(ctx, servo.PowerStateWarmReset); err != nil {
			s.Fatal("Failed to warm reset DUT: ", err)
		}

		s.Log("Waiting for DUT to get into firmware screen")
		if err := testing.Sleep(ctx, h.Config.FirmwareScreen); err != nil {
			s.Fatalf("Failed to sleep for %s to wait for firmware screen: %v", h.Config.FirmwareScreen, err)
		}

		if goRoutineRequired {
			s.Log("Pressing <up> key in the background")
			go func() {
				for {
					if err := h.Servo.RunECCommand(ctx, holdUp); err != nil {
						s.Fatalf("Failed to press and hold %s key: %v", holdUp, err)
					}
					// Delay for 2 seconds to ensure that the press was effective.
					if err := testing.Sleep(ctx, 2*time.Second); err != nil {
						s.Fatal("Failed to sleep for 2 seconds: ", err)
					}

					if err := h.Servo.RunECCommand(ctx, releaseUp); err != nil {
						s.Fatalf("Failed to release %s key: %v", releaseUp, err)
					}

					select {
					case <-done:
						return
					default:
					}
				}
			}()

			// The default timeout at the firmware screen is 30 seconds.
			// Check that pressing the <up> key has worked around this timeout,
			// and that DUT remains disconnected.
			s.Log("Checking for DUT disconnected")
			waitConnectShortCtx, cancelWaitConnectShort := context.WithTimeout(ctx, 1*time.Minute)
			defer cancelWaitConnectShort()
			err := h.WaitConnect(waitConnectShortCtx)
			if err == nil {
				s.Fatal("DUT exited fw screen and reconnected unexpectedly")
			}
			if !strings.Contains(err.Error(), context.DeadlineExceeded.Error()) {
				s.Fatal("Unexpected error in waiting for DUT to reconnect: ", err)
			}
			s.Log("DUT is still at dev screen")
		}

		for _, shortcut := range steps.testedShortCuts {
			s.Logf("Testing shortcuts %q", shortcut)
			if err := h.Servo.KeypressWithDuration(ctx, shortcut, servo.DurTab); err != nil {
				s.Fatalf("Failed to press %s: %v", shortcut, err)
			}

			s.Log("Sleeping for 2 second")
			if err := testing.Sleep(ctx, 2*time.Second); err != nil {
				s.Fatal("Failed to sleep for 2 seconds: ", err)
			}
		}

		// For DUTs using KeyboardDevSwitcher, pressing a space key would allow
		// an extended stay at the firmware screen. If ctrl+D worked, pressing
		// a space key here would not have any effects, and DUT would
		// eventually boot to ChromeOS. But, if ctrl+D did not work, the space
		// key would stop the boot up process, and DUT would end up disconnected.
		if !goRoutineRequired {
			s.Log(ctx, "Pressing SPACE key to keep DUT in dev screen")
			if err := h.Servo.PressKey(ctx, " ", servo.DurTab); err != nil {
				s.Fatal("Failed to press SPACE to stop in dev screen: ", err)
			}
		}

		s.Log("Waiting for DUT to reconnect")
		waitConnectCtx, cancelWaitConnect := context.WithTimeout(ctx, 2*time.Minute)
		defer cancelWaitConnect()

		if err := h.WaitConnect(waitConnectCtx); err != nil {
			s.Fatal("Failed to reconnect to DUT: ", err)
		}
		if goRoutineRequired {
			done <- true
		}

		s.Log("Checking for DUT in developer mode")
		curr, err := h.Reporter.CurrentBootMode(ctx)
		if err != nil {
			s.Fatal("Failed to check for boot mode: ", err)
		}
		if curr != fwCommon.BootModeDev {
			s.Fatalf("Expected DUT in dev mode, but got: %s", curr)
		}

		s.Log("Checking for boot from main")
		bootedFromRemovableDevice, err := h.Reporter.BootedFromRemovableDevice(ctx)
		if err != nil {
			s.Fatal("Failed to check for boot device type: ", err)
		}
		if bootedFromRemovableDevice {
			s.Fatal("DUT booted from USB unexpectedly")
		}
	}

}

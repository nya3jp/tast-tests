// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package firmware

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	fwCommon "chromiumos/tast/common/firmware"
	"chromiumos/tast/common/servo"
	"chromiumos/tast/common/testexec"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/remote/firmware"
	"chromiumos/tast/remote/firmware/fixture"
	"chromiumos/tast/testing"
)

type params struct {
	//Set up based on the need for a USB.
	usbPresent       bool
	reconnectTimeout time.Duration
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         DevModeFwScreen,
		Desc:         "Verify the functionality of Ctrl+D and Ctrl+U while on the dev screen",
		Contacts:     []string{"cienet-firmware@cienet.corp-partner.google.com", "chromeos-firmware@google.com"},
		Attr:         []string{"group:firmware", "firmware_unstable"},
		SoftwareDeps: []string{"crossystem"},
		Vars:         []string{"firmware.skipFlashUSB"},
		Fixture:      fixture.DevMode,
		Params: []testing.Param{{
			Val: &params{
				usbPresent:       false,
				reconnectTimeout: 2 * time.Minute,
			},
			Timeout: 20 * time.Minute,
		}, {
			Name: "usb",
			Val: &params{
				usbPresent:       true,
				reconnectTimeout: 10 * time.Minute,
			},
			ExtraAttr: []string{"firmware_usb"},
			Timeout:   60 * time.Minute,
		}},
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

	// Set up USB when there is one present, and
	// for cases that depend on it.
	testOpt := s.Param().(*params)
	if testOpt.usbPresent {
		s.Log("Setup USB key")
		skipFlashUSB := false
		if skipFlashUSBStr, ok := s.Var("firmware.skipFlashUSB"); ok {
			var err error
			skipFlashUSB, err = strconv.ParseBool(skipFlashUSBStr)
			if err != nil {
				s.Fatalf("Invalid value for var firmware.skipFlashUSB: got %q, want true/false", skipFlashUSBStr)
			}
		}
		var cs *testing.CloudStorage
		if !skipFlashUSB {
			cs = s.CloudStorage()
		}
		if err := h.SetupUSBKey(ctx, cs); err != nil {
			s.Fatal("USBKey not working: ", err)
		}

		s.Logf("Setting USBMux to %s", servo.USBMuxDUT)
		if err := h.Servo.SetUSBMuxState(ctx, servo.USBMuxDUT); err != nil {
			s.Fatal("Failed to set USBMux: ", err)
		}
	} else {
		s.Logf("Setting USBMux to %s", servo.USBMuxOff)
		if err := h.Servo.SetUSBMuxState(ctx, servo.USBMuxOff); err != nil {
			s.Fatal("Failed to set USBMux: ", err)
		}
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

	// Ensure release on a key that was pressed at the end of test.
	defer func(ctx context.Context, pressedKey, releaseKey string) {
		if pressedKey != "" {
			if err := h.Servo.RunECCommand(ctx, releaseKey); err != nil {
				s.Fatalf("Failed to release %s key: %v", pressedKey, err)
			}
		}
	}(cleanupCtx, pressedKey, releaseUp)

	/*
		Notes: This test is parameterized so that steps 1~4 are run in
		'DevModeFwScreen.usb', and steps 5~8 in 'DevModeFwScreen'.

		Ctrl+D/Ctrl+U functionality is tested for scenarios as follows:
		1. Set 'crossystem dev_boot_usb=0', attach USB device to DUT,
			reboot, press ctrl+D and expect boot success into dev mode
			and from main storage.
		2. Set 'crossystem dev_boot_usb=1', attach USB device to DUT,
			reboot, press ctrl+D and expect boot success into dev mode
			and from main storage.
		3. Set 'crossystem dev_boot_usb=0', attach USB device to DUT,
			reboot, press ctrl+U, expect boot to fail. But, following up
			with ctrl+D would allow boot to continue into dev mode and
			from main storage.
		4. Set 'crossystem dev_boot_usb=1', attach USB device to DUT,
			reboot, press ctrl+U, expect boot from USB.
		5. Set 'crossystem dev_boot_usb=0', detach USB device from DUT,
			reboot, press ctrl+D and expect boot success into dev mode
			and from main storage.
		6. Set 'crossystem dev_boot_usb=1', detach USB device from DUT,
			reboot, press ctrl+D and expect boot success into dev mode
			and from main storage.
		7. Set 'crossystem dev_boot_usb=0', detach USB device from DUT,
			reboot, press ctrl+U expect boot to fail. But, following up
			with ctrl+D would allow boot to continue into dev mode and
			from main storage.
		8. Set 'crossystem dev_boot_usb=1', detach USB device from DUT,
			reboot, press ctrl+U, expect boot to fail. But, following up
			with ctrl+D would allow boot to continue into dev mode and
			from main storage.
	*/

	done := make(chan bool, 1)
	defer func() {
		close(done)
	}()
	for _, steps := range []struct {
		devBootUSB      string
		testedShortCuts []servo.KeypressControl
		usbRequired     bool
		expectedMode    fwCommon.BootMode
	}{
		{"0", []servo.KeypressControl{servo.CtrlD}, true, fwCommon.BootModeDev},
		{"1", []servo.KeypressControl{servo.CtrlD}, true, fwCommon.BootModeDev},
		{"0", []servo.KeypressControl{servo.CtrlU, servo.CtrlD}, true, fwCommon.BootModeDev},
		{"1", []servo.KeypressControl{servo.CtrlU, servo.CtrlD}, true, fwCommon.BootModeUSBDev},
		{"0", []servo.KeypressControl{servo.CtrlD}, false, fwCommon.BootModeDev},
		{"1", []servo.KeypressControl{servo.CtrlD}, false, fwCommon.BootModeDev},
		{"0", []servo.KeypressControl{servo.CtrlU, servo.CtrlD}, false, fwCommon.BootModeDev},
		{"1", []servo.KeypressControl{servo.CtrlU, servo.CtrlD}, false, fwCommon.BootModeDev},
	} {
		// Run test steps that depend on a usb when there's one present.
		if testOpt.usbPresent != steps.usbRequired {
			continue
		}

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
					if err := func() error {
						if err := h.Servo.RunECCommand(ctx, holdUp); err != nil {
							return errors.Wrapf(err, "failed to press and hold %s key", pressedKey)
						}
						// Delay for 2 seconds to ensure that the press was effective.
						if err := testing.Sleep(ctx, 2*time.Second); err != nil {
							return errors.Wrap(err, "failed to sleep for 2 seconds")
						}
						if err := h.Servo.RunECCommand(ctx, releaseUp); err != nil {
							return errors.Wrapf(err, "failed to release %s key", pressedKey)
						}
						return nil
					}(); err != nil && !errors.Is(err, context.Canceled) {
						s.Fatal("Found unexpected error: ", err)
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
		waitConnectCtx, cancelWaitConnect := context.WithTimeout(ctx, testOpt.reconnectTimeout)
		defer cancelWaitConnect()

		if err := h.WaitConnect(waitConnectCtx); err != nil {
			s.Fatal("Failed to reconnect to DUT: ", err)
		}
		if goRoutineRequired {
			done <- true
		}

		s.Logf("Checking for DUT in %s mode", steps.expectedMode)
		curr, err := h.Reporter.CurrentBootMode(ctx)
		if err != nil {
			s.Fatal("Failed to check for boot mode: ", err)
		}
		if curr != steps.expectedMode {
			s.Fatalf("Expected DUT in %s mode, but got: %s", steps.expectedMode, curr)
		}
	}

}

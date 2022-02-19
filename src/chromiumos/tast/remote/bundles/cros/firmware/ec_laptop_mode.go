// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package firmware

import (
	"context"
	"strings"
	"time"

	"github.com/golang/protobuf/ptypes/empty"

	"chromiumos/tast/common/servo"
	"chromiumos/tast/errors"
	"chromiumos/tast/remote/firmware"
	"chromiumos/tast/remote/firmware/fixture"
	"chromiumos/tast/services/cros/graphics"
	"chromiumos/tast/services/cros/ui"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         ECLaptopMode,
		LacrosStatus: testing.LacrosVariantUnknown,
		Desc:         "Checks that power button actions at varied durations behave as expected in laptop mode",
		Contacts:     []string{"cienet-firmware@cienet.corp-partner.google.com", "chromeos-firmware@google.com"},
		Attr:         []string{"group:firmware", "firmware_unstable"},
		SoftwareDeps: []string{"chrome"},
		VarDeps:      []string{"ui.signinProfileTestExtensionManifestKey"},
		Fixture:      fixture.NormalMode,
		ServiceDeps:  []string{"tast.cros.ui.ScreenLockService", "tast.cros.ui.PowerMenuService", "tast.cros.graphics.ScreenshotService", "tast.cros.firmware.UtilsService"},
		HardwareDeps: hwdep.D(hwdep.ChromeEC(), hwdep.Battery()),
		Params: []testing.Param{{
			ExtraHardwareDeps: hwdep.D(hwdep.FormFactor(hwdep.Convertible)),
			Val:               "convertible",
		}, {
			Name:              "clamshell",
			ExtraHardwareDeps: hwdep.D(hwdep.FormFactor(hwdep.Clamshell)),
			Val:               "clamshell",
		}, {
			Name:              "detachable",
			ExtraHardwareDeps: hwdep.D(hwdep.FormFactor(hwdep.Detachable), hwdep.Keyboard()),
			Val:               "detachable",
		}},
	})
}

func ECLaptopMode(ctx context.Context, s *testing.State) {
	atSignin := "atSignin"
	afterSignin := "afterSignin"
	atLockScreen := "atLockScreen"

	// powerMenuOffPollOptions is the time to wait for DUT to check power menu has been turned off.
	powerMenuOffPollOptions := testing.PollOptions{
		Timeout:  15 * time.Second,
		Interval: 1 * time.Second,
	}

	h := s.FixtValue().(*fixture.Value).Helper
	h.CloseRPCConnection(ctx)

	if err := h.RequireServo(ctx); err != nil {
		s.Fatal("Failed to connect to servo: ", err)
	}
	if err := h.RequireConfig(ctx); err != nil {
		s.Fatal("Failed to get config: ", err)
	}

	ms, err := firmware.NewModeSwitcher(ctx, h)
	if err != nil {
		s.Fatal("Failed to create mode switcher: ", err)
	}

	checkLaptopMode := func(ctx context.Context) error {
		if err := h.RequireRPCUtils(ctx); err != nil {
			return errors.Wrap(err, "requiring RPC utils")
		}
		if _, err := h.RPCUtils.NewChrome(ctx, &empty.Empty{}); err != nil {
			return errors.Wrap(err, "failed to create instance of chrome")
		}
		res, err := h.RPCUtils.EvalTabletMode(ctx, &empty.Empty{})
		if err != nil {
			return err
		}
		if res.TabletModeEnabled {
			return errors.New("DUT's tablet mode status is currently on")
		}
		return nil
	}

	dutType := s.Param().(string)
	if dutType == "convertible" {
		s.Log("Checking initial state of laptop mode")
		if err := checkLaptopMode(ctx); err != nil {
			s.Fatal("Unable to check DUT in laptop mode before powering off: ", err)
		}

		s.Log("Setting power off")
		if err := ms.PowerOff(ctx); err != nil {
			s.Fatal("Failed to power off DUT: ", err)
		}

		s.Log("Tapping on the power button to power on DUT")
		if err := h.Servo.KeypressWithDuration(ctx, servo.PowerKey, servo.DurTab); err != nil {
			s.Fatal("Failed to tap on the power button: ", err)
		}

		s.Log("Waiting for the boot to complete")
		waitConnectCtx, cancelWaitConnect := context.WithTimeout(ctx, 2*time.Minute)
		defer cancelWaitConnect()
		if err := h.WaitConnect(waitConnectCtx); err != nil {
			s.Fatal("Failed to reconnect to DUT: ", err)
		}
		h.CloseRPCConnection(ctx)
		s.Log("Checking DUT remains in laptop mode after having rebooted")
		if err := checkLaptopMode(ctx); err != nil {
			s.Fatal("Unable to determine if DUT is in laptop mode after reboot: ", err)
		}
	}

	// Connect to the RPC service on the DUT.
	if err := h.RequireRPCClient(ctx); err != nil {
		s.Fatal("Failed to connect to the RPC service on the DUT: ", err)
	}

	// The checkDisplay function checks whether display is on/off
	// by attempting to capture a screenshot. If capturing a screenshot fails,
	// the returned stderr message, "CRTC not found. Is the screen on?", would
	// be returned and checked. Also, in this case, since the screenshot file
	// saved is not needed, it would always get deleted immediately.
	screenshotService := graphics.NewScreenshotServiceClient(h.RPCClient.Conn)
	checkDisplay := func(ctx context.Context) error {
		if _, err := screenshotService.CaptureScreenAndDelete(ctx, &empty.Empty{}); err != nil {
			return errors.Wrap(err, "failed to take screenshot")
		}
		return nil
	}

	// The checkPowerMenu function will check whether the power menu is present
	// after holding the power button for about one second.
	powerMenuService := ui.NewPowerMenuServiceClient(h.RPCClient.Conn)
	checkPowerMenu := func(ctx context.Context, expected bool) error {
		res, err := powerMenuService.PowerMenuPresent(ctx, &empty.Empty{})
		if err != nil {
			return errors.Wrap(err, "failed to check power menu")
		}
		if res.IsMenuPresent != expected {
			return errors.Errorf("expected %t but got: %t", expected, res.IsMenuPresent)
		}
		return nil
	}

	screenLockService := ui.NewScreenLockServiceClient(h.RPCClient.Conn)
	lockScreen := func(ctx context.Context) error {
		if _, err := screenLockService.Lock(ctx, &empty.Empty{}); err != nil {
			return errors.Wrap(err, "failed to lock screen")
		}
		return nil
	}

	repeatedSteps := func(testCase string) {
		switch testCase {
		case atSignin:
			s.Logf("------------------------Perform testCase: %s------------------------", testCase)
			// Chrome instance is necessary to check for the presence of the power menu.
			// Start Chrome to show the login screen with a user pod.
			manifestKey := s.RequiredVar("ui.signinProfileTestExtensionManifestKey")
			signInRequest := ui.NewChromeRequest{
				Login: false,
				Key:   manifestKey,
			}
			if _, err := powerMenuService.NewChrome(ctx, &signInRequest); err != nil {
				s.Fatal("Failed to create new chrome instance with no login for powerMenuService: ", err)
			}
			// Close chrome instance and restart one again with login.
			defer powerMenuService.CloseChrome(ctx, &empty.Empty{})

		case afterSignin:
			s.Logf("------------------------Perform testCase: %s------------------------", testCase)
			// Start Chrome and log in as testuser.
			signInRequest := ui.NewChromeRequest{
				Login: true,
				Key:   "",
			}
			if _, err := powerMenuService.NewChrome(ctx, &signInRequest); err != nil {
				s.Fatal("Failed to create new chrome instance with no login for powerMenuService: ", err)
			}

		case atLockScreen:
			s.Logf("------------------------Perform testCase: %s------------------------", testCase)
			// Reuse the existing login session from same user.
			if _, err := screenLockService.ReuseChrome(ctx, &empty.Empty{}); err != nil {
				s.Fatal("Failed to reuse existing chrome session for screenLockService: ", err)
			}
			s.Log("Locking Screen")
			if err := lockScreen(ctx); err != nil {
				s.Fatal("Lock-screen did not behave as expected: ", err)
			}
			// Close chrome instance at the end of the test.
			defer screenLockService.CloseChrome(ctx, &empty.Empty{})
		}

		// Wait for some delay for display to fully settle down,
		// after a transition between Chrome sessions.
		if err := testing.Sleep(ctx, 3*time.Second); err != nil {
			s.Fatal("Failed to sleep: ", err)
		}

		s.Log("Tapping on the power button")
		if err := h.Servo.KeypressWithDuration(ctx, servo.PowerKey, servo.DurTab); err != nil {
			s.Fatal("Failed to tap on the power button: ", err)
		}

		// Wait for a short delay.
		if err := testing.Sleep(ctx, time.Second); err != nil {
			s.Fatal("Failed to sleep: ", err)
		}

		// When DUT is a detachable, a tap on the power button would turn off the screen.
		// We are yet to verify if this would be the case in general for all detachables.
		// As of now, we've observed such behavior on Kukui and Soraka.
		if dutType != "detachable" {
			s.Log("Checking that display remains on")
			if err := checkDisplay(ctx); err != nil {
				s.Fatal("Error in verifying display on: ", err)
			}
		} else {
			s.Log("Checking that display remains off")
			err := checkDisplay(ctx)
			if err == nil {
				s.Fatal("Unexpectedly able to take screenshot after setting display power off")
			}
			if !strings.Contains(err.Error(), "CRTC not found. Is the screen on?") {
				s.Fatal("Unexpected error when taking screenshot: ", err)
			}

			// Turn the screen on before testing the power menu.
			s.Log("Tapping on the power button to turn the screen on again")
			if err := h.Servo.KeypressWithDuration(ctx, servo.PowerKey, servo.DurTab); err != nil {
				s.Fatal("Failed to tap on the power button: ", err)
			}

			// Wait for a short delay.
			if err := testing.Sleep(ctx, time.Second); err != nil {
				s.Fatal("Failed to sleep: ", err)
			}

			s.Log("Checking that display remains on")
			if err := checkDisplay(ctx); err != nil {
				s.Fatal("Error in verifying display on: ", err)
			}
		}

		s.Log("Pressing and holding the power button for 1 second to turn on the power menu")
		if err := h.Servo.KeypressWithDuration(ctx, servo.PowerKey, servo.DurPress); err != nil {
			s.Fatal("Failed to press and hold on the power button for 1 second: ", err)
		}

		// Wait for a short delay.
		if err := testing.Sleep(ctx, time.Second); err != nil {
			s.Fatal("Failed to sleep: ", err)
		}

		// Check that pressing the power button for 1 second brings up the power menu.
		s.Log("Checking that the power menu has appeared")
		if err := checkPowerMenu(ctx, true); err != nil {
			s.Fatal("Failed to check the power menu: ", err)
		}

		s.Log("Checking that power menu items are displayed correctly")
		var expected []string
		switch testCase {
		case atSignin:
			expected = []string{"Shut down", "Feedback"}
		case afterSignin:
			expected = []string{"Shut down", "Sign out", "Lock", "Feedback"}
		case atLockScreen:
			expected = []string{"Shut down", "Sign out"}
		}

		// The PowerMenuItem function will return the names of power menu items.
		res, err := powerMenuService.PowerMenuItem(ctx, &empty.Empty{})
		if err != nil {
			s.Fatal("Failed to get power menu items: ", err)
		}
		if len(res.MenuItems) != len(expected) {
			s.Fatalf("Found mismatched number of power menu items: expected: %v but got: %v", expected, res.MenuItems)
		}
		for _, receivedItem := range res.MenuItems {
			check := false
			for _, expetedItem := range expected {
				if receivedItem == expetedItem {
					check = true
				}
			}
			if !check {
				s.Fatalf("Did not find %s on the power menu", receivedItem)
			}
		}

		// When DUT is a detachable, a tap on the power button will
		// turn off the screen, instead of the power menu.
		if dutType != "detachable" {
			s.Log("Tapping on the power button to turn off the power menu")
			if err := testing.Poll(ctx, func(ctx context.Context) error {
				if err := h.Servo.KeypressWithDuration(ctx, servo.PowerKey, servo.DurTab); err != nil {
					return testing.PollBreak(errors.Wrap(err, "failed to tap on the power button"))
				}

				// Wait for a short delay.
				if err := testing.Sleep(ctx, time.Second); err != nil {
					s.Fatal("Failed to sleep: ", err)
				}

				if err := checkPowerMenu(ctx, false); err != nil {
					return errors.Wrap(err, "failed to check the power menu")
				}
				return nil
			}, &powerMenuOffPollOptions); err != nil {
				s.Fatal("Failed to turn off the power menu: ", err)
			}
		}

		s.Log("Pressing and holding the power button for 2~3 seconds")
		if err := h.Servo.KeypressWithDuration(ctx, servo.PowerKey, servo.Dur((h.Config.HoldPwrButtonPowerOff)/3)); err != nil {
			s.Fatal("Failed to press and hold on the power button for 3 second: ", err)
		}

		// To avoid false positive cases, delay for checking on the power states.
		// Without this delay, if DUT turns down after 2~3 seconds, checking on
		// the power state during shutdown might still return S0.
		s.Logf("Sleeping for %v before checking on the power state", h.Config.Shutdown)
		if err := testing.Sleep(ctx, h.Config.Shutdown); err != nil {
			s.Fatal("Failed to sleep: ", err)
		}

		s.Log("Checking that the power state remains S0")
		if err := h.WaitForPowerStates(ctx, firmware.PowerStateInterval, firmware.PowerStateTimeout, "S0"); err != nil {
			s.Fatal("Failed to get S0 powerstate: ", err)
		}
	}
	var testCases = []string{atSignin, afterSignin, atLockScreen}
	for _, testCase := range testCases {
		repeatedSteps(testCase)
	}

	s.Log("Setting power off")
	if err := ms.PowerOff(ctx); err != nil {
		s.Fatal("Failed to power off DUT: ", err)
	}

	s.Log("Pressing and holding the power button for 3~8 seconds")
	if err := h.Servo.KeypressWithDuration(ctx, servo.PowerKey, servo.Dur(h.Config.HoldPwrButtonPowerOff)); err != nil {
		s.Fatal("Failed to press and hold on the power button for 3~8 second: ", err)
	}

	// On some DUTs, pressing 3~8 seconds would leave them in the off state, while some others would power on.
	// We are currently in the process of defining DUT categories for the respective behaviors.
	s.Log("Waiting for power state to become G3 or S0")
	if err := h.WaitForPowerStates(ctx, firmware.PowerStateInterval, firmware.PowerStateTimeout, "G3", "S0"); err != nil {
		s.Fatal("Failed to get G3 or S0 powerstate: ", err)
	}

	s.Log("Getting powerstate information")
	powerState, err := h.Servo.GetECSystemPowerState(ctx)
	if err != nil {
		s.Fatal("Failed to get powerstate: ", err)
	}
	s.Logf("Power state: %s", powerState)

	// While DUT is powered off, the test would be pending until timeout.
	// We are restoring DUT's power state to on here to finish the test.
	if powerState == "G3" {
		h.Servo.SetPowerState(ctx, servo.PowerStateOn)
	}
}

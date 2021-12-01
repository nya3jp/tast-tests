// Copyright 2021 The Chromium OS Authors. All rights reserved.
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
	"chromiumos/tast/services/cros/graphics"
	"chromiumos/tast/services/cros/ui"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         ECLaptopMode,
		Desc:         "Checks that power button actions at varied durations behave as expected in laptop mode",
		Contacts:     []string{"cienet-firmware@cienet.corp-partner.google.com", "chromeos-firmware@google.com"},
		Attr:         []string{"group:firmware", "firmware_experimental"},
		SoftwareDeps: []string{"chrome"},
		VarDeps:      []string{"ui.signinProfileTestExtensionManifestKey"},
		Fixture:      fixture.NormalMode,
		ServiceDeps:  []string{"tast.cros.ui.ScreenLockService", "tast.cros.ui.PowerMenuService", "tast.cros.graphics.ScreenshotService"},
		HardwareDeps: hwdep.D(hwdep.ChromeEC()),
		Params: []testing.Param{{
			Name:              "clamshell",
			ExtraHardwareDeps: hwdep.D(hwdep.FormFactor(hwdep.Clamshell)),
			Val:               true,
		}, {
			ExtraHardwareDeps: hwdep.D(hwdep.SkipOnFormFactor(hwdep.Clamshell)),
			Val:               false,
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
	ffIsClamshell := s.Param().(bool)

	// checkLaptopMode checks if DUT is in laptop mode by comparing its lid angle against the tablet mode settings.
	checkLaptopMode := func(ctx context.Context) error {
		// Get the current lid angle, using ectool.
		data, err := s.DUT().Conn().CommandContext(ctx, "ectool", "motionsense", "lid_angle").Output()
		if err != nil {
			return errors.Wrap(err, "failed to get DUT's lid angle from ectool")
		}
		re := regexp.MustCompile(fmt.Sprintf(`Lid angle: (\d+)`))
		m := re.FindSubmatch([]byte(data))
		if len(m) != 2 {
			return errors.New("failed to get lid angle")
		}
		lidAngle, err := strconv.Atoi(string(m[1][:]))
		if err != nil {
			return errors.Wrap(err, "failed to parse lid angle")
		}

		// Get tablet mode angle from the tablet mode settings.
		data, err = s.DUT().Conn().CommandContext(ctx, "ectool", "motionsense", "tablet_mode_angle").Output()
		if err != nil {
			return errors.Wrap(err, "failed to retrieve tablet_mode_angle settings")
		}
		re = regexp.MustCompile(fmt.Sprintf(`tablet_mode_angle=(\d+)`))
		m = re.FindSubmatch([]byte(data))
		if len(m) != 2 {
			errors.New("failed to get tablet mode angle")
		}
		tabletModeAngle, err := strconv.Atoi(string(m[1][:]))
		if err != nil {
			return errors.Wrap(err, "failed to parse tablet mode angle")
		}

		if lidAngle > tabletModeAngle {
			return errors.New("current lid angle appears to be greater than the one from tablet mode settings")
		}
		return nil
	}

	if !ffIsClamshell {
		s.Log("Check initial state of laptop mode")
		if err := checkLaptopMode(ctx); err != nil {
			s.Fatal("Unable to check whether DUT is in laptop mode before powering off: ", err)
		}

		s.Log("Set power off")
		if err := ms.PowerOff(ctx); err != nil {
			s.Fatal("Failed to power off DUT: ", err)
		}

		s.Log("Tap on the power button to power on DUT")
		if err := h.Servo.KeypressWithDuration(ctx, servo.PowerKey, servo.DurTab); err != nil {
			s.Fatal("Failed to tap on the power button: ", err)
		}

		s.Log("Wait for the boot to complete")
		waitConnectCtx, cancelWaitConnect := context.WithTimeout(ctx, 2*time.Minute)
		defer cancelWaitConnect()
		if err := h.WaitConnect(waitConnectCtx); err != nil {
			s.Fatal("Failed to reconnect to DUT: ", err)
		}

		s.Log("Check DUT remains in laptop mode after having rebooted")
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
			s.Log("Lock Screen")
			if err := lockScreen(ctx); err != nil {
				s.Fatal("Lock-screen did not behave as expected: ", err)
			}
			// Close chrome instance at the end of the test.
			defer screenLockService.CloseChrome(ctx, &empty.Empty{})
		}

		s.Log("Tap on the power button")
		if err := h.Servo.KeypressWithDuration(ctx, servo.PowerKey, servo.DurTab); err != nil {
			s.Fatal("Failed to tap on the power button: ", err)
		}

		// Wait for a short delay.
		if err := testing.Sleep(ctx, time.Second); err != nil {
			s.Fatal("Failed to sleep: ", err)
		}

		s.Log("Check that display remains on")
		if err := checkDisplay(ctx); err != nil {
			s.Fatal("Display has turned off unexpectedly: ", err)
		}

		s.Log("Press and hold the power button for 1 second to turn on the power menu")
		if err := h.Servo.KeypressWithDuration(ctx, servo.PowerKey, servo.DurPress); err != nil {
			s.Fatal("Failed to press and hold on the power button for 1 second: ", err)
		}

		// Wait for a short delay.
		if err := testing.Sleep(ctx, time.Second); err != nil {
			s.Fatal("Failed to sleep: ", err)
		}

		// Check that pressing the power button for 1 second brings up the power menu.
		s.Log("Check that the power menu has appeared")
		if err := checkPowerMenu(ctx, true); err != nil {
			s.Fatal("Failed to check the power menu: ", err)
		}

		// Check that power menu items are displayed correctly.
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
			s.Fatal("Found mismatched number of power menu items")
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

		s.Log("Tap on the power button to turn off the power menu")
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

		s.Log("Press and hold the power button for 2~3 seconds")
		if err := h.Servo.KeypressWithDuration(ctx, servo.PowerKey, servo.Dur((h.Config.HoldPwrButtonPowerOff)/3)); err != nil {
			s.Fatal("Failed to press and hold on the power button for 3 second: ", err)
		}

		s.Log("Check that the power state remains S0")
		if err := ms.WaitForPowerStates(ctx, firmware.PowerStateInterval, firmware.PowerStateTimeout, "S0"); err != nil {
			s.Fatal("Failed to get S0 powerstate: ", err)
		}
	}
	var testCases = []string{atSignin, afterSignin, atLockScreen}
	for _, testCase := range testCases {
		repeatedSteps(testCase)
	}

	s.Log("Set power off")
	if err := ms.PowerOff(ctx); err != nil {
		s.Fatal("Failed to power off DUT: ", err)
	}

	s.Log("Press and hold the power button for 3~8 seconds")
	if err := h.Servo.KeypressWithDuration(ctx, servo.PowerKey, servo.Dur(h.Config.HoldPwrButtonPowerOff)); err != nil {
		s.Fatal("Failed to press and hold on the power button for 3~8 second: ", err)
	}

	// On some DUTs, pressing 3~8 seconds would leave them in the off state, while some others would power on.
	// We are currently in the process of defining DUT categories for the respective behaviors.
	s.Log("Wait for power state to become G3 or S0")
	if err := ms.WaitForPowerStates(ctx, firmware.PowerStateInterval, firmware.PowerStateTimeout, "G3", "S0"); err != nil {
		s.Fatal("Failed to get G3 or S0 powerstate: ", err)
	}

	s.Log("Get powerstate information")
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

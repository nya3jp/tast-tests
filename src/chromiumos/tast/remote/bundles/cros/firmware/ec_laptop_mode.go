// Copyright 2021 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package firmware

import (
	"bufio"
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/golang/protobuf/ptypes/empty"

	"chromiumos/tast/common/servo"
	"chromiumos/tast/errors"
	"chromiumos/tast/remote/firmware"
	"chromiumos/tast/remote/firmware/fixture"
	"chromiumos/tast/services/cros/graphics"
	"chromiumos/tast/services/cros/ui"
	"chromiumos/tast/ssh"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

// testArgs represents the arguments passed to each parameterized test.
type testArgs struct {
	formFactor    string
	setLaptopMode string
}

type compareDispBusInfo struct {
	runtimeUsageBefore int
	runtimeUsageAfter  int
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         ECLaptopMode,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Checks that power button actions at varied durations behave as expected in laptop mode",
		Contacts:     []string{"cienet-firmware@cienet.corp-partner.google.com", "chromeos-firmware@google.com"},
		Attr:         []string{"group:firmware", "firmware_unstable", "firmware_detachable"},
		SoftwareDeps: []string{"chrome"},
		VarDeps:      []string{"ui.signinProfileTestExtensionManifestKey"},
		Fixture:      fixture.NormalMode,
		ServiceDeps:  []string{"tast.cros.ui.ScreenLockService", "tast.cros.ui.PowerMenuService", "tast.cros.graphics.ScreenshotService", "tast.cros.firmware.UtilsService"},
		HardwareDeps: hwdep.D(hwdep.ChromeEC(), hwdep.Battery()),
		Params: []testing.Param{{
			ExtraHardwareDeps: hwdep.D(hwdep.FormFactor(hwdep.Convertible)),
			Val: &testArgs{
				formFactor:    "convertible",
				setLaptopMode: "tabletmode off",
			},
		}, {
			Name:              "clamshell",
			ExtraHardwareDeps: hwdep.D(hwdep.FormFactor(hwdep.Clamshell)),
			Val: &testArgs{
				formFactor: "clamshell",
			},
		}, {
			Name:              "detachable",
			ExtraHardwareDeps: hwdep.D(hwdep.FormFactor(hwdep.Detachable), hwdep.Keyboard(), hwdep.Touchpad()),
			Val: &testArgs{
				formFactor:    "detachable",
				setLaptopMode: "basestate attach",
			},
		}},
		Timeout: 10 * time.Minute,
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

	checkLaptopMode := func(ctx context.Context) (bool, error) {
		if err := h.RequireRPCUtils(ctx); err != nil {
			return false, errors.Wrap(err, "requiring RPC utils")
		}

		s.Log("Sleeping for a few seconds before starting a new Chrome")
		if err := testing.Sleep(ctx, 5*time.Second); err != nil {
			return false, errors.Wrap(err, "failed to wait for a few seconds")
		}

		if _, err := h.RPCUtils.NewChrome(ctx, &empty.Empty{}); err != nil {
			return false, errors.Wrap(err, "failed to create instance of chrome")
		}
		res, err := h.RPCUtils.EvalTabletMode(ctx, &empty.Empty{})
		if err != nil {
			return false, err
		}
		return res.TabletModeEnabled, nil
	}

	args := s.Param().(*testArgs)
	if args.formFactor == "detachable" {
		// When base detached, a detachable would likely be stuck in tablet mode.
		// Check whether base is attached/detached for debugging purposes.
		// The hardware dependencies should have eliminated detachables with
		// detached base at start. But, just in case that some DUTs get left
		// out, explicitly log base-pogo pin's gpio value.
		possibleNames := []firmware.GpioName{firmware.ENBASE, firmware.ENPP3300POGO}
		cmd := firmware.NewECTool(s.DUT(), firmware.ECToolNameMain)
		if _, err := cmd.FindBaseGpio(ctx, possibleNames); err != nil {
			s.Logf("While looking for %q: %v", possibleNames, err)
		}
	}
	if args.formFactor != "clamshell" {
		s.Log("Checking initial state of laptop mode")
		if inTabletMode, err := checkLaptopMode(ctx); err != nil {
			s.Fatal("Unable to check DUT in laptop mode before powering off: ", err)
		} else if inTabletMode {
			s.Log("DUT is in tablet mode at start. Attempting to turn tablet mode off")
			if err := checkAndSetLaptopMode(ctx, h, args.setLaptopMode); err != nil {
				s.Fatalf("Failed to set laptop mode using command %s, and got: %v", args.setLaptopMode, err)
			}
		}

		s.Log("Setting power off")
		if err := ms.PowerOff(ctx); err != nil {
			s.Fatal("Failed to power off DUT: ", err)
		}

		// Rather than send a tab on power button, set DUT's powerstate to ON.
		// Some DUTs might require longer press on the power button to power
		// on, i.e. Kukui/Kakadu. But, if SetPowerState fails, retry with the
		// power button.
		if err := h.Servo.SetPowerState(ctx, servo.PowerStateOn); err != nil {
			s.Log("Failed to set powerstate to ON, retrying with power button: ", err)
			if err := h.Servo.KeypressWithDuration(ctx, servo.PowerKey, servo.DurTab); err != nil {
				s.Fatal("Failed to press power button: ", err)
			}
		}

		s.Log("Waiting for the boot to complete")
		waitConnectCtx, cancelWaitConnect := context.WithTimeout(ctx, 2*time.Minute)
		defer cancelWaitConnect()
		if err := h.WaitConnect(waitConnectCtx); err != nil {
			s.Fatal("Failed to reconnect to DUT: ", err)
		}
		h.CloseRPCConnection(ctx)
		s.Log("Checking DUT remains in laptop mode after having rebooted")
		if inTabletMode, err := checkLaptopMode(ctx); err != nil {
			s.Fatal("Unable to determine if DUT is in laptop mode after reboot: ", err)
		} else if inTabletMode {
			s.Fatal("DUT booted into tabletmode")
		}
	} else {
		if err := h.RequireRPCUtils(ctx); err != nil {
			s.Fatal("Requiring RPC utils: ", err)
		}

		s.Log("Sleeping for a few seconds before starting a new Chrome")
		if err := testing.Sleep(ctx, 5*time.Second); err != nil {
			s.Fatal("Failed to wait for a few seconds: ", err)
		}

		// Logging in with a testuser session would default
		// and preserve language setting in English. This would
		// be useful for later verification on the power menu items.
		s.Log("Starting a new Chrome service")
		if _, err := h.RPCUtils.NewChrome(ctx, &empty.Empty{}); err != nil {
			s.Fatal("Failed to create instance of chrome: ", err)
		}
		defer h.RPCUtils.CloseChrome(ctx, &empty.Empty{})
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
			vals := compareDispBusInfo{}
			if out, err := readDispBusRuntime(ctx, h); err != nil {
				s.Log("Failed to read display bus info: ", err)
			} else {
				vals.runtimeUsageBefore = out
			}
			s.Log("Locking Screen")
			if err := lockScreen(ctx); err != nil {
				s.Fatal("Lock-screen did not behave as expected: ", err)
			}
			// Close chrome instance at the end of the test.
			defer screenLockService.CloseChrome(ctx, &empty.Empty{})

			// Allow some delay for enhancing stability.
			if err := testing.Sleep(ctx, 10*time.Second); err != nil {
				s.Fatal("Failed to sleep for 10 seconds: ", err)
			}
			// lockScreen calls the js function 'chrome.autotestPrivate.lockScreen()'.
			// While this function locked the screen, it also turned the display off.
			// If display was off, send a tap on the power button to bring it on before
			// continuing the rest of the test.
			if out, err := readDispBusRuntime(ctx, h); err != nil {
				s.Log("Failed to read display bus info: ", err)
			} else {
				vals.runtimeUsageAfter = out
				s.Logf("Got runtime usage before: %d, and after: %d", vals.runtimeUsageBefore, vals.runtimeUsageAfter)
			}
			if err := checkDisplay(ctx); err != nil {
				if strings.Contains(err.Error(), "CRTC not found. Is the screen on?") {
					s.Log("Display turned off at lock screen. Sending a tap on power button")
					if err := h.Servo.KeypressWithDuration(ctx, servo.PowerKey, servo.DurTab); err != nil {
						s.Fatal("Failed to send a tap on power button: ", err)
					}
				} else {
					s.Fatal("Failed to check for display: ", err)
				}
			}
		}

		// Wait for some delay for display to fully settle down,
		// after a transition between Chrome sessions.
		if err := testing.Sleep(ctx, 3*time.Second); err != nil {
			s.Fatal("Failed to sleep: ", err)
		}

		vals := compareDispBusInfo{}
		if out, err := readDispBusRuntime(ctx, h); err != nil {
			s.Log("Failed to read display bus info: ", err)
		} else {
			vals.runtimeUsageBefore = out
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
		// As of now, we've observed such behavior on Kukui and Soraka, but not on Strongbad.
		// ModeSwitcherType seems to be one indicator that'll distinguish between them.
		// We're continuing to identify better indicators.
		if out, err := readDispBusRuntime(ctx, h); err != nil {
			s.Log("Failed to read display bus info: ", err)
		} else {
			vals.runtimeUsageAfter = out
			s.Logf("Got runtime usage before: %d, and after: %d", vals.runtimeUsageBefore, vals.runtimeUsageAfter)
		}
		if args.formFactor != "detachable" || h.Config.ModeSwitcherType != firmware.TabletDetachableSwitcher {
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
			if err := testing.Sleep(ctx, 2*time.Second); err != nil {
				s.Fatal("Failed to sleep: ", err)
			}

			s.Log("Checking that display remains on")
			if err := checkDisplay(ctx); err != nil {
				s.Fatal("Error in verifying display on: ", err)
			}
		}

		s.Log("Pressing and holding the power button to bring up the power menu")
		powerMenuDur := 1 * time.Second
		i := 0
		if err := testing.Poll(ctx, func(ctx context.Context) error {
			s.Logf("Pressing for %q", powerMenuDur)
			if err := h.Servo.KeypressWithDuration(ctx, servo.PowerKey, servo.Dur(powerMenuDur)); err != nil {
				return errors.Wrap(err, "failed to press and hold on the power button for 1 second")
			}
			// On Stainless, power menu was reported to be absent on some DUTs.
			// Increment the press duration by 100 millisecond during each retry till
			// a total of 1.8 seconds is reached.
			i++
			if i <= 8 {
				powerMenuDur += 100 * time.Millisecond
			}
			// Wait for a short delay.
			if err := testing.Sleep(ctx, 2*time.Second); err != nil {
				return errors.Wrap(err, "failed to sleep")
			}
			// Check that pressing the power button for 1 second brings up the power menu.
			if err := checkPowerMenu(ctx, true); err != nil {
				return errors.Wrap(err, "failed to check the power menu")
			}
			return nil
		}, &testing.PollOptions{Timeout: 40 * time.Second, Interval: 2 * time.Second}); err != nil {
			if err := validatePressOnPower(ctx, h, 1*time.Second); err != nil {
				s.Log("Unable to validate press on power: ", err)
			}
			s.Fatal("Power menu was absent following a 1 second press on the power button: ", err)
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
		if args.formFactor != "detachable" || h.Config.ModeSwitcherType != firmware.TabletDetachableSwitcher {
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

		// Differentiate the press durations on Zork from the other platforms.
		// Depending on Stainless results, a new flag may be created from
		// fw-testing-configs for a more general use.
		s.Log("Pressing and holding the power button for 2~3 seconds")
		var whiteScreenPwrDur time.Duration
		if h.Config.Platform == "zork" {
			whiteScreenPwrDur = 1500 * time.Millisecond
		} else {
			whiteScreenPwrDur = (h.Config.HoldPwrButtonPowerOff) / 3
		}
		if err := h.Servo.KeypressWithDuration(ctx, servo.PowerKey, servo.Dur(whiteScreenPwrDur)); err != nil {
			s.Fatal("Failed to press and hold on the power button for 3 second: ", err)
		}

		// To avoid false positive cases, delay for checking on the power states.
		// Without this delay, if DUT turns down after 2~3 seconds, checking on
		// the power state during shutdown might still return S0.
		s.Logf("Sleeping for %v before checking on the power state", h.Config.ShutdownTimeout)
		if err := testing.Sleep(ctx, h.Config.ShutdownTimeout); err != nil {
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
}

// checkAndSetLaptopMode first checks if the passed EC command exists, and uses it to turn off tablet mode.
func checkAndSetLaptopMode(ctx context.Context, h *firmware.Helper, action string) error {
	// regular expressions.
	var (
		tabletmodeNotFound = `Command 'tabletmode' not found or ambiguous`
		tabletmodeStatus   = `\[\S+ tablet mode disabled\]`
		basestateNotFound  = `Command 'basestate' not found or ambiguous`
		basestateStatus    = `\[\S+ base state: attached\]`
		checkLaptopMode    = `(` + tabletmodeNotFound + `|` + tabletmodeStatus + `|` + basestateNotFound + `|` + basestateStatus + `)`
	)
	// Run EC command to turn on/off tablet mode.
	testing.ContextLogf(ctx, "Check command %q exists", action)
	out, err := h.Servo.RunECCommandGetOutput(ctx, action, []string{checkLaptopMode})
	if err != nil {
		return errors.Wrapf(err, "failed to run command %q", action)
	}
	tabletModeCmdUnavailable := []*regexp.Regexp{regexp.MustCompile(tabletmodeNotFound), regexp.MustCompile(basestateNotFound)}
	for _, v := range tabletModeCmdUnavailable {
		if match := v.FindStringSubmatch(out[0][0]); match != nil {
			return errors.Errorf("device does not support tablet mode: %q", match)
		}
	}
	testing.ContextLogf(ctx, "Current tabletmode status: %q", out[0][1])
	return nil
}

// findPowerDeviceName returns the device name for DUT's power button.
func findPowerDeviceName(ctx context.Context, h *firmware.Helper) (string, error) {
	// When tested, different models may use different device names
	// for their power button. Some use "Power Button", while others
	// name it "cros_ec_buttons". Search for both names, and return
	// one that exists.
	namePatterns := fmt.Sprintf(`NAME=(\S+%s|\S+%s)\S`, "Power Button", "cros_ec_buttons")
	findName := fmt.Sprintf("udevadm info --export-db | grep -E '%s'", namePatterns)
	name, err := h.DUT.Conn().CommandContext(ctx, "bash", "-c", findName).Output(ssh.DumpLogOnError)
	if err != nil {
		return "", err
	}
	expMatch := regexp.MustCompile(`NAME=(?i)[^\n\r]*`)
	foundMatch := expMatch.FindSubmatch(name)
	if foundMatch == nil {
		return "", errors.New("unable to find any match")
	}
	devName := strings.TrimLeft(string(foundMatch[0]), "NAME=")
	return devName, nil
}

// scanPowerPress starts evtest to scan for power presses.
func scanPowerPress(ctx context.Context, h *firmware.Helper, devName string) (*bufio.Scanner, error) {
	// Search for device path from the udev database.
	findDevPath := fmt.Sprintf("udevadm info --export-db | awk '/%s/,/DEVNAME/'", devName)
	out, err := h.DUT.Conn().CommandContext(ctx, "bash", "-c", findDevPath).Output(ssh.DumpLogOnError)
	if err != nil {
		return nil, errors.Wrap(err, "unable to export the content of udev database")
	}
	expMatch := regexp.MustCompile(`DEVNAME=\S*`)
	foundMatch := expMatch.FindSubmatch(out)
	if foundMatch != nil {
		testing.ContextLogf(ctx, "Found device path for power button: %s", string(foundMatch[0]))
	}
	devPath := strings.TrimLeft(string(foundMatch[0]), "DEVNAME=")
	// Start evtest to scan for power presses.
	cmd := h.DUT.Conn().CommandContext(ctx, "evtest", devPath)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, errors.Wrap(err, "failed to create stdout pipe")
	}
	if err := cmd.Start(); err != nil {
		return nil, errors.Wrap(err, "failed to start scanner")
	}
	scanner := bufio.NewScanner(stdout)
	return scanner, nil
}

// readPowerPress reads from the evtest scan and checks for any presses
// detected on the power button.
func readPowerPress(ctx context.Context, scanner *bufio.Scanner) error {
	regex := `Event.*time.*code\s(\d*)\s\(` + `KEY_POWER` + `\)`
	expMatch := regexp.MustCompile(regex)
	const scanTimeout = 5 * time.Second
	text := make(chan string)
	go func() {
		defer close(text)
		for scanner.Scan() {
			text <- scanner.Text()
		}
	}()
	for {
		select {
		case <-time.After(scanTimeout):
			return errors.New("Did not detect any power presses")
		case out := <-text:
			match := expMatch.FindStringSubmatch(out)
			if len(match) > 0 {
				testing.ContextLogf(ctx, "Detected power press: %s", match)
				return nil
			}
		}
	}
}

// validatePressOnPower checks that sending a press on power button via servo is a valid one.
// Based on Stainless results, one error surfaced occasionally, in which power menu appeared to
// be absent following a short press on the power button.
func validatePressOnPower(ctx context.Context, h *firmware.Helper, pressDur time.Duration) error {
	testing.ContextLog(ctx, "Checking if power presses are detectable")
	devName, err := findPowerDeviceName(ctx, h)
	if err != nil {
		return errors.Wrap(err, "while looking for device name")
	}
	scanner, err := scanPowerPress(ctx, h, devName)
	if err != nil {
		return errors.Wrap(err, "unable to scan for power press")
	}
	testing.ContextLogf(ctx, "Pressing on power for %s seconds", servo.Dur(pressDur))
	if err := h.Servo.KeypressWithDuration(ctx, servo.PowerKey, servo.Dur(pressDur)); err != nil {
		return errors.Wrap(err, "pressing power button")
	}
	if err := readPowerPress(ctx, scanner); err != nil {
		return errors.Wrap(err, "reading for power press")
	}
	return nil
}

// readDispBusRuntime reads and outputs runtime usage count from the display's bus info. This count
// is expected to decrement when display turns off, and increment when on.
func readDispBusRuntime(ctx context.Context, h *firmware.Helper) (int, error) {
	lshwCommand := "lshw -businfo | grep -i display"
	lshwOut, err := h.DUT.Conn().CommandContext(ctx, "bash", "-c", lshwCommand).Output()
	if err != nil {
		return 0, errors.Wrap(err, "running lshw command")
	}
	re := regexp.MustCompile(`(\w*@\S*)[^\n\r]*`)
	match := re.FindStringSubmatch(string(lshwOut))
	if len(match) != 2 {
		return 0, errors.Errorf("found unexpected length %d, wanted 2", len(match))
	}
	busInfoPath := match[1]
	data := strings.SplitN(busInfoPath, "@", 2)
	if len(data) != 2 {
		return 0, errors.Errorf("expected two tokens in %q, but got %d", busInfoPath, len(data))
	}
	dispBusPath := fmt.Sprintf("/sys/bus/%s/devices/%s/power/runtime_usage", data[0], data[1])
	kernalOut, err := h.DUT.Conn().CommandContext(ctx, "cat", dispBusPath).Output()
	if err != nil {
		return 0, errors.Wrap(err, "reading display runtime usage")
	}
	endVal, err := strconv.Atoi(strings.TrimSpace(string(kernalOut)))
	if err != nil {
		return 0, errors.Wrapf(err, "parsing kernel reading: %s", strings.TrimSpace(string(kernalOut)))
	}
	testing.ContextLogf(ctx, "Display bus runtime usage value: %d", endVal)
	return endVal, nil
}

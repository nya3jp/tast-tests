// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package firmware

import (
	"bufio"
	"context"
	"regexp"
	"strings"
	"time"

	"github.com/golang/protobuf/ptypes/empty"

	"chromiumos/tast/common/servo"
	"chromiumos/tast/errors"
	"chromiumos/tast/remote/firmware/fixture"
	"chromiumos/tast/services/cros/firmware"
	"chromiumos/tast/services/cros/graphics"
	"chromiumos/tast/services/cros/inputs"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

// A screenWakeTrigger is a string representing an option to wake DUT's screen.
type screenWakeTrigger string

// Below are the string values of all possible triggers to wake the screen.
var (
	screenWakeByPowerButton    screenWakeTrigger = "byPowerButton"
	screenWakeByScreenTouch    screenWakeTrigger = "byScreenTouch"
	screenWakeByMovingLid      screenWakeTrigger = "byMovingDUTLid"
	screenWakeByCloseOpenLid   screenWakeTrigger = "byCloseOpenLid"
	screenWakeByEjectingStylus screenWakeTrigger = "byEjectingPen"
)

// A screeState is a bool value indicating the on/off state of the screen.
type screenState string

// Below are values associated with the screenState, and used
// as arguments for the verifyScreenState function.
var (
	expectOn  screenState = "on"
	expectOff screenState = "off"
)

// An evtestEvent is a string that specifies the device to be monitored by evtest.
type evtestEvent string

// Below are the target devices to run evtests on.
var (
	evKeyboard    evtestEvent = "keyboard"
	evTouchpad    evtestEvent = "touchpad"
	evTouchscreen evtestEvent = "touchscreen"
)

// Below are the respective stdout scanners for devices monitored with evtest.
var (
	scannKeyboard    *bufio.Scanner
	scannTouchpad    *bufio.Scanner
	scannTouchscreen *bufio.Scanner
)

// Note: while in tablet mode, some models were observed to still have their keyboards seen
// by evtests. At the moment, these models are filtered out by hardware dependencies, and defined
// by keyboardScannedTabletMode. Investigation is still underway.
type screenWakeTabletModeArgs struct {
	hasLid                  bool
	evTestdetectStylus      bool
	evTestdetectKeyboard    bool
	evTestdetectTouchpad    bool
	evTestdetectTouchscreen bool
}

// Models in keyboardScannedTabletMode are convertibles that appeared to have keyboard
// detected by evtest when DUT was in tablet mode.
var keyboardScannedTabletMode = []string{
	"akemi",
	"blooguard",
	"delbin",
	"dragonair",
	"storo360",
	"jinlon",
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         ScreenWakeTabletMode,
		LacrosStatus: testing.LacrosVariantUnknown,
		Desc:         "Check that tablet mode allows waking screen from additional triggers",
		Contacts:     []string{"arthur.chuang@cienet.com", "chromeos-firmware@google.com"},
		Attr:         []string{"group:firmware", "firmware_unstable"},
		SoftwareDeps: []string{"chrome"},
		ServiceDeps:  []string{"tast.cros.firmware.UtilsService", "tast.cros.graphics.ScreenshotService", "tast.cros.inputs.TouchpadService", "tast.cros.inputs.TouchscreenService"},
		Fixture:      fixture.NormalMode,
		HardwareDeps: hwdep.D(hwdep.ChromeEC(), hwdep.Battery()),
		Params: []testing.Param{{
			Name: "keyboard_scanned_tablet_mode",
			ExtraHardwareDeps: hwdep.D(
				hwdep.Model(keyboardScannedTabletMode...),
				hwdep.Keyboard(),
				hwdep.Touchpad(),
				hwdep.TouchScreen(),
			),
			Val: &screenWakeTabletModeArgs{
				hasLid:                  true,
				evTestdetectStylus:      false,
				evTestdetectKeyboard:    true,
				evTestdetectTouchpad:    false,
				evTestdetectTouchscreen: true,
			},
		}, {
			Name: "chromeslates",
			ExtraHardwareDeps: hwdep.D(
				hwdep.FormFactor(hwdep.Chromeslate),
				hwdep.TouchScreen(),
				hwdep.SkipOnModel(keyboardScannedTabletMode...),
			),
			Val: &screenWakeTabletModeArgs{
				hasLid:                  false,
				evTestdetectStylus:      false,
				evTestdetectKeyboard:    false,
				evTestdetectTouchpad:    false,
				evTestdetectTouchscreen: true,
			},
		}, {
			ExtraHardwareDeps: hwdep.D(
				hwdep.FormFactor(hwdep.Convertible),
				hwdep.Keyboard(),
				hwdep.Touchpad(),
				hwdep.TouchScreen(),
				hwdep.SkipOnModel(keyboardScannedTabletMode...),
			),
			Val: &screenWakeTabletModeArgs{
				hasLid:                  true,
				evTestdetectStylus:      false,
				evTestdetectKeyboard:    false,
				evTestdetectTouchpad:    false,
				evTestdetectTouchscreen: true,
			},
		}},
	})
}

func ScreenWakeTabletMode(ctx context.Context, s *testing.State) {

	h := s.FixtValue().(*fixture.Value).Helper

	if err := h.RequireConfig(ctx); err != nil {
		s.Fatal("Failed to get config: ", err)
	}

	if err := h.RequireServo(ctx); err != nil {
		s.Fatal("Failed to init servo: ", err)
	}

	// Connect to the RPC service on the DUT.
	if err := h.RequireRPCClient(ctx); err != nil {
		s.Fatal("Failed to connect to the RPC service on the DUT: ", err)
	}

	// Declare the touchscreen service.
	touchscreen := inputs.NewTouchscreenServiceClient(h.RPCClient.Conn)

	// Start a logged-in Chrome session, which is required prior to TouchscreenTap in the screenWake function.
	if _, err := touchscreen.NewChrome(ctx, &empty.Empty{}); err != nil {
		s.Fatal("Failed to start a new Chrome for the touchscreen service: ", err)
	}
	defer touchscreen.CloseChrome(ctx, &empty.Empty{})

	// Declare the keyboard service.
	keyboard := firmware.NewUtilsServiceClient(h.RPCClient.Conn)

	// Declare the touchpad service.
	touchpad := inputs.NewTouchpadServiceClient(h.RPCClient.Conn)

	testArgs := s.Param().(*screenWakeTabletModeArgs)
	deviceScanner := func(ctx context.Context, evEvent evtestEvent) (*bufio.Scanner, error) {
		var devPath string
		switch evEvent {
		case evKeyboard:
			if !testArgs.hasLid {
				s.Log("Skip scanning for keyboard because DUT is a chromeslate")
				return nil, nil
			}
			res, err := keyboard.FindPhysicalKeyboard(ctx, &empty.Empty{})
			if err != nil {
				return nil, errors.Wrap(err, "during FindPhysicalKeyboard")
			}
			devPath = res.Path
		case evTouchpad:
			if !testArgs.hasLid {
				s.Log("Skip scanning for touchpad because DUT is a chromeslate")
				return nil, nil
			}
			res, err := touchpad.FindPhysicalTouchpad(ctx, &empty.Empty{})
			if err != nil {
				return nil, errors.Wrap(err, "during FindPhysicalTouchpad")
			}
			devPath = res.Path
		case evTouchscreen:
			res, err := touchscreen.FindPhysicalTouchscreen(ctx, &empty.Empty{})
			if err != nil {
				return nil, errors.Wrap(err, "during FindPhysicalTouchscreen")
			}
			devPath = res.Path
		}

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

	// Use evtestMonitor to check whether events sent to the devices are picked up by the respective evtest.
	evtestMonitor := func(ctx context.Context, evEvent evtestEvent, scanner *bufio.Scanner, shouldRespond bool) error {
		var expMatch *regexp.Regexp

		s.Logf("Monitor evtest on %s", evEvent)
		if evEvent == evKeyboard {
			regex := `Event.*time.*code\s(\d*)\s\(` + `KEY_ENTER` + `\)`
			expMatch = regexp.MustCompile(regex)
		} else {
			regex := `Event.*time.*code\s(\d*)\s\(` + `ABS_PRESSURE` + `\)`
			expMatch = regexp.MustCompile(regex)
		}

		const scanTimeout = 5 * time.Second
		text := make(chan string)
		go func() {
			for scanner.Scan() {
				text <- scanner.Text()
			}
			close(text)
		}()
		for {
			select {
			case <-time.After(scanTimeout):
				if shouldRespond {
					return errors.Errorf("did not detect evtest event for %s within expected time", evEvent)
				}
				s.Logf("%s was disabled", evEvent)
				return nil
			case out := <-text:
				match := expMatch.FindStringSubmatch(out)
				if !shouldRespond && match != nil {
					return errors.Errorf("%s was unexpectedly active in tablet mode", evEvent)
				}
				if match != nil {
					s.Logf("Detected %s: %s", evEvent, match)
					return nil
				}
			}
		}
	}

	// The checkDisplay function checks whether display is on/off by capturing a screenshot.
	screenshotService := graphics.NewScreenshotServiceClient(h.RPCClient.Conn)
	checkDisplay := func(ctx context.Context) error {
		if _, err := screenshotService.CaptureScreenAndDelete(ctx, &empty.Empty{}); err != nil {
			return errors.Wrap(err, "failed to take screenshot")
		}
		return nil
	}

	// The verifyScreenState function uses checkDisplay to verify if the screen behaves as expected.
	var screenIsOn bool
	verifyScreenState := func(ctx context.Context, value screenState) error {
		switch value {
		case "off":
			err := checkDisplay(ctx)
			if err == nil {
				return errors.New("unexpectedly able to take screenshot after setting display power off")
			}
			if !strings.Contains(err.Error(), "CRTC not found. Is the screen on?") {
				return errors.Wrap(err, "unexpected error when taking screenshot")
			}
			screenIsOn = false
		case "on":
			if err := checkDisplay(ctx); err != nil {
				return errors.Wrap(err, "display was not on as expected")
			}
			screenIsOn = true
		}
		return nil
	}

	powerBtnPollOptions := testing.PollOptions{
		Timeout:  30 * time.Second,
		Interval: 1 * time.Second,
	}

	// The screenWake function attempts one of the screenWakeTrigger options to wake the screen.
	screenWake := func(ctx context.Context, option screenWakeTrigger) error {
		// Ensure that DUT's screen is off before sending a trigger to wake the screen.
		if screenIsOn {
			s.Log("Turn off DUT's screen before testing a screen wake trigger")
			if err := testing.Poll(ctx, func(ctx context.Context) error {
				if err := h.Servo.KeypressWithDuration(ctx, servo.PowerKey, servo.DurTab); err != nil {
					return errors.Wrap(err, "error in pressing power button")
				}
				if err := testing.Sleep(ctx, 1*time.Second); err != nil {
					return errors.Wrap(err, "error in sleeping for 1 second")
				}
				if err := verifyScreenState(ctx, expectOff); err != nil {
					return errors.Wrapf(err, "error in verifying DUT's screen state: %q", expectOff)
				}
				return nil
			}, &powerBtnPollOptions); err != nil {
				return errors.Wrapf(err, "while attempting to turn off the screen before screenWakeTrigger: %q", option)
			}
		}
		switch option {
		case screenWakeByPowerButton:
			s.Log("Press power button to wake DUT's screen")
			if err := testing.Poll(ctx, func(ctx context.Context) error {
				if err := h.Servo.KeypressWithDuration(ctx, servo.PowerKey, servo.DurTab); err != nil {
					return errors.Wrap(err, "error in pressing power button")
				}
				if err := testing.Sleep(ctx, 1*time.Second); err != nil {
					return errors.Wrap(err, "error in sleeping for 1 second")
				}
				if err := verifyScreenState(ctx, expectOn); err != nil {
					return errors.Wrapf(err, "error in verifying DUT's screen state: %q", expectOn)
				}
				return nil
			}, &powerBtnPollOptions); err != nil {
				return errors.Wrap(err, "tapping on power button did not turn the screen on")
			}
			return nil
		case screenWakeByEjectingStylus:
			if testArgs.evTestdetectStylus {
				s.Log("To-do: implement a trigger to eject Stylus")
				// Nothing is done here yet.
			} else {
				s.Log("To-do: read Stylus device information, and skip if DUT doesn't have a Stylus available")
				// Nothing is done here yet.
			}
			return nil
		case screenWakeByScreenTouch:
			// Emulate the action of tapping on a touch screen.
			if _, err := touchscreen.TouchscreenTap(ctx, &empty.Empty{}); err != nil {
				return errors.Wrap(err, "error in performing a tap on the touch screen")
			}
			if err := evtestMonitor(ctx, evTouchscreen, scannTouchscreen, testArgs.evTestdetectTouchscreen); err != nil {
				return errors.Wrap(err, "during the evtest for touchscreen")
			}
		case screenWakeByMovingLid:
			if !testArgs.hasLid {
				s.Log("Skip because DUT does not have a lid")
				return nil
			}
			// Emulate the action of moving lid by running `tabletmode off` through EC.
			s.Log("Turn off tablet mode to wake the screen")
			if err := h.Servo.RunECCommand(ctx, "tabletmode off"); err != nil {
				return errors.Wrap(err, "error resetting tabletmode from the EC command")
			}
			if err := testing.Sleep(ctx, 1*time.Second); err != nil {
				return errors.Wrap(err, "error in sleeping for 1 second")
			}
		case screenWakeByCloseOpenLid:
			if !testArgs.hasLid {
				s.Log("Skip because DUT does not have a lid")
				return nil
			}
			s.Log("Close DUT's lid")
			if err := h.Servo.SetStringAndCheck(ctx, servo.LidOpen, string(servo.LidOpenNo)); err != nil {
				return errors.Wrap(err, "error in closing the lid")
			}
			s.Log("Wait for power state to become S0ix or S3")
			if err := testing.Poll(ctx, func(ctx context.Context) error {
				state, err := h.Servo.GetECSystemPowerState(ctx)
				if err != nil {
					return testing.PollBreak(errors.Wrap(err, "failed to get power state"))
				}
				if state != "S0ix" && state != "S3" {
					return errors.New("power state is " + state)
				}
				return nil
			}, &testing.PollOptions{Interval: 1 * time.Second, Timeout: 20 * time.Second}); err != nil {
				return errors.Wrap(err, "error in waiting for power state to be S0ix or S3")
			}
			s.Log("Wait for a few seconds before opening DUT's lid")
			if err := testing.Sleep(ctx, 5*time.Second); err != nil {
				return errors.Wrap(err, "error in sleeping before opening DUT's lid")
			}
			s.Log("Open DUT's lid")
			if err := h.Servo.SetStringAndCheck(ctx, servo.LidOpen, string(servo.LidOpenYes)); err != nil {
				return errors.Wrap(err, "error in opening DUT's lid")
			}
		}

		// Verify that DUT's screen was awakened by one of the screenWakeTrigger options, except for screenWakeByScreenTouch.
		if err := testing.Poll(ctx, func(ctx context.Context) error {
			if option == screenWakeByScreenTouch {
				if err := verifyScreenState(ctx, expectOff); err != nil {
					return errors.Wrapf(err, "error in verifying DUT's screen state: %q", expectOff)
				}
				s.Log("Screen has remained off")
			} else {
				if err := verifyScreenState(ctx, expectOn); err != nil {
					return errors.Wrapf(err, "error in verifying DUT's screen state: %q", expectOn)
				}
			}
			return nil
		}, &testing.PollOptions{Interval: 1 * time.Second, Timeout: 10 * time.Second}); err != nil {
			return errors.Wrapf(err, "after enforcing the screenWakeTrigger: %q", option)
		}
		return nil
	}

	for _, device := range []evtestEvent{evKeyboard, evTouchpad, evTouchscreen} {
		var err error
		switch device {
		case evKeyboard:
			scannKeyboard, err = deviceScanner(ctx, device)
		case evTouchpad:
			scannTouchpad, err = deviceScanner(ctx, device)
		case evTouchscreen:
			scannTouchscreen, err = deviceScanner(ctx, device)
		}
		if err != nil {
			s.Fatalf("While scanning for %s: %v ", device, err)
		}
	}

	if testArgs.hasLid {
		// Run EC command to turn on tablet mode.
		s.Log("Put DUT in tablet mode")
		if err := h.Servo.RunECCommand(ctx, "tabletmode on"); err != nil {
			s.Fatal("Failed to set DUT into tablet mode: ", err)
		}

		if err := verifyScreenState(ctx, expectOn); err != nil {
			s.Fatal("After turning on tablemode: ", err)
		}
	}

	s.Log("Tab power button to turn display off")
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		if err := h.Servo.KeypressWithDuration(ctx, servo.PowerKey, servo.DurTab); err != nil {
			return errors.Wrap(err, "error in pressing power button")
		}
		if err := testing.Sleep(ctx, 1*time.Second); err != nil {
			return errors.Wrap(err, "error in sleeping for 1 second")
		}
		if err := verifyScreenState(ctx, expectOff); err != nil {
			return errors.Wrapf(err, "error in verifying DUT's screen state: %q", expectOff)
		}
		return nil
	}, &powerBtnPollOptions); err != nil {
		s.Fatal("During setting tabletmode on and a tab on power button: ", err)
	}

	if scannKeyboard != nil {
		// Read information from keyboard scan state.
		keyboardStateOut, err := h.Servo.RunECCommandGetOutput(ctx, "ksstate", []string{`Keyboard scan disable mask:(\s+\w+)`})
		if err != nil {
			s.Fatal("Failed to run command ksstate: ", err)
		}
		keyboardStateStr := keyboardStateOut[0][1]
		s.Logf("Keyboard scan disable mask value:%s", keyboardStateStr)

		// Emulate pressing a keyboard key.
		if err := h.Servo.ECPressKey(ctx, "<enter>"); err != nil {
			s.Fatal("Failed to type key: ", err)
		}

		// Monitor keyboard using evtest.
		if err := evtestMonitor(ctx, evKeyboard, scannKeyboard, testArgs.evTestdetectKeyboard); err != nil {
			s.Fatal("During the evtest for keyboard: ", err)
		}
	}

	if scannTouchpad != nil {
		// Emulate swiping on a touch pad.
		if _, err := touchpad.TouchpadSwipe(ctx, &empty.Empty{}); err != nil {
			s.Fatal("Failed to swipe on touch pad: ", err)
		}

		// Monitor touch pad using evtest.
		if err := evtestMonitor(ctx, evTouchpad, scannTouchpad, testArgs.evTestdetectTouchpad); err != nil {
			s.Fatal("During the evtest for touchpad: ", err)
		}
	}

	// Attempt to wake DUT's screen by controls defined under screenWakeTrigger.
	var triggerOptions = []screenWakeTrigger{screenWakeByEjectingStylus, screenWakeByScreenTouch, screenWakeByPowerButton, screenWakeByCloseOpenLid, screenWakeByMovingLid}
	for _, triggerOpt := range triggerOptions {
		s.Logf("---------------------- Wake DUT's screen: %s ---------------------- ", triggerOpt)
		if err := screenWake(ctx, triggerOpt); err != nil {
			s.Fatalf("Unexpected behavior in waking the screen %s: %v", triggerOpt, err)
		}
	}
}

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
	"chromiumos/tast/remote/firmware/pre"
	"chromiumos/tast/rpc"
	"chromiumos/tast/services/cros/graphics"
	"chromiumos/tast/services/cros/inputs"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         ScreenWakeTabletMode,
		Desc:         "Check various ways to wake the screen in tablet mode",
		Contacts:     []string{"arthur.chuang@cienet.com", "chromeos-firmware@google.com"},
		Attr:         []string{"group:firmware", "firmware_experimental"},
		SoftwareDeps: append([]string{"chrome"}, pre.SoftwareDeps...),
		Data:         pre.Data,
		Vars:         pre.Vars,
		ServiceDeps:  append([]string{"tast.cros.graphics.ScreenshotService", "tast.cros.inputs.TouchpadService", "tast.cros.inputs.TouchscreenService"}, pre.ServiceDeps...),
		Pre:          pre.DevModeGBB(),
		HardwareDeps: hwdep.D(hwdep.ChromeEC()),
	})
}

// A screenWakeTrigger is a string representing an option to wake DUT's screen.
type screenWakeTrigger string

// Below are the string values of all possible triggers to wake the screen.
const (
	screenWakeByPowerBtn       screenWakeTrigger = "byPowerBtn"
	screenWakeByScreenTouch    screenWakeTrigger = "byScreenTouch"
	screenWakeByMoveLid        screenWakeTrigger = "byMoveLid"
	screenWakeByCloseOpenLid   screenWakeTrigger = "byCloseOpenLid"
	screenWakeByEjectingStylus screenWakeTrigger = "byEjectingStylus"
)

// A screeState is a bool value indicating the on/off states of the screen.
type screenState string

// Below are the values associated with the screenState, and to be passed in
// as arguments for the verifyScreenState function.
const (
	expectOn  screenState = "on"
	expectOff screenState = "off"
)

// An evtestEvent is a string that specifies the device to be monitored by evtest.
type evtestEvent string

// Below are all the available devices to run evtests on.
const (
	evKeyboard    evtestEvent = "keyboard"
	evTouchpad    evtestEvent = "touchpad"
	evTouchscreen evtestEvent = "touchscreen"
)

func ScreenWakeTabletMode(ctx context.Context, s *testing.State) {

	h := s.PreValue().(*pre.Value).Helper

	if err := h.RequireConfig(ctx); err != nil {
		s.Fatal("Failed to get config: ", err)
	}

	if err := h.RequireServo(ctx); err != nil {
		s.Fatal("Failed to init servo: ", err)
	}

	// Connect to the gRPC server on the DUT
	cl, err := rpc.Dial(ctx, s.DUT(), s.RPCHint(), "cros")
	if err != nil {
		s.Fatal("Failed to connect to the RPC service on the DUT: ", err)
	}
	defer cl.Close(ctx)

	// Declare the touchscreen service.
	touchscreen := inputs.NewTouchscreenServiceClient(cl.Conn)

	// Start a logged-in Chrome session, which is required for TouchscreenTap in the
	// screenWake function.
	if _, err := touchscreen.NewChrome(ctx, &empty.Empty{}); err != nil {
		s.Fatal("Failed to start a new Chrome for the touchscreen service: ", err)
	}
	defer touchscreen.CloseChrome(ctx, &empty.Empty{})

	// Declare the touchpad service.
	touchpad := inputs.NewTouchpadServiceClient(cl.Conn)

	evtestMonitor := func(ctx context.Context, evEvent evtestEvent) error {
		var err error
		const listenSecs uint32 = 5
		s.Logf("Monitor %q activity received by evtest on DUT", evEvent)
		switch evEvent {
		case evKeyboard:
			// To-do: read evtest output for the keyboard device.
		case evTouchpad:
			var out *inputs.ReadEvtestTouchpadResponse
			out, err = touchpad.ReadEvtestTouchpad(ctx, &inputs.ReadEvtestTouchpadRequest{Duration: listenSecs})
			if out.TpEventDetected == true {
				return errors.New("Touchpad is still unexpectedly active in tablet mode")
			}
		case evTouchscreen:
			var out *inputs.ReadEvtestTouchscreenResponse
			out, err = touchscreen.ReadEvtestTouchscreen(ctx, &inputs.ReadEvtestTouchscreenRequest{Duration: listenSecs})
			if out.TscreenEventDetected == false {
				return errors.New("Touchscreen is unexpectedly inactive in tablet mode")
			}
		}
		if err != nil {
			return errors.Wrapf(err, "error while collecting evtest results from %q", evEvent)
		}
		return nil
	}

	// The checkDisplay function checks whether display is on/off
	// by attempting to capture a screenshot. If capturing a screenshot fails,
	// the returned stderr message, "CRTC not found. Is the screen on?", would
	// be returned and checked.
	screenshotService := graphics.NewScreenshotServiceClient(cl.Conn)
	checkDisplay := func(ctx context.Context) error {
		if _, err := screenshotService.CaptureScreenAndDelete(ctx, &empty.Empty{}); err != nil {
			return errors.Wrap(err, "failed to take screenshot")
		}
		return nil
	}

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
				return errors.Wrap(err, "display was not turned ON")
			}
			screenIsOn = true
		}
		return nil
	}

	// The screenWake function uses one of the screenWakeTrigger options to turn on the screen.
	screenWake := func(ctx context.Context, option screenWakeTrigger) error {
		if screenIsOn && option != screenWakeByCloseOpenLid {
			// Turn off DUT's screen before testing the screeWakeTrigger option
			if err := h.Servo.KeypressWithDuration(ctx, servo.PowerKey, servo.DurTab); err != nil {
				return errors.Wrap(err, "error pressing power_key:tab")
			}

			// Verify that DUT's screen is off before sending a trigger to wake the screen.
			if err := testing.Poll(ctx, func(ctx context.Context) error {
				if err := verifyScreenState(ctx, expectOff); err != nil {
					return errors.Wrapf(err, "error verifying DUT's screen state: %q", expectOff)
				}
				return nil
			}, &testing.PollOptions{Interval: 1 * time.Second, Timeout: 15 * time.Second}); err != nil {
				return errors.Wrapf(err, "DUT's screen was not turned off before sending screenWakeTrigger: %q", option)
			}
		}
		switch option {
		case screenWakeByPowerBtn:
			if err := h.Servo.KeypressWithDuration(ctx, servo.PowerKey, servo.DurTab); err != nil {
				return errors.Wrap(err, "error pressing power_key:tab")
			}
		case screenWakeByEjectingStylus:
			// To-do: implement the trigger here to eject Stylus.
		case screenWakeByScreenTouch:
			_, err = touchscreen.TouchscreenTap(ctx, &empty.Empty{})
			if err != nil {
				return errors.Wrap(err, "error performing a tap on touch screen")
			}
		case screenWakeByMoveLid:
			if err := h.Servo.RunECCommand(ctx, "tabletmode reset"); err != nil {
				return errors.Wrap(err, "error resetting tabletmode from the EC command")
			}
		case screenWakeByCloseOpenLid:
			s.Log("Close DUT's lid")
			if err := h.Servo.SetStringAndCheck(ctx, servo.LidOpen, string(servo.LidOpenNo)); err != nil {
				return errors.Wrap(err, "error setting the lid state to closed")
			}

			s.Log("Wait for power state to become S0ix")
			if err := testing.Poll(ctx, func(ctx context.Context) error {
				state, err := h.Servo.GetECSystemPowerState(ctx)
				if err != nil {
					return testing.PollBreak(errors.Wrap(err, "failed to get power state"))
				}
				if state != "S0ix" {
					return errors.New("power state is " + state)
				}
				return nil
			}, &testing.PollOptions{Interval: 1 * time.Second, Timeout: 20 * time.Second}); err != nil {
				return errors.Wrap(err, "error waiting for power state to be S0ix")
			}

			s.Log("Wait for a few seconds before re-opening DUT's lid")
			if err := testing.Sleep(ctx, 5*time.Second); err != nil {
				return errors.Wrap(err, "error while trying to sleep")
			}

			s.Log("Open DUT's lid")
			if err := h.Servo.SetStringAndCheck(ctx, servo.LidOpen, string(servo.LidOpenYes)); err != nil {
				return errors.Wrap(err, "error setting the lid state to open")
			}
		}

		// Verify that DUT's screen was turned on by one of the screenWakeTrigger options, except screenWakeByScreenTouch.
		if err := testing.Poll(ctx, func(ctx context.Context) error {
			if option == "byScreenTouch" {
				if err := verifyScreenState(ctx, expectOff); err != nil {
					return errors.Wrapf(err, "error verifying DUT's screen state: %q", expectOff)
				}
			} else {
				if err := verifyScreenState(ctx, expectOn); err != nil {
					return errors.Wrapf(err, "error verifying DUT's screen state: %q", expectOn)
				}
			}
			return nil
		}, &testing.PollOptions{Interval: 1 * time.Second, Timeout: 5 * time.Second}); err != nil {
			return errors.Wrapf(err, "Verifying DUT's screen state failed for the screenWakeTrigger: %q", option)
		}
		return nil
	}

	// Run EC command to turn on tablet mode, and at this moment DUT's screen is on.
	s.Log("Put DUT in tablet mode")
	if err := h.Servo.RunECCommand(ctx, "tabletmode on"); err != nil {
		s.Fatal("Failed to set DUT into tablet mode: ", err)
	}
	screenIsOn = true

	// Tp-do: Start listening on DUT for key press activity sent by the EC console command kbpress.
	// To-do: Send a keyboard event to emulate pressing the "Space" key.

	// Start listening on DUT for touchpad activity sent by the touchpad service via TouchpadSwipe.
	readTouchpad := make(chan struct{})
	go func() {
		defer close(readTouchpad)
		if err := evtestMonitor(ctx, evTouchpad); err != nil {
			s.Fatal("Failed to monitor evtest: ", err)
		}
	}()

	// Send a touchpad event to emulate the action of swiping on the touchpad.
	if _, err := touchpad.TouchpadSwipe(ctx, &empty.Empty{}); err != nil {
		s.Fatal("Failed to perform a swipe action on touch pad: ", err)
	}
	<-readTouchpad

	// To-do: attempt to wake DUT's screen by ejecting Stylus.

	// Start listening on DUT for touchscreen activity sent by the touchscreen service via TouchscreenTap.
	readTouchscreen := make(chan struct{})
	go func() {
		defer close(readTouchscreen)
		if err := evtestMonitor(ctx, evTouchscreen); err != nil {
			s.Fatal("Failed to monitor evtest: ", err)
		}
	}()

	// Attempt to wake DUT's screen by tapping on the touchscreen.
	s.Log("Wake DUT's screen by tapping on the touch screen")
	if err := screenWake(ctx, screenWakeByScreenTouch); err != nil {
		s.Fatal("Tapping on the touch screen unexpectedly turned the screen on: ", err)
	}
	<-readTouchscreen

	// Attempt to wake DUT's screen by pressing then releasing power button within 0.5 seconds.
	s.Log("Wake DUT's screen by a tab on the power button")
	if err := screenWake(ctx, screenWakeByPowerBtn); err != nil {
		s.Fatal("Failed to turn on the screen by pressing the power button: ", err)
	}

	// Attempt tp wake DUT's screen by closing, then re-opening DUT's lid.
	s.Log("Wake DUT's screen by closing/re-opening lid")
	if err := screenWake(ctx, screenWakeByCloseOpenLid); err != nil {
		s.Fatal("Failed to wake the screen: ", err)
	}

	// Attempt to wake DUT's screen by running EC command `tabletmode reset` to emulate moving DUT's lid,
	// which would convert DUT from tablet mode back to the original setting.
	s.Log("Wake DUT's screen by moving lid")
	if err := screenWake(ctx, screenWakeByMoveLid); err != nil {
		s.Fatal("Failed to turn on the screen: ", err)
	}

}

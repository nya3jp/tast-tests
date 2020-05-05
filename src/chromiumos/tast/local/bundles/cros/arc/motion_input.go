// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"encoding/json"
	"math"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/arc/ui"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/display"
	"chromiumos/tast/local/coords"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         MotionInput,
		Desc:         "Checks motion input (touch/mouse) works in various window states on Android",
		Contacts:     []string{"prabirmsp@chromium.org", "arc-framework@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome", "android_vm"},
		Pre:          arc.VMBooted(),
	})
}

type motionEventAction string
type motionEventAxis string
type inputDeviceSource string

const (
	// These constants are from Android's MotionEvent.java.
	// See: https://cs.android.com/android/platform/superproject/+/master:frameworks/base/core/java/android/view/MotionEvent.java
	actionDown          motionEventAction = "ACTION_DOWN"
	actionUp            motionEventAction = "ACTION_UP"
	actionMove          motionEventAction = "ACTION_MOVE"
	actionHoverMove     motionEventAction = "ACTION_HOVER_MOVE"
	actionHoverEnter    motionEventAction = "ACTION_HOVER_ENTER"
	actionHoverExit     motionEventAction = "ACTION_HOVER_EXIT"
	actionButtonPress   motionEventAction = "ACTION_BUTTON_PRESS"
	actionButtonRelease motionEventAction = "ACTION_BUTTON_RELEASE"

	// These constants are from Android's MotionEvent.java.
	// See: https://cs.android.com/android/platform/superproject/+/master:frameworks/base/core/java/android/view/MotionEvent.java
	axisX        motionEventAxis = "AXIS_X"
	axisY        motionEventAxis = "AXIS_Y"
	axisPressure motionEventAxis = "AXIS_PRESSURE"

	// These constants must be kept in sync with ArcMotionInputTest.apk.
	srcTouchscreen inputDeviceSource = "touchscreen"
	srcMouse       inputDeviceSource = "mouse"
)

// motionEvent represents a MotionEvent that was received by the Android application.
// For all motionEventAxis values that represent an absolute location, the values are in the
// coordinate space of the Android window (i.e. 0,0 is the top left corner of the application
// window in Android).
type motionEvent struct {
	Action      motionEventAction             `json:"action"`
	DeviceID    int                           `json:"device_id"`
	Sources     []inputDeviceSource           `json:"sources"`
	PointerAxes []map[motionEventAxis]float64 `json:"pointer_axes"`
}

const (
	motionInputTestAPK               = "ArcMotionInputTest.apk"
	motionInputTestPKG               = "org.chromium.arc.testapp.motioninput"
	motionInputTestCLS               = ".MainActivity"
	motionInputTestActionClearEvents = motionInputTestPKG + ".ACTION_CLEAR_EVENTS"
)

// readMotionEvents unmarshalls the JSON string in the TextView representing the MotionEvents
// received by ArcMotionInputTest.apk, and returns it as a slice of motionEvent.
func readMotionEvents(ctx context.Context, d *ui.Device) ([]motionEvent, error) {
	view := d.Object(ui.ID(motionInputTestPKG + ":id/motion_event"))
	text, err := view.GetText(ctx)
	if err != nil {
		return nil, err
	}

	var events []motionEvent
	if err := json.Unmarshal([]byte(text), &events); err != nil {
		return nil, err
	}
	return events, nil
}

// motionEventMatcher represents a matcher for motionEvent.
type motionEventMatcher func(*motionEvent) error

// expectMotionEvents polls readMotionEvents repeatedly until it receives motionEvents that
// successfully match all of the provided motionEventMatchers in order, or until it times out.
func expectMotionEvents(ctx context.Context, d *ui.Device, matchers ...motionEventMatcher) error {
	return testing.Poll(ctx, func(ctx context.Context) error {
		events, err := readMotionEvents(ctx, d)
		if err != nil {
			return testing.PollBreak(errors.Wrap(err, "failed to read motion event"))
		}

		if len(events) != len(matchers) {
			return errors.Errorf("did not receive the exact number of events as expected; got: %d, want: %d", len(events), len(matchers))
		}

		for i := 0; i < len(matchers); i++ {
			if err := matchers[i](&events[i]); err != nil {
				return err
			}
		}
		return nil
	}, nil)
}

// clearMotionEvents tells the test application to clear the events that it is currently reporting,
// and verifies that no events are reported. This is done by sending an intent with the appropriate
// action to Android, which is subsequently picked up by the MotionInputTest application and handled
// appropriately.
func clearMotionEvents(ctx context.Context, a *arc.ARC, d *ui.Device) error {
	if err := a.SendIntentCommand(ctx, motionInputTestActionClearEvents, "").Run(); err != nil {
		return errors.Wrap(err, "failed to send the clear events intent")
	}
	if err := expectMotionEvents(ctx, d); err != nil {
		return errors.Wrap(err, "failed to verify that the reported MotionEvents were cleared")
	}
	return nil
}

const (
	// coordinateAxisEpsilon is the epsilon value to be used when comparing axis values that
	// represent absolute display coordinates. Scaling and conversions from Chrome to Android's
	// display spaces means absolute coordinates can be off by up to one pixel.
	coordinateAxisEpsilon = 1e0

	// defaultAxisEpsilon is the epsilon value to be used when comparing axis values that do
	// not need to be scaled or converted, like pressure (which is in the range [0,1]). We
	// expect these values to be more precise.
	defaultAxisEpsilon = 1e-5
)

// singlePointerMatcher returns a motionEventMatcher that matches a motionEvent with a single
// pointer that has the following axes: axisX, axisY, and axisPressure.
func singlePointerMatcher(a motionEventAction, s inputDeviceSource, p coords.Point, pressure float64) motionEventMatcher {
	return func(event *motionEvent) error {
		sourceMatches := false
		for _, v := range event.Sources {
			if v == s {
				sourceMatches = true
				break
			}
		}
		axisMatcher := func(axis motionEventAxis, expected, epsilon float64) error {
			v := event.PointerAxes[0][axis]
			if math.Abs(v-expected) > epsilon {
				return errors.Errorf("value of axis %s did not match: got %.5f; want %.5f; epsilon %.5f", axis, v, expected, epsilon)
			}
			return nil
		}
		if !sourceMatches {
			return errors.Errorf("source does not match: got %v; want %s", event.Sources, s)
		} else if event.Action != a {
			return errors.Errorf("action does not match: got %s; want: %s", event.Action, a)
		} else if pointerCount := len(event.PointerAxes); pointerCount != 1 {
			return errors.Errorf("pointer count does not match: got: %d; want: %d", pointerCount, 1)
		} else if err := axisMatcher(axisX, float64(p.X), coordinateAxisEpsilon); err != nil {
			return err
		} else if err := axisMatcher(axisY, float64(p.Y), coordinateAxisEpsilon); err != nil {
			return err
		} else if err := axisMatcher(axisPressure, pressure, defaultAxisEpsilon); err != nil {
			return err
		}
		return nil
	}
}

// singleTouchMatcher returns a motionEventMatcher that matches events from a Touchscreen device.
func singleTouchMatcher(a motionEventAction, p coords.Point) motionEventMatcher {
	return singlePointerMatcher(a, srcTouchscreen, p, 1)
}

// mouseMatcher returns a motionEventMatcher that matches events from a Mouse device.
func mouseMatcher(a motionEventAction, p coords.Point) motionEventMatcher {
	pressure := 0.
	if a == actionMove || a == actionDown || a == actionButtonPress || a == actionHoverExit {
		pressure = 1.
	}
	return singlePointerMatcher(a, srcMouse, p, pressure)
}

// MotionInput runs several sub-tests, where each sub-test sets up the Chrome WM environment as
// specified by the motionInputSubtestParams. Each sub-test installs and runs an Android application
// (ArcMotionInputTest.apk), injects various input events into ChromeOS through uinput devices,
// and verifies that those events were received by the Android application in the expected screen
// locations.
func MotionInput(ctx context.Context, s *testing.State) {
	p := s.PreValue().(arc.PreData)
	cr := p.Chrome
	a := p.ARC

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create test API connection: ", err)
	}

	// runSubtest runs the provided sub-test after setting up the WM environment as specified by the
	// provided motionInputSubtestParams.
	runSubtest := func(ctx context.Context, s *testing.State, params *motionInputSubtestParams, subtest motionInputSubtestFunc) {

		test := motionInputSubtestState{}
		test.tconn = tconn
		test.a = a
		test.params = params

		deviceMode := "clamshell"
		if params.TabletMode {
			deviceMode = "tablet"
		}
		s.Logf("Setting device to %v mode", deviceMode)
		cleanup, err := ash.EnsureTabletModeEnabled(ctx, tconn, params.TabletMode)
		if err != nil {
			s.Fatal("Failed to ensure tablet mode enabled: ", err)
		}
		defer cleanup(ctx)

		test.d, err = ui.NewDevice(ctx, a)
		if err != nil {
			s.Fatal("Failed initializing UI Automator: ", err)
		}
		defer test.d.Close()

		infos, err := display.GetInfo(ctx, tconn)
		if err != nil {
			s.Fatal("Failed to get display info: ", err)
		}
		if len(infos) == 0 {
			s.Fatal("No display found")
		}
		for i := range infos {
			if infos[i].IsInternal {
				test.displayInfo = &infos[i]
				break
			}
		}
		if test.displayInfo == nil {
			s.Log("No internal display found. Default to the first display")
			test.displayInfo = &infos[0]
		}

		test.scale, err = test.displayInfo.GetEffectiveDeviceScaleFactor()
		if err != nil {
			s.Fatal("Failed to get effective device scale factor: ", err)
		}

		s.Log("Installing apk ", motionInputTestAPK)
		if err := a.Install(ctx, arc.APKPath(motionInputTestAPK)); err != nil {
			s.Fatalf("Failed installing %s: %v", motionInputTestAPK, err)
		}

		act, err := arc.NewActivity(a, motionInputTestPKG, motionInputTestCLS)
		if err != nil {
			s.Fatal("Failed to create an activity: ", err)
		}
		defer act.Close()

		if err := act.Start(ctx, tconn); err != nil {
			s.Fatal("Failed to start an activity: ", err)
		}
		defer act.Stop(ctx)

		if err := ash.WaitForVisible(ctx, tconn, motionInputTestPKG); err != nil {
			s.Fatal("Failed to wait for activity to be visible: ", err)
		}

		if params.WmEventToSend != nil {
			event := params.WmEventToSend.(ash.WMEventType)
			s.Log("Sending wm event: ", event)
			if _, err := ash.SetARCAppWindowState(ctx, tconn, motionInputTestPKG, event); err != nil {
				s.Fatalf("Failed to set ARC app window state with event %s: %v", event, err)
			}
		}

		if params.WmStateToVerify != nil {
			windowState := params.WmStateToVerify.(ash.WindowStateType)
			s.Log("Verifying window state: ", windowState)
			if err := ash.WaitForARCAppWindowState(ctx, tconn, motionInputTestPKG, windowState); err != nil {
				s.Fatal("Failed to verify app window state: ", windowState)
			}
		}

		if err := test.d.WaitForIdle(ctx, 10*time.Second); err != nil {
			s.Log("Failed to wait for idle, ignoring: ", err)
		}

		test.w, err = ash.GetARCAppWindowInfo(ctx, tconn, motionInputTestPKG)
		if err != nil {
			s.Fatal("Failed to get ARC app window info: ", err)
		}

		subtest(ctx, s, test)
	}

	for _, params := range []motionInputSubtestParams{
		{
			Name:            "Clamshell Normal",
			TabletMode:      false,
			WmEventToSend:   ash.WMEventNormal,
			WmStateToVerify: ash.WindowStateNormal,
		}, {
			Name:            "Clamshell Fullscreen",
			TabletMode:      false,
			WmEventToSend:   ash.WMEventFullscreen,
			WmStateToVerify: ash.WindowStateFullscreen,
		}, {
			Name:            "Clamshell Maximized",
			TabletMode:      false,
			WmEventToSend:   ash.WMEventMaximize,
			WmStateToVerify: ash.WindowStateMaximized,
		},
		// TODO(b/155500968): Investigate why the first motion event received by the test app has
		//  incorrect coordinates in tablet mode, and add test params for tablet mode when resolved.
	} {
		s.Run(ctx, params.Name, func(ctx context.Context, s *testing.State) {
			runSubtest(ctx, s, &params, verifyTouchscreen)
			runSubtest(ctx, s, &params, verifyMouse)
		})
	}
}

// wmEventToSend holds an ash.WMEventType or nil.
type wmEventToSend interface{}

// wmWindowState holds an ash.WindowStateType or nil.
type wmWindowState interface{}

// motionInputSubtestParams holds the test parameters used to set up the WM environment in Chrome, and
// represents a single sub-test.
type motionInputSubtestParams struct {
	Name            string        // A description of the subtest.
	TabletMode      bool          // If true, the device will be put in tablet mode.
	WmEventToSend   wmEventToSend // This must be of type ash.WMEventType, and can be nil.
	WmStateToVerify wmWindowState // This must be of type ash.WindowStateType, and can be nil.
}

// isCaptionVisible returns true if the caption should be visible for the Android application when
// the respective params are applied.
func (params *motionInputSubtestParams) isCaptionVisible() bool {
	return !params.TabletMode && params.WmEventToSend.(ash.WMEventType) != ash.WMEventFullscreen
}

// motionInputSubtestState holds various values that represent the test state for each sub-test.
// It is created for convenience to reduce the number of function parameters.
type motionInputSubtestState struct {
	tconn       *chrome.TestConn
	a           *arc.ARC
	displayInfo *display.Info
	scale       float64
	w           *ash.Window
	d           *ui.Device
	params      *motionInputSubtestParams
}

// expectedPoint takes a coords.Point representing the coordinate where an input event is injected
// in DP in the display space, and returns a coords.Point representing where it is expected to be
// injected in the Android application window's coordinate space.
func (test *motionInputSubtestState) expectedPoint(p coords.Point) coords.Point {
	insetLeft := test.w.BoundsInRoot.Left
	insetTop := test.w.BoundsInRoot.Top
	if test.params.isCaptionVisible() {
		insetTop += test.w.CaptionHeight
	}
	return coords.NewPoint(int(float64(p.X-insetLeft)*test.scale), int(float64(p.Y-insetTop)*test.scale))
}

// expectEventsAndClear is a convenience function that verifies expected events and clears the
// events to be ready for the next assertions.
func (test *motionInputSubtestState) expectEventsAndClear(ctx context.Context, matchers ...motionEventMatcher) error {
	if err := expectMotionEvents(ctx, test.d, matchers...); err != nil {
		return errors.Wrap(err, "failed to verify expected events")
	}
	if err := clearMotionEvents(ctx, test.a, test.d); err != nil {
		return errors.Wrap(err, "failed to clear events")
	}
	return nil
}

const (
	// numMotionEventIterations is the number of times certain motion events should be repeated in
	// a test. For example, it could be the number of times a move event should be injected during
	// a drag. Increasing this number will increase the time it takes to run the test.
	numMotionEventIterations = 5
)

// motionInputSubtestFunc represents the subtest function that takes the motionInputSubtestState,
// injects input events and makes the assertions.
type motionInputSubtestFunc func(ctx context.Context, s *testing.State, test motionInputSubtestState)

// verifyTouchscreen tests the behavior of events injected from a uinput touchscreen device. It
// injects a down event, followed by several move events, and finally an up event with a single
// touch pointer.
func verifyTouchscreen(ctx context.Context, s *testing.State, test motionInputSubtestState) {
	s.Log("Verifying Touchscreen")

	tew, err := input.Touchscreen(ctx)
	if err != nil {
		s.Fatal("Failed to create touchscreen: ", err)
	}
	defer tew.Close()

	stw, err := tew.NewSingleTouchWriter()
	if err != nil {
		s.Fatal("Failed to create SingleTouchEventWriter: ", err)
	}
	defer stw.Close()

	tcc := tew.NewTouchCoordConverter(test.displayInfo.Bounds.Size())

	pointDP := test.w.BoundsInRoot.CenterPoint()
	expected := test.expectedPoint(pointDP)

	s.Log("Verifying touch down event at ", expected)
	x, y := tcc.ConvertLocation(pointDP)
	if err := stw.Move(x, y); err != nil {
		s.Fatalf("Could not inject move at (%d, %d)", x, y)
	}
	if err := test.expectEventsAndClear(ctx, singleTouchMatcher(actionDown, expected)); err != nil {
		s.Fatal("Failed to expect events and clear: ", err)
	}

	// deltaDP is the amount we want to move the touch pointer between each successive injected
	// event. We use an arbitrary value that is not too large so that we can safely assume that
	// the injected events stay within the bounds of the display.
	const deltaDP = 5

	for i := 0; i < numMotionEventIterations; i++ {
		pointDP.X += deltaDP
		pointDP.Y += deltaDP
		expected = test.expectedPoint(pointDP)

		s.Log("Verifying touch move event at ", expected)
		x, y := tcc.ConvertLocation(pointDP)
		if err := stw.Move(x, y); err != nil {
			s.Fatalf("Could not inject move at (%d, %d): %v", x, y, err)
		}
		if err := test.expectEventsAndClear(ctx, singleTouchMatcher(actionMove, expected)); err != nil {
			s.Fatal("Failed to expect events and clear: ", err)
		}
	}

	s.Log("Verifying touch up event at ", expected)
	x, y = tcc.ConvertLocation(pointDP)
	if err := stw.End(); err != nil {
		s.Fatalf("Could not inject end at (%d, %d)", x, y)
	}
	if err := test.expectEventsAndClear(ctx, singleTouchMatcher(actionUp, expected)); err != nil {
		s.Fatal("Failed to expect events and clear: ", err)
	}
}

// verifyMouse tests the behavior of mouse events injected into Ash on Android apps. It tests hover,
// button, and drag events. It does not use the uinput mouse to inject events because the scale
// relation between the relative movements injected by a relative mouse device and the display
// pixels is determined by ChromeOS and could vary between devices.
func verifyMouse(ctx context.Context, s *testing.State, t motionInputSubtestState) {
	s.Log("Verifying Mouse")

	p := t.w.BoundsInRoot.CenterPoint()
	e := t.expectedPoint(p)

	s.Log("Injected initial move, waiting... ")
	// TODO(b/155783589): Investigate why injecting only one initial move event (by setting the
	//  duration to 0) produces ACTION_HOVER_ENTER, ACTION_HOVER_MOVE, and ACTION_HOVER_EXIT,
	//  instead of the expected single event with action ACTION_HOVER_ENTER.
	if err := ash.MouseMove(ctx, t.tconn, p, 500*time.Millisecond); err != nil {
		s.Fatalf("Failed to inject move at %v: %v", e, err)
	}
	// TODO(b/155783589): Investigate why there are sometimes two ACTION_HOVER_ENTER events being
	//  sent. Once resolved, add expectation for ACTION_HOVER_ENTER and remove sleep.
	if err := testing.Sleep(ctx, time.Second); err != nil {
		s.Fatal("Failed to sleep: ", err)
	}
	if err := clearMotionEvents(ctx, t.a, t.d); err != nil {
		s.Fatal("Failed to clear events: ", err)
	}

	// deltaDP is the amount we want to move the mouse pointer between each successive injected
	// event. We use an arbitrary value that is not too large so that we can safely assume that
	// the injected events stay within the bounds of the application in the various WM states, so
	// that clicks performed after moving the mouse are still inside the application.
	const deltaDP = 5

	for i := 0; i < numMotionEventIterations; i++ {
		p.X += deltaDP
		p.Y += deltaDP
		e = t.expectedPoint(p)

		s.Log("Verifying mouse move event at ", e)
		if err := ash.MouseMove(ctx, t.tconn, p, 0); err != nil {
			s.Fatalf("Failed to inject move at %v: %v", e, err)
		}
		if err := t.expectEventsAndClear(ctx, mouseMatcher(actionHoverMove, e)); err != nil {
			s.Fatal("Failed to expect events and clear: ", err)
		}
	}

	if err := ash.MousePress(ctx, t.tconn, ash.LeftButton); err != nil {
		s.Fatal("Failed to press button on mouse: ", err)
	}
	if err := t.expectEventsAndClear(ctx, mouseMatcher(actionHoverExit, e), mouseMatcher(actionDown, e), mouseMatcher(actionButtonPress, e)); err != nil {
		s.Fatal("Failed to expect events and clear: ", err)
	}

	for i := 0; i < numMotionEventIterations; i++ {
		p.X -= deltaDP
		p.Y -= deltaDP
		e = t.expectedPoint(p)

		s.Log("Verifying mouse move event at ", e)
		if err := ash.MouseMove(ctx, t.tconn, p, 0); err != nil {
			s.Fatalf("Failed to inject move at %v: %v", e, err)
		}
		if err := t.expectEventsAndClear(ctx, mouseMatcher(actionMove, e)); err != nil {
			s.Fatal("Failed to expect events and clear: ", err)
		}
	}

	if err := ash.MouseRelease(ctx, t.tconn, ash.LeftButton); err != nil {
		s.Fatal("Failed to release mouse button: ", err)
	}
	if err := t.expectEventsAndClear(ctx, mouseMatcher(actionButtonRelease, e), mouseMatcher(actionUp, e)); err != nil {
		s.Fatal("Failed to expect events and clear: ", err)
	}

	p.X -= deltaDP
	p.Y -= deltaDP
	e = t.expectedPoint(p)

	if err := ash.MouseMove(ctx, t.tconn, p, 0); err != nil {
		s.Fatalf("Failed to inject move at %v: %v", e, err)
	}
	if err := t.expectEventsAndClear(ctx, mouseMatcher(actionHoverEnter, e), mouseMatcher(actionHoverMove, e)); err != nil {
		s.Fatal("Failed to expect events and clear: ", err)
	}
}

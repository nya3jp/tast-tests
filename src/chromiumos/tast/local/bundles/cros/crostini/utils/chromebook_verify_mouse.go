// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package utils

import (
	"context"
	"encoding/json"
	"math"
	"time"

	"chromiumos/tast/common/android/ui"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/display"
	"chromiumos/tast/local/chrome/uiauto/mouse"
	"chromiumos/tast/local/coords"
	"chromiumos/tast/testing"
)

// Constants for the test application ArcMotionInputTest.apk.
const (
	APK     = "ArcMotionInputTest.apk"
	Package = "org.chromium.arc.testapp.motioninput"

	EventReportingActivity     = ".MotionEventReportingActivity"
	AutoPointerCaptureActivity = ".AutoPointerCaptureActivity"

	intentActionClearEvents = Package + ".ACTION_CLEAR_EVENTS"
)

// VerifyMouse refer to mouse_input.go notice: need to add this {Fixture: "arcBooted"}
func VerifyMouse(ctx context.Context, s *testing.State) error {
	p := s.FixtValue().(*arc.PreData)
	cr := p.Chrome
	a := p.ARC

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create test API connection: ", err)
	}

	d, err := a.NewUIDevice(ctx)
	if err != nil {
		s.Fatal("Failed initializing UI Automator: ", err)
	}
	defer d.Close(ctx)

	if err := a.Install(ctx, arc.APKPath(APK)); err != nil {
		s.Fatal("Failed installing ", APK, ": ", err)
	}

	for _, params := range []wMTestParams{
		{
			Name:          "Clamshell fullscreen",
			TabletMode:    false,
			WmEventToSend: ash.WMEventFullscreen,
		},
	} {
		s.Run(ctx, params.Name+": Verify Mouse", func(ctx context.Context, s *testing.State) {
			runTestWithWMParams(ctx, s, tconn, d, a, &params, verifyMouse)
		})
	}

	return nil
}

// Matcher represents a matcher for motionEvent.
type Matcher func(*motionEvent) error

// mouseMatcher returns a motionEventMatcher that matches events from a Mouse device.
func mouseMatcher(a Action, p coords.Point) Matcher {
	pressure := 0.
	if a == ActionMove || a == ActionDown || a == ActionButtonPress || a == ActionHoverExit {
		pressure = 1.
	}
	return singlePointerMatcher(a, SourceMouse, p, pressure)
}

func verifyMouse(ctx context.Context, s *testing.State, tconn *chrome.TestConn, t *wMTestState, tester *tester) {
	// verifyMouse tests the behavior of mouse events injected into Ash on Android apps. It tests hover,
	// button, and drag events. It does not use the uinput mouse to inject events because the scale
	// relation between the relative movements injected by a relative mouse device and the display
	// pixels is determined by ChromeOS and could vary between devices.
	s.Log("Verifying Mouse")

	p := t.centerOfWindow()
	e := t.expectedPoint(p)

	s.Log("Injected initial move, waiting... ")
	if err := mouse.Move(tconn, p, 0)(ctx); err != nil {
		s.Fatalf("Failed to inject move at %v: %v", e, err)
	}

	if err := tester.clearMotionEvents(ctx); err != nil {
		s.Fatal("Failed to clear events: ", err)
	}

	p = coords.NewPoint(0, 0)
	e = t.expectedPoint(p)

	s.Log("Verifying mouse move event at ", e)
	if err := mouse.Move(tconn, p, 0)(ctx); err != nil {
		s.Fatalf("Failed to inject move at %v: %v", e, err)
	}

	if err := tester.expectEventsAndClear(ctx, mouseMatcher(ActionHoverMove, e)); err != nil {
		s.Fatal("Failed to expect events and clear: ", err)
	}

	return

}

// wMEventToSend holds an ash.WMEventType or nil.
type wMEventToSend interface{}

// wMTestParams holds the test parameters used to set up the WM environment in Chrome, and
// represents a single sub-test.
type wMTestParams struct {
	Name          string        // A description of the subtest.
	TabletMode    bool          // If true, the device will be put in tablet mode.
	WmEventToSend wMEventToSend // This must be of type ash.WMEventType, and can be nil.
}

// wMTestState holds various values that represent the test state for each sub-test.
// It is created for convenience to reduce the number of function parameters.
type wMTestState struct {
	VerifiedWindowState *ash.WindowStateType // The window state of the test Activity after it is confirmed by Chrome. This can be nil if the window state was not verified.
	VerifiedTabletMode  bool                 // The state of tablet mode after it is confirmed by Chrome.
	DisplayInfo         *display.Info        // The info for the display the Activity is in.
	Scale               float64              // The scale factor used to convert Chrome's DP to Android's pixels.
	Window              *ash.Window          // The state of the test Activity's window.
}

// runTestWithWMParams sets up the window management state of the test device to that specified in the given
// wMTestParams, and runs the wMTestFunc. The APK must be installed on the device before using this helper.
func runTestWithWMParams(ctx context.Context, s *testing.State, tconn *chrome.TestConn, d *ui.Device, a *arc.ARC, params *wMTestParams, testFunc wMTestFunc) {
	t := &wMTestState{}

	deviceMode := "clamshell"
	if params.TabletMode {
		deviceMode = "tablet"
	}
	s.Logf("Setting device to %v mode", deviceMode)
	cleanup, err := ash.EnsureTabletModeEnabled(ctx, tconn, params.TabletMode)
	if err != nil {
		s.Fatal("Failed to ensure tablet mode enabled: ", err)
	} else {
		t.VerifiedTabletMode = params.TabletMode
	}
	defer cleanup(ctx)

	infos, err := display.GetInfo(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to get display info: ", err)
	}
	if len(infos) == 0 {
		s.Fatal("No display found")
	}
	for i := range infos {
		if infos[i].IsInternal {
			t.DisplayInfo = &infos[i]
			break
		}
	}
	if t.DisplayInfo == nil {
		s.Log("No internal display found. Default to the first display")
		t.DisplayInfo = &infos[0]
	}

	t.Scale, err = t.DisplayInfo.GetEffectiveDeviceScaleFactor()
	if err != nil {
		s.Fatal("Failed to get effective device scale factor: ", err)
	}

	act, err := arc.NewActivity(a, Package, EventReportingActivity)
	if err != nil {
		s.Fatal("Failed to create an activity: ", err)
	}
	defer act.Close()

	if err := act.Start(ctx, tconn); err != nil {
		s.Fatal("Failed to start an activity: ", err)
	}
	defer act.Stop(ctx, tconn)

	if err := ash.WaitForVisible(ctx, tconn, Package); err != nil {
		s.Fatal("Failed to wait for activity to be visible: ", err)
	}

	if params.WmEventToSend != nil {
		event := params.WmEventToSend.(ash.WMEventType)
		s.Log("Sending wm event: ", params.WmEventToSend)
		windowState, err := ash.SetARCAppWindowState(ctx, tconn, Package, event)
		if err != nil {
			s.Fatalf("Failed to set ARC app window state with event %s: %v", event, err)
		}
		s.Log("Verifying window state: ", windowState)
		if err := ash.WaitForARCAppWindowState(ctx, tconn, Package, windowState); err != nil {
			s.Fatal("Failed to verify app window state: ", windowState)
		}
		t.VerifiedWindowState = &windowState
	}

	if err := d.WaitForIdle(ctx, 10*time.Second); err != nil {
		s.Log("Failed to wait for idle, ignoring: ", err)
	}

	t.Window, err = ash.GetARCAppWindowInfo(ctx, tconn, Package)
	if err != nil {
		s.Fatal("Failed to get ARC app window info: ", err)
	}

	tester := newTester(tconn, d, act)
	testFunc(ctx, s, tconn, t, tester)
}

// centerOfWindow locates the center of the Activity's window in DP in the display's coordinates.
func (t *wMTestState) centerOfWindow() coords.Point {
	return t.Window.BoundsInRoot.CenterPoint()
}

// expectedPoint takes a coords.Point representing the coordinate where an input event is injected
// in DP in the display space, and returns a coords.Point representing where it is expected to be
// injected in the Android application window's coordinate space.
func (t *wMTestState) expectedPoint(p coords.Point) coords.Point {
	insetLeft := t.Window.BoundsInRoot.Left
	insetTop := t.Window.BoundsInRoot.Top
	if t.shouldCaptionBeVisible() {
		insetTop += t.Window.CaptionHeight
	}
	return coords.NewPoint(int(float64(p.X-insetLeft)*t.Scale), int(float64(p.Y-insetTop)*t.Scale))
}

// shouldCaptionBeVisible returns true if the caption should be visible for the Android application when
// the respective WM params are applied.
func (t *wMTestState) shouldCaptionBeVisible() bool {
	return !t.VerifiedTabletMode && t.VerifiedWindowState != nil && *t.VerifiedWindowState != ash.WindowStateFullscreen
}

// wMTestFunc represents the sub-test function that verifies certain motion input functionality
// using the tester and the provided wMTestState.
type wMTestFunc func(ctx context.Context, s *testing.State, tconn *chrome.TestConn, t *wMTestState, tester *tester)

// tester holds resources associated with ArcMotionInputTest activity.
type tester struct {
	tconn *chrome.TestConn
	d     *ui.Device
	act   *arc.Activity
}

var defaultPollOptions = &testing.PollOptions{Timeout: 30 * time.Second}

// newTester creates a new instance of a tester.
// The provided activity should be started before any of the tester's methods are called.
// All provided arguments must outlive the tester.
func newTester(tconn *chrome.TestConn, d *ui.Device, act *arc.Activity) *tester {
	return &tester{
		tconn: tconn,
		d:     d,
		act:   act,
	}
}

// clearMotionEvents tells the test application to clear the events that it is currently reporting,
// and verifies that no events are reported. This is done by sending an intent with the appropriate
// action to Android, which is subsequently picked up by the MotionInputTest application and handled
// appropriately.
func (t *tester) clearMotionEvents(ctx context.Context) error {
	prefixes := []string{"-a", intentActionClearEvents}
	if err := t.act.StartWithArgs(ctx, t.tconn, prefixes, nil); err != nil {
		return errors.Wrap(err, "failed to send the clear events intent")
	}
	if err := t.expectMotionEvents(ctx); err != nil {
		return errors.Wrap(err, "failed to verify that the reported MotionEvents were cleared")
	}
	return nil
}

// expectEventsAndClear is a convenience function that verifies expected events and clears the
// events to be ready for the next assertions.
func (t *tester) expectEventsAndClear(ctx context.Context, matchers ...Matcher) error {
	if err := t.expectMotionEvents(ctx, matchers...); err != nil {
		return errors.Wrap(err, "failed to verify expected events")
	}
	if err := t.clearMotionEvents(ctx); err != nil {
		return errors.Wrap(err, "failed to clear events")
	}
	return nil
}

// expectMotionEvents polls readMotionEvents repeatedly until it receives motionEvents that
// successfully match all of the provided motionEventMatchers in order, or until it times out.
func (t *tester) expectMotionEvents(ctx context.Context, matchers ...Matcher) error {
	return testing.Poll(ctx, func(ctx context.Context) error {
		events, err := t.readMotionEvents(ctx)
		if err != nil {
			return errors.Wrap(err, "failed to read motion event")
		}

		// TODO(b/156655077): Remove filtering of batched events after the bug is fixed.
		//  We filter out batched events because in some cases, we observe extraneous events that
		//  are automatically generated in the input pipeline. Since the extraneous events are
		//  generated immediately after real events, they are batched, so we skip batched events.
		for i := 0; i < len(events); {
			if events[i].Batched {
				events = append(events[:i], events[i+1:]...)
				continue
			}
			i++
		}

		if len(events) != len(matchers) {
			return errors.Errorf("did not receive the exact number of events as expected; got: %d, want: %d", len(events), len(matchers))
		}

		for i := 0; i < len(matchers); i++ {
			if err := matchers[i](&events[i]); err != nil {
				return testing.PollBreak(err)
			}
		}
		return nil
	}, defaultPollOptions)
}

// readMotionEvents unmarshalls the JSON string in the TextView representing the MotionEvents
// received by ArcMotionInputTest.apk, and returns it as a slice of motionEvent.
func (t *tester) readMotionEvents(ctx context.Context) ([]motionEvent, error) {
	view := t.d.Object(ui.ID(Package + ":id/motion_event"))
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

// motionEvent represents a motionEvent that was received by the Android application.
// For all Axis values that represent an absolute location, the values are in the
// coordinate space of the Android window (i.e. 0,0 is the top left corner of the application
// window in Android).
type motionEvent struct {
	Action      Action             `json:"action"`
	DeviceID    int                `json:"device_id"`
	Sources     []Source           `json:"sources"`
	PointerAxes []map[Axis]float64 `json:"pointer_axes"`
	// Batched is true if this event was included in the history of another motionEvent in Android,
	// and false otherwise. See more information about batching at:
	// https://cs.android.com/android/platform/superproject/+/HEAD:frameworks/base/core/java/android/view/motionEvent.java;l=93
	Batched bool `json:"batched"`
}

// Action represents a motionEvent's action key.
// The values are from Android's motionEvent.java.
// See: https://cs.android.com/android/platform/superproject/+/HEAD:frameworks/base/core/java/android/view/motionEvent.java
type Action string

// Axis represents a motionEvent's axis key.
// The values are from Android's motionEvent.java.
// See: https://cs.android.com/android/platform/superproject/+/HEAD:frameworks/base/core/java/android/view/motionEvent.java
type Axis string

// Source represents an input device's source.
// The values should be kept in sync with ArcMotionInputTest.apk.
type Source string

// ActionSourceMatcher returns a motionEventMatcher that matches a motionEvent with the provided
// action and source.
func ActionSourceMatcher(a Action, s Source) Matcher {
	return func(event *motionEvent) error {
		sourceMatches := false
		for _, v := range event.Sources {
			if v == s {
				sourceMatches = true
				break
			}
		}
		var err error
		if !sourceMatches {
			err = errors.Wrapf(err, "source does not match: got %v; want %s", event.Sources, s)
		}
		if event.Action != a {
			err = errors.Wrapf(err, "action does not match: got %s; want: %s", event.Action, a)
		}
		return err
	}
}

// singlePointerMatcher returns a motionEventMatcher that matches a motionEvent with a single
// pointer that has the following axes: axisX, axisY, and axisPressure.
func singlePointerMatcher(a Action, s Source, p coords.Point, pressure float64) Matcher {
	return func(event *motionEvent) error {
		if err := ActionSourceMatcher(a, s)(event); err != nil {
			return err
		}
		if pointerCount := len(event.PointerAxes); pointerCount != 1 {
			return errors.Errorf("pointer count does not match: got: %d; want: %d", pointerCount, 1)
		}
		axisMatcher := func(axis Axis, expected, epsilon float64) error {
			v := event.PointerAxes[0][axis]
			if math.Abs(v-expected) > epsilon {
				return errors.Errorf("value of axis %s did not match: got %.5f; want %.5f; epsilon %.5f", axis, v, expected, epsilon)
			}
			return nil
		}
		const (
			// coordinateAxisEpsilon is the epsilon value to be used when comparing axis values that
			// represent absolute display coordinates. Scaling and conversions from Chrome to Android's
			// display spaces means absolute coordinates can be off by up to two pixels.
			coordinateAxisEpsilon = 2e0

			// defaultAxisEpsilon is the epsilon value to be used when comparing axis values that do
			// not need to be scaled or converted, like pressure (which is in the range [0,1]). We
			// expect these values to be more precise.
			defaultAxisEpsilon = 1e-5
		)
		var err error
		if e := axisMatcher(AxisX, float64(p.X), coordinateAxisEpsilon); e != nil {
			err = errors.Wrap(err, e.Error())
		}
		if e := axisMatcher(AxisY, float64(p.Y), coordinateAxisEpsilon); e != nil {
			err = errors.Wrap(err, e.Error())
		}
		if e := axisMatcher(AxisPressure, pressure, defaultAxisEpsilon); e != nil {
			err = errors.Wrap(err, e.Error())
		}
		return err
	}
}

// Constant definitions for MotionEvent that should be kept in sync with its respective sources.
const (
	ActionDown          Action = "ACTION_DOWN"
	ActionUp            Action = "ACTION_UP"
	ActionMove          Action = "ACTION_MOVE"
	ActionHoverMove     Action = "ACTION_HOVER_MOVE"
	ActionHoverEnter    Action = "ACTION_HOVER_ENTER"
	ActionHoverExit     Action = "ACTION_HOVER_EXIT"
	ActionButtonPress   Action = "ACTION_BUTTON_PRESS"
	ActionButtonRelease Action = "ACTION_BUTTON_RELEASE"

	AxisX        Axis = "AXIS_X"
	AxisY        Axis = "AXIS_Y"
	AxisPressure Axis = "AXIS_PRESSURE"

	SourceTouchscreen   Source = "touchscreen"
	SourceMouse         Source = "mouse"
	SourceMouseRelative Source = "mouse_relative"
)

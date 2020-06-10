// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"time"

	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/arc/ui"
	"chromiumos/tast/local/bundles/cros/arc/motioninput"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/display"
	"chromiumos/tast/local/chrome/ui/mouse"
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
		Pre:          arc.Booted(),
	})
}

// singleTouchMatcher returns a motionEventMatcher that matches events from a Touchscreen device.
func singleTouchMatcher(a motioninput.Action, p coords.Point) motioninput.Matcher {
	return motioninput.SinglePointerMatcher(a, motioninput.SourceTouchscreen, p, 1)
}

// mouseMatcher returns a motionEventMatcher that matches events from a Mouse device.
func mouseMatcher(a motioninput.Action, p coords.Point) motioninput.Matcher {
	pressure := 0.
	if a == motioninput.ActionMove || a == motioninput.ActionDown || a == motioninput.ActionButtonPress || a == motioninput.ActionHoverExit {
		pressure = 1.
	}
	return motioninput.SinglePointerMatcher(a, motioninput.SourceMouse, p, pressure)
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

		d, err := ui.NewDevice(ctx, a)
		if err != nil {
			s.Fatal("Failed initializing UI Automator: ", err)
		}
		defer d.Close()

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

		s.Log("Installing apk ", motioninput.APK)
		if err := a.Install(ctx, arc.APKPath(motioninput.APK)); err != nil {
			s.Fatalf("Failed installing %s: %v", motioninput.APK, err)
		}

		act, err := arc.NewActivity(a, motioninput.Package, motioninput.EventReportingActivity)
		if err != nil {
			s.Fatal("Failed to create an activity: ", err)
		}
		defer act.Close()

		if err := act.Start(ctx, tconn); err != nil {
			s.Fatal("Failed to start an activity: ", err)
		}
		defer act.Stop(ctx, tconn)

		if err := ash.WaitForVisible(ctx, tconn, motioninput.Package); err != nil {
			s.Fatal("Failed to wait for activity to be visible: ", err)
		}

		if params.WmEventToSend != nil {
			event := params.WmEventToSend.(ash.WMEventType)
			s.Log("Sending wm event: ", event)
			if _, err := ash.SetARCAppWindowState(ctx, tconn, motioninput.Package, event); err != nil {
				s.Fatalf("Failed to set ARC app window state with event %s: %v", event, err)
			}
		}

		if params.WmStateToVerify != nil {
			windowState := params.WmStateToVerify.(ash.WindowStateType)
			s.Log("Verifying window state: ", windowState)
			if err := ash.WaitForARCAppWindowState(ctx, tconn, motioninput.Package, windowState); err != nil {
				s.Fatal("Failed to verify app window state: ", windowState)
			}
		}

		if err := d.WaitForIdle(ctx, 10*time.Second); err != nil {
			s.Log("Failed to wait for idle, ignoring: ", err)
		}

		test.w, err = ash.GetARCAppWindowInfo(ctx, tconn, motioninput.Package)
		if err != nil {
			s.Fatal("Failed to get ARC app window info: ", err)
		}

		test.tester = motioninput.NewTester(tconn, d, act)

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
	tester      *motioninput.Tester
	displayInfo *display.Info
	scale       float64
	w           *ash.Window
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
	if err := test.tester.ExpectEventsAndClear(ctx, singleTouchMatcher(motioninput.ActionDown, expected)); err != nil {
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
		if err := test.tester.ExpectEventsAndClear(ctx, singleTouchMatcher(motioninput.ActionMove, expected)); err != nil {
			s.Fatal("Failed to expect events and clear: ", err)
		}
	}

	s.Log("Verifying touch up event at ", expected)
	x, y = tcc.ConvertLocation(pointDP)
	if err := stw.End(); err != nil {
		s.Fatalf("Could not inject end at (%d, %d)", x, y)
	}
	if err := test.tester.ExpectEventsAndClear(ctx, singleTouchMatcher(motioninput.ActionUp, expected)); err != nil {
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
	if err := mouse.Move(ctx, t.tconn, p, 500*time.Millisecond); err != nil {
		s.Fatalf("Failed to inject move at %v: %v", e, err)
	}
	// TODO(b/155783589): Investigate why there are sometimes two ACTION_HOVER_ENTER events being
	//  sent. Once resolved, add expectation for ACTION_HOVER_ENTER and remove sleep.
	if err := testing.Sleep(ctx, time.Second); err != nil {
		s.Fatal("Failed to sleep: ", err)
	}
	if err := t.tester.ClearMotionEvents(ctx); err != nil {
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
		if err := mouse.Move(ctx, t.tconn, p, 0); err != nil {
			s.Fatalf("Failed to inject move at %v: %v", e, err)
		}
		if err := t.tester.ExpectEventsAndClear(ctx, mouseMatcher(motioninput.ActionHoverMove, e)); err != nil {
			s.Fatal("Failed to expect events and clear: ", err)
		}
	}

	if err := mouse.Press(ctx, t.tconn, mouse.LeftButton); err != nil {
		s.Fatal("Failed to press button on mouse: ", err)
	}
	if err := t.tester.ExpectEventsAndClear(ctx, mouseMatcher(motioninput.ActionHoverExit, e), mouseMatcher(motioninput.ActionDown, e), mouseMatcher(motioninput.ActionButtonPress, e)); err != nil {
		s.Fatal("Failed to expect events and clear: ", err)
	}

	for i := 0; i < numMotionEventIterations; i++ {
		p.X -= deltaDP
		p.Y -= deltaDP
		e = t.expectedPoint(p)

		s.Log("Verifying mouse move event at ", e)
		if err := mouse.Move(ctx, t.tconn, p, 0); err != nil {
			s.Fatalf("Failed to inject move at %v: %v", e, err)
		}
		if err := t.tester.ExpectEventsAndClear(ctx, mouseMatcher(motioninput.ActionMove, e)); err != nil {
			s.Fatal("Failed to expect events and clear: ", err)
		}
	}

	if err := mouse.Release(ctx, t.tconn, mouse.LeftButton); err != nil {
		s.Fatal("Failed to release mouse button: ", err)
	}
	if err := t.tester.ExpectEventsAndClear(ctx, mouseMatcher(motioninput.ActionButtonRelease, e), mouseMatcher(motioninput.ActionUp, e)); err != nil {
		s.Fatal("Failed to expect events and clear: ", err)
	}

	p.X -= deltaDP
	p.Y -= deltaDP
	e = t.expectedPoint(p)

	if err := mouse.Move(ctx, t.tconn, p, 0); err != nil {
		s.Fatalf("Failed to inject move at %v: %v", e, err)
	}
	if err := t.tester.ExpectEventsAndClear(ctx, mouseMatcher(motioninput.ActionHoverEnter, e), mouseMatcher(motioninput.ActionHoverMove, e)); err != nil {
		s.Fatal("Failed to expect events and clear: ", err)
	}
}

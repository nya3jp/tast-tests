// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"time"

	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/bundles/cros/arc/motioninput"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/uiauto/mouse"
	"chromiumos/tast/local/coords"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

// mouseInputParams holds a collection of tests to run in the given test setup.
type mouseInputParams struct {
	tests []motioninput.WMTestParams
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         MouseInput,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Verifies mouse input in various window states on Android",
		Contacts:     []string{"prabirmsp@chromium.org", "arc-framework+tast@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome", "android_vm"},
		Fixture:      "arcBooted",
		Timeout:      10 * time.Minute,
		Params: []testing.Param{{
			Name:              "tablet",
			ExtraHardwareDeps: hwdep.D(hwdep.InternalDisplay(), hwdep.TouchScreen()),
			Val: mouseInputParams{[]motioninput.WMTestParams{
				{
					Name:          "Tablet",
					TabletMode:    true,
					WmEventToSend: nil,
				}, {
					Name:          "Tablet Snapped Left",
					TabletMode:    true,
					WmEventToSend: ash.WMEventSnapLeft,
				}, {
					Name:          "Tablet Snapped Right",
					TabletMode:    true,
					WmEventToSend: ash.WMEventSnapRight,
				},
			}},
		}, {
			Name: "clamshell",
			Val: mouseInputParams{[]motioninput.WMTestParams{
				{
					Name:          "Clamshell Normal",
					TabletMode:    false,
					WmEventToSend: ash.WMEventNormal,
				}, {
					Name:          "Clamshell Fullscreen",
					TabletMode:    false,
					WmEventToSend: ash.WMEventFullscreen,
				}, {
					Name:          "Clamshell Maximized",
					TabletMode:    false,
					WmEventToSend: ash.WMEventMaximize,
				},
			}},
		}},
	})
}

// MouseInput runs several sub-tests, where each sub-test sets up the Chrome WM environment as
// specified by the motioninput.WMTestParams. Each sub-test installs and runs the test application
// (ArcMotionInputTest.apk), injects various input events into ChromeOS through the test API,
// and verifies that those events were received by the Android application in the expected screen
// locations.
func MouseInput(ctx context.Context, s *testing.State) {
	p := s.FixtValue().(*arc.PreData)
	cr := p.Chrome
	a := p.ARC
	d := p.UIDevice
	testParams := s.Param().(mouseInputParams)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create test API connection: ", err)
	}

	if err := a.Install(ctx, arc.APKPath(motioninput.APK)); err != nil {
		s.Fatal("Failed installing ", motioninput.APK, ": ", err)
	}

	for _, params := range testParams.tests {
		s.Run(ctx, params.Name+": Verify Mouse", func(ctx context.Context, s *testing.State) {
			motioninput.RunTestWithWMParams(ctx, s, tconn, d, a, &params, verifyMouse)
		})
	}
}

// mouseMatcher returns a motionEventMatcher that matches events from a Mouse device.
func mouseMatcher(a motioninput.Action, p coords.Point) motioninput.Matcher {
	pressure := 0.
	if a == motioninput.ActionMove || a == motioninput.ActionDown || a == motioninput.ActionButtonPress || a == motioninput.ActionHoverExit {
		pressure = 1.
	}
	return motioninput.SinglePointerMatcher(a, motioninput.SourceMouse, p, pressure)
}

// initialEventMatcher returns a motionEventMatcher that matches the first mouse event
// that should be received by an app.
func initialEventMatcher(p coords.Point) motioninput.Matcher {
	return motioninput.MatcherOr(mouseMatcher(motioninput.ActionHoverEnter, p), mouseMatcher(motioninput.ActionHoverMove, p))
}

// verifyMouse tests the behavior of mouse events injected into Ash on Android apps. It tests hover,
// button, and drag events. It does not use the uinput mouse to inject events because the scale
// relation between the relative movements injected by a relative mouse device and the display
// pixels is determined by ChromeOS and could vary between devices.
func verifyMouse(ctx context.Context, s *testing.State, tconn *chrome.TestConn, t *motioninput.WMTestState, tester *motioninput.Tester) {
	s.Log("Verifying Mouse")

	p := t.CenterOfWindow()
	e := t.ExpectedPoint(p)

	s.Log("Injected initial move, waiting... ")
	if err := mouse.Move(tconn, p, 0)(ctx); err != nil {
		s.Fatalf("Failed to inject move at %v: %v", e, err)
	}
	if err := tester.WaitUntilEvent(ctx, initialEventMatcher(e)); err != nil {
		s.Fatal("Failed to wait for the initial hover event: ", err)
	}
	if err := tester.ClearMotionEvents(ctx); err != nil {
		s.Fatal("Failed to clear events: ", err)
	}

	// numMouseMoveIterations is the number of times certain motion events should be repeated in
	// a test. Increasing this number will increase the time it takes to run the test.
	const numMouseMoveIterations = 1

	// deltaDP is the amount we want to move the mouse pointer between each successive injected
	// event. We use an arbitrary value that is not too large so that we can safely assume that
	// the injected events stay within the bounds of the application in the various WM states, so
	// that clicks performed after moving the mouse are still inside the application.
	const deltaDP = 5

	for i := 0; i < numMouseMoveIterations; i++ {
		p.X += deltaDP
		p.Y += deltaDP
		e = t.ExpectedPoint(p)

		s.Log("Verifying mouse move event at ", e)
		if err := mouse.Move(tconn, p, 0)(ctx); err != nil {
			s.Fatalf("Failed to inject move at %v: %v", e, err)
		}
		if err := tester.ExpectEventsAndClear(ctx, mouseMatcher(motioninput.ActionHoverMove, e)); err != nil {
			s.Fatal("Failed to expect events and clear: ", err)
		}
	}

	if err := mouse.Press(tconn, mouse.LeftButton)(ctx); err != nil {
		s.Fatal("Failed to press button on mouse: ", err)
	}
	if err := tester.ExpectEventsAndClear(
		ctx,
		mouseMatcher(motioninput.ActionHoverExit, e),
		mouseMatcher(motioninput.ActionDown, e),
		mouseMatcher(motioninput.ActionButtonPress, e)); err != nil {
		s.Fatal("Failed to expect events and clear: ", err)
	}

	for i := 0; i < numMouseMoveIterations; i++ {
		p.X -= deltaDP
		p.Y -= deltaDP
		e = t.ExpectedPoint(p)

		s.Log("Verifying mouse move event at ", e)
		if err := mouse.Move(tconn, p, 0)(ctx); err != nil {
			s.Fatalf("Failed to inject move at %v: %v", e, err)
		}
		if err := tester.ExpectEventsAndClear(ctx, mouseMatcher(motioninput.ActionMove, e)); err != nil {
			s.Fatal("Failed to expect events and clear: ", err)
		}
	}

	if err := mouse.Release(tconn, mouse.LeftButton)(ctx); err != nil {
		s.Fatal("Failed to release mouse button: ", err)
	}
	if err := tester.ExpectEventsAndClear(
		ctx,
		mouseMatcher(motioninput.ActionButtonRelease, e),
		mouseMatcher(motioninput.ActionUp, e),
		mouseMatcher(motioninput.ActionHoverEnter, e),
		mouseMatcher(motioninput.ActionHoverMove, e)); err != nil {
		s.Fatal("Failed to expect events and clear: ", err)
	}

	p.X -= deltaDP
	p.Y -= deltaDP
	e = t.ExpectedPoint(p)

	if err := mouse.Move(tconn, p, 0)(ctx); err != nil {
		s.Fatalf("Failed to inject move at %v: %v", e, err)
	}
	if err := tester.ExpectEventsAndClear(ctx, mouseMatcher(motioninput.ActionHoverMove, e)); err != nil {
		s.Fatal("Failed to expect events and clear: ", err)
	}
}

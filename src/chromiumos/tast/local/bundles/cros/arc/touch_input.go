// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"

	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/bundles/cros/arc/motioninput"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/coords"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         TouchInput,
		Desc:         "Verifies touch input in various window states on Android",
		Contacts:     []string{"prabirmsp@chromium.org", "arc-framework+tast@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome", "android_vm"},
		Fixture:      "arcBooted",
	})
}

// TouchInput runs several sub-tests, where each sub-test sets up the Chrome WM environment as
// specified by the motioninput.WMTestParams. Each sub-test installs and runs the test application
// (ArcMotionInputTest.apk), injects various input events into ChromeOS through uinput devices,
// and verifies that those events were received by the Android application in the expected screen
// locations.
func TouchInput(ctx context.Context, s *testing.State) {
	p := s.FixtValue().(*arc.PreData)
	cr := p.Chrome
	a := p.ARC

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create test API connection: ", err)
	}
	defer tconn.Close()

	d, err := a.NewUIDevice(ctx)
	if err != nil {
		s.Fatal("Failed initializing UI Automator: ", err)
	}
	defer d.Close(ctx)

	if err := a.Install(ctx, arc.APKPath(motioninput.APK)); err != nil {
		s.Fatal("Failed installing ", motioninput.APK, ": ", err)
	}

	for _, params := range []motioninput.WMTestParams{
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
		// TODO(b/155500968): Investigate why a touched location on the touchscreen does not match
		//   up with the same location on the display for some ChromeOS devices.
	} {
		s.Run(ctx, params.Name+": Verify Touch", func(ctx context.Context, s *testing.State) {
			motioninput.RunTestWithWMParams(ctx, s, tconn, d, a, &params, verifyTouchscreen)
		})
	}
}

// singleTouchMatcher returns a motioninput.Matcher that matches events from a Touchscreen device.
func singleTouchMatcher(a motioninput.Action, p coords.Point) motioninput.Matcher {
	return motioninput.SinglePointerMatcher(a, motioninput.SourceTouchscreen, p, 1)
}

// verifyTouchscreen tests the behavior of events injected from a uinput touchscreen device. It
// injects a down event, followed by several move events, and finally an up event with a single
// touch pointer.
func verifyTouchscreen(ctx context.Context, s *testing.State, tconn *chrome.TestConn, t *motioninput.WMTestState, tester *motioninput.Tester) {
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

	tcc := tew.NewTouchCoordConverter(t.DisplayInfo.Bounds.Size())

	pointDP := t.CenterOfWindow()
	expected := t.ExpectedPoint(pointDP)

	s.Log("Verifying touch down event at ", expected)
	x, y := tcc.ConvertLocation(pointDP)
	if err := stw.Move(x, y); err != nil {
		s.Fatalf("Could not inject move at (%d, %d)", x, y)
	}
	if err := tester.ExpectEventsAndClear(ctx, singleTouchMatcher(motioninput.ActionDown, expected)); err != nil {
		s.Fatal("Failed to expect events and clear: ", err)
	}

	const (
		// numTouchIterations is the number of times touch events should be repeated in a test.
		// Increasing this number will increase the duration.
		numTouchIterations = 5

		// deltaDP is the amount we want to move the touch pointer between each successive injected
		// event. We use an arbitrary value that is not too large so that we can safely assume that
		// the injected events stay within the bounds of the display.
		deltaDP = 5
	)

	for i := 0; i < numTouchIterations; i++ {
		pointDP.X += deltaDP
		pointDP.Y += deltaDP
		expected = t.ExpectedPoint(pointDP)

		s.Log("Verifying touch move event at ", expected)
		x, y := tcc.ConvertLocation(pointDP)
		if err := stw.Move(x, y); err != nil {
			s.Fatalf("Could not inject move at (%d, %d): %v", x, y, err)
		}
		if err := tester.ExpectEventsAndClear(ctx, singleTouchMatcher(motioninput.ActionMove, expected)); err != nil {
			s.Fatal("Failed to expect events and clear: ", err)
		}
	}

	s.Log("Verifying touch up event at ", expected)
	x, y = tcc.ConvertLocation(pointDP)
	if err := stw.End(); err != nil {
		s.Fatalf("Could not inject end at (%d, %d)", x, y)
	}
	if err := tester.ExpectEventsAndClear(ctx, singleTouchMatcher(motioninput.ActionUp, expected)); err != nil {
		s.Fatal("Failed to expect events and clear: ", err)
	}
}

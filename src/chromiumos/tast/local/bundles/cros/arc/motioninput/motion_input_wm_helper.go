// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package motioninput

import (
	"context"
	"time"

	"chromiumos/tast/local/android/ui"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/display"
	"chromiumos/tast/local/coords"
	"chromiumos/tast/testing"
)

// WMEventToSend holds an ash.WMEventType or nil.
type WMEventToSend interface{}

// WMTestParams holds the test parameters used to set up the WM environment in Chrome, and
// represents a single sub-test.
type WMTestParams struct {
	Name          string        // A description of the subtest.
	TabletMode    bool          // If true, the device will be put in tablet mode.
	WmEventToSend WMEventToSend // This must be of type ash.WMEventType, and can be nil.
}

// WMTestState holds various values that represent the test state for each sub-test.
// It is created for convenience to reduce the number of function parameters.
type WMTestState struct {
	VerifiedWindowState *ash.WindowStateType // The window state of the test Activity after it is confirmed by Chrome. This can be nil if the window state was not verified.
	VerifiedTabletMode  bool                 // The state of tablet mode after it is confirmed by Chrome.
	DisplayInfo         *display.Info        // The info for the display the Activity is in.
	Scale               float64              // The scale factor used to convert Chrome's DP to Android's pixels.
	Window              *ash.Window          // The state of the test Activity's window.
}

// CenterOfWindow locates the center of the Activity's window in DP in the display's coordinates.
func (t *WMTestState) CenterOfWindow() coords.Point {
	return t.Window.BoundsInRoot.CenterPoint()
}

// ExpectedPoint takes a coords.Point representing the coordinate where an input event is injected
// in DP in the display space, and returns a coords.Point representing where it is expected to be
// injected in the Android application window's coordinate space.
func (t *WMTestState) ExpectedPoint(p coords.Point) coords.Point {
	insetLeft := t.Window.BoundsInRoot.Left
	insetTop := t.Window.BoundsInRoot.Top
	if t.shouldCaptionBeVisible() {
		insetTop += t.Window.CaptionHeight
	}
	return coords.NewPoint(int(float64(p.X-insetLeft)*t.Scale), int(float64(p.Y-insetTop)*t.Scale))
}

// shouldCaptionBeVisible returns true if the caption should be visible for the Android application when
// the respective WM params are applied.
func (t *WMTestState) shouldCaptionBeVisible() bool {
	return !t.VerifiedTabletMode && t.VerifiedWindowState != nil && *t.VerifiedWindowState != ash.WindowStateFullscreen
}

// WMTestFunc represents the sub-test function that verifies certain motion input functionality
// using the Tester and the provided WMTestState.
type WMTestFunc func(ctx context.Context, s *testing.State, tconn *chrome.TestConn, t *WMTestState, tester *Tester)

// RunTestWithWMParams sets up the window management state of the test device to that specified in the given
// WMTestParams, and runs the WMTestFunc. The APK must be installed on the device before using this helper.
func RunTestWithWMParams(ctx context.Context, s *testing.State, tconn *chrome.TestConn, d *ui.Device, a *arc.ARC, params *WMTestParams, testFunc WMTestFunc) {
	t := &WMTestState{}

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

	err = testing.Poll(ctx, func(ctx context.Context) error {
		return act.Start(ctx, tconn)
	}, &testing.PollOptions{Timeout: 30 * time.Second})
	if err != nil {
		s.Fatal("Failed to start activity: ", err)
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

	tester := NewTester(tconn, d, act)
	testFunc(ctx, s, tconn, t, tester)
}

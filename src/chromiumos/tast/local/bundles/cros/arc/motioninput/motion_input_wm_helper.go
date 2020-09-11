// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package motioninput

import (
	"context"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/arc/ui"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/display"
	"chromiumos/tast/local/coords"
	"chromiumos/tast/testing"
)

// WmEventToSend holds an ash.WMEventType or nil.
type WmEventToSend interface{}

// WmTestParams holds the test parameters used to set up the WM environment in Chrome, and
// represents a single sub-test.
type WmTestParams struct {
	Name          string        // A description of the subtest.
	TabletMode    bool          // If true, the device will be put in tablet mode.
	WmEventToSend WmEventToSend // This must be of type ash.WMEventType, and can be nil.
}

// WmTestState holds various values that represent the test state for each sub-test.
// It is created for convenience to reduce the number of function parameters.
type WmTestState struct {
	Tconn            *chrome.TestConn
	AckedWindowState *ash.WindowStateType
	DisplayInfo      *display.Info
	Scale            float64
	W                *ash.Window
	Params           *WmTestParams
}

// CenterOfWindow locates the center of the Activity's window in DP in the display's coordinates.
func (t *WmTestState) CenterOfWindow() coords.Point {
	return t.W.BoundsInRoot.CenterPoint()
}

// ExpectedPoint takes a coords.Point representing the coordinate where an input event is injected
// in DP in the display space, and returns a coords.Point representing where it is expected to be
// injected in the Android application window's coordinate space.
func (t *WmTestState) ExpectedPoint(p coords.Point) coords.Point {
	insetLeft := t.W.BoundsInRoot.Left
	insetTop := t.W.BoundsInRoot.Top
	if t.isCaptionVisible() {
		insetTop += t.W.CaptionHeight
	}
	return coords.NewPoint(int(float64(p.X-insetLeft)*t.Scale), int(float64(p.Y-insetTop)*t.Scale))
}

// isCaptionVisible returns true if the caption should be visible for the Android application when
// the respective WM params are applied.
func (t *WmTestState) isCaptionVisible() bool {
	return !t.Params.TabletMode && t.AckedWindowState != nil && *t.AckedWindowState != ash.WindowStateFullscreen
}

// WmTestRunner holds the prerequisites required for this helper, and is used to run WmTestFuncs.
type WmTestRunner struct {
	tconn *chrome.TestConn
	arc   *arc.ARC
	d     *ui.Device
}

// NewWmTestRunner creates a new instance of the WmTestRunner, and performs any one-time
// initializations that do not need to be repeated between successive WmTestFunc runs.
func NewWmTestRunner(ctx context.Context, tconn *chrome.TestConn, a *arc.ARC, d *ui.Device) (WmTestRunner, error) {
	if err := a.Install(ctx, arc.APKPath(APK)); err != nil {
		return WmTestRunner{}, errors.Wrapf(err, "failed installing %s", APK)
	}
	return WmTestRunner{
		tconn: tconn,
		arc:   a,
		d:     d,
	}, nil
}

// WmTestFunc represents the sub-test function that verifies certain motion input functionality
// using the Tester and the provided WmTestState.
type WmTestFunc func(ctx context.Context, s *testing.State, t *WmTestState, tester *Tester)

// RunTest sets up the window management state of the test device to that specified in the given
// WmTestParams, and runs the WmTestFunc.
func (r *WmTestRunner) RunTest(ctx context.Context, s *testing.State, params *WmTestParams, name string, testFunc WmTestFunc) {
	t := &WmTestState{}
	t.Tconn = r.tconn
	t.Params = params

	deviceMode := "clamshell"
	if params.TabletMode {
		deviceMode = "tablet"
	}
	s.Logf("Setting device to %v mode", deviceMode)
	cleanup, err := ash.EnsureTabletModeEnabled(ctx, r.tconn, params.TabletMode)
	if err != nil {
		s.Fatal("Failed to ensure tablet mode enabled: ", err)
	}
	defer cleanup(ctx)

	infos, err := display.GetInfo(ctx, r.tconn)
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

	act, err := arc.NewActivity(r.arc, Package, EventReportingActivity)
	if err != nil {
		s.Fatal("Failed to create an activity: ", err)
	}
	defer act.Close()

	if err := act.Start(ctx, r.tconn); err != nil {
		s.Fatal("Failed to start an activity: ", err)
	}
	defer act.Stop(ctx, r.tconn)

	if err := ash.WaitForVisible(ctx, r.tconn, Package); err != nil {
		s.Fatal("Failed to wait for activity to be visible: ", err)
	}

	if params.WmEventToSend != nil {
		event := params.WmEventToSend.(ash.WMEventType)
		s.Log("Sending wm event: ", params.WmEventToSend)
		windowState, err := ash.SetARCAppWindowState(ctx, r.tconn, Package, event)
		if err != nil {
			s.Fatalf("Failed to set ARC app window state with event %s: %v", event, err)
		}
		s.Log("Verifying window state: ", windowState)
		if err := ash.WaitForARCAppWindowState(ctx, r.tconn, Package, windowState); err != nil {
			s.Fatal("Failed to verify app window state: ", windowState)
		}
		t.AckedWindowState = &windowState
	}

	if err := r.d.WaitForIdle(ctx, 10*time.Second); err != nil {
		s.Log("Failed to wait for idle, ignoring: ", err)
	}

	t.W, err = ash.GetARCAppWindowInfo(ctx, r.tconn, Package)
	if err != nil {
		s.Fatal("Failed to get ARC app window info: ", err)
	}

	tester := NewTester(r.tconn, r.d, act)

	s.Run(ctx, params.Name+": "+name, func(ctx context.Context, s *testing.State) {
		testFunc(ctx, s, t, tester)
	})
}

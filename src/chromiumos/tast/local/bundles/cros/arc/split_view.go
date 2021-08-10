// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/android/ui"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/display"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

type splitViewTestParams struct {
	tabletMode            bool
	startFromHomeLauncher bool
}

func init() {
	testing.AddTest(&testing.Test{
		Func:     SplitView,
		Desc:     "Tests split view works properly with ARC apps",
		Contacts: []string{"tetsui@chromium.org", "amusbach@chromium.org", "arc-framework+tast@google.com"},
		Attr:     []string{"group:mainline", "informational"},
		// TODO(b/188754062): Add support for mouse input and remove the internal display deps
		HardwareDeps: hwdep.D(hwdep.InternalDisplay()),
		SoftwareDeps: []string{"chrome"},
		Timeout:      4 * time.Minute,
		Fixture:      "arcBooted",
		Params: []testing.Param{
			{
				Name:              "clamshell_mode",
				ExtraSoftwareDeps: []string{"android_p"},
				Val:               splitViewTestParams{tabletMode: false, startFromHomeLauncher: false},
			},
			{
				Name:              "clamshell_mode_vm",
				ExtraSoftwareDeps: []string{"android_vm"},
				Val:               splitViewTestParams{tabletMode: false, startFromHomeLauncher: false},
			},
			{
				Name:              "tablet_mode",
				ExtraSoftwareDeps: []string{"android_p"},
				Val:               splitViewTestParams{tabletMode: true, startFromHomeLauncher: false},
			},
			{
				Name:              "tablet_mode_vm",
				ExtraSoftwareDeps: []string{"android_vm"},
				Val:               splitViewTestParams{tabletMode: true, startFromHomeLauncher: false},
			},
			{
				Name:              "tablet_home_launcher",
				ExtraSoftwareDeps: []string{"android_p"},
				Val:               splitViewTestParams{tabletMode: true, startFromHomeLauncher: true},
			},
			{
				Name:              "tablet_home_launcher_vm",
				ExtraSoftwareDeps: []string{"android_vm"},
				Val:               splitViewTestParams{tabletMode: true, startFromHomeLauncher: true},
			},
		},
	})
}

// dragToSnapFirstOverviewWindow finds the first window in overview, and drags
// to snap it. This function assumes that overview is already active.
func dragToSnapFirstOverviewWindow(ctx context.Context, tconn *chrome.TestConn, tew *input.TouchscreenEventWriter, stw *input.SingleTouchEventWriter, targetX input.TouchCoord) error {
	info, err := display.GetPrimaryInfo(ctx, tconn)
	if err != nil {
		return errors.Wrap(err, "failed to get the primary display info")
	}
	tcc := tew.NewTouchCoordConverter(info.Bounds.Size())

	w, err := ash.FindFirstWindowInOverview(ctx, tconn)
	if err != nil {
		// If you see this error on the second window snap (to the right), check if
		// b/143499564 has been reintroduced.
		return errors.Wrap(err, "failed to find window in overview grid")
	}

	centerX, centerY := tcc.ConvertLocation(w.OverviewInfo.Bounds.CenterPoint())
	if err := stw.LongPressAt(ctx, centerX, centerY); err != nil {
		return errors.Wrap(err, "failed to long-press to start dragging window")
	}
	if err := stw.Swipe(ctx, centerX, centerY, targetX, tew.Height()/2, time.Second); err != nil {
		return errors.Wrap(err, "failed to swipe for snapping window")
	}
	if err := stw.End(); err != nil {
		return errors.Wrap(err, "failed to end swipe")
	}

	return nil
}

type windowStateExpectations []struct {
	act      *arc.Activity
	ashState ash.WindowStateType
	arcState arc.WindowState
}

// waitForWindowStates waits for specified ARC apps to reach specified window
// states on both the Ash side and the ARC side.
func waitForWindowStates(ctx context.Context, tconn *chrome.TestConn, expectations windowStateExpectations) error {
	return testing.Poll(ctx, func(ctx context.Context) error {
		for _, test := range expectations {
			if actual, err := ash.GetARCAppWindowState(ctx, tconn, test.act.PackageName()); err != nil {
				return testing.PollBreak(errors.Wrap(err, "failed to get Ash window state"))
			} else if actual != test.ashState {
				return errors.Errorf("Ash window state was %v but should be %v", actual, test.ashState)
			}

			if actual, err := test.act.GetWindowState(ctx); err != nil {
				return testing.PollBreak(errors.Wrap(err, "failed to get ARC window state"))
			} else if actual != test.arcState {
				return errors.Errorf("ARC window state was %v but should be %v", actual, test.arcState)
			}
		}
		return nil
	}, nil)
}

// showActivityForSplitViewTest starts an activity and waits for it to be idle.
func showActivityForSplitViewTest(ctx context.Context, tconn *chrome.TestConn, a *arc.ARC, pkgName, activityName string) (*arc.Activity, error) {
	act, err := arc.NewActivity(a, pkgName, activityName)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create a new activity")
	}
	if err := act.Start(ctx, tconn); err != nil {
		act.Close()
		return nil, errors.Wrap(err, "failed to start the activity")
	}
	return act, nil
}

// waitForArcAppWindowAnimation waits for an ARC app window's animation.
func waitForArcAppWindowAnimation(ctx context.Context, tconn *chrome.TestConn, d *ui.Device, pkgName string) error {
	if err := d.WaitForIdle(ctx, 10*time.Second); err != nil {
		return errors.Wrap(err, "failed to wait for Android to be idle")
	}

	window, err := ash.GetARCAppWindowInfo(ctx, tconn, pkgName)
	if err != nil {
		return errors.Wrap(err, "failed to get window info")
	}

	if err := ash.WaitWindowFinishAnimating(ctx, tconn, window.ID); err != nil {
		return errors.Wrap(err, "failed to wait for the window animation")
	}
	return nil
}

func SplitView(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(*arc.PreData).Chrome
	a := s.FixtValue().(*arc.PreData).ARC

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Creating test API connection failed: ", err)
	}

	d, err := a.NewUIDevice(ctx)
	if err != nil {
		s.Fatal("Failed to initialize UI Automator: ", err)
	}
	defer d.Close(ctx)

	params := s.Param().(splitViewTestParams)
	cleanup, err := ash.EnsureTabletModeEnabled(ctx, tconn, params.tabletMode)
	if err != nil {
		s.Fatalf("Failed to ensure tablet-mode status to %t: %v", params.tabletMode, err)
	}
	defer cleanup(ctx)

	tew, err := input.Touchscreen(ctx)
	if err != nil {
		s.Fatal("Failed to access to the touch screen: ", err)
	}
	defer tew.Close()

	// Ensure landscape orientation so this test can assume that windows snap on
	// the left and right. Windows snap on the top and bottom in portrait-oriented
	// tablet mode. They snap on the left and right in portrait-oriented clamshell
	// mode, but there are active (although contentious) proposals to change that.
	orientation, err := display.GetOrientation(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to obtain the orientation info: ", err)
	}
	rotation := -orientation.Angle
	if orientation.Type == display.OrientationPortraitPrimary {
		info, err := display.GetPrimaryInfo(ctx, tconn)
		if err != nil {
			s.Fatal("Failed to obtain primary display info: ", err)
		}
		if err = display.SetDisplayRotationSync(ctx, tconn, info.ID, display.Rotate90); err != nil {
			s.Fatal("Failed to rotate display: ", err)
		}
		defer display.SetDisplayRotationSync(ctx, tconn, info.ID, display.Rotate0)
		rotation += 90
	}
	tew.SetRotation(rotation)

	// Show two activities. As the content of the activities doesn't matter,
	// use two activities available by default.
	rightAct, err := showActivityForSplitViewTest(
		ctx, tconn, a, "com.android.storagemanager", ".deletionhelper.DeletionHelperActivity")
	if err != nil {
		s.Fatal("Failed to show an activity: ", err)
	}
	defer rightAct.Close()
	if err := d.WaitForIdle(ctx, 10*time.Second); err != nil {
		s.Fatal("Failed to wait for idle: ", err)
	}

	leftAct, err := showActivityForSplitViewTest(ctx, tconn, a, "com.android.settings", ".Settings")
	if err != nil {
		s.Fatal("Failed to show an activity: ", err)
	}
	defer leftAct.Close()
	if err := d.WaitForIdle(ctx, 10*time.Second); err != nil {
		s.Fatal("Failed to wait for idle: ", err)
	}

	stw, err := tew.NewSingleTouchWriter()
	if err != nil {
		s.Fatal("Failed to create a single touch writer: ", err)
	}
	defer stw.Close()

	if params.startFromHomeLauncher {
		if err := ash.DragToShowHomescreen(ctx, tew.Width(), tew.Height(), stw, tconn); err != nil {
			s.Fatal("Failed to drag to show home launcher: ", err)
		}
		if err := waitForWindowStates(ctx, tconn,
			windowStateExpectations{
				{leftAct, ash.WindowStateMinimized, arc.WindowStateMinimized},
				{rightAct, ash.WindowStateMinimized, arc.WindowStateMinimized},
			}); err != nil {
			// If you see this error, check if https://crbug.com/1109250 has been reintroduced.
			s.Fatal("Failed to wait until window state change: ", err)
		}
		if err := waitForArcAppWindowAnimation(ctx, tconn, d, leftAct.PackageName()); err != nil {
			s.Fatal("Failed to wait for the left snapped window animation: ", err)
		}
		if err := waitForArcAppWindowAnimation(ctx, tconn, d, rightAct.PackageName()); err != nil {
			s.Fatal("Failed to wait for the right snapped window animation: ", err)
		}
	}

	if err := ash.SetOverviewModeAndWait(ctx, tconn, true); err != nil {
		s.Fatal("Failed to enter overview: ", err)
	}

	// Snap activity to left.
	if err := dragToSnapFirstOverviewWindow(ctx, tconn, tew, stw, 0); err != nil {
		s.Fatal("Failed to drag window from overview and snap left: ", err)
	}
	if err := waitForWindowStates(ctx, tconn, windowStateExpectations{{leftAct, ash.WindowStateLeftSnapped, arc.WindowStatePrimarySnapped}}); err != nil {
		s.Fatal("Failed to wait until window state change: ", err)
	}
	if err := waitForArcAppWindowAnimation(ctx, tconn, d, leftAct.PackageName()); err != nil {
		s.Fatal("Failed to wait for the left snapped window animation: ", err)
	}

	// Snap activity to right.
	if err := dragToSnapFirstOverviewWindow(ctx, tconn, tew, stw, tew.Width()-1); err != nil {
		s.Fatal("Failed to drag window from overview and snap right: ", err)
	}
	if err := waitForWindowStates(ctx, tconn, windowStateExpectations{{rightAct, ash.WindowStateRightSnapped, arc.WindowStateSecondarySnapped}}); err != nil {
		s.Fatal("Failed to wait until window state change: ", err)
	}
	if err := waitForArcAppWindowAnimation(ctx, tconn, d, rightAct.PackageName()); err != nil {
		s.Fatal("Failed to wait for the right snapped window animation: ", err)
	}

	if params.tabletMode {
		// Swap the left activity and the right activity.
		if err := ash.SwapWindowsInSplitView(ctx, tconn); err != nil {
			s.Fatal("Failed to swap windows in split view: ", err)
		}
		leftAct, rightAct = rightAct, leftAct
		if err := waitForWindowStates(ctx, tconn,
			windowStateExpectations{
				{leftAct, ash.WindowStateLeftSnapped, arc.WindowStatePrimarySnapped},
				{rightAct, ash.WindowStateRightSnapped, arc.WindowStateSecondarySnapped},
			}); err != nil {
			s.Fatal("Failed to wait until window state change: ", err)
		}
	}
}

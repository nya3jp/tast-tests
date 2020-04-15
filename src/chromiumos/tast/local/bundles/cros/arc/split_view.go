// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/display"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/ui"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         SplitView,
		Desc:         "Tests split view works properly with ARC apps",
		Contacts:     []string{"tetsui@chromium.org", "amusbach@chromium.org", "arc-framework+tast@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"android_p", "chrome"},
	})
}

// waitUntilStateChangeInSplitView waits for window state changes on both Ash
// and ARC sides. It assumes Ash is currently in split view mode, and ARC
// activities passed as left and right are both shown side by side.
func waitUntilStateChangeInSplitView(ctx context.Context, tconn *chrome.TestConn, left, right *arc.Activity) error {
	return testing.Poll(ctx, func(ctx context.Context) error {
		for _, test := range []struct {
			act      *arc.Activity
			ashState ash.WindowStateType
			arcState arc.WindowState
		}{
			{left, ash.WindowStateLeftSnapped, arc.WindowStatePrimarySnapped},
			{right, ash.WindowStateRightSnapped, arc.WindowStateSecondarySnapped}} {
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
		return nil, errors.Wrap(err, "failed to start the activity")
	}
	return act, nil
}

func SplitView(ctx context.Context, s *testing.State) {
	cr, err := chrome.New(ctx, chrome.ARCEnabled(), chrome.ExtraArgs("--enable-features=DragToSnapInClamshellMode"))
	if err != nil {
		s.Fatal("Failed to connect to Chrome: ", err)
	}
	defer cr.Close(ctx)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Creating test API connection failed: ", err)
	}

	cleanup, err := ash.EnsureTabletModeEnabled(ctx, tconn, true)
	if err != nil {
		s.Fatal("Failed to ensure in tablet mode: ", err)
	}
	defer cleanup(ctx)

	s.Run(ctx, "WindowStateSync", func(ctx context.Context, s *testing.State) {
		a, err := arc.New(ctx, s.OutDir())
		if err != nil {
			s.Fatal("Failed to start ARC: ", err)
		}
		defer a.Close()

		// Show two activities. As the content of the activities doesn't matter,
		// use two activities available by default.
		rightAct, err := showActivityForSplitViewTest(
			ctx, tconn, a, "com.android.storagemanager", ".deletionhelper.DeletionHelperActivity")
		if err != nil {
			s.Fatal("Failed to show an activity: ", err)
		}
		defer rightAct.Close()
		defer rightAct.Stop(ctx)
		leftAct, err := showActivityForSplitViewTest(ctx, tconn, a, "com.android.settings", ".Settings")
		if err != nil {
			s.Fatal("Failed to show an activity: ", err)
		}
		defer leftAct.Close()
		defer leftAct.Stop(ctx)

		// Snap activities to left and right.
		if _, err := ash.SetARCAppWindowState(ctx, tconn, leftAct.PackageName(), ash.WMEventSnapLeft); err != nil {
			s.Fatal("Failed to snap app in split view: ", err)
		}
		if _, err := ash.SetARCAppWindowState(ctx, tconn, rightAct.PackageName(), ash.WMEventSnapRight); err != nil {
			s.Fatal("Failed to snap app in split view: ", err)
		}

		if err := waitUntilStateChangeInSplitView(ctx, tconn, leftAct, rightAct); err != nil {
			s.Fatal("Failed to wait until window state change: ", err)
		}

		// Swap the left activity and the right activity.
		if err := ash.SwapWindowsInSplitView(ctx, tconn); err != nil {
			s.Fatal("Failed to swap windows in split view: ", err)
		}
		leftAct, rightAct = rightAct, leftAct

		if err := waitUntilStateChangeInSplitView(ctx, tconn, leftAct, rightAct); err != nil {
			s.Fatal("Failed to wait until window state change: ", err)
		}
	})

	tew, err := input.Touchscreen(ctx)
	if err != nil {
		s.Fatal("Failed to access to the touch screen: ", err)
	}
	defer tew.Close()

	// Ensures landscape orientation so this test can assume that windows snap on
	// the left and right. Windows snap on the top and bottom in portrait-oriented
	// tablet mode. They snap on the left and right in portrait-oriented clamshell
	// mode, but there are active (although contentious) proposals to change that.
	orientation, err := display.GetOrientation(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to obtain the orientation info: ", err)
	}
	rotation := -orientation.Angle
	if orientation.Type == display.OrientationPortraitPrimary {
		info, err := display.GetInternalInfo(ctx, tconn)
		if err != nil {
			s.Fatal("Failed to obtain internal display info: ", err)
		}
		if err = display.SetDisplayRotationSync(ctx, tconn, info.ID, display.Rotate90); err != nil {
			s.Fatal("Failed to rotate display: ", err)
		}
		defer display.SetDisplayRotationSync(ctx, tconn, info.ID, display.Rotate0)
		rotation += 90
	}
	tew.SetRotation(rotation)

	info, err := display.GetInternalInfo(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to get the internal display info: ", err)
	}
	tcc := tew.NewTouchCoordConverter(info.Bounds.Size())

	stw, err := tew.NewSingleTouchWriter()
	if err != nil {
		s.Fatal("Failed to create a single touch writer: ", err)
	}

	conns, err := ash.CreateWindows(ctx, cr, ui.PerftestURL, 1)
	if err != nil {
		s.Fatal("Failed to open a non-ARC window: ", err)
	}
	conns.Close()

	subtestNotEndOverview := func(ctx context.Context, s *testing.State) {
		a, err := arc.New(ctx, s.OutDir())
		if err != nil {
			s.Fatal("Failed to start ARC: ", err)
		}
		defer a.Close()

		act, err := showActivityForSplitViewTest(ctx, tconn, a, "com.android.settings", ".Settings")
		if err != nil {
			s.Fatal("Failed to show an activity: ", err)
		}
		defer act.Close()
		defer act.Stop(ctx)

		if err := ash.SetOverviewModeAndWait(ctx, tconn, true); err != nil {
			s.Fatal("Failed to enter overview: ", err)
		}
		arcWin, err := ash.FindFirstWindowInOverview(ctx, tconn)
		if err != nil {
			s.Fatal("Failed to find the ARC window in the overview grid: ", err)
		}
		centerX, centerY := tcc.ConvertLocation(arcWin.OverviewInfo.Bounds.CenterPoint())
		if err := stw.LongPressAt(ctx, centerX, centerY); err != nil {
			s.Fatal("Failed to long-press to start dragging the ARC window: ", err)
		}
		if err := stw.Swipe(ctx, centerX, centerY, 0, tew.Height()/2, time.Second); err != nil {
			s.Fatal("Failed to swipe for snapping the ARC window: ", err)
		}
		if err := stw.End(); err != nil {
			s.Fatal("Failed to end the swipe: ", err)
		}
		if err := ash.WaitForCondition(ctx, tconn, func(w *ash.Window) bool {
			return w.ID == arcWin.ID && !w.IsAnimating && w.State == ash.WindowStateLeftSnapped
		}, &testing.PollOptions{Timeout: 15 * time.Second}); err != nil {
			s.Fatal("Failed to wait for the ARC window to be left-snapped: ", err)
		}

		// Check for https://b.corp.google.com/issues/143499564.
		if _, err := ash.FindFirstWindowInOverview(ctx, tconn); err != nil {
			s.Fatal("Failed to find the non-ARC window in the overview grid: ", err)
		}
	}

	s.Run(ctx, "TabletSplitViewNotEndOverview", subtestNotEndOverview)

	cleanup2, err := ash.EnsureTabletModeEnabled(ctx, tconn, false)
	if err != nil {
		s.Fatal("Failed to ensure in clamshell mode: ", err)
	}
	defer cleanup2(ctx)

	s.Run(ctx, "ClamshellSplitViewNotEndOverview", subtestNotEndOverview)
}

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
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         SplitView,
		Desc:         "Tests split view works properly with ARC apps",
		Contacts:     []string{"tetsui@chromium.org", "amusbach@chromium.org", "arc-framework+tast@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"android_p", "chrome"},
		Timeout:      4 * time.Minute,
		Pre:          arc.Booted(),
		Params: []testing.Param{
			{
				Name: "clamshell_mode",
				Val:  false,
			},
			{
				Name:              "tablet_mode",
				ExtraSoftwareDeps: []string{"tablet_mode"},
				Val:               true,
			},
		},
	})
}

// dragToSnapFirstOverviewWindow finds the first window in overview, and drags
// to snap it. This function assumes that overview is already active.
func dragToSnapFirstOverviewWindow(ctx context.Context, tconn *chrome.TestConn, tew *input.TouchscreenEventWriter, targetX input.TouchCoord) error {
	info, err := display.GetInternalInfo(ctx, tconn)
	if err != nil {
		return errors.Wrap(err, "failed to get the internal display info")
	}
	tcc := tew.NewTouchCoordConverter(info.Bounds.Size())

	stw, err := tew.NewSingleTouchWriter()
	if err != nil {
		return errors.Wrap(err, "failed to create a single touch writer")
	}
	defer stw.Close()

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

// waitUntilStateChange waits for window state changes on both Ash and ARC
// sides. If left is not nil, it is expected to become left snapped.
// Likewise, if right is not nil, it is expected to become right snapped.
func waitUntilStateChange(ctx context.Context, tconn *chrome.TestConn, left, right *arc.Activity) error {
	return testing.Poll(ctx, func(ctx context.Context) error {
		for _, test := range []struct {
			act      *arc.Activity
			ashState ash.WindowStateType
			arcState arc.WindowState
		}{
			{left, ash.WindowStateLeftSnapped, arc.WindowStatePrimarySnapped},
			{right, ash.WindowStateRightSnapped, arc.WindowStateSecondarySnapped}} {
			if test.act == nil {
				continue
			}

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

func SplitView(ctx context.Context, s *testing.State) {
	cr := s.PreValue().(arc.PreData).Chrome
	a := s.PreValue().(arc.PreData).ARC

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Creating test API connection failed: ", err)
	}

	tabletMode := s.Param().(bool)
	cleanup, err := ash.EnsureTabletModeEnabled(ctx, tconn, tabletMode)
	if err != nil {
		s.Fatalf("Failed to ensure tablet-mode status to %t: %v", tabletMode, err)
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

	// Show two activities. As the content of the activities doesn't matter,
	// use two activities available by default.
	rightAct, err := showActivityForSplitViewTest(
		ctx, tconn, a, "com.android.storagemanager", ".deletionhelper.DeletionHelperActivity")
	if err != nil {
		s.Fatal("Failed to show an activity: ", err)
	}
	defer rightAct.Close()
	leftAct, err := showActivityForSplitViewTest(ctx, tconn, a, "com.android.settings", ".Settings")
	if err != nil {
		s.Fatal("Failed to show an activity: ", err)
	}
	defer leftAct.Close()

	if err := ash.SetOverviewModeAndWait(ctx, tconn, true); err != nil {
		s.Fatal("Failed to enter overview: ", err)
	}

	// Snap activities to left and right.
	if err := dragToSnapFirstOverviewWindow(ctx, tconn, tew, 0); err != nil {
		s.Fatal("Failed to drag window from overview and snap left: ", err)
	}
	if err := waitUntilStateChange(ctx, tconn, leftAct, nil); err != nil {
		s.Fatal("Failed to wait until window state change: ", err)
	}
	if err := dragToSnapFirstOverviewWindow(ctx, tconn, tew, tew.Width()-1); err != nil {
		s.Fatal("Failed to drag window from overview and snap right: ", err)
	}
	if err := waitUntilStateChange(ctx, tconn, nil, rightAct); err != nil {
		s.Fatal("Failed to wait until window state change: ", err)
	}

	if tabletMode {
		// Swap the left activity and the right activity.
		if err := ash.SwapWindowsInSplitView(ctx, tconn); err != nil {
			s.Fatal("Failed to swap windows in split view: ", err)
		}
		leftAct, rightAct = rightAct, leftAct
		if err := waitUntilStateChange(ctx, tconn, leftAct, rightAct); err != nil {
			s.Fatal("Failed to wait until window state change: ", err)
		}
	}
}

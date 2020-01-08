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
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         SplitView,
		Desc:         "Tests split view works properly with ARC apps",
		Contacts:     []string{"tetsui@chromium.org", "arc-framework+tast@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"android_p", "chrome"},
		Pre:          arc.BootedInTabletMode(),
	})
}

// waitUntilStateChangeInSplitView waits for window state changes on both Ash
// and ARC sides. It assumes Ash is currently in split view mode, and ARC
// activities passed as left and right are both shown side by side.
func waitUntilStateChangeInSplitView(ctx context.Context, c *chrome.Conn, left *arc.Activity, right *arc.Activity) error {
	return testing.Poll(ctx, func(ctx context.Context) error {
		for _, test := range []struct {
			act      *arc.Activity
			ashState ash.WindowStateType
			arcState arc.WindowState
		}{
			{left, ash.WindowStateLeftSnapped, arc.WindowStatePrimarySnapped},
			{right, ash.WindowStateRightSnapped, arc.WindowStateSecondarySnapped}} {
			if actual, err := ash.GetARCAppWindowState(ctx, c, test.act.PackageName()); err != nil {
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
func showActivityForSplitViewTest(ctx context.Context, a *arc.ARC, pkgName, activityName string) (*arc.Activity, error) {
	act, err := arc.NewActivity(a, pkgName, activityName)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create a new activity")
	}
	if err := act.Start(ctx); err != nil {
		return nil, errors.Wrap(err, "failed to start the activity")
	}
	if err := act.WaitForResumed(ctx, 30*time.Second); err != nil {
		return nil, errors.Wrap(err, "failed to wait for the activity to resume")
	}
	return act, nil
}

func SplitView(ctx context.Context, s *testing.State) {
	p := s.PreValue().(arc.PreData)
	cr := p.Chrome
	a := p.ARC

	tsw, err := input.Touchscreen(ctx)
	if err != nil {
		s.Fatal("Failed to open the touchscreen device: ", err)
	}
	defer tsw.Close()

	stw, err := tsw.NewSingleTouchWriter()
	if err != nil {
		s.Fatal("Failed to create TouchEventWriter: ", err)
	}
	defer stw.Close()

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Creating test API connection failed: ", err)
	}

	// Show two activities. As the content of the activities doesn't matter,
	// use two activities available by default.
	rightAct, err := showActivityForSplitViewTest(
		ctx, a, "com.android.storagemanager", ".deletionhelper.DeletionHelperActivity")
	if err != nil {
		s.Fatal("Failed to show an activity: ", err)
	}
	defer rightAct.Close()
	leftAct, err := showActivityForSplitViewTest(ctx, a, "com.android.settings", ".Settings")
	if err != nil {
		s.Fatal("Failed to show an activity: ", err)
	}
	defer leftAct.Close()

	// Snap activities to left and right.
	if err := stw.Swipe(ctx, tsw.Width()/2, 0, tsw.Width()/2, tsw.Height()/2, time.Second); err != nil {
		s.Fatal("Failed to swipe")
	}

	if err := stw.Swipe(ctx, tsw.Width()/2, tsw.Height()/2, 0, tsw.Height()/2, time.Second); err != nil {
		s.Fatal("Failed to swipe")
	}
	if err := stw.End(); err != nil {
		s.Fatal("Failed to end touch")
	}

	testing.Sleep(ctx, time.Second)

	if err := stw.Move(tsw.Width()*3/4, tsw.Height()/2); err != nil {
		s.Fatal("a")
	}
	testing.Sleep(ctx, 200*time.Millisecond)
	stw.End()

	if err := waitUntilStateChangeInSplitView(ctx, tconn, leftAct, rightAct); err != nil {
		s.Fatal("Failed to wait until window state change: ", err)
	}

	// Swap the left activity and the right activity.
	testing.Sleep(ctx, 5*time.Second)
	if err := stw.DoubleTap(ctx, tsw.Width()/2, tsw.Height()/2); err != nil {
		s.Fatal("Failed to double tap: ", err)
	}
	testing.Sleep(ctx, 5*time.Second)

	leftAct, rightAct = rightAct, leftAct

	if err := waitUntilStateChangeInSplitView(ctx, tconn, leftAct, rightAct); err != nil {
		s.Fatal("Failed to wait until window state change: ", err)
	}
}

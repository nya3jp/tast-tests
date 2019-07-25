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
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         SplitView,
		Desc:         "Tests split view works properly with ARC apps",
		Contacts:     []string{"tetsui@chromium.org", "arc-framework+tast@google.com"},
		Attr:         []string{"informational"},
		SoftwareDeps: []string{"android_p", "chrome"},
		Timeout:      4 * time.Minute,
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
	if err := act.WaitForIdle(ctx, 30*time.Second); err != nil {
		return nil, errors.Wrap(err, "failed to wait for the activity")
	}
	return act, nil
}

func SplitView(ctx context.Context, s *testing.State) {
	cr, err := chrome.New(ctx, chrome.ARCEnabled(), chrome.ExtraArgs("--force-tablet-mode=touch_view"))
	if err != nil {
		s.Fatal("Failed to connect to Chrome: ", err)
	}
	defer cr.Close(ctx)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Creating test API connection failed: ", err)
	}

	a, err := arc.New(ctx, s.OutDir())
	if err != nil {
		s.Fatal("Failed to start ARC: ", err)
	}
	defer a.Close()

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
}

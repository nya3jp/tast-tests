// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/arc/ui"
	"chromiumos/tast/local/bundles/cros/arc/wm"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/display"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         WMNonresizableClamshell,
		Desc:         "Verifies that Window Manager non-resizable clamshell use-cases behave as described in go/arc-wm-r",
		Contacts:     []string{"armenk@google.com", "arc-framework+tast@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"android_vm", "chrome"},
		Pre:          arc.Booted(),
		Timeout:      8 * time.Minute,
	})
}

func WMNonresizableClamshell(ctx context.Context, s *testing.State) {
	wm.SetupAndRunTestCases(ctx, s, false, []wm.TestCase{
		wm.TestCase{
			// non-resizable/clamshell: default launch behavior
			Name: "NC_default_launch_behavior",
			Func: wmNC01,
		},
		wm.TestCase{
			// non-resizable/clamshell: user immerse portrait app (pillarbox)
			Name: "NC_user_immerse_portrait",
			Func: wmNC04,
		},
		wm.TestCase{
			// non-resizable/clamshell: user immerse non-portrait app
			Name: "NC_user_immerse_non_portrait",
			Func: wmNC05,
		},
		wm.TestCase{
			// non-resizable/clamshell: hide shelf when app maximized
			Name: "NC_hide_shelf_app_max",
			Func: wmNC12,
		},
	})
}

// wmNC01 covers non-resizable/clamshell default launch behavior.
// Expected behavior is defined in: go/arc-wm-r NC01: non-resizable/clamshell: default launch behavior.
func wmNC01(ctx context.Context, tconn *chrome.TestConn, a *arc.ARC, d *ui.Device) error {
	for _, activityName := range []string{
		wm.NonResizablePortraitActivity,
		wm.NonResizableLandscapeActivity,
	} {
		if err := func() error {
			act, err := arc.NewActivity(a, wm.Pkg24, activityName)
			if err != nil {
				return err
			}
			defer act.Close()

			if err := act.Start(ctx, tconn); err != nil {
				return err
			}
			defer act.Stop(ctx, tconn)

			if err := wm.WaitUntilActivityIsReady(ctx, tconn, act, d); err != nil {
				return err
			}

			return wm.CheckMaximizeNonResizable(ctx, tconn, act, d)
		}(); err != nil {
			return errors.Wrapf(err, "%q test failed", activityName)
		}
	}
	return nil
}

// wmNC04 covers non-resizable/clamshell: user immerse portrait app (pillarbox) behavior.
// Expected behavior is defined in: go/arc-wm-r NC04: non-resizable/clamshell: user immerse portrait app (pillarbox).
func wmNC04(ctx context.Context, tconn *chrome.TestConn, a *arc.ARC, d *ui.Device) error {
	return checkMaxActivityToFullscreen(ctx, tconn, a, d, wm.NonResizablePortraitActivity)
}

// wmNC05 covers non-resizable/clamshell: user immerse non-portrait app behavior.
// Expected behavior is defined in: go/arc-wm-r NC05: non-resizable/clamshell: user immerse non-portrait app.
func wmNC05(ctx context.Context, tconn *chrome.TestConn, a *arc.ARC, d *ui.Device) error {
	for _, activityName := range []string{
		wm.NonResizableLandscapeActivity,
		wm.NonResizableUnspecifiedActivity,
	} {
		if err := checkMaxActivityToFullscreen(ctx, tconn, a, d, activityName); err != nil {
			return err
		}
	}
	return nil
}

// wmNC12 covers non-resizable/clamshell: hide shelf when app maximized.
// Expected behavior is defined in: go/arc-wm-r NC12: non-resizable/clamshell: hide shelf when app maximized.
func wmNC12(ctx context.Context, tconn *chrome.TestConn, a *arc.ARC, d *ui.Device) error {
	act, err := arc.NewActivity(a, wm.Pkg24, wm.NonResizableUnspecifiedActivity)
	if err != nil {
		return err
	}
	defer act.Close()

	// Get primary display info to set shelf behavior.
	primaryDisplayInfo, err := display.GetPrimaryInfo(ctx, tconn)
	if err != nil {
		return err
	}
	if primaryDisplayInfo == nil {
		return errors.New("failed to find primary display info")
	}

	// Get initial shelf behavior.
	initSB, err := ash.GetShelfBehavior(ctx, tconn, primaryDisplayInfo.ID)
	if err != nil {
		return err
	}
	if initSB != ash.ShelfBehaviorNeverAutoHide {
		// Set shelf behavior to never auto hide for test's initial state.
		if err := ash.SetShelfBehavior(ctx, tconn, primaryDisplayInfo.ID, ash.ShelfBehaviorNeverAutoHide); err != nil {
			return err
		}
	}

	// Start the activity.
	if err := act.Start(ctx, tconn); err != nil {
		return err
	}
	defer act.Stop(ctx, tconn)

	if err := wm.WaitUntilActivityIsReady(ctx, tconn, act, d); err != nil {
		return err
	}
	if err := wm.CheckMaximizeNonResizable(ctx, tconn, act, d); err != nil {
		return err
	}

	// Store initial window info to compare with after hiding and showing the shelf.
	winInfoInitialState, err := ash.GetARCAppWindowInfo(ctx, tconn, wm.Pkg24)
	if err != nil {
		return err
	}

	winID := winInfoInitialState.ID

	// Set shelf behavior to auto hide.
	if err := ash.SetShelfBehavior(ctx, tconn, primaryDisplayInfo.ID, ash.ShelfBehaviorAlwaysAutoHide); err != nil {
		return err
	}

	// Wait for shelf animation to complete.
	if err := ash.WaitForCondition(ctx, tconn, func(w *ash.Window) bool {
		shelfInfo, err := ash.FetchScrollableShelfInfoForState(ctx, tconn, &ash.ShelfState{})
		if err != nil {
			return false
		}
		return w.ID == winID && shelfInfo.IsShelfWidgetAnimating == false
	}, &testing.PollOptions{Timeout: 5 * time.Second}); err != nil {
		return errors.Wrap(err, "failed to wait for shelf animation to complete")
	}

	winInfoShelfHidden, err := ash.GetARCAppWindowInfo(ctx, tconn, wm.Pkg24)
	if err != nil {
		return err
	}

	// Compare window bounds before and after hiding the shelf. It should be larger when shelf is hidden.
	if winInfoShelfHidden.BoundsInRoot.Height <= winInfoInitialState.BoundsInRoot.Height {
		return errors.Errorf("invalid window bounds when shelf is shown, got: %s, want smaller than: %s", winInfoInitialState.BoundsInRoot, winInfoShelfHidden.BoundsInRoot)
	}

	// Show the shelf.
	if err := ash.SetShelfBehavior(ctx, tconn, primaryDisplayInfo.ID, ash.ShelfBehaviorNeverAutoHide); err != nil {
		return err
	}

	// Wait for shelf animation to complete.
	if err := ash.WaitForCondition(ctx, tconn, func(w *ash.Window) bool {
		shelfInfo, err := ash.FetchScrollableShelfInfoForState(ctx, tconn, &ash.ShelfState{})
		if err != nil {
			return false
		}
		return w.ID == winID && shelfInfo.IsShelfWidgetAnimating == false
	}, &testing.PollOptions{Timeout: 5 * time.Second}); err != nil {
		return errors.Wrap(err, "failed to wait for shelf animation to complete")
	}

	if err := wm.CheckMaximizeNonResizable(ctx, tconn, act, d); err != nil {
		return err
	}

	winInfoShelfReShown, err := ash.GetARCAppWindowInfo(ctx, tconn, wm.Pkg24)
	if err != nil {
		return err
	}

	// Compare window bounds after showing the shelf with initial bounds. They should be equal.
	if winInfoInitialState.BoundsInRoot != winInfoShelfReShown.BoundsInRoot {
		return errors.Errorf("invalid window bounds after hiding and showing the shelf, got: %s, want: %s", winInfoShelfReShown.BoundsInRoot, winInfoInitialState.BoundsInRoot)
	}

	return nil
}

// checkMaxActivityToFullscreen creates a new activity, lunches it and toggles to fullscreen and checks for validity of window info.
func checkMaxActivityToFullscreen(ctx context.Context, tconn *chrome.TestConn, a *arc.ARC, d *ui.Device, activityName string) error {
	act, err := arc.NewActivity(a, wm.Pkg24, activityName)
	if err != nil {
		return err
	}
	defer act.Close()

	if err := act.Start(ctx, tconn); err != nil {
		return err
	}
	defer act.Stop(ctx, tconn)

	if err := wm.WaitUntilActivityIsReady(ctx, tconn, act, d); err != nil {
		return err
	}

	if err := wm.CheckMaximizeNonResizable(ctx, tconn, act, d); err != nil {
		return err
	}

	windowInfoMaximized, err := ash.GetARCAppWindowInfo(ctx, tconn, wm.Pkg24)
	if err != nil {
		return err
	}
	if err := wm.ToggleFullscreen(ctx, tconn); err != nil {
		return err
	}

	if err := ash.WaitForARCAppWindowState(ctx, tconn, wm.Pkg24, ash.WindowStateFullscreen); err != nil {
		return err
	}

	if err := ash.WaitWindowFinishAnimating(ctx, tconn, windowInfoMaximized.ID); err != nil {
		return err
	}

	windowInfoFullscreen, err := ash.GetARCAppWindowInfo(ctx, tconn, wm.Pkg24)
	if err != nil {
		return err
	}
	return wm.CheckMaximizeToFullscreenToggle(ctx, tconn, windowInfoMaximized.TargetBounds, *windowInfoFullscreen)
}

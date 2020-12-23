// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"math"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/android/ui"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/bundles/cros/arc/wm"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/display"
	crui "chromiumos/tast/local/chrome/ui"
	"chromiumos/tast/local/chrome/ui/mouse"
	"chromiumos/tast/local/coords"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         WMResizableClamshell,
		Desc:         "Verifies that Window Manager resizable clamshell use-cases behave as described in go/arc-wm-r",
		Contacts:     []string{"xutan@google.com", "arc-framework+tast@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"android_vm", "chrome"},
		Fixture:      "arcBooted",
		Timeout:      8 * time.Minute,
	})
}

func WMResizableClamshell(ctx context.Context, s *testing.State) {
	wm.SetupAndRunTestCases(ctx, s, false, []wm.TestCase{
		wm.TestCase{
			// resizable/clamshell: default launch behavior
			Name: "RC01_launch",
			Func: wmRC01,
		},
		wm.TestCase{
			// resizable/clamshell: maximize portrait app (pillarbox)
			Name: "RC02_maximize_portrait",
			Func: wmRC02,
		},
		wm.TestCase{
			// resizable/clamshell: maximize non-portrait app
			Name: "RC03_maximize_non_portrait",
			Func: wmRC03,
		},
		wm.TestCase{
			// resizable/clamshell: user immerse portrait app (pillarbox)
			Name: "RC04_user_immerse_portrait",
			Func: wmRC04,
		},
		wm.TestCase{
			// resizable/clamshell: user immerse non-portrait app
			Name: "RC05_user_immerse_non_portrait",
			Func: wmRC05,
		},
		wm.TestCase{
			// resizable/clamshell: immerse via API ignored if windowed
			Name: "RC06_immerse_via_API_ignored_if_windowed",
			Func: wmRC06,
		},
		wm.TestCase{
			// resizable/clamshell: immerse via API from maximized portrait (pillarbox)
			Name: "RC07_immerse_via_API_from_maximized_portrait",
			Func: wmRC07,
		},
		wm.TestCase{
			// resizable/clamshell: immerse via API from maximized non-portrait
			Name: "RC08_immerse_via_API_from_maximized_non_portrait",
			Func: wmRC08,
		},
		wm.TestCase{
			// resizable/clamshell: new activity follows root activity
			Name: "RC09_new_activity_follows_root_activity",
			Func: wmRC09,
		},
		wm.TestCase{
			// resizable/clamshell: font size change
			Name: "RC10_font_size_change",
			Func: wmRC10,
		},
		wm.TestCase{
			// resizable/clamshell: hide Shelf when app maximized
			Name: "RC12_hide_Shelf_when_app_maximized",
			Func: wmRC12,
		},
		wm.TestCase{
			// resizable/clamshell: freeform resize
			Name: "RC13_freeform_resize",
			Func: wmRC13,
		},
		wm.TestCase{
			// resizable/clamshell: snap to half screen
			Name: "RC14_snap_to_half_screen",
			Func: wmRC14,
		},
		wm.TestCase{
			// resizable/clamshell: display size change
			Name: "RC15_display_size_change",
			Func: wmRC15,
		},
		wm.TestCase{
			// resizable/clamshell: font size change
			Name: "RC17_font_size_change",
			Func: wmRC17,
		},
		wm.TestCase{
			// resizable/clamshell: snap to half screen
			Name: "RC22_split_screen",
			Func: wmRC22,
		},
	})
}

// wmRC01 covers resizable/clamshell default launch behavior.
// Expected behavior is defined in: go/arc-wm-r RC01: resizable/clamshell: default launch behavior.
func wmRC01(ctx context.Context, tconn *chrome.TestConn, a *arc.ARC, d *ui.Device) error {
	for activityName, desiredOrientation := range map[string]string{
		wm.ResizablePortraitActivity:  wm.Portrait,
		wm.ResizableLandscapeActivity: wm.Landscape,
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

			if err := wm.CheckRestoreResizable(ctx, tconn, act, d); err != nil {
				return err
			}

			orientation, err := wm.UIOrientation(ctx, act, d)
			if err != nil {
				return err
			}
			if orientation != desiredOrientation {
				return errors.Errorf("invalid orientation: got %v; want %v", orientation, desiredOrientation)
			}

			window, err := ash.GetARCAppWindowInfo(ctx, tconn, wm.Pkg24)
			if err != nil {
				return err
			}
			orientationFromBounds := wm.OrientationFromBounds(window.BoundsInRoot)
			if orientationFromBounds != desiredOrientation {
				return errors.Errorf("invalid bounds orientation: got %v; want %v",
					orientationFromBounds, desiredOrientation)
			}
			return nil
		}(); err != nil {
			return errors.Wrapf(err, "%q test failed", activityName)
		}
	}
	return nil
}

// wmRC02 covers resizable/clamshell: maximize portrait app (pillarbox).
// Expected behavior is defined in: go/arc-wm-r RC02: resizable/clamshell: maximize portrait app (pillarbox).
func wmRC02(ctx context.Context, tconn *chrome.TestConn, a *arc.ARC, d *ui.Device) error {
	for _, eTC := range []struct {
		Name string
		Func func(context.Context, *chrome.TestConn, string) error
	}{
		{"touchCaptionButton", touchCaptionButton},
		{"leftClickCaptionButton", leftClickCaptionButton},
	} {
		if err := rcMaxRestoreTestHelper(ctx, tconn, a, d, wm.ResizablePortraitActivity, eTC.Func); err != nil {
			return errors.Wrapf(err, "%q event type test case for wm.ResizablePortraitActivity failed", eTC.Name)
		}
	}
	return nil
}

// wmRC03 covers resizable/clamshell: maximize non-portrait app.
// Expected behavior is defined in: go/arc-wm-r RC02: resizable/clamshell: maximize non-portrait app.
func wmRC03(ctx context.Context, tconn *chrome.TestConn, a *arc.ARC, d *ui.Device) error {
	for _, eTC := range []struct {
		Name string
		Func func(context.Context, *chrome.TestConn, string) error
	}{
		{"touchCaptionButton", touchCaptionButton},
		{"leftClickCaptionButton", leftClickCaptionButton},
	} {
		for _, actName := range []string{
			wm.ResizableLandscapeActivity,
			wm.ResizableUnspecifiedActivity,
		} {
			if err := rcMaxRestoreTestHelper(ctx, tconn, a, d, actName, eTC.Func); err != nil {
				return errors.Wrapf(err, "%q event type test case for %q failed", eTC.Name, actName)
			}
		}
	}
	return nil
}

// wmRC04 covers resizable/clamshell: user immerse portrait app (pillarbox).
// Expected behavior is defined in: go/arc-wm-r RC04: resizable/clamshell: user immerse portrait app (pillarbox).
func wmRC04(ctx context.Context, tconn *chrome.TestConn, a *arc.ARC, d *ui.Device) error {
	return checkRestoreActivityToFullscreen(ctx, tconn, a, d, wm.ResizablePortraitActivity)
}

// wmRC05 covers resizable/clamshell: user immerse non-portrait app.
// Expected behavior is defined in: go/arc-wm-r RC05: resizable/clamshell: user immerse non-portrait app.
func wmRC05(ctx context.Context, tconn *chrome.TestConn, a *arc.ARC, d *ui.Device) error {
	for _, actName := range []string{
		wm.ResizableLandscapeActivity,
		wm.ResizableUnspecifiedActivity,
	} {
		if err := checkRestoreActivityToFullscreen(ctx, tconn, a, d, actName); err != nil {
			return errors.Wrapf(err, "%q test failed", actName)
		}
	}
	return nil
}

// wmRC06 covers resizable/clamshell: immerse via API ignored if windowed.
// Expected behavior is defined in: go/arc-wm-r RC06: resizable/clamshell: immerse via API ignored if windowed.
func wmRC06(ctx context.Context, tconn *chrome.TestConn, a *arc.ARC, d *ui.Device) error {
	// Start a new activity.
	act, err := arc.NewActivity(a, wm.Pkg24, wm.ResizablePortraitActivity)
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

	// Get window info before clicking on the immersive button.
	originalWindowInfo, err := ash.GetARCAppWindowInfo(ctx, tconn, wm.Pkg24)
	if err != nil {
		return err
	}

	// Click on the immersive button.
	if err := wm.UIClickImmersive(ctx, act, d); err != nil {
		return err
	}
	if err := wm.WaitUntilActivityIsReady(ctx, tconn, act, d); err != nil {
		return err
	}

	// Get window info after the immersive button is clicked.
	windowInfoUIImmersive, err := ash.GetARCAppWindowInfo(ctx, tconn, wm.Pkg24)
	if err != nil {
		return err
	}

	// There should be no changes on window bounds in root after clicking on the immersive button.
	if originalWindowInfo.BoundsInRoot != windowInfoUIImmersive.BoundsInRoot {
		return errors.Errorf("invalid window bounds after UI immersive clicked. got: %q, want: %q", windowInfoUIImmersive.BoundsInRoot, originalWindowInfo.BoundsInRoot)
	}
	return nil
}

// wmRC07 covers resizable/clamshell: immerse via API from maximized portrait.
// Expected behavior is defined in: go/arc-wm-r RC07: resizable/clamshell: immerse via API from maximized portrait.
func wmRC07(ctx context.Context, tconn *chrome.TestConn, a *arc.ARC, d *ui.Device) error {
	return immerseViaAPIHelper(ctx, tconn, a, d, wm.ResizablePortraitActivity)
}

// wmRC08 covers resizable/clamshell: immerse via API from maximized non-portrait.
// Expected behavior is defined in: go/arc-wm-r RC08: resizable/clamshell: immerse via API from maximized non-portrait.
func wmRC08(ctx context.Context, tconn *chrome.TestConn, a *arc.ARC, d *ui.Device) error {
	for _, actName := range []string{
		wm.ResizableLandscapeActivity,
		wm.ResizableUnspecifiedActivity,
	} {
		if err := immerseViaAPIHelper(ctx, tconn, a, d, actName); err != nil {
			return errors.Wrapf(err, "%q test failed", actName)
		}
	}
	return nil
}

// wmRC09 covers resizable/clamshell: new activity follows root activity.
// Expected behavior is defined in: go/arc-wm-r RC09: resizable/clamshell: new activity follows root activity.
func wmRC09(ctx context.Context, tconn *chrome.TestConn, a *arc.ARC, d *ui.Device) error {
	// Start the activity.
	act, err := arc.NewActivity(a, wm.Pkg24, wm.ResizablePortraitActivity)
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

	// Store root window info.
	rootWindowInfo, err := ash.GetARCAppWindowInfo(ctx, tconn, wm.Pkg24)
	if err != nil {
		return err
	}

	// Launch a new activity.
	if err := wm.UIClickLaunchActivity(ctx, act, d); err != nil {
		return err
	}
	if err := wm.WaitUntilActivityIsReady(ctx, tconn, act, d); err != nil {
		return err
	}

	// Get number of activities, it must be 2.
	if nrActivities, err := wm.UINumberActivities(ctx, act, d); err != nil {
		return err
	} else if nrActivities != 2 {
		return errors.Errorf("invalid number of activities: got %d; want 2", nrActivities)
	}

	// Get new activity window info.
	childWindowInfo, err := ash.GetARCAppWindowInfo(ctx, tconn, wm.Pkg24)
	if err != nil {
		return err
	}

	// New activitiy's orientation and size muse be the same as root.
	if rootWindowInfo.BoundsInRoot != childWindowInfo.BoundsInRoot {
		return errors.Errorf("invalid child activity window bounds, got: %q, want: %q", childWindowInfo.BoundsInRoot, rootWindowInfo.BoundsInRoot)
	}
	return nil
}

// wmRC10 covers resizable/clamshell: new activity replaces root activity (springboard).
// Expected behavior is defined in: go/arc-wm-r NC10: resizable/clamshell: new activity replaces root activity (springboard).
func wmRC10(ctx context.Context, tconn *chrome.TestConn, a *arc.ARC, d *ui.Device) error {
	// Start the activity.
	act, err := arc.NewActivity(a, wm.Pkg24, wm.ResizableUnspecifiedActivity)
	if err != nil {
		return errors.Wrap(err, "unable to create new activity")
	}
	defer act.Close()

	if err := act.Start(ctx, tconn); err != nil {
		return errors.Wrap(err, "unable to start new activity")
	}
	defer func(ctx context.Context) {
		act.Stop(ctx, tconn)
	}(ctx)
	ctx, cancel := ctxutil.Shorten(ctx, wm.TimeReservedForStop)
	defer cancel()

	if err := wm.WaitUntilActivityIsReady(ctx, tconn, act, d); err != nil {
		return errors.Wrap(err, "unable to wait until activity is ready")
	}

	// Get the root window info so it could be compared with child window.
	rootWindowInfo, err := ash.GetARCAppWindowInfo(ctx, tconn, wm.Pkg24)
	if err != nil {
		return errors.Wrap(err, "unable to get arc app window info")
	}

	if err := wm.UIClickRootActivity(ctx, act, d); err != nil {
		return errors.Wrap(err, "unable to click on root activity button")
	}
	if err := wm.UIClickLaunchActivity(ctx, act, d); err != nil {
		return errors.Wrap(err, "unable to click on launch activity button")
	}
	if err := wm.WaitUntilActivityIsReady(ctx, tconn, act, d); err != nil {
		return errors.Wrap(err, "unable to wait until activity is ready")
	}

	if nrActivities, err := wm.UINumberActivities(ctx, act, d); err != nil {
		return err
	} else if nrActivities != 1 {
		return errors.Errorf("invalid number of activities: got %d; want 1", nrActivities)
	}

	childWindowInfo, err := ash.GetARCAppWindowInfo(ctx, tconn, wm.Pkg24)
	if err != nil {
		return errors.Wrap(err, "unable to get arc app window info")
	}

	if childWindowInfo.BoundsInRoot != rootWindowInfo.BoundsInRoot {
		return errors.Errorf("invalid child activity window bounds: got %q; want %q", childWindowInfo.BoundsInRoot, rootWindowInfo.BoundsInRoot)
	}

	return nil
}

// wmRC12 covers resizable/clamshell: hide shelf when app maximized.
// Expected behavior is defined in: go/arc-wm-r RC12: resizable/clamshell: hide shelf when app maximized.
func wmRC12(ctx context.Context, tconn *chrome.TestConn, a *arc.ARC, d *ui.Device) error {
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
	act, err := arc.NewActivity(a, wm.Pkg24, wm.ResizableUnspecifiedActivity)
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

	// Maximize the window (Required based on the spec).
	if err := act.SetWindowState(ctx, tconn, arc.WindowStateMaximized); err != nil {
		return err
	}
	if err := ash.WaitForARCAppWindowState(ctx, tconn, wm.Pkg24, ash.WindowStateMaximized); err != nil {
		return err
	}

	// Store initial window info to compare with after hiding and showing the shelf.
	winInfoInitialState, err := ash.GetARCAppWindowInfo(ctx, tconn, wm.Pkg24)
	if err != nil {
		return err
	}

	// Set shelf behavior to auto hide.
	if err := ash.SetShelfBehavior(ctx, tconn, primaryDisplayInfo.ID, ash.ShelfBehaviorAlwaysAutoHide); err != nil {
		return err
	}

	// Wait for shelf animation to complete.
	if err := wm.WaitForShelfAnimationComplete(ctx, tconn); err != nil {
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
	if err := wm.WaitForShelfAnimationComplete(ctx, tconn); err != nil {
		return errors.Wrap(err, "failed to wait for shelf animation to complete")
	}

	return testing.Poll(ctx, func(ctx context.Context) error {
		winInfoShelfReShown, err := ash.GetARCAppWindowInfo(ctx, tconn, wm.Pkg24)
		if err != nil {
			return testing.PollBreak(err)
		}
		// Compare window bounds after showing the shelf with initial bounds. They should be equal.
		if winInfoInitialState.BoundsInRoot != winInfoShelfReShown.BoundsInRoot {
			return errors.Errorf("invalid window bounds after hiding and showing the shelf, got: %s, want: %s", winInfoShelfReShown.BoundsInRoot, winInfoInitialState.BoundsInRoot)
		}

		return nil
	}, &testing.PollOptions{Timeout: 5 * time.Second})
}

// wmRC13 covers resizable/clamshell: freeform resize.
// Expected behavior is defined in: go/arc-wm-r RC13: resizable/clamshell: freeform resize.
func wmRC13(ctx context.Context, tconn *chrome.TestConn, a *arc.ARC, d *ui.Device) error {
	// Start a new activity.
	act, err := arc.NewActivity(a, wm.Pkg24, wm.ResizableUnspecifiedActivity)
	if err != nil {
		return errors.Wrap(err, "failed to create new activity")
	}
	defer act.Close()

	if err := act.Start(ctx, tconn); err != nil {
		return errors.Wrap(err, "failed to start new activity")
	}
	defer func(ctx context.Context) {
		act.Stop(ctx, tconn)
	}(ctx)

	if err := wm.WaitUntilActivityIsReady(ctx, tconn, act, d); err != nil {
		return errors.Wrap(err, "failed to wait until activity is ready")
	}

	owInfo, err := ash.GetARCAppWindowInfo(ctx, tconn, wm.Pkg24)
	if err != nil {
		return errors.Wrap(err, "failed to get arc app window info")
	}

	dInfo, err := display.GetPrimaryInfo(ctx, tconn)
	if err != nil {
		return errors.Wrap(err, "failed to get primary display info")
	}
	if dInfo == nil {
		return errors.New("failed to find primary display info")
	}

	// The actual anchor point that the cursor changes to resize icon is not on the activity's frame.
	// It starts from one pixel out of the activity's frame, adding one pixel is for that reason.
	bottomRight := coords.NewPoint(owInfo.TargetBounds.Left+owInfo.TargetBounds.Width+1, owInfo.TargetBounds.Top+owInfo.TargetBounds.Height+1)

	// Drag the activity +20% horizontaly.
	toX := owInfo.TargetBounds.Left + 6*owInfo.TargetBounds.Width/5

	// If it goes out of display work area, take the display work area width as the destination point.
	if toX > dInfo.WorkArea.Width {
		toX = dInfo.WorkArea.Width
	}

	// Drag the activity -20% vertically.
	toY := owInfo.TargetBounds.Top + 4*owInfo.TargetBounds.Height/5

	// Destination point.
	to := coords.NewPoint(toX, toY)

	if err := mouse.Drag(ctx, tconn, bottomRight, to, 600*time.Millisecond); err != nil {
		return errors.Wrap(err, "failed to drag the mouse")
	}

	wInfo, err := ash.GetARCAppWindowInfo(ctx, tconn, wm.Pkg24)
	if err != nil {
		return errors.Wrap(err, "unable to get arc app window info after freeform resizing")
	}

	if wInfo.TargetBounds == owInfo.TargetBounds {
		return errors.Errorf("invalid window bounds after freeform resizing: got %q; want different than %q", wInfo.TargetBounds, owInfo.TargetBounds)
	}
	return nil
}

// wmRC14 covers resizable/clamshell: snap to half screen
// Expected behavior is defined in: go/arc-wm-r RC14: resizable/clamshell: snap to half screen.
func wmRC14(ctx context.Context, tconn *chrome.TestConn, a *arc.ARC, d *ui.Device) error {
	// Snap to half by long pressing on the maximize caption button and drag to the right.
	if err := snapToHalfHelper(ctx, tconn, a, d, false, false); err != nil {
		return errors.Wrap(err, "snap to half by long pressing on the maximize caption button and drag to the right failed")
	}

	// Snap to half by long pressing on the maximize caption button and drag to the left.
	if err := snapToHalfHelper(ctx, tconn, a, d, false, true); err != nil {
		return errors.Wrap(err, "snap to half by long pressing on the maximize caption button and drag to the left failed")
	}

	// Snap to half by dragging the activity to the top right corner.
	if err := snapToHalfHelper(ctx, tconn, a, d, true, false); err != nil {
		return errors.Wrap(err, "snap to half by dragging the activity to the top right corner failed")
	}
	// Snap to half by dragging the activity to the top left corner.
	if err := snapToHalfHelper(ctx, tconn, a, d, true, true); err != nil {
		return errors.Wrap(err, "snap to half by dragging the activity to the top left corner failed")
	}
	return nil
}

// wmRC15 covers resizable/clamshell: display size change.
// Expected behavior is defined in: go/arc-wm-r RC15: resizable/clamshell: display size change.
func wmRC15(ctx context.Context, tconn *chrome.TestConn, a *arc.ARC, d *ui.Device) error {
	for _, actName := range []string{
		wm.ResizableLandscapeActivity,
		wm.ResizablePortraitActivity,
		wm.ResizableUnspecifiedActivity,
	} {
		if err := rcDisplaySizeChangeTestsHelper(ctx, tconn, a, d, actName); err != nil {
			errors.Wrapf(err, "%s test failed", actName)
		}
	}

	return nil
}

// wmRC17 covers resizable/clamshell: font size change.
// Expected behavior is defined in: go/arc-wm-r RC17: resizable/clamshell: font size change.
func wmRC17(ctx context.Context, tconn *chrome.TestConn, a *arc.ARC, d *ui.Device) error {
	// Font scale, this const must not be 1.
	const fsc = 1.2

	// Start a new activity.
	act, err := arc.NewActivity(a, wm.Pkg24, wm.ResizableUnspecifiedActivity)
	if err != nil {
		return errors.Wrap(err, "unable to create new activity")
	}
	defer act.Close()

	if err := act.Start(ctx, tconn); err != nil {
		return errors.Wrap(err, "unable to start new activity")
	}
	defer act.Stop(ctx, tconn)

	if err := wm.WaitUntilActivityIsReady(ctx, tconn, act, d); err != nil {
		return errors.Wrap(err, "unable to wait until activity is ready")
	}
	// Store original window info.
	owInfo, err := ash.GetARCAppWindowInfo(ctx, tconn, wm.Pkg24)
	if err != nil {
		return errors.Wrap(err, "unable to get arc app window info")
	}

	// Change the font scale.
	if err := wm.EnsureARCFontScaleChanged(ctx, a, fsc); err != nil {
		return errors.Wrap(err, "unable to set font scale")
	}
	defer wm.EnsureARCFontScaleChanged(ctx, a, 1)

	// Get the font scale.
	nfs, err := wm.GetARCFontScale(ctx, a)
	if err != nil {
		return errors.Wrap(err, "unable to get font scale")
	}

	// Get window info after font size change.
	wInfo, err := ash.GetARCAppWindowInfo(ctx, tconn, wm.Pkg24)
	if err != nil {
		return errors.Wrap(err, "unable to get arc app window info")
	}
	if owInfo.TargetBounds != wInfo.TargetBounds {
		return errors.Errorf("invalid window bounds after font scale is changed: got %q; want %q", wInfo.TargetBounds, owInfo.TargetBounds)
	}

	// Compare font scale before and after scale change.
	if nfs != fsc {
		return errors.Errorf("invalid font scale after font scale is changed: got %.1f; want %.1f", nfs, fsc)
	}

	return nil
}

// wmRC22 covers resizable/clamshell: snap to half screen
// Expected behavior is defined in: go/arc-wm-r RC22: resizable/clamshell: split screen
func wmRC22(ctx context.Context, tconn *chrome.TestConn, a *arc.ARC, d *ui.Device) (retErr error) {
	leftAct, err := arc.NewActivity(a, wm.Pkg24, wm.ResizableUnspecifiedActivity)
	if err != nil {
		return errors.Wrap(err, "failed to create left activity")
	}
	defer leftAct.Close()

	if err := leftAct.Start(ctx, tconn); err != nil {
		return errors.Wrap(err, "failed to start left activity")
	}
	defer func(ctx context.Context) {
		if err := leftAct.Stop(ctx, tconn); err != nil {
			if retErr == nil {
				retErr = errors.Wrap(err, "failed to stop left activity")
			} else {
				testing.ContextLog(ctx, "Failed to stop left activity: ", err)
			}
		}
	}(ctx)

	if err := wm.WaitUntilActivityIsReady(ctx, tconn, leftAct, d); err != nil {
		return errors.Wrap(err, "failed to wait until left activity is ready")
	}

	// Snap the activity to the left.
	if err := leftClickDragCaptionButton(ctx, tconn, "Maximize", true); err != nil {
		return errors.New("failed to left click and drag Maximize caption button")
	}

	if err := testing.Poll(ctx, func(ctx context.Context) error {
		leftWInfo, err := ash.GetARCAppWindowInfo(ctx, tconn, wm.Pkg24)
		if err != nil {
			return testing.PollBreak(errors.Wrap(err, "failed to get arc app window info for left activity"))
		}

		if leftWInfo.State != ash.WindowStateLeftSnapped {
			return errors.Errorf("invalid window state: got %q; want LeftSnapped", leftWInfo.State)
		}
		return nil
	}, &testing.PollOptions{Timeout: 5 * time.Second}); err != nil {
		return err
	}

	if err := a.Install(ctx, arc.APKPath(wm.APKNameArcWMTestApp24Secondary)); err != nil {
		return errors.Wrap(err, "failed to install extra APK")
	}
	rightAct, err := arc.NewActivity(a, wm.Pkg24Secondary, wm.ResizableUnspecifiedActivity)
	if err != nil {
		return err
	}
	defer rightAct.Close()

	if err := rightAct.Start(ctx, tconn); err != nil {
		return errors.Wrap(err, "failed to start right activity")
	}
	defer func(ctx context.Context) {
		if err := rightAct.Stop(ctx, tconn); err != nil {
			if retErr == nil {
				retErr = errors.Wrap(err, "failed to stop right activity")
			} else {
				testing.ContextLog(ctx, "Failed to stop right activity: ", err)
			}
		}
	}(ctx)

	// The second activity will be automatically snapped to the right.
	if err := wm.WaitUntilActivityIsReady(ctx, tconn, rightAct, d); err != nil {
		return errors.Wrap(err, "failed to wait until right activity is ready")
	}

	if err := testing.Poll(ctx, func(ctx context.Context) error {
		rightWInfo, err := ash.GetARCAppWindowInfo(ctx, tconn, wm.Pkg24Secondary)
		if err != nil {
			return testing.PollBreak(errors.Wrap(err, "failed to get arc app window info for right activity"))
		}

		if rightWInfo.State != ash.WindowStateRightSnapped {
			return errors.Errorf("invalid window state: got %q; want RightSnapped", rightWInfo.State)
		}
		return nil
	}, &testing.PollOptions{Timeout: 5 * time.Second}); err != nil {
		return err
	}

	pdInfo, err := display.GetPrimaryInfo(ctx, tconn)
	if err != nil {
		return err
	}

	leftWInfo, err := ash.GetARCAppWindowInfo(ctx, tconn, wm.Pkg24)
	if err != nil {
		return errors.Wrap(err, "failed to get arc app window info for left activity")
	}

	rightWInfo, err := ash.GetARCAppWindowInfo(ctx, tconn, wm.Pkg24Secondary)
	if err != nil {
		return errors.Wrap(err, "failed to get arc app window info for right activity")
	}

	lWant := coords.NewRect(0, 0, pdInfo.WorkArea.Width/2, pdInfo.WorkArea.Height)

	if leftWInfo.BoundsInRoot != lWant {
		return errors.Errorf("invalid snapped to the left activity bounds: got %+v; want %+v",
			leftWInfo.BoundsInRoot, lWant)
	}

	rWant := coords.NewRect(pdInfo.WorkArea.Width/2, 0, pdInfo.WorkArea.Width/2, pdInfo.WorkArea.Height)

	if rightWInfo.BoundsInRoot != rWant {
		return errors.Errorf("invalid snapped to the right activity bounds: got %+v; want %+v",
			rightWInfo.BoundsInRoot, rWant)
	}

	return nil

}

// snapToHalfHelper runs snap to half test cases by either
// long pressing on the maximize caption button (dragTheActivity = false)  and drag to the left (isLeft = true) or right (isLeft = false),
// or by dragging the activity (dragTheActivity = true) to the top left (isLeft = true) or right (isLeft = false) corner of the screen.
func snapToHalfHelper(ctx context.Context, tconn *chrome.TestConn, a *arc.ARC, d *ui.Device, dragTheActivity, isLeft bool) error {
	// Start a new activity.
	act, err := arc.NewActivity(a, wm.Pkg24, wm.ResizableUnspecifiedActivity)
	if err != nil {
		return errors.Wrap(err, "failed to create new activity")
	}
	defer act.Close()

	if err := act.Start(ctx, tconn); err != nil {
		return errors.Wrap(err, "failed to start new activity")
	}
	defer func(ctx context.Context) {
		act.Stop(ctx, tconn)
	}(ctx)

	if err := wm.WaitUntilActivityIsReady(ctx, tconn, act, d); err != nil {
		return errors.Wrap(err, "failed to wait until activity is ready")
	}

	dInfo, err := display.GetPrimaryInfo(ctx, tconn)
	if err != nil {
		return errors.Wrap(err, "failed to get primary display info")
	}
	if dInfo == nil {
		return errors.New("failed to find primary display info")
	}

	if dragTheActivity {
		wInfo, err := ash.GetARCAppWindowInfo(ctx, tconn, wm.Pkg24)
		if err != nil {
			return errors.Wrap(err, "failed to get arc app window info")
		}

		source := coords.NewPoint(wInfo.TargetBounds.Left+wInfo.TargetBounds.Width/2, wInfo.TargetBounds.Top+5)
		if err := leftClickDragSource(ctx, tconn, source, dInfo.WorkArea.Width, isLeft); err != nil {
			return errors.Wrap(err, "failed to drag caption bar to corner of screen")
		}
	} else {
		if err := leftClickDragCaptionButton(ctx, tconn, "Maximize", isLeft); err != nil {
			return errors.New("failed to left click and drag Maximize caption button")
		}
	}

	// Desired left border of activity after snap.
	left := 0
	if !isLeft {
		// if snap to right, left border is the middle of screen.
		left = dInfo.WorkArea.Width / 2
	}

	return testing.Poll(ctx, func(ctx context.Context) error {
		snpInfo, err := ash.GetARCAppWindowInfo(ctx, tconn, wm.Pkg24)
		if err != nil {
			return errors.Wrap(err, "failed to get arc app window info")
		}

		if snpInfo.TargetBounds.Top != 0 || snpInfo.TargetBounds.Left != left ||
			snpInfo.TargetBounds.Width != dInfo.WorkArea.Width/2 || snpInfo.TargetBounds.Height != dInfo.WorkArea.Height {
			halfBounds := coords.NewRect(0, left, dInfo.WorkArea.Width/2, dInfo.WorkArea.Height)
			return errors.Errorf("invalid window bounds after snap to half screen; got: %s, want: %s", snpInfo.TargetBounds, halfBounds)
		}

		return nil
	}, &testing.PollOptions{Timeout: 5 * time.Second})
}

// rcDisplaySizeChangeTestsHelper is used for Tast-tests that are testing resolution change and its effects on an activity.
func rcDisplaySizeChangeTestsHelper(ctx context.Context, tconn *chrome.TestConn, a *arc.ARC, d *ui.Device, activityName string) error {
	// Start a new activity.
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

	// Get primary display info before zoom.
	dIBZ, err := display.GetPrimaryInfo(ctx, tconn)
	if err != nil {
		return err
	}
	if dIBZ == nil {
		return errors.New("failed to find primary display info")
	}

	dID := dIBZ.ID
	oz := dIBZ.DisplayZoomFactor

	// Get app window info to get window bounds before zoom.
	wIBZ, err := ash.GetARCAppWindowInfo(ctx, tconn, wm.Pkg24)
	if err != nil {
		return err
	}
	wBBZ := wIBZ.BoundsInRoot

	// Get buttons info before zoom.
	bBBZ, err := wm.GetButtonBounds(ctx, d, act.PackageName())
	if err != nil {
		return err
	}

	nz := 0.

	zfs := dIBZ.AvailableDisplayZoomFactors
	for _, z := range zfs {
		if z > oz {
			nz = z
			break
		}
	}
	if nz == 0 {
		return errors.Errorf("invalid AvailableDisplayZoomFactors: got an empty array; want array with at least one value grater than '%.2f'", oz)
	}

	if err := wm.ChangeDisplayZoomFactor(ctx, tconn, dID, nz); err != nil {
		return err
	}
	defer wm.ChangeDisplayZoomFactor(ctx, tconn, dID, oz)

	// Get buttons info after zoom.
	bBAZ, err := wm.GetButtonBounds(ctx, d, act.PackageName())
	if err != nil {
		return err
	}

	// Get app window info to get window bounds After change the display resolution.
	wIAZ, err := ash.GetARCAppWindowInfo(ctx, tconn, wm.Pkg24)
	if err != nil {
		return err
	}
	wBAZ := wIAZ.BoundsInRoot

	// Get primary display info after display resolution change.
	dIAZ, err := display.GetPrimaryInfo(ctx, tconn)
	if err != nil {
		return err
	}
	if dIAZ == nil {
		return errors.New("failed to find primary display info")
	}

	if bBBZ == bBAZ {
		return errors.Errorf("invalid button bounds after resolution changed, got: %q, want different: %q", bBAZ, bBBZ)
	}

	if wBBZ != wBAZ {
		return errors.Errorf("invalid app window bounds after resolution changed, got: %q, want: %q", wBAZ, wBBZ)
	}

	if math.Abs(nz-dIBZ.DPIX/dIAZ.DPIX) > 0.01 {
		return errors.Errorf("invalid DPIX ration after resolution changed, got: %.3f, want: %.3f", dIBZ.DPIX/dIAZ.DPIX, nz)
	}

	if math.Abs(nz-dIBZ.DPIY/dIAZ.DPIY) > 0.01 {
		return errors.Errorf("invalid DPIY ration after resolution changed, got: %.3f, want: %.3f", dIBZ.DPIY/dIAZ.DPIY, nz)
	}

	return nil
}

// immerseViaAPIHelper used to run immerse via API from maximized by activity name.
func immerseViaAPIHelper(ctx context.Context, tconn *chrome.TestConn, a *arc.ARC, d *ui.Device, actName string) error {
	// Start a new activity.
	act, err := arc.NewActivity(a, wm.Pkg24, actName)
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

	windowInfoRestore, err := ash.GetARCAppWindowInfo(ctx, tconn, wm.Pkg24)
	if err != nil {
		return err
	}

	if err := leftClickCaptionButton(ctx, tconn, "Maximize"); err != nil {
		return err
	}
	if err := ash.WaitForARCAppWindowState(ctx, tconn, wm.Pkg24, ash.WindowStateMaximized); err != nil {
		return err
	}
	if err := ash.WaitWindowFinishAnimating(ctx, tconn, windowInfoRestore.ID); err != nil {
		return err
	}
	if err := wm.CheckMaximizeResizable(ctx, tconn, act, d); err != nil {
		return err
	}

	// Get window info before clicking on the immersive button.
	windowInfoBefore, err := ash.GetARCAppWindowInfo(ctx, tconn, wm.Pkg24)
	if err != nil {
		return err
	}

	// Click on the immersive button.
	if err := wm.UIClickImmersive(ctx, act, d); err != nil {
		return err
	}
	if err := wm.WaitUntilActivityIsReady(ctx, tconn, act, d); err != nil {
		return err
	}
	if err := ash.WaitWindowFinishAnimating(ctx, tconn, windowInfoBefore.ID); err != nil {
		return err
	}

	testing.Poll(ctx, func(ctx context.Context) error {
		// Get window info after the immersive button is clicked.
		windowInfoUIImmersive, err := ash.GetARCAppWindowInfo(ctx, tconn, wm.Pkg24)
		if err != nil {
			return testing.PollBreak(err)
		}
		if err := wm.CheckMaximizeToFullscreenToggle(ctx, tconn, windowInfoBefore.TargetBounds, *windowInfoUIImmersive); err != nil {
			return testing.PollBreak(err)
		}
		return nil
	}, &testing.PollOptions{Timeout: 5 * time.Second})

	// Click on the normal button.
	if err := wm.UIClickNormal(ctx, act, d); err != nil {
		return err
	}
	if err := wm.WaitUntilActivityIsReady(ctx, tconn, act, d); err != nil {
		return err
	}
	if err := ash.WaitWindowFinishAnimating(ctx, tconn, windowInfoBefore.ID); err != nil {
		return err
	}

	return testing.Poll(ctx, func(ctx context.Context) error {
		// Get window info after the normal button is clicked.
		windowInfoAfter, err := ash.GetARCAppWindowInfo(ctx, tconn, wm.Pkg24)
		if err != nil {
			return testing.PollBreak(err)
		}
		if windowInfoBefore.BoundsInRoot != windowInfoAfter.BoundsInRoot {
			return errors.Errorf("invalid window bounds after click on the immersive button, got: %q, want: %q", windowInfoAfter.BoundsInRoot, windowInfoBefore.BoundsInRoot)
		}
		return nil
	}, &testing.PollOptions{Timeout: 5 * time.Second})

}

// checkRestoreActivityToFullscreen creates a new activity, lunches it and toggles to fullscreen and checks for validity of window info.
func checkRestoreActivityToFullscreen(ctx context.Context, tconn *chrome.TestConn, a *arc.ARC, d *ui.Device, activityName string) error {
	// Start the activity
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

	// Check the activity
	if err := wm.CheckRestoreResizable(ctx, tconn, act, d); err != nil {
		return err
	}

	windowInfoBefore, err := ash.GetARCAppWindowInfo(ctx, tconn, wm.Pkg24)
	if err != nil {
		return err
	}

	// Toggle to fullscreen
	if err := wm.ToggleFullscreen(ctx, tconn); err != nil {
		return err
	}

	if err := ash.WaitForARCAppWindowState(ctx, tconn, wm.Pkg24, ash.WindowStateFullscreen); err != nil {
		return err
	}
	if err := ash.WaitWindowFinishAnimating(ctx, tconn, windowInfoBefore.ID); err != nil {
		return err
	}

	// Toggle back from fullscreen
	if err := wm.ToggleFullscreen(ctx, tconn); err != nil {
		return err
	}

	if err := ash.WaitForARCAppWindowState(ctx, tconn, wm.Pkg24, ash.WindowStateNormal); err != nil {
		return err
	}
	if err := ash.WaitWindowFinishAnimating(ctx, tconn, windowInfoBefore.ID); err != nil {
		return err
	}

	windowInfoAfter, err := ash.GetARCAppWindowInfo(ctx, tconn, wm.Pkg24)
	if err != nil {
		return err
	}

	if windowInfoBefore.BoundsInRoot != windowInfoAfter.BoundsInRoot {
		return errors.Errorf("invalid window bounds after switching from fullscreen, got: %q, want: %q", windowInfoAfter.BoundsInRoot, windowInfoBefore.BoundsInRoot)
	}

	return nil
}

// touchCaptionButton function will simulate touch event on a caption button by button's name.
func touchCaptionButton(ctx context.Context, tconn *chrome.TestConn, btnName string) error {
	captionBtn, err := crui.Find(ctx, tconn, crui.FindParams{ClassName: "FrameCaptionButton", Name: btnName})
	if err != nil {
		return errors.Errorf("failed to find \"%q\" caption button", btnName)
	}
	defer captionBtn.Release(ctx)

	tsw, err := input.Touchscreen(ctx)
	if err != nil {
		return errors.New("failed to get TouchscreenEventWriter")
	}
	defer tsw.Close()

	stw, err := tsw.NewSingleTouchWriter()
	if err != nil {
		return errors.New("failed to get SingleTouchEventWriter")
	}
	defer stw.Close()

	// Get display info for touch coords calculation.
	primaryDisplayInfo, err := display.GetPrimaryInfo(ctx, tconn)
	if err != nil {
		return errors.New("failed to get display info")
	}
	if primaryDisplayInfo == nil {
		return errors.New("no primary display info found")
	}

	cBCX, cBCY := tsw.NewTouchCoordConverter(primaryDisplayInfo.Bounds.Size()).ConvertLocation(captionBtn.Location.CenterPoint())

	// Touch caption button center.
	if err := stw.Move(cBCX, cBCY); err != nil {
		return errors.Errorf("failed to move touch event writer on \"%q\" button", btnName)
	}
	if err := stw.End(); err != nil {
		return errors.Errorf("failed to end touch event writer on \"%q\" button", btnName)
	}

	return nil
}

// leftClickCaptionButton function will simulate left click event on a caption button by button's name.
func leftClickCaptionButton(ctx context.Context, tconn *chrome.TestConn, btnName string) error {
	captionBtn, err := crui.Find(ctx, tconn, crui.FindParams{ClassName: "FrameCaptionButton", Name: btnName})
	if err != nil {
		return errors.Errorf("failed to find \"%q\" caption button", btnName)
	}
	defer captionBtn.Release(ctx)

	if err := captionBtn.LeftClick(ctx); err != nil {
		return errors.Errorf("failed to perform left click on \"%q\" button", btnName)
	}

	return nil
}

// leftClickDragCaptionButton function will simulate left click long press event on a caption button by button's name.
func leftClickDragCaptionButton(ctx context.Context, tconn *chrome.TestConn, btnName string, toLeft bool) error {
	captionBtn, err := crui.Find(ctx, tconn, crui.FindParams{ClassName: "FrameCaptionButton", Name: btnName})
	if err != nil {
		return errors.Errorf("failed to find \"%q\" caption button", btnName)
	}

	d := 25
	if toLeft {
		d = -d
	}

	dest := coords.NewPoint(captionBtn.Location.CenterPoint().X+d, captionBtn.Location.CenterPoint().Y)
	return mouse.Drag(ctx, tconn, captionBtn.Location.CenterPoint(), dest, 500*time.Millisecond)
}

// leftClickDragSource function will simulate left click on source coordinate and drag to left/right top corner of screen.
func leftClickDragSource(ctx context.Context, tconn *chrome.TestConn, source coords.Point, screenWidth int, toLeft bool) error {
	destX := 0
	destY := 0

	if !toLeft {
		destX = screenWidth
	}

	dest := coords.NewPoint(destX, destY)
	return mouse.Drag(ctx, tconn, source, dest, 750*time.Millisecond)
}

// rcMaxRestoreTestHelper performs RC02 test either by left clicking or touching the caption button.
func rcMaxRestoreTestHelper(ctx context.Context, tconn *chrome.TestConn, a *arc.ARC, d *ui.Device, actName string, etFunc func(context.Context, *chrome.TestConn, string) error) error {
	act, err := arc.NewActivity(a, wm.Pkg24, actName)
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

	if err := wm.CheckRestoreResizable(ctx, tconn, act, d); err != nil {
		return err
	}

	// Store windows info before maximizing the activity to compare it with after restoring it.
	winInfoBeforeMax, err := ash.GetARCAppWindowInfo(ctx, tconn, wm.Pkg24)
	if err != nil {
		return err
	}

	// Store windowID to wait for animating finishes.
	windowID := winInfoBeforeMax.ID

	// Touch/Click maximize button on caption bar.
	if err := etFunc(ctx, tconn, "Maximize"); err != nil {
		return err
	}

	if err := ash.WaitForARCAppWindowState(ctx, tconn, wm.Pkg24, ash.WindowStateMaximized); err != nil {
		return err
	}
	if err := ash.WaitWindowFinishAnimating(ctx, tconn, windowID); err != nil {
		return err
	}

	if err := wm.CheckMaximizeResizable(ctx, tconn, act, d); err != nil {
		return err
	}

	// Touch/Click restore button on caption bar.
	if err := etFunc(ctx, tconn, "Restore"); err != nil {
		return err
	}

	if err := ash.WaitForARCAppWindowState(ctx, tconn, wm.Pkg24, ash.WindowStateNormal); err != nil {
		return err
	}
	if err := ash.WaitWindowFinishAnimating(ctx, tconn, windowID); err != nil {
		return err
	}

	if err := wm.CheckRestoreResizable(ctx, tconn, act, d); err != nil {
		return err
	}

	// Get window info after restoring, this should be equal to winInfoBeforeMax.
	winInfoAfterMax, err := ash.GetARCAppWindowInfo(ctx, tconn, wm.Pkg24)
	if err != nil {
		return err
	}

	// Compare BoundsInRoot of the activity before and after switching to maximize and restore button on caption bar.
	if winInfoBeforeMax.BoundsInRoot != winInfoAfterMax.BoundsInRoot {
		return errors.Errorf("failed to validate window bounds after restoring from maximize state, got: %q, want: %q", winInfoAfterMax.BoundsInRoot, winInfoBeforeMax.BoundsInRoot)
	}

	return nil
}

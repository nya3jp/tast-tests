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
	"chromiumos/tast/local/coords"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         WMNonresizableClamshell,
		Desc:         "Verifies that Window Manager non-resizable clamshell use-cases behave as described in go/arc-wm-r",
		Contacts:     []string{"armenk@google.com", "arc-framework+tast@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"android_vm", "chrome"},
		Fixture:      "arcBooted",
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
			// non-resizable/clamshell: immerse via API from maximized
			Name: "NC_immerse_via_API_from_maximized",
			Func: wmNC07,
		},
		wm.TestCase{
			// non-resizable/clamshell: new activity follows root activity
			Name: "NC_new_activity_follows_root_activity",
			Func: wmNC09,
		},
		wm.TestCase{
			// non-resizable/clamshell: new activity replaces root activity
			Name: "NC_new_activity_replaces_root_activity",
			Func: wmNC10,
		},
		wm.TestCase{
			// non-resizable/clamshell: hide shelf when app maximized
			Name: "NC_hide_shelf_app_max",
			Func: wmNC12,
		},
		wm.TestCase{
			// non-resizable/clamshell: display size change
			Name: "NC_display_size_change",
			Func: wmNC15,
		},
		wm.TestCase{
			// non-resizable/clamshell: font size change
			Name: "NC_font_size_change",
			Func: wmNC17,
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

// wmNC07 covers non-resizable/clamshell: immerse via API from maximized.
// Expected behavior is defined in: go/arc-wm-r NC07: non-resizable/clamshell: immerse via API from maximized.
func wmNC07(ctx context.Context, tconn *chrome.TestConn, a *arc.ARC, d *ui.Device) error {
	// Start a new activity.
	act, err := arc.NewActivity(a, wm.Pkg24, wm.NonResizablePortraitActivity)
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

	// Get window info after the immersive button is clicked.
	windowInfoUIImmersive, err := ash.GetARCAppWindowInfo(ctx, tconn, wm.Pkg24)
	if err != nil {
		return err
	}
	if err := ash.WaitWindowFinishAnimating(ctx, tconn, windowInfoBefore.ID); err != nil {
		return err
	}
	if err := wm.CheckMaximizeToFullscreenToggle(ctx, tconn, windowInfoBefore.TargetBounds, *windowInfoUIImmersive); err != nil {
		return err
	}

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

	windowInfoAfter, err := ash.GetARCAppWindowInfo(ctx, tconn, wm.Pkg24)
	if err != nil {
		return err
	}

	if windowInfoBefore.BoundsInRoot != windowInfoAfter.BoundsInRoot {
		return errors.Errorf("invalid window bounds after click on the immersive button, got: %q, want: %q", windowInfoAfter.BoundsInRoot, windowInfoBefore.BoundsInRoot)
	}

	return nil
}

// wmNC09 covers non-resizable/clamshell: new activity follows root activity.
// Expected behavior is defined in: go/arc-wm-r NC09: resizable/clamshell: new activity follows root activity.
func wmNC09(ctx context.Context, tconn *chrome.TestConn, a *arc.ARC, d *ui.Device) error {
	// Start the activity
	act, err := arc.NewActivity(a, wm.Pkg24, wm.NonResizableUnspecifiedActivity)
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

	// Get the root window info so it could be compared with child window.
	rootWindowInfo, err := ash.GetARCAppWindowInfo(ctx, tconn, wm.Pkg24)
	if err != nil {
		return err
	}

	if err := wm.UIClickLaunchActivity(ctx, act, d); err != nil {
		return err
	}
	if err := wm.WaitUntilActivityIsReady(ctx, tconn, act, d); err != nil {
		return err
	}

	if nrActivities, err := wm.UINumberActivities(ctx, act, d); err != nil {
		return err
	} else if nrActivities != 2 {
		return errors.Errorf("invalid number of activities: got %d; want 2", nrActivities)
	}

	childWindowInfo, err := ash.GetARCAppWindowInfo(ctx, tconn, wm.Pkg24)
	if err != nil {
		return err
	}

	if rootWindowInfo.BoundsInRoot != childWindowInfo.BoundsInRoot {
		return errors.Errorf("invalid child activity window bounds, got: %q, want: %q", childWindowInfo.BoundsInRoot, rootWindowInfo.BoundsInRoot)
	}

	return nil
}

// wmNC10 covers non-resizable/clamshell: new activity replaces root activity.
// Expected behavior is defined in: go/arc-wm-r NC10: non-resizable/clamshell: new activity replaces root activity.
func wmNC10(ctx context.Context, tconn *chrome.TestConn, a *arc.ARC, d *ui.Device) error {
	// Start the activity.
	act, err := arc.NewActivity(a, wm.Pkg24, wm.NonResizableUnspecifiedActivity)
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

	// Get the root window info so it could be compared with child window.
	rootWindowInfo, err := ash.GetARCAppWindowInfo(ctx, tconn, wm.Pkg24)
	if err != nil {
		return err
	}

	if err := wm.UIClickRootActivity(ctx, act, d); err != nil {
		return err
	}
	if err := wm.UIClickLaunchActivity(ctx, act, d); err != nil {
		return err
	}
	if err := wm.WaitUntilActivityIsReady(ctx, tconn, act, d); err != nil {
		return err
	}

	if nrActivities, err := wm.UINumberActivities(ctx, act, d); err != nil {
		return err
	} else if nrActivities != 1 {
		return errors.Errorf("invalid number of activities: got %d; want 1", nrActivities)
	}

	childWindowInfo, err := ash.GetARCAppWindowInfo(ctx, tconn, wm.Pkg24)
	if err != nil {
		return err
	}

	if rootWindowInfo.BoundsInRoot != childWindowInfo.BoundsInRoot {
		return errors.Errorf("invalid child activity window bounds, got: %q, want: %q", childWindowInfo.BoundsInRoot, rootWindowInfo.BoundsInRoot)
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

// wmNC15 covers non-resizable/clamshell: display size change.
// Expected behavior is defined in: go/arc-wm-r NC15: non-resizable/clamshell: display size change.
func wmNC15(ctx context.Context, tconn *chrome.TestConn, a *arc.ARC, d *ui.Device) error {
	for _, actName := range []string{
		wm.NonResizableLandscapeActivity,
		wm.NonResizablePortraitActivity,
		wm.NonResizableUnspecifiedActivity,
	} {
		if err := ncDisplaySizeChangeTestsHelper(ctx, tconn, a, d, actName); err != nil {
			return err
		}
	}

	return nil
}

// wmNC17 covers non-resizable/clamshell: font size change.
// Expected behavior is defined in: go/arc-wm-r NC17: non-resizable/clamshell: font size change.
func wmNC17(ctx context.Context, tconn *chrome.TestConn, a *arc.ARC, d *ui.Device) (retErr error) {
	timeReservedForStop := 750 * time.Millisecond
	timeReservedForCleanup := 750 * time.Millisecond

	// Font scale, this const must not be 1.
	const fsc = 1.2

	// Save time for stopping the activity.
	ctxForStopActivity := ctx
	ctx, cancelForStopActivity := ctxutil.Shorten(ctx, timeReservedForStop)
	defer cancelForStopActivity()

	ctxForCleanup := ctx
	ctx, cancelForCleanup := ctxutil.Shorten(ctx, timeReservedForCleanup)
	defer cancelForCleanup()

	// Start a new activity.
	act, err := arc.NewActivity(a, wm.Pkg24, wm.NonResizableUnspecifiedActivity)
	if err != nil {
		return errors.Wrap(err, "unable to create new activity")
	}
	defer act.Close()

	if err := act.Start(ctx, tconn); err != nil {
		return errors.Wrap(err, "unable to start new activity")
	}
	defer func() {
		if err := act.Stop(ctxForStopActivity, tconn); err != nil {
			if retErr == nil {
				retErr = errors.Wrap(err, "unable to stop activity")
			} else {
				testing.ContextLog(ctx, "Unable to stop activity")
			}
		}
	}()

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
	defer func() {
		if err := wm.EnsureARCFontScaleChanged(ctxForCleanup, a, 1); err != nil {
			if retErr == nil {
				retErr = errors.Wrap(err, "unable to clean up font scale changes")
			} else {
				testing.ContextLog(ctx, "Unlable to clean up font scale changes")
			}
		}
	}()

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

// ncDisplaySizeChangeTestsHelper is used for non-resizable Tast-tests that are testing resolution change.
func ncDisplaySizeChangeTestsHelper(ctx context.Context, tconn *chrome.TestConn, a *arc.ARC, d *ui.Device, activityName string) error {
	const roundingErrorDecimal = 0.01

	act, err := arc.NewActivity(a, wm.Pkg24, activityName)
	if err != nil {
		return err
	}
	defer act.Close()

	// Start the activity.
	if err := act.Start(ctx, tconn); err != nil {
		return err
	}
	defer act.Stop(ctx, tconn)

	if err := wm.WaitUntilActivityIsReady(ctx, tconn, act, d); err != nil {
		return err
	}

	// Get primary display info before zoom.
	dispInfoBeforeZoom, err := display.GetPrimaryInfo(ctx, tconn)
	if err != nil {
		return err
	}
	if dispInfoBeforeZoom == nil {
		return errors.New("failed to find primary display info")
	}

	displayID := dispInfoBeforeZoom.ID
	originalZoom := dispInfoBeforeZoom.DisplayZoomFactor

	// Get buttons info before zoom.
	buttonBoundsBeforeZoom, err := wm.GetButtonBounds(ctx, d, act.PackageName())
	if err != nil {
		return err
	}

	newZoom := 0.

	displayZoomFactors := dispInfoBeforeZoom.AvailableDisplayZoomFactors
	for _, z := range displayZoomFactors {
		if z > originalZoom {
			newZoom = z
			break
		}
	}
	if newZoom == 0 {
		return errors.Errorf("invalid AvailableDisplayZoomFactors: got an empty array; want array with at least one value greater than '%.2f'", originalZoom)
	}

	if err := wm.ChangeDisplayZoomFactor(ctx, tconn, displayID, newZoom); err != nil {
		return err
	}
	defer wm.ChangeDisplayZoomFactor(ctx, tconn, displayID, dispInfoBeforeZoom.DisplayZoomFactor)

	// Get buttons info after zoom.
	buttonBoundsAfterZoom, err := wm.GetButtonBounds(ctx, d, act.PackageName())
	if err != nil {
		return err
	}

	// Get primary display info after zoom.
	dispInfoAfterZoom, err := display.GetPrimaryInfo(ctx, tconn)
	if err != nil {
		return err
	}
	if dispInfoAfterZoom == nil {
		return errors.New("failed to find primary display info")
	}

	// The window is maximized, so the content should be relayout (button bounds must be different).
	if buttonBoundsBeforeZoom == buttonBoundsAfterZoom {
		return errors.Errorf("invalid button bounds after resolution changed, got: %q, want different than: %q", buttonBoundsAfterZoom, buttonBoundsBeforeZoom)
	}

	if err := buttonBoundsCheckAfterZoom(buttonBoundsAfterZoom, buttonBoundsBeforeZoom, newZoom/originalZoom); err != nil {
		return err
	}

	// DPI before zoom divided by DPI after zoom should be equal to zoom coefficient.
	// Because of possible roundings, the difference is calculated that should be less than roundingErrorDecimal (0.01) to have up to 2 decimal points of precision.
	if math.Abs(newZoom/originalZoom-dispInfoBeforeZoom.DPIX/dispInfoAfterZoom.DPIX) > roundingErrorDecimal {
		return errors.Errorf("invalid DPIX ratio after resolution changed, got: %.3f, want: %.3f", dispInfoBeforeZoom.DPIX/dispInfoAfterZoom.DPIX, newZoom)
	}
	if math.Abs(newZoom/originalZoom-dispInfoBeforeZoom.DPIY/dispInfoAfterZoom.DPIY) > roundingErrorDecimal {
		return errors.Errorf("invalid DPIY ratio after resolution changed, got: %.3f, want: %.3f", dispInfoBeforeZoom.DPIY/dispInfoAfterZoom.DPIY, newZoom)
	}

	return nil
}

// buttonBoundsCheckAfterZoom calculates old rect (before zoom) values based on the new rect and the ratio.
// If the difference between calculated values and old rect values is greater than roundingErrorInt (1), the function will return an error.
func buttonBoundsCheckAfterZoom(newRect, oldRect coords.Rect, ratio float64) error {
	const roundingErrorInt = 1

	oldTopCalc := (int)(math.Round((float64)(newRect.Top) * ratio))
	oldLeftCalc := (int)(math.Round((float64)(newRect.Left) * ratio))
	oldWidthCalc := (int)(math.Round((float64)(newRect.Width) * ratio))
	oldHeightCalc := (int)(math.Round((float64)(newRect.Height) * ratio))

	if intAbs(oldRect.Top-oldTopCalc) > roundingErrorInt {
		return errors.Errorf("Calculated Top is incorrect, got: %d, want: %d", oldTopCalc, oldRect.Top)
	}
	if intAbs(oldRect.Left-oldLeftCalc) > roundingErrorInt {
		return errors.Errorf("Calculated Left is incorrect, got: %d, want: %d", oldLeftCalc, oldRect.Left)
	}
	if intAbs(oldRect.Width-oldWidthCalc) > roundingErrorInt {
		return errors.Errorf("Calculated Width is incorrect, got: %d, want: %d", oldWidthCalc, oldRect.Width)
	}
	if intAbs(oldRect.Height-oldHeightCalc) > roundingErrorInt {
		return errors.Errorf("Calculated Height is incorrect, got: %d, want: %d", oldHeightCalc, oldRect.Height)
	}

	return nil
}

// intAbs returns absolute value of an integer input.
func intAbs(v int) int {
	if v < 0 {
		return -1 * v
	}
	return v
}

// checkMaxActivityToFullscreen creates a new activity, lunches it and toggles to fullscreen and checks for validity of window info.
func checkMaxActivityToFullscreen(ctx context.Context, tconn *chrome.TestConn, a *arc.ARC, d *ui.Device, activityName string) error {
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
	if err := wm.CheckMaximizeNonResizable(ctx, tconn, act, d); err != nil {
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

	windowInfoFullscreen, err := ash.GetARCAppWindowInfo(ctx, tconn, wm.Pkg24)
	if err != nil {
		return err
	}
	if err := wm.CheckMaximizeToFullscreenToggle(ctx, tconn, windowInfoBefore.TargetBounds, *windowInfoFullscreen); err != nil {
		return err
	}

	// Toggle back from fullscreen
	if err := wm.ToggleFullscreen(ctx, tconn); err != nil {
		return err
	}

	if err := ash.WaitForARCAppWindowState(ctx, tconn, wm.Pkg24, ash.WindowStateMaximized); err != nil {
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

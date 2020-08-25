// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"math"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/arc/ui"
	"chromiumos/tast/local/bundles/cros/arc/wm"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/display"
	crui "chromiumos/tast/local/chrome/ui"
	"chromiumos/tast/local/coords"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

const (
	pkgMaximized  = "org.chromium.arc.testapp.windowmanager24.inmaximizedlist"
	pkgPhoneSize  = "org.chromium.arc.testapp.windowmanager24.inphonesizelist"
	pkgTabletSize = "org.chromium.arc.testapp.windowmanager24.intabletsizelist"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         WMResizableClamshell,
		Desc:         "Verifies that Window Manager resizable clamshell use-cases behave as described in go/arc-wm-r",
		Contacts:     []string{"xutan@google.com", "arc-framework+tast@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"android_vm", "chrome"},
		Pre:          arc.Booted(),
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
			// resizable/clamshell: hide Shelf when app maximized
			Name: "RC12_hide_Shelf_when_app_maximized",
			Func: wmRC12,
		},
		wm.TestCase{
			// resizable/clamshell: display size change
			Name: "RC15_display_size_change",
			Func: wmRC15,
		},
	})
}

// wmRC01 covers resizable/clamshell default launch behavior.
// Expected behavior is defined in: go/arc-wm-r RC01: resizable/clamshell: default launch behavior.
func wmRC01(ctx context.Context, tconn *chrome.TestConn, a *arc.ARC, d *ui.Device) error {
	// launchBoundsThreshold stores the launch bounds of the last activity launch. This test is
	// written in the order from small launch bounds to big launch bounds so this variable
	// serves as the lower bound of launch bounds.
	var launchBoundsThreshold coords.Rect

	// Start with ordinary case where we are launching apps not in the whitelist.
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

			if desiredOrientation == wm.Portrait {
				launchBoundsThreshold = window.BoundsInRoot
			}

			return nil
		}(); err != nil {
			return errors.Wrapf(err, "%q test failed", activityName)
		}
	}

	// Then we verify the launch logic for whitelisted apps is correct.
	apkPath := map[string]string{
		pkgMaximized:  "ArcWMTestApp_24_InMaximizedList.apk",
		pkgPhoneSize:  "ArcWMTestApp_24_InPhoneSizeList.apk",
		pkgTabletSize: "ArcWMTestApp_24_InTabletSizeList.apk",
	}
	verifyFuncMap := map[string]func(*arc.Activity, *ash.Window) error{
		pkgPhoneSize: func(act *arc.Activity, window *ash.Window) error {
			if err := wm.CheckRestoreResizable(ctx, tconn, act, d); err != nil {
				return err
			}

			orientation, err := wm.UIOrientation(ctx, act, d)
			if err != nil {
				return err
			}
			if orientation != wm.Portrait {
				return errors.Errorf("invalid orientation: got %v; want portrait", orientation)
			}
			orientationFromBounds := wm.OrientationFromBounds(window.BoundsInRoot)
			if orientationFromBounds != wm.Portrait {
				return errors.Errorf("invalid bounds orientation: got %v; want portrait", orientationFromBounds)
			}

			if launchBoundsThreshold.Width > window.BoundsInRoot.Width {
				return errors.Errorf("phone size width shouldn't be smaller than %v, but it's %v",
					launchBoundsThreshold.Width, window.BoundsInRoot.Width)
			}
			if launchBoundsThreshold.Height > window.BoundsInRoot.Height {
				return errors.Errorf("phone size height shouldn't be smaller than %v, but it's %v",
					launchBoundsThreshold.Height, window.BoundsInRoot.Height)
			}
			return nil
		},
		pkgTabletSize: func(act *arc.Activity, window *ash.Window) error {
			if window.State == ash.WindowStateMaximized {
				// We might be running on a small device that can't hold a freeform window of tablet size.
				// However we don't have concrete display value to verify it, so we won't check display size.
				return wm.CheckMaximizeResizable(ctx, tconn, act, d)
			}

			// The majority of our devices is large enough to hold a freeform window of tablet size so it should
			// come here.
			if err := wm.CheckRestoreResizable(ctx, tconn, act, d); err != nil {
				return err
			}

			orientation, err := wm.UIOrientation(ctx, act, d)
			if err != nil {
				return err
			}
			if orientation != wm.Landscape {
				return errors.Errorf("invalid orientation: got %v; want landscape", orientation)
			}
			orientationFromBounds := wm.OrientationFromBounds(window.BoundsInRoot)
			if orientationFromBounds != wm.Landscape {
				return errors.Errorf("invalid bounds orientation: got %v; want landscape", orientationFromBounds)
			}

			if launchBoundsThreshold.Width > window.BoundsInRoot.Width {
				return errors.Errorf("tablet size width shouldn't be smaller than %v, but it's %v",
					launchBoundsThreshold.Width, window.BoundsInRoot.Width)
			}
			if launchBoundsThreshold.Height > window.BoundsInRoot.Height {
				return errors.Errorf("tablet size height shouldn't be smaller than %v, but it's %v",
					launchBoundsThreshold.Height, window.BoundsInRoot.Height)
			}
			return nil
		},
		pkgMaximized: func(act *arc.Activity, window *ash.Window) error {
			return wm.CheckMaximizeResizable(ctx, tconn, act, d)
		},
	}

	for _, pkgName := range []string{pkgPhoneSize, pkgTabletSize, pkgMaximized} {
		verifyFunc := verifyFuncMap[pkgName]
		if err := func() error {
			if err := a.Install(ctx, arc.APKPath(apkPath[pkgName])); err != nil {
				return err
			}
			defer a.Uninstall(ctx, pkgName)

			act, err := arc.NewActivity(a, pkgName, wm.ResizableUnspecifiedActivity)
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

			if err := ash.WaitForVisible(ctx, tconn, pkgName); err != nil {
				return err
			}
			window, err := ash.GetARCAppWindowInfo(ctx, tconn, pkgName)
			if err != nil {
				return err
			}

			if err := verifyFunc(act, window); err != nil {
				return err
			}

			launchBoundsThreshold = window.BoundsInRoot
			return nil
		}(); err != nil {
			return err
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
		if err := runRC02ByEventTypeFunc(ctx, tconn, a, d, eTC.Func); err != nil {
			return errors.Wrapf(err, "%q event type test case failed", eTC.Name)
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

// wmRC15 covers resizable/clamshell: display size change.
// Expected behavior is defined in: go/arc-wm-r RC15: resizable/clamshell: display size change.
func wmRC15(ctx context.Context, tconn *chrome.TestConn, a *arc.ARC, d *ui.Device) error {
	for _, actName := range []string{
		wm.ResizableLandscapeActivity,
		wm.ResizablePortraitActivity,
		wm.ResizableUnspecifiedActivity,
	} {
		if err := rCDisplaySizeChangeTestsHelper(ctx, tconn, a, d, actName); err != nil {
			errors.Wrapf(err, "%s test failed", actName)
		}
	}

	return nil
}

// rCDisplaySizeChangeTestsHelper is used for Tast-tests that are testing resolution change and its effects on an activity.
func rCDisplaySizeChangeTestsHelper(ctx context.Context, tconn *chrome.TestConn, a *arc.ARC, d *ui.Device, activityName string) error {
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
	defer wm.ChangeDisplayZoomFactor(ctx, tconn, dID, dIBZ.DisplayZoomFactor)

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

// runRC02ByEventTypeFunc performs RC02 test either by left clicking or touching the caption button.
func runRC02ByEventTypeFunc(ctx context.Context, tconn *chrome.TestConn, a *arc.ARC, d *ui.Device, etFunc func(context.Context, *chrome.TestConn, string) error) error {
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

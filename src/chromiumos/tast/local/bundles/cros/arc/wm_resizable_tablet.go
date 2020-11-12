// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
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
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

const (
	extraPkgName = "org.chromium.arc.testapp.windowmanager24.inmaximizedlist"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         WMResizableTablet,
		Desc:         "Verifies that Window Manager resizable tablet use-cases behave as described in go/arc-wm-r",
		Contacts:     []string{"armenk@google.com", "arc-framework+tast@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"android_vm", "chrome", "tablet_mode"},
		Pre:          arc.Booted(),
		Timeout:      8 * time.Minute,
	})
}

func WMResizableTablet(ctx context.Context, s *testing.State) {
	wm.SetupAndRunTestCases(ctx, s, true, []wm.TestCase{
		wm.TestCase{
			// resizable/tablet: default launch behavior
			Name: "RT_default_launch_behavior",
			Func: wmRT01,
		},
		wm.TestCase{
			// resizable/tablet: immerse via API from maximized
			Name: "RT_immerse_via_API_from_maximized",
			Func: wmRT07,
		},
		wm.TestCase{
			// resizable/tablet: hide Shelf behavior
			Name: "RT_hide_Shelf_behavior",
			Func: wmRT12,
		},
		wm.TestCase{
			// resizable/tablet: display size change
			Name: "RT_display_size_change",
			Func: wmRT15,
		},
		wm.TestCase{
			// resizable/tablet: font size change
			Name: "RT_font_size_change",
			Func: wmRT17,
		},
		wm.TestCase{
			// resizable/tablet: split screen
			Name: "RT_split_screen",
			Func: wmRT22,
		},
	})
}

// wmRT01 covers resizable/tablet: default launch behavior.
// Expected behavior is defined in: go/arc-wm-r RT01: resizable/tablet: default launch behavior.
func wmRT01(ctx context.Context, tconn *chrome.TestConn, a *arc.ARC, d *ui.Device) error {
	ntActivities := []wm.TabletLaunchActivityInfo{
		wm.TabletLaunchActivityInfo{
			ActivityName: wm.ResizableLandscapeActivity,
			DesiredDO:    display.OrientationLandscapePrimary,
		},
		wm.TabletLaunchActivityInfo{
			ActivityName: wm.ResizablePortraitActivity,
			DesiredDO:    display.OrientationPortraitPrimary,
		},
	}

	return wm.TabletDefaultLaunchHelper(ctx, tconn, a, d, ntActivities, true)
}

// wmRT07 covers resizable/tablet: immerse via API from maximized.
// Expected behavior is defined in: go/arc-wm-r RT07: resizable/tablet: immerse via API from maximized.
func wmRT07(ctx context.Context, tconn *chrome.TestConn, a *arc.ARC, d *ui.Device) error {
	acts := []wm.TabletLaunchActivityInfo{
		wm.TabletLaunchActivityInfo{
			ActivityName: wm.ResizableLandscapeActivity,
			DesiredDO:    display.OrientationLandscapePrimary,
		},
		wm.TabletLaunchActivityInfo{
			ActivityName: wm.ResizableUnspecifiedActivity,
			DesiredDO:    display.OrientationLandscapePrimary,
		},
		wm.TabletLaunchActivityInfo{
			ActivityName: wm.ResizablePortraitActivity,
			DesiredDO:    display.OrientationPortraitPrimary,
		},
		wm.TabletLaunchActivityInfo{
			ActivityName: wm.ResizableUnspecifiedActivity,
			DesiredDO:    display.OrientationPortraitPrimary,
		},
	}

	return wm.TabletImmerseViaAPI(ctx, tconn, a, d, acts)
}

// wmRT12 covers resizable/tablet: hide Shelf behavior.
// Expected behavior is defined in: go/arc-wm-r RT12: resizable/tablet: hide Shelf.
func wmRT12(ctx context.Context, tconn *chrome.TestConn, a *arc.ARC, d *ui.Device) error {
	// landscape | undefined activities.
	luActivities := []wm.TabletLaunchActivityInfo{
		wm.TabletLaunchActivityInfo{
			ActivityName: wm.ResizableLandscapeActivity,
			DesiredDO:    display.OrientationLandscapePrimary,
		},
		wm.TabletLaunchActivityInfo{
			ActivityName: wm.ResizableUnspecifiedActivity,
			DesiredDO:    display.OrientationLandscapePrimary,
		},
	}

	if err := wm.TabletShelfHideShowHelper(ctx, tconn, a, d, luActivities, wm.CheckMaximizeResizable); err != nil {
		return err
	}

	// portrait | undefined activities.
	puActivities := []wm.TabletLaunchActivityInfo{
		wm.TabletLaunchActivityInfo{
			ActivityName: wm.ResizablePortraitActivity,
			DesiredDO:    display.OrientationPortraitPrimary,
		},
		wm.TabletLaunchActivityInfo{
			ActivityName: wm.ResizableUnspecifiedActivity,
			DesiredDO:    display.OrientationPortraitPrimary,
		},
	}

	return wm.TabletShelfHideShowHelper(ctx, tconn, a, d, puActivities, wm.CheckMaximizeResizable)
}

// wmRT15 covers resizable/tablet: display size change.
// Expected behavior is defined in: go/arc-wm-r RT15: resizable/tablet: display size change.
func wmRT15(ctx context.Context, tconn *chrome.TestConn, a *arc.ARC, d *ui.Device) error {
	ntActivities := []wm.TabletLaunchActivityInfo{
		wm.TabletLaunchActivityInfo{
			ActivityName: wm.ResizableLandscapeActivity,
			DesiredDO:    display.OrientationLandscapePrimary,
		},
		wm.TabletLaunchActivityInfo{
			ActivityName: wm.ResizableUnspecifiedActivity,
			DesiredDO:    display.OrientationLandscapePrimary,
		},
		wm.TabletLaunchActivityInfo{
			ActivityName: wm.ResizablePortraitActivity,
			DesiredDO:    display.OrientationPortraitPrimary,
		},
		wm.TabletLaunchActivityInfo{
			ActivityName: wm.ResizableUnspecifiedActivity,
			DesiredDO:    display.OrientationPortraitPrimary,
		},
	}

	return wm.TabletDisplaySizeChangeHelper(ctx, tconn, a, d, ntActivities)
}

// wmRT17 covers resizable/tablet: font size change.
// Expected behavior is defined in: go/arc-wm-r RT17: resizable/tablet: font size change.
func wmRT17(ctx context.Context, tconn *chrome.TestConn, a *arc.ARC, d *ui.Device) error {
	acts := []wm.TabletLaunchActivityInfo{
		wm.TabletLaunchActivityInfo{
			ActivityName: wm.ResizableLandscapeActivity,
			DesiredDO:    display.OrientationLandscapePrimary,
		},
		wm.TabletLaunchActivityInfo{
			ActivityName: wm.ResizableUnspecifiedActivity,
			DesiredDO:    display.OrientationLandscapePrimary,
		},
		wm.TabletLaunchActivityInfo{
			ActivityName: wm.ResizablePortraitActivity,
			DesiredDO:    display.OrientationPortraitPrimary,
		},
		wm.TabletLaunchActivityInfo{
			ActivityName: wm.ResizableUnspecifiedActivity,
			DesiredDO:    display.OrientationPortraitPrimary,
		},
	}

	return wm.TabletFontSizeChangeHelper(ctx, tconn, a, d, acts)
}

// wmRT22 covers resizable/tablet: split screen.
// Expected behavior is defined in: go/arc-wm-r RT22: resizable/tablet: split screen.
func wmRT22(ctx context.Context, tconn *chrome.TestConn, a *arc.ARC, d *ui.Device) (retErr error) {
	extraApkPath := "ArcWMTestApp_24_InMaximizedList.apk"

	if err := a.Install(ctx, arc.APKPath(extraApkPath)); err != nil {
		return errors.Wrap(err, "failed to install extra APK")
	}

	if err := rt22Helper(ctx, tconn, a, d, true); err != nil {
		return err
	}
	return rt22Helper(ctx, tconn, a, d, false)
}

// rt22Helper runs two ARC activities and snaps them to opposite sides and checks the bounds validity.
func rt22Helper(ctx context.Context, tconn *chrome.TestConn, a *arc.ARC, d *ui.Device, isLandscape bool) (retErr error) {
	timeReservedForStop := 500 * time.Millisecond
	waitForSnappingAnimation := 750 * time.Millisecond

	if !isLandscape {
		cleanupRotation, err := wm.RotateDisplay(ctx, tconn, display.Rotate270)
		if err != nil {
			return err
		}
		defer cleanupRotation()
	}

	ctxForStopOverActivity := ctx
	ctx, cancelForStopOverActivity := ctxutil.Shorten(ctx, timeReservedForStop)
	defer cancelForStopOverActivity()

	ctxForStopUnderActivity := ctx
	ctx, cancelForStopUnderActivity := ctxutil.Shorten(ctx, timeReservedForStop)
	defer cancelForStopUnderActivity()

	underActivity, err := arc.NewActivity(a, wm.Pkg24, wm.ResizableUnspecifiedActivity)
	if err != nil {
		return errors.Wrap(err, "failed to create new activity")
	}
	defer underActivity.Close()

	if err := underActivity.Start(ctx, tconn); err != nil {
		return errors.Wrap(err, "failed to start new activity")
	}
	defer func() {
		if err := underActivity.Stop(ctxForStopUnderActivity, tconn); err != nil {
			if retErr == nil {
				retErr = errors.Wrap(err, "failed to stop activity")
			} else {
				testing.ContextLog(ctx, "Failed to stop activity")
			}
		}
	}()

	if err := wm.WaitUntilActivityIsReady(ctx, tconn, underActivity, d); err != nil {
		return errors.Wrap(err, "failed to wait until activity is ready")
	}

	overActivity, err := arc.NewActivity(a, extraPkgName, wm.ResizableUnspecifiedActivity)
	if err != nil {
		return err
	}
	defer overActivity.Close()

	if err := overActivity.Start(ctx, tconn); err != nil {
		return errors.Wrap(err, "failed to start new activity")
	}
	defer func() {
		if err := overActivity.Stop(ctxForStopOverActivity, tconn); err != nil {
			if retErr == nil {
				retErr = errors.Wrap(err, "failed to stop activity")
			} else {
				testing.ContextLog(ctx, "Failed to stop activity")
			}
		}
	}()

	if err := wm.WaitUntilActivityIsReady(ctx, tconn, overActivity, d); err != nil {
		return errors.Wrap(err, "failed to wait until activity is ready")
	}

	pdInfo, err := display.GetPrimaryInfo(ctx, tconn)
	if err != nil {
		return err
	}
	if pdInfo == nil {
		return errors.New("failed to find primary display info")
	}

	tew, err := input.Touchscreen(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to create touch screen event writer")
	}
	defer tew.Close()

	stw, err := tew.NewSingleTouchWriter()
	if err != nil {
		return errors.Wrap(err, "failed to create single touch screen writer")
	}
	defer stw.Close()

	tcc := tew.NewTouchCoordConverter(pdInfo.Bounds.Size())

	from := coords.NewPoint(0, pdInfo.WorkArea.Height/2)
	if isLandscape {
		from = coords.NewPoint(pdInfo.WorkArea.Width/2, 0)
	}
	to := coords.NewPoint(pdInfo.WorkArea.Width/2, pdInfo.WorkArea.Height/2)
	x0, y0 := tcc.ConvertLocation(from)
	x1, y1 := tcc.ConvertLocation(to)

	if err := stw.Swipe(ctx, x0, y0, x1, y1, 500*time.Millisecond); err != nil {
		return errors.Wrap(err, "failed to swipe")
	}

	to = coords.NewPoint(pdInfo.WorkArea.Width-10, pdInfo.WorkArea.Height/2)
	if isLandscape {
		to = coords.NewPoint(1, 1)
	}
	x2, y2 := tcc.ConvertLocation(to)

	if err := stw.Swipe(ctx, x1, y1, x2, y2, 500*time.Millisecond); err != nil {
		return errors.Wrap(err, "failed to swipe")
	}
	if err := stw.End(); err != nil {
		return errors.Wrap(err, "failed to finish the swipe gesture")
	}
	// Wait for over activity to snap to bottom/left.
	if err := testing.Sleep(ctx, waitForSnappingAnimation); err != nil {
		return errors.Wrap(err, "failed to sleep for sanpping animation to finish")
	}

	mid := coords.NewPoint(pdInfo.WorkArea.Width/4, pdInfo.WorkArea.Height/2)
	if isLandscape {
		mid = coords.NewPoint(pdInfo.WorkArea.Width*3/4, pdInfo.WorkArea.Height/2)
	}
	x, y := tcc.ConvertLocation(mid)
	if err := stw.Move(x, y); err != nil {
		return errors.Wrap(err, "failed to move touch")
	}
	if err := stw.End(); err != nil {
		return errors.Wrap(err, "failed to end touch")
	}

	// Wait for under activity to snap to top/right.
	if err := testing.Sleep(ctx, waitForSnappingAnimation); err != nil {
		return errors.Wrap(err, "failed to sleep for sanpping animation to finish")
	}

	underActivityWInfo, err := ash.GetARCAppWindowInfo(ctx, tconn, wm.Pkg24)
	if err != nil {
		return errors.Wrap(err, "failed to get arc app window info")
	}

	overActivityWInfo, err := ash.GetARCAppWindowInfo(ctx, tconn, extraPkgName)
	if err != nil {
		return errors.Wrap(err, "failed to get arc app window info")
	}

	if isLandscape {
		if overActivityWInfo.BoundsInRoot.Left != 0 ||
			overActivityWInfo.BoundsInRoot.Top != 0 ||
			overActivityWInfo.BoundsInRoot.Width >= pdInfo.WorkArea.Width/2 ||
			overActivityWInfo.BoundsInRoot.Height != pdInfo.WorkArea.Height {
			return errors.Wrap(err, "invalid snapped to the left activity bounds")
		}
		if underActivityWInfo.BoundsInRoot.Left <= pdInfo.WorkArea.Width/2 ||
			underActivityWInfo.BoundsInRoot.Top != 0 ||
			underActivityWInfo.BoundsInRoot.Width >= pdInfo.WorkArea.Width/2 ||
			underActivityWInfo.BoundsInRoot.Height != pdInfo.WorkArea.Height {
			return errors.Wrap(err, "invalid snapped to the right activity bounds")
		}
	} else {
		if overActivityWInfo.BoundsInRoot.Left != 0 ||
			overActivityWInfo.BoundsInRoot.Top <= pdInfo.WorkArea.Height/2 ||
			overActivityWInfo.BoundsInRoot.Width != pdInfo.WorkArea.Width ||
			overActivityWInfo.BoundsInRoot.Height >= pdInfo.WorkArea.Height/2 {
			return errors.Wrap(err, "invalid snapped to the bottom activity bounds")
		}
		if underActivityWInfo.BoundsInRoot.Left != 0 ||
			underActivityWInfo.BoundsInRoot.Top != 0 ||
			underActivityWInfo.BoundsInRoot.Width != pdInfo.WorkArea.Width ||
			underActivityWInfo.BoundsInRoot.Height >= pdInfo.WorkArea.Height/2 {
			return errors.Wrap(err, "invalid snapped to the top activity bounds")

		}
	}

	return nil
}

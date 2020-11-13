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
	extraPkgName              = "org.chromium.arc.testapp.windowmanager24.inmaximizedlist"
	extraApkPath              = "ArcWMTestApp_24_InMaximizedList.apk"
	timeReservedForStop       = 500 * time.Millisecond
	rotationAnimationDuration = 750 * time.Millisecond
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
	ctxForStopOverActivity := ctx
	ctx, cancelForStopOverActivity := ctxutil.Shorten(ctx, timeReservedForStop)
	defer cancelForStopOverActivity()

	ctxForStopUnderActivity := ctx
	ctx, cancelForStopUnderActivity := ctxutil.Shorten(ctx, timeReservedForStop)
	defer cancelForStopUnderActivity()

	underActivity, err := arc.NewActivity(a, wm.Pkg24, wm.ResizableUnspecifiedActivity)
	if err != nil {
		return errors.Wrap(err, "failed to create under activity")
	}
	defer underActivity.Close()

	if err := underActivity.Start(ctx, tconn); err != nil {
		return errors.Wrap(err, "failed to start under activity")
	}
	defer func() {
		if err := underActivity.Stop(ctxForStopUnderActivity, tconn); err != nil {
			if retErr == nil {
				retErr = errors.Wrap(err, "failed to stop under activity")
			} else {
				testing.ContextLog(ctx, "Failed to stop under activity: ", err)
			}
		}
	}()

	if err := wm.WaitUntilActivityIsReady(ctx, tconn, underActivity, d); err != nil {
		return errors.Wrap(err, "failed to wait until under activity is ready")
	}
	if err := a.Install(ctx, arc.APKPath(extraApkPath)); err != nil {
		return errors.Wrap(err, "failed to install extra APK")
	}
	overActivity, err := arc.NewActivity(a, extraPkgName, wm.ResizableUnspecifiedActivity)
	if err != nil {
		return err
	}
	defer overActivity.Close()

	if err := overActivity.Start(ctx, tconn); err != nil {
		return errors.Wrap(err, "failed to start over activity")
	}
	defer func() {
		if err := overActivity.Stop(ctxForStopOverActivity, tconn); err != nil {
			if retErr == nil {
				retErr = errors.Wrap(err, "failed to stop over activity")
			} else {
				testing.ContextLog(ctx, "Failed to stop over activity: ", err)
			}
		}
	}()

	if err := wm.WaitUntilActivityIsReady(ctx, tconn, overActivity, d); err != nil {
		return errors.Wrap(err, "failed to wait until over activity is ready")
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
	// Swipe from the top-center of the screen.
	from := coords.NewPoint(pdInfo.WorkArea.Width/2, 0)
	// To the center of the screen.
	to := coords.NewPoint(pdInfo.WorkArea.Width/2, pdInfo.WorkArea.Height/2)

	x0, y0 := tcc.ConvertLocation(from)
	x1, y1 := tcc.ConvertLocation(to)

	if err := stw.Swipe(ctx, x0, y0, x1, y1, 500*time.Millisecond); err != nil {
		return errors.Wrap(err, "failed to swipe")
	}

	// Drag the activity to the top-left corner.
	to = coords.NewPoint(1, 1)
	x2, y2 := tcc.ConvertLocation(to)

	if err := stw.Swipe(ctx, x1, y1, x2, y2, 500*time.Millisecond); err != nil {
		return errors.Wrap(err, "failed to swipe")
	}
	if err := stw.End(); err != nil {
		return errors.Wrap(err, "failed to finish the swipe gesture")
	}

	overActivityWInfo, err := ash.GetARCAppWindowInfo(ctx, tconn, extraPkgName)
	if err != nil {
		return errors.Wrap(err, "failed to get arc app window info for over activity")
	}

	testing.Poll(ctx, func(ctx context.Context) error {
		overActivityWInfo, err := ash.GetARCAppWindowInfo(ctx, tconn, extraPkgName)
		if err != nil {
			return testing.PollBreak(errors.Wrap(err, "failed to get arc app window info for over activity"))
		}

		if overActivityWInfo.State != ash.WindowStateLeftSnapped {
			return errors.Errorf("invalid window state, got: %q, want: LeftSnapped", overActivityWInfo.State)
		}
		return nil
	}, &testing.PollOptions{Timeout: 5 * time.Second})

	overActivityWInfo, err = ash.GetARCAppWindowInfo(ctx, tconn, extraPkgName)
	if err != nil {
		return errors.Wrap(err, "failed to get arc app window info for over activity")
	}

	// Touch the center of the right half of the screen to snap under activity to the right.
	mid := coords.NewPoint(pdInfo.WorkArea.Width*3/4, pdInfo.WorkArea.Height/2)
	x, y := tcc.ConvertLocation(mid)
	if err := stw.Move(x, y); err != nil {
		return errors.Wrap(err, "failed to move touch")
	}
	if err := stw.End(); err != nil {
		return errors.Wrap(err, "failed to end touch")
	}

	testing.Poll(ctx, func(ctx context.Context) error {
		underActivityWInfo, err := ash.GetARCAppWindowInfo(ctx, tconn, wm.Pkg24)
		if err != nil {
			return testing.PollBreak(errors.Wrap(err, "failed to get arc app window info for under activity"))
		}

		if underActivityWInfo.State != ash.WindowStateRightSnapped {
			return errors.Errorf("invalid window state, got: %q, want: RightSnapped", overActivityWInfo.State)
		}
		return nil
	}, &testing.PollOptions{Timeout: 5 * time.Second})

	overActivityWInfo, err = ash.GetARCAppWindowInfo(ctx, tconn, extraPkgName)
	if err != nil {
		return errors.Wrap(err, "failed to get arc app window info for over activity")
	}

	underActivityWInfo, err := ash.GetARCAppWindowInfo(ctx, tconn, wm.Pkg24)
	if err != nil {
		return errors.Wrap(err, "failed to get arc app window info for under activity")
	}

	if overActivityWInfo.BoundsInRoot.Left != 0 ||
		overActivityWInfo.BoundsInRoot.Top != 0 ||
		overActivityWInfo.BoundsInRoot.Width >= pdInfo.WorkArea.Width/2 ||
		overActivityWInfo.BoundsInRoot.Height != pdInfo.WorkArea.Height {
		return errors.Errorf("invalid snapped to the left activity bounds before rotation, got: Left = %d, Top = %d, Width = %d, Height = %d; want: Left = 0, Top = 0, Width < %d, Height = %d",
			overActivityWInfo.BoundsInRoot.Left, overActivityWInfo.BoundsInRoot.Top, overActivityWInfo.BoundsInRoot.Width, overActivityWInfo.BoundsInRoot.Height, pdInfo.WorkArea.Width/2, pdInfo.WorkArea.Height)
	}
	if underActivityWInfo.BoundsInRoot.Left <= pdInfo.WorkArea.Width/2 ||
		underActivityWInfo.BoundsInRoot.Top != 0 ||
		underActivityWInfo.BoundsInRoot.Width >= pdInfo.WorkArea.Width/2 ||
		underActivityWInfo.BoundsInRoot.Height != pdInfo.WorkArea.Height {
		return errors.Errorf("invalid snapped to the right activity bounds before rotation, got: Left = %d, Top = %d, Width = %d, Height = %d; want: Left > %d, Top = 0, Width < %d, Height = %d",
			underActivityWInfo.BoundsInRoot.Left, underActivityWInfo.BoundsInRoot.Top, underActivityWInfo.BoundsInRoot.Width, underActivityWInfo.BoundsInRoot.Height, pdInfo.WorkArea.Width/2, pdInfo.WorkArea.Width/2, pdInfo.WorkArea.Height)

	}

	cleanupRotation, err := wm.RotateDisplay(ctx, tconn, display.Rotate270)
	if err != nil {
		return err
	}
	defer cleanupRotation()

	if err := testing.Sleep(ctx, rotationAnimationDuration); err != nil {
		return errors.Wrap(err, "failed to sleep for rotation to portrait animation to finish")
	}

	underActivityWInfo, err = ash.GetARCAppWindowInfo(ctx, tconn, wm.Pkg24)
	if err != nil {
		return errors.Wrap(err, "failed to get arc app window info for under activity in portrait mode")
	}

	overActivityWInfo, err = ash.GetARCAppWindowInfo(ctx, tconn, extraPkgName)
	if err != nil {
		return errors.Wrap(err, "failed to get arc app window info for over activity in portrait mode")
	}

	if underActivityWInfo.BoundsInRoot.Left != 0 ||
		underActivityWInfo.BoundsInRoot.Top <= pdInfo.WorkArea.Height/2 ||
		underActivityWInfo.BoundsInRoot.Width != pdInfo.WorkArea.Width ||
		underActivityWInfo.BoundsInRoot.Height >= pdInfo.WorkArea.Height/2 {
		return errors.Errorf("invalid snapped to the bottom activity bounds in portrait mode, got: Left = %d, Top = %d, Width = %d, Height = %d; want: Left = 0, Top > %d, Width = %d, Height < %d",
			underActivityWInfo.BoundsInRoot.Left, underActivityWInfo.BoundsInRoot.Top, underActivityWInfo.BoundsInRoot.Width, underActivityWInfo.BoundsInRoot.Height, pdInfo.WorkArea.Height/2, pdInfo.WorkArea.Width, pdInfo.WorkArea.Height/2)
	}
	if overActivityWInfo.BoundsInRoot.Left != 0 ||
		overActivityWInfo.BoundsInRoot.Top != 0 ||
		overActivityWInfo.BoundsInRoot.Width != pdInfo.WorkArea.Width ||
		overActivityWInfo.BoundsInRoot.Height >= pdInfo.WorkArea.Height/2 {
		return errors.Errorf("invalid snapped to the top activity bounds in portrait mode, got: Left = %d, Top = %d, Width = %d, Height = %d; want: Left = 0, Top = 0, Width = %d, Height < %d",
			overActivityWInfo.BoundsInRoot.Left, overActivityWInfo.BoundsInRoot.Top, overActivityWInfo.BoundsInRoot.Width, overActivityWInfo.BoundsInRoot.Height, pdInfo.WorkArea.Width, pdInfo.WorkArea.Height/2)
	}

	cleanupRotation, err = wm.RotateDisplay(ctx, tconn, display.Rotate0)
	if err != nil {
		return err
	}
	defer cleanupRotation()

	if err := testing.Sleep(ctx, rotationAnimationDuration); err != nil {
		return errors.Wrap(err, "failed to sleep for rotation to landscape animation to finish")
	}

	underActivityWInfo, err = ash.GetARCAppWindowInfo(ctx, tconn, wm.Pkg24)
	if err != nil {
		return errors.Wrap(err, "failed to get arc app window info for under activity in landscape mode")
	}

	overActivityWInfo, err = ash.GetARCAppWindowInfo(ctx, tconn, extraPkgName)
	if err != nil {
		return errors.Wrap(err, "failed to get arc app window info for over activity in landscape mode")
	}
	if overActivityWInfo.BoundsInRoot.Left != 0 ||
		overActivityWInfo.BoundsInRoot.Top != 0 ||
		overActivityWInfo.BoundsInRoot.Width >= pdInfo.WorkArea.Width/2 ||
		overActivityWInfo.BoundsInRoot.Height != pdInfo.WorkArea.Height {
		return errors.Errorf("invalid snapped to the left activity bounds after rotation in landscape mode, got: Left = %d, Top = %d, Width = %d, Height = %d; want: Left = 0, Top = 0, Width < %d, Height = %d",
			overActivityWInfo.BoundsInRoot.Left, overActivityWInfo.BoundsInRoot.Top, overActivityWInfo.BoundsInRoot.Width, overActivityWInfo.BoundsInRoot.Height, pdInfo.WorkArea.Width/2, pdInfo.WorkArea.Height)
	}
	if underActivityWInfo.BoundsInRoot.Left <= pdInfo.WorkArea.Width/2 ||
		underActivityWInfo.BoundsInRoot.Top != 0 ||
		underActivityWInfo.BoundsInRoot.Width >= pdInfo.WorkArea.Width/2 ||
		underActivityWInfo.BoundsInRoot.Height != pdInfo.WorkArea.Height {
		return errors.Errorf("invalid snapped to the right activity bounds after rotation in landscape mode, got: Left = %d, Top = %d, Width = %d, Height = %d; want: Left > %d, Top = 0, Width < %d, Height = %d",
			underActivityWInfo.BoundsInRoot.Left, underActivityWInfo.BoundsInRoot.Top, underActivityWInfo.BoundsInRoot.Width, underActivityWInfo.BoundsInRoot.Height, pdInfo.WorkArea.Width/2, pdInfo.WorkArea.Width/2, pdInfo.WorkArea.Height)
	}

	return nil
}

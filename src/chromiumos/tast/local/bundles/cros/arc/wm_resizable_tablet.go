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

func init() {
	testing.AddTest(&testing.Test{
		Func:         WMResizableTablet,
		Desc:         "Verifies that Window Manager resizable tablet use-cases behave as described in go/arc-wm-r",
		Contacts:     []string{"armenk@google.com", "arc-framework+tast@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"android_vm", "chrome", "tablet_mode"},
		Fixture:      "arcBooted",
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
	// Test Steps:
	// This test starts at landscape mode.
	// 1- Start the first activity - Under activity.
	// 2- Start the second activity - over activity.
	// 3- Snap the over activity to the left:
	//   3-1- Swipe the over activity from top-center to the middle of the screen.
	//   3-2- Continue swiping the over activity from the middle of the screen to the top-left corner.
	//   3-3- Make sure the over activity is snapped to the left.
	// 4- Snap the under activity to the right:
	//   4-1- Touch the middle of the right half of the screen.
	//   4-2- Make sure the under activity is snapped to the right.
	// 5- Get app window info for assertions:
	//   5-1- Over activity must be snapped to the left.
	//   5-2- Under activity must be snapped to the right.
	// 6- Rotate the screen by 270 degrees - to portrait mode.
	// 7- Get app window info for assertions:
	//   7-1- Over activity must be snapped to the top.
	//   7-2- Under activity must be snapped to the bottom.
	// 8- Rotate the screen to 0 degrees - Back to landscape mode.
	// 9- Get app window info for assertions:
	//   9-1- Over activity must be snapped to the left.
	//   9-2- Under activity must be snapped to the right.

	ctxForStopOverActivity := ctx
	ctx, cancelForStopOverActivity := ctxutil.Shorten(ctx, wm.TimeReservedForStop)
	defer cancelForStopOverActivity()

	ctxForStopUnderActivity := ctx
	ctx, cancelForStopUnderActivity := ctxutil.Shorten(ctx, wm.TimeReservedForStop)
	defer cancelForStopUnderActivity()

	underActivity, err := arc.NewActivity(a, wm.Pkg24, wm.ResizableUnspecifiedActivity)
	if err != nil {
		return errors.Wrap(err, "failed to create under activity")
	}
	defer underActivity.Close()

	// 1- Start the first activity - Under activity.
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
	if err := a.Install(ctx, arc.APKPath(wm.APKNameArcWMTestApp24Secondary)); err != nil {
		return errors.Wrap(err, "failed to install extra APK")
	}
	overActivity, err := arc.NewActivity(a, wm.Pkg24Secondary, wm.ResizableUnspecifiedActivity)
	if err != nil {
		return err
	}
	defer overActivity.Close()

	// 2- Start the second activity - over activity.
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

	// 3- Snap the over activity to the left - Start.
	from := coords.NewPoint(pdInfo.WorkArea.Width/2, 0)
	to := coords.NewPoint(pdInfo.WorkArea.Width/2, pdInfo.WorkArea.Height/2)

	x0, y0 := tcc.ConvertLocation(from)
	x1, y1 := tcc.ConvertLocation(to)

	//   3-1- Swipe the over activity from top-center to the middle of the screen.
	if err := stw.Swipe(ctx, x0, y0, x1, y1, 500*time.Millisecond); err != nil {
		return errors.Wrap(err, "failed to swipe")
	}

	//   3-2- Continue swiping the over activity from the middle of the screen to the top-left corner.
	to = coords.NewPoint(1, 1)
	x2, y2 := tcc.ConvertLocation(to)

	if err := stw.Swipe(ctx, x1, y1, x2, y2, 500*time.Millisecond); err != nil {
		return errors.Wrap(err, "failed to swipe")
	}
	if err := stw.End(); err != nil {
		return errors.Wrap(err, "failed to finish the swipe gesture")
	}

	overActivityWInfo, err := ash.GetARCAppWindowInfo(ctx, tconn, wm.Pkg24Secondary)
	if err != nil {
		return errors.Wrap(err, "failed to get arc app window info for over activity")
	}

	//   3-3- Make sure the over activity is snapped to the left.
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		overActivityWInfo, err := ash.GetARCAppWindowInfo(ctx, tconn, wm.Pkg24Secondary)
		if err != nil {
			return testing.PollBreak(errors.Wrap(err, "failed to get arc app window info for over activity"))
		}

		if overActivityWInfo.State != ash.WindowStateLeftSnapped {
			return errors.Errorf("invalid window state, got: %q, want: LeftSnapped", overActivityWInfo.State)
		}
		return nil
	}, &testing.PollOptions{Timeout: 5 * time.Second}); err != nil {
		return err
	}
	// 3- Snap the over activity to the left - End.

	// 4- Snap the under activity to the right - Start.
	overActivityWInfo, err = ash.GetARCAppWindowInfo(ctx, tconn, wm.Pkg24Secondary)
	if err != nil {
		return errors.Wrap(err, "failed to get arc app window info for over activity")
	}

	mid := coords.NewPoint(pdInfo.WorkArea.Width*3/4, pdInfo.WorkArea.Height/2)
	x, y := tcc.ConvertLocation(mid)

	//   4-1- Touch the middle of the right half of the screen.
	if err := stw.Move(x, y); err != nil {
		return errors.Wrap(err, "failed to move touch")
	}
	if err := stw.End(); err != nil {
		return errors.Wrap(err, "failed to end touch")
	}

	//   4-2- Make sure the under activity is snapped to the right.
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		underActivityWInfo, err := ash.GetARCAppWindowInfo(ctx, tconn, wm.Pkg24)
		if err != nil {
			return testing.PollBreak(errors.Wrap(err, "failed to get arc app window info for under activity"))
		}

		if underActivityWInfo.State != ash.WindowStateRightSnapped {
			return errors.Errorf("invalid window state, got: %q, want: RightSnapped", overActivityWInfo.State)
		}
		return nil
	}, &testing.PollOptions{Timeout: 5 * time.Second}); err != nil {
		return err
	}
	// 4- Snap the under activity to the right - End.

	// 5- Get app window info for assertions. Over activity must be snapped to the left and under activity to the right.
	if err := wm.CheckVerticalTabletSplit(ctx, tconn, pdInfo.WorkArea); err != nil {
		return errors.Wrap(err, "failed to assert vertical split window bounds before rotation")
	}

	// 6- Rotate the screen by 270 degrees - to portrait mode.
	cleanupRotation, err := wm.RotateDisplay(ctx, tconn, display.Rotate270)
	if err != nil {
		return err
	}
	defer cleanupRotation()

	if err := testing.Sleep(ctx, wm.RotationAnimationDuration); err != nil {
		return errors.Wrap(err, "failed to sleep for rotation to portrait animation to finish")
	}

	// 7- Get app window info for assertions. Over activity must be snapped to the top and under activity to the bottom.
	if err := wm.CheckHorizontalTabletSplit(ctx, tconn, pdInfo.WorkArea); err != nil {
		return errors.Wrap(err, "failed to assert horizontal split window bounds in portrait mode")
	}

	// 8- Rotate the screen to 0 degrees - Back to landscape mode.
	cleanupRotation, err = wm.RotateDisplay(ctx, tconn, display.Rotate0)
	if err != nil {
		return err
	}
	defer cleanupRotation()

	if err := testing.Sleep(ctx, wm.RotationAnimationDuration); err != nil {
		return errors.Wrap(err, "failed to sleep for rotation to landscape animation to finish")
	}

	// 9- Get app window info for assertions. Over activity must be snapped to the left and under activity to the right.
	if err := wm.CheckVerticalTabletSplit(ctx, tconn, pdInfo.WorkArea); err != nil {
		return errors.Wrap(err, "failed to assert vertical split window bounds after rotation")
	}

	return nil
}

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
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         WMResizableTablet,
		Desc:         "Verifies that Window Manager resizable tablet use-cases behave as described in go/arc-wm-r",
		Contacts:     []string{"armenk@google.com", "arc-framework+tast@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"android_vm", "chrome"},
		HardwareDeps: hwdep.D(hwdep.InternalDisplay()),
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

	if err := wm.TabletShelfHideShowHelper(ctx, tconn, a, d, luActivities, wm.CheckMaximizeNonResizable); err != nil {
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

	return wm.TabletShelfHideShowHelper(ctx, tconn, a, d, puActivities, wm.CheckMaximizeNonResizable)
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
	ctx, cancelForStopOverActivity := ctxutil.Shorten(ctx, wm.TimeReservedForStop)
	defer cancelForStopOverActivity()

	ctxForStopUnderActivity := ctx
	ctx, cancelForStopUnderActivity := ctxutil.Shorten(ctx, wm.TimeReservedForStop)
	defer cancelForStopUnderActivity()

	cleanupRotation, err := wm.RotateToLandscape(ctx, tconn)
	if err != nil {
		return err
	}
	defer cleanupRotation()

	underActivity, err := arc.NewActivity(a, wm.Pkg24, wm.ResizableUnspecifiedActivity)
	if err != nil {
		return errors.Wrap(err, "failed to create under activity")
	}
	defer underActivity.Close()

	// Start the first activity - Under activity.
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

	// Start the second activity - over activity.
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

	overActivityWInfo, err := ash.GetARCAppWindowInfo(ctx, tconn, wm.Pkg24Secondary)
	if err != nil {
		return errors.Wrap(err, "failed to get arc app window info for over activity")
	}

	if _, err := ash.SetARCAppWindowState(ctx, tconn, wm.Pkg24Secondary, ash.WMEventSnapLeft); err != nil {
		return errors.Wrapf(err, "failed to left snap %s", wm.Pkg24Secondary)
	}

	//  Make sure the over activity is snapped to the left.
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
	// Snap the over activity to the left - End.

	// Snap the under activity to the right - Start.
	if _, err := ash.SetARCAppWindowState(ctx, tconn, wm.Pkg24, ash.WMEventSnapRight); err != nil {
		return errors.Wrapf(err, "failed to right snap %s", wm.Pkg24)
	}

	if err := testing.Poll(ctx, func(ctx context.Context) error {
		// Make sure the under activity is snapped to the right.
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
	// Snap the under activity to the right - End.

	// Get app window info for assertions. Over activity must be snapped to the left and under activity to the right.
	if err := wm.CheckVerticalTabletSplit(ctx, tconn); err != nil {
		return errors.Wrap(err, "failed to assert vertical split window bounds before rotation")
	}

	// Rotate the screen by 270 degrees - to portrait mode.
	cleanupRotation, err = wm.RotateDisplay(ctx, tconn, display.Rotate270)
	if err != nil {
		return err
	}
	defer cleanupRotation()

	// Get app window info for assertions. Over activity must be snapped to the top and under activity to the bottom.
	if err := wm.CheckHorizontalTabletSplit(ctx, tconn); err != nil {
		return errors.Wrap(err, "failed to assert horizontal split window bounds in portrait mode")
	}

	// Rotate the screen to 0 degrees - Back to landscape mode.
	cleanupRotation, err = wm.RotateDisplay(ctx, tconn, display.Rotate0)
	if err != nil {
		return err
	}
	defer cleanupRotation()

	if err := wm.CheckVerticalTabletSplit(ctx, tconn); err != nil {
		return errors.Wrap(err, "failed to assert vertical split window bounds after rotation")
	}

	return nil
}

// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"time"

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
		Func:         WMResizableConversion,
		Desc:         "Verifies that Window Manager resizable/conversion use-cases behaves as described in go/arc-wm-r",
		Contacts:     []string{"armenk@google.com", "arc-framework+tast@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"android_vm", "chrome", "tablet_mode"},
		Fixture:      "arcBooted",
		Timeout:      8 * time.Minute,
	})
}

func WMResizableConversion(ctx context.Context, s *testing.State) {
	wm.SetupAndRunTestCases(ctx, s, false, []wm.TestCase{
		wm.TestCase{
			// resizable/conversion: landscape
			Name: "RV_conversion_landscape",
			Func: wmRV19,
		},
		wm.TestCase{
			// resizable/conversion: portrait
			Name: "RV_conversion_portrait",
			Func: wmRV20,
		},
		wm.TestCase{
			// resizable/conversion: undefined orientation
			Name: "RV_undefined_orientation",
			Func: wmRV21,
		},
		wm.TestCase{
			// resizable/conversion: split screen
			Name: "RV_split_screen",
			Func: wmRV22,
		},
	})
}

// wmRV19 covers resizable/conversion behavior in landscape mode.
// Expected behavior is defined in: go/arc-wm-r RV19 resizable/conversion: landscape.
func wmRV19(ctx context.Context, tconn *chrome.TestConn, a *arc.ARC, d *ui.Device) error {
	act, err := arc.NewActivity(a, wm.Pkg24, wm.ResizableLandscapeActivity)
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

	// Store activity's original window info to be compared with after tablet mode disabled.
	windowInfoBeforeTabletMode, err := ash.GetARCAppWindowInfo(ctx, tconn, wm.Pkg24)
	if err != nil {
		return nil
	}

	if err := wm.CheckRestoreResizable(ctx, tconn, act, d); err != nil {
		return err
	}

	// windowID will be used to wait on certain conditions.
	windowID := windowInfoBeforeTabletMode.ID

	// Enable tablet mode, the activity should go to Maximized state.
	cleanup, err := ash.EnsureTabletModeEnabled(ctx, tconn, true)
	if err != nil {
		return errors.Wrap(err, "failed to ensure if tablet mode is enabled")
	}
	defer cleanup(ctx)

	if err := ash.WaitForARCAppWindowState(ctx, tconn, wm.Pkg24, ash.WindowStateMaximized); err != nil {
		return err
	}
	if err := ash.WaitWindowFinishAnimating(ctx, tconn, windowID); err != nil {
		return err
	}
	if err := ash.WaitForCondition(ctx, tconn, func(w *ash.Window) bool {
		return w.ID == windowID && w.IsFrameVisible == false
	}, &testing.PollOptions{Timeout: 5 * time.Second}); err != nil {
		return errors.Wrap(err, "failed to wait for frame to get hidden")
	}

	// Store activity's window info when tablet mode is enabled to make sure it is in Maximized state.
	windowInfoAtTabletMode, err := ash.GetARCAppWindowInfo(ctx, tconn, wm.Pkg24)
	if err != nil {
		return err
	}

	// Compare activity's window TargetBounds to primary display work area.
	if err := wm.CheckMaximizeWindowInTabletMode(ctx, tconn, *windowInfoAtTabletMode); err != nil {
		return err
	}

	// Disable tablet mode.
	if err := ash.SetTabletModeEnabled(ctx, tconn, false); err != nil {
		return errors.Wrap(err, "failed to disable tablet mode")
	}

	if err := ash.WaitForARCAppWindowState(ctx, tconn, wm.Pkg24, ash.WindowStateNormal); err != nil {
		return errors.Wrap(err, "failed to wait for the window to be normal state")
	}
	if err := ash.WaitWindowFinishAnimating(ctx, tconn, windowID); err != nil {
		return err
	}
	if err := ash.WaitForCondition(ctx, tconn, func(w *ash.Window) bool {
		return w.ID == windowID && w.IsFrameVisible == true
	}, &testing.PollOptions{Timeout: 5 * time.Second}); err != nil {
		return errors.Wrap(err, "failed to wait for frame to become visible")
	}

	windowInfoAfterTabletMode, err := ash.GetARCAppWindowInfo(ctx, tconn, wm.Pkg24)
	if err != nil {
		return err
	}

	if err := wm.CheckRestoreResizable(ctx, tconn, act, d); err != nil {
		return err
	}

	// Activity should have same TargetBounds that it had before enabling tablet mode.
	if windowInfoBeforeTabletMode.TargetBounds != windowInfoAfterTabletMode.TargetBounds {
		return errors.Errorf("failed to retrieve original window bounds after switching back from tablet mode, got: %s, want: %s", windowInfoAfterTabletMode.TargetBounds, windowInfoBeforeTabletMode.TargetBounds)
	}

	return nil
}

// wmRV20 covers resizable/conversion behavior in portrait mode.
// Expected behavior is defined in: go/arc-wm-r RV20 resizable/conversion: portrait.
func wmRV20(ctx context.Context, tconn *chrome.TestConn, a *arc.ARC, d *ui.Device) error {
	// Store original display orientation.
	oDO, err := display.GetOrientation(ctx, tconn)
	if err != nil {
		return err
	}

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

	// Store activity's original window info to be compared with after tablet mode disabled.
	ow, err := ash.GetARCAppWindowInfo(ctx, tconn, wm.Pkg24)
	if err != nil {
		return nil
	}

	if err := wm.CheckRestoreResizable(ctx, tconn, act, d); err != nil {
		return err
	}

	// window id will be used to wait on certain conditions.
	wID := ow.ID

	// Enable tablet mode, the activity should go to Maximized state.
	cleanup, err := ash.EnsureTabletModeEnabled(ctx, tconn, true)
	if err != nil {
		return errors.Wrap(err, "failed to ensure if tablet mode is enabled")
	}
	defer cleanup(ctx)

	if err := ash.WaitForARCAppWindowState(ctx, tconn, wm.Pkg24, ash.WindowStateMaximized); err != nil {
		return err
	}
	if err := ash.WaitWindowFinishAnimating(ctx, tconn, wID); err != nil {
		return err
	}
	if err := ash.WaitForCondition(ctx, tconn, func(w *ash.Window) bool {
		return w.ID == wID && w.IsFrameVisible == false
	}, &testing.PollOptions{Timeout: 5 * time.Second}); err != nil {
		return errors.Wrap(err, "failed to wait for frame to get hidden")
	}

	// Store activity's window info when tablet mode is enabled to make sure it is in Maximized state.
	wt, err := ash.GetARCAppWindowInfo(ctx, tconn, wm.Pkg24)
	if err != nil {
		return err
	}

	// Compare activity's window TargetBounds to primary display work area.
	if err := wm.CheckMaximizeWindowInTabletMode(ctx, tconn, *wt); err != nil {
		return err
	}

	// Get display orientation in tablet mode.
	tDO, err := display.GetOrientation(ctx, tconn)
	if err != nil {
		return err
	}
	if tDO.Type != display.OrientationPortraitPrimary {
		return errors.Errorf("invalid display orientation in tablet mode, got: %q, want: %q", tDO.Type, display.OrientationPortraitPrimary)
	}

	// Disable tablet mode.
	if err := ash.SetTabletModeEnabled(ctx, tconn, false); err != nil {
		return errors.Wrap(err, "failed to disable tablet mode")
	}
	if err := ash.WaitForCondition(ctx, tconn, func(w *ash.Window) bool {
		return w.ID == wID && w.IsFrameVisible == true
	}, &testing.PollOptions{Timeout: 5 * time.Second}); err != nil {
		return errors.Wrap(err, "failed to wait for frame to become visible")
	}
	if err := ash.WaitForARCAppWindowState(ctx, tconn, wm.Pkg24, ash.WindowStateMaximized); err != nil {
		return err
	}
	if err := ash.WaitWindowFinishAnimating(ctx, tconn, wID); err != nil {
		return err
	}

	// Get display orientaiton after switching to clamshell mode.
	cDO, err := display.GetOrientation(ctx, tconn)
	if err != nil {
		return err
	}
	if cDO.Type != oDO.Type {
		return errors.Errorf("invalid display orientation after switching back to clamshell, got: %q, want: %q", cDO.Type, oDO.Type)
	}

	return nil

	// TODO(b/162387612): After the bug is fixed, compare w
	// return testing.Poll(ctx, func(ctx context.Context) error {
	// 	windowInfoAfterTabletMode, err := ash.GetARCAppWindowInfo(ctx, tconn, Pkg24)
	// 	if err != nil {
	// 		return testing.PollBreak(err)
	// 	}
	// 	if err := assertFunc(ctx, tconn, act, d); err != nil {
	// 		return testing.PollBreak(err)
	// 	}
	// 	// Activity should have same TargetBounds that it had before enabling tablet mode.
	// 	if windowInfoBeforeTabletMode.TargetBounds != windowInfoAfterTabletMode.TargetBounds {
	// 		return errors.Errorf("failed to retrieve original window bounds after switching back from tablet mode, got: %s, want: %s", windowInfoAfterTabletMode.TargetBounds, windowInfoBeforeTabletMode.TargetBounds)
	// 	}
	// 	return nil
	// }, &testing.PollOptions{Timeout: 5 * time.Second})}
}

// wmRV21 covers resizable/conversion undefined orientation.
// Expected behavior is defined in: go/arc-wm-r RV21 resizable/conversion: undefined orientation.
func wmRV21(ctx context.Context, tconn *chrome.TestConn, a *arc.ARC, d *ui.Device) error {
	// Start an unspecified activity.
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
		return errors.Wrap(err, "failed to get original ARC app window info")
	}
	wID := owInfo.ID

	// Enable tablet mode.
	cleanup, err := ash.EnsureTabletModeEnabled(ctx, tconn, true)
	if err != nil {
		return errors.Wrap(err, "failed to ensure if tablet mode is enabled")
	}
	defer func(ctx context.Context) {
		cleanup(ctx)
	}(ctx)

	if err := wm.WaitForARCAppWindowState(ctx, tconn, ash.WindowStateMaximized, wID, false); err != nil {
		return errors.Wrap(err, "failed to wait for ARC app window state to change to maximized 1")
	}

	if err := testing.Poll(ctx, func(ctx context.Context) error {
		// Get activity's window info in landscape tablet mode to make sure it is in Maximized state.
		lwInfo, err := ash.GetARCAppWindowInfo(ctx, tconn, wm.Pkg24)
		if err != nil {
			return testing.PollBreak(err)
		}
		// Compare activity bounds to make sure it covers the primary display work area.
		if err := wm.CheckMaximizeWindowInTabletMode(ctx, tconn, *lwInfo); err != nil {
			return errors.Wrap(err, "failed to check maximize window in tablet mode")
		}
		return nil
	}, &testing.PollOptions{Timeout: 5 * time.Second}); err != nil {
		return err
	}

	// Get display orientation in tablet mode.
	tDO, err := display.GetOrientation(ctx, tconn)
	if err != nil {
		return errors.Wrap(err, "failed to get display orientation")
	}

	// Display should be landscape (default).
	if tDO.Type != display.OrientationLandscapePrimary {
		return errors.Errorf("invalid display orientation in tablet mode; got: %q, want: landscape-primary", tDO.Type)
	}

	// Disable tablet mode.
	if err := ash.SetTabletModeEnabled(ctx, tconn, false); err != nil {
		return errors.Wrap(err, "failed to disable tablet mode")
	}

	if err := wm.WaitForARCAppWindowState(ctx, tconn, ash.WindowStateMaximized, wID, true); err != nil {
		return errors.Wrap(err, "failed to wait for ARC app window state to change to maximized 2")
	}
	if err := ash.WaitWindowFinishAnimating(ctx, tconn, wID); err != nil {
		return errors.Wrap(err, "failed to wait finishing animating")
	}

	if err := testing.Poll(ctx, func(ctx context.Context) error {
		// Get activity's window info after switching back from tablet mode.
		alwInfo, err := ash.GetARCAppWindowInfo(ctx, tconn, wm.Pkg24)
		if err != nil {
			return testing.PollBreak(err)
		}
		// Activity bounds should be equal to the original bounds.
		if alwInfo.BoundsInRoot != owInfo.BoundsInRoot {
			return errors.Errorf("invalid window bounds after switching back from landscape tablet mode; got: %q, want: %q", alwInfo.BoundsInRoot, owInfo.BoundsInRoot)
		}
		return nil
	}, &testing.PollOptions{Timeout: 5 * time.Second}); err != nil {
		return err
	}

	// Enable tablet mode.
	if err := ash.SetTabletModeEnabled(ctx, tconn, true); err != nil {
		return errors.Wrap(err, "failed to enable tablet mode")
	}

	if err := wm.WaitForARCAppWindowState(ctx, tconn, ash.WindowStateMaximized, wID, false); err != nil {
		return errors.Wrap(err, "failed to wait for ARC app window state to change to maximized 3")
	}

	// Rotate the screen 270 degree.
	cleanupRotation, err := wm.RotateDisplay(ctx, tconn, display.Rotate270)
	if err != nil {
		return errors.Wrap(err, "failed to rotate the display by 270 degrees")
	}
	defer cleanupRotation()

	// Display should be portrait.
	// Wait until display rotates to portrait-primary orientation.
	if err := wm.WaitForDisplayOrientation(ctx, tconn, display.OrientationPortraitPrimary); err != nil {
		return errors.Wrap(err, "failed to wait for display orientation")
	}

	if err := testing.Poll(ctx, func(ctx context.Context) error {
		// Get activity's window info in portrait tablet mode to make sure it is in Maximized state.
		pwInfo, err := ash.GetARCAppWindowInfo(ctx, tconn, wm.Pkg24)
		if err != nil {
			return testing.PollBreak(err)
		}
		// Compare activity bounds to make sure it covers the primary display work area.
		if err := wm.CheckMaximizeWindowInTabletMode(ctx, tconn, *pwInfo); err != nil {
			return errors.Wrap(err, "failed to check maximize window in tablet mode")
		}
		return nil
	}, &testing.PollOptions{Timeout: 5 * time.Second}); err != nil {
		return err
	}

	// Disable tablet mode.
	if err := ash.SetTabletModeEnabled(ctx, tconn, false); err != nil {
		return errors.Wrap(err, "failed to disable tablet mode")
	}

	if err := wm.WaitForARCAppWindowState(ctx, tconn, ash.WindowStateMaximized, wID, true); err != nil {
		return errors.Wrap(err, "failed to wait for ARC app window state to change to maximized 4")
	}

	// Display should be landscape.
	// Wait until display rotates to landscape-primary orientation.
	if err := wm.WaitForDisplayOrientation(ctx, tconn, display.OrientationLandscapePrimary); err != nil {
		return errors.Wrap(err, "failed to wait for display orientation")
	}

	// TODO(b/162387612): After the bug is fixed, compare window bounds.
	// return testing.Poll(ctx, func(ctx context.Context) error {
	// 	// Get activity's window info after switching back from tablet mode.
	// 	apwInfo, err := ash.GetARCAppWindowInfo(ctx, tconn, wm.Pkg24)
	// 	if err != nil {
	// 		return testing.PollBreak(err)
	// 	}
	// 	// Activity bounds should be equal to the original bounds.
	// 	if apwInfo.BoundsInRoot != owInfo.BoundsInRoot {
	// 		return errors.Errorf("invalid window bounds after switching back from portrait tablet mode; got: %q, want: %q", apwInfo.BoundsInRoot, owInfo.BoundsInRoot)
	// 	}
	// 	return nil
	// }, &testing.PollOptions{Timeout: 5 * time.Second})

	return nil
}

// wmRV22 covers resizable/conversion: split screen.
// Expected behavior is defined in: go/arc-wm-r RV22: resizable/conversion: split screen.
func wmRV22(ctx context.Context, tconn *chrome.TestConn, a *arc.ARC, d *ui.Device) (retErr error) {
	// Test Steps:
	// This test starts at landscape tablet mode.
	// 1- Start the first activity - Under activity.
	// 2- Start the second activity - over activity.
	// 3- Snap the over activity to the left:
	//   3-1- Swipe the over activity from top-center to the middle of the screen.
	//   3-2- Continue swiping the over activity from the middle of the screen to the top-left corner.
	//   3-3- Make sure the over activity is snapped to the left.
	// 4- Snap the under activity to the right:
	//   4-1- Touch the middle of the right half of the screen.
	//   4-2- Make sure the under activity is snapped to the right.
	// 5- Check vertical split in landscape tablet mode.
	// 6- Convert to clamshell mode
	// 7- Check clamshell split.
	// 8- Convert back to tablet mode
	// 9- Check vertical split in landscape tablet mode.
	// 10- Rotate the screen by 270 degrees - to portrait mode.
	// 11- Check horizontal split in portrait tablet mode.
	// 12- Convert to clamshell mode
	// 13- Check clamshell split.
	// 14- Convert back to tablet mode
	// 15- Check horizontal split in portrait tablet mode.

	// This test starts at landscape tablet mode.
	clnEnabled, err := ash.EnsureTabletModeEnabled(ctx, tconn, true)
	if err != nil {
		return errors.Wrap(err, "failed to ensure if tablet mode is enabled")
	}
	defer clnEnabled(ctx)

	underActivity, err := arc.NewActivity(a, wm.Pkg24, wm.ResizableUnspecifiedActivity)
	if err != nil {
		return errors.Wrap(err, "failed to create under activity")
	}
	defer underActivity.Close()

	// 1- Start the first activity - Under activity.
	if err := underActivity.Start(ctx, tconn); err != nil {
		return errors.Wrap(err, "failed to start under activity")
	}
	defer func(ctx context.Context) {
		if err := underActivity.Stop(ctx, tconn); err != nil {
			if retErr == nil {
				retErr = errors.Wrap(err, "failed to stop under activity")
			} else {
				testing.ContextLog(ctx, "Failed to stop under activity: ", err)
			}
		}
	}(ctx)

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
	defer func(ctx context.Context) {
		if err := overActivity.Stop(ctx, tconn); err != nil {
			if retErr == nil {
				retErr = errors.Wrap(err, "failed to stop over activity")
			} else {
				testing.ContextLog(ctx, "Failed to stop over activity: ", err)
			}
		}
	}(ctx)

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
		return errors.Wrap(err, "failed to swipe from top-center to the middle of the screen")
	}

	//   3-2- Continue swiping the over activity from the middle of the screen to the top-left corner.
	to = coords.NewPoint(1, 1)
	x2, y2 := tcc.ConvertLocation(to)

	if err := stw.Swipe(ctx, x1, y1, x2, y2, 500*time.Millisecond); err != nil {
		return errors.Wrap(err, "failed to swipe from the middle of the screen to the top-left corner")
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
			return errors.Errorf("invalid window state: got %+v; want LeftSnapped", overActivityWInfo.State)
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
			return errors.Errorf("invalid window state: got %+v; want RightSnapped", overActivityWInfo.State)
		}
		return nil
	}, &testing.PollOptions{Timeout: 5 * time.Second}); err != nil {
		return err
	}
	// 4- Snap the under activity to the right - End.

	// 5- Check vertical split in landscape tablet mode.
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		if err := wm.CheckVerticalTabletSplit(ctx, tconn, pdInfo.WorkArea); err != nil {
			return errors.Wrap(err, "failed to assert vertical split window bounds before conversion")
		}
		return nil
	}, &testing.PollOptions{Timeout: 5 * time.Second}); err != nil {
		return err
	}

	// 6- Convert to clamshell mode
	clnDisabled, err := ash.EnsureTabletModeEnabled(ctx, tconn, false)
	if err != nil {
		return errors.Wrap(err, "failed to ensure if tablet mode is disabled from landscape tablet mode")
	}
	defer clnDisabled(ctx)

	// 7- Check clamshell split.
	if err := checkClamshellSplit(ctx, tconn); err != nil {
		return errors.Wrap(err, "failed to check clamshell split after converting from landscape tablet mode")
	}

	// 8- Convert back to tablet mode
	clnEnabled2, err := ash.EnsureTabletModeEnabled(ctx, tconn, true)
	if err != nil {
		return errors.Wrap(err, "failed to ensure if tablet mode is enabled from clamshell mode")
	}
	defer clnEnabled2(ctx)

	// 9- Check vertical split in landscape tablet mode.
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		if err := wm.CheckVerticalTabletSplit(ctx, tconn, pdInfo.WorkArea); err != nil {
			return errors.Wrap(err, "failed to assert vertical split window bounds after converting to landscape tablet mode from clamshell")
		}
		return nil
	}, &testing.PollOptions{Timeout: 5 * time.Second}); err != nil {
		return err
	}

	// 10- Rotate the screen by 270 degrees - to portrait mode.
	clnRotation, err := wm.RotateDisplay(ctx, tconn, display.Rotate270)
	if err != nil {
		return errors.Wrap(err, "failed to rotate the display by 270 degrees")
	}
	defer clnRotation()

	if err := testing.Sleep(ctx, wm.RotationAnimationDuration); err != nil {
		return errors.Wrap(err, "failed to sleep for rotation to landscape animation to finish")
	}

	pdInfo, err = display.GetPrimaryInfo(ctx, tconn)
	if err != nil {
		return err
	}

	// 11- Check horizontal split in portrait tablet mode.
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		if err := wm.CheckHorizontalTabletSplit(ctx, tconn, pdInfo.WorkArea); err != nil {
			return errors.Wrap(err, "failed to assert horizontal split window bounds after rotating to portrait tablet mode from landscape tablet mode")
		}
		return nil
	}, &testing.PollOptions{Timeout: 5 * time.Second}); err != nil {
		return err
	}

	// 12- Convert to clamshell mode
	clnDisabled2, err := ash.EnsureTabletModeEnabled(ctx, tconn, false)
	if err != nil {
		return errors.Wrap(err, "failed to ensure if tablet mode is disabled from portrait tablet mode")
	}
	defer clnDisabled2(ctx)

	// 13- Check clamshell split.
	if err := checkClamshellSplit(ctx, tconn); err != nil {
		return errors.Wrap(err, "failed to check clamshell split")
	}

	// 14- Convert to tablet mode
	clnEnabled3, err := ash.EnsureTabletModeEnabled(ctx, tconn, true)
	if err != nil {
		return errors.Wrap(err, "failed to ensure if tablet mode is enabled from clamshell mode")
	}
	defer clnEnabled3(ctx)

	// 15- Check horizontal split in portrait tablet mode.
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		if err := wm.CheckHorizontalTabletSplit(ctx, tconn, pdInfo.WorkArea); err != nil {
			return errors.Wrap(err, "failed to assert horizontal split window bounds after converting to portrait tablet mode from clamshell")
		}
		return nil
	}, &testing.PollOptions{Timeout: 5 * time.Second}); err != nil {
		return err
	}

	return nil
}

// checkClamshellSplit helps to assert window bounds in clamshell split mode.
func checkClamshellSplit(ctx context.Context, tconn *chrome.TestConn) error {
	// 7- Get app window info in clamshell mode - Start.
	pdInfo, err := display.GetPrimaryInfo(ctx, tconn)
	if err != nil {
		return err
	}

	leftWInfo, err := ash.GetARCAppWindowInfo(ctx, tconn, wm.Pkg24Secondary)
	if err != nil {
		return errors.Wrap(err, "failed to get arc app window info for left activity")
	}

	if leftWInfo.State != ash.WindowStateLeftSnapped {
		return errors.Errorf("invlaid window state: got %+v; want LeftSnapped", leftWInfo.State)
	}

	rightWInfo, err := ash.GetARCAppWindowInfo(ctx, tconn, wm.Pkg24)
	if err != nil {
		return errors.Wrap(err, "failed to get arc app window info for right activity")
	}

	if err := ash.WaitWindowFinishAnimating(ctx, tconn, rightWInfo.ID); err != nil {
		return errors.Wrap(err, "failed to wait for window finish animating")
	}

	if rightWInfo.State != ash.WindowStateRightSnapped {
		return errors.Errorf("invlaid window state: got %+v; want RightSnapped", rightWInfo.State)
	}

	lWant := coords.NewRect(0, 0, pdInfo.WorkArea.Width/2, pdInfo.WorkArea.Height)
	rWant := coords.NewRect(pdInfo.WorkArea.Width/2, 0, pdInfo.WorkArea.Width/2, pdInfo.WorkArea.Height)

	if err := testing.Poll(ctx, func(ctx context.Context) error {
		if leftWInfo.BoundsInRoot != lWant {
			return errors.Errorf("invalid snapped to the left activity bounds: got %+v; want %+v",
				leftWInfo.BoundsInRoot, lWant)
		}

		if rightWInfo.BoundsInRoot != rWant {
			return errors.Errorf("invalid snapped to the right activity bounds: got %+v; want %+v",
				rightWInfo.BoundsInRoot, rWant)
		}
		return nil
	}, &testing.PollOptions{Timeout: 5 * time.Second}); err != nil {
		return err
	}
	return nil
}

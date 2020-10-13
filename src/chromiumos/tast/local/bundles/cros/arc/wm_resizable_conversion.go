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
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         WMResizableConversion,
		Desc:         "Verifies that Window Manager resizable/conversion use-cases behaves as described in go/arc-wm-r",
		Contacts:     []string{"armenk@google.com", "arc-framework+tast@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"android_vm", "chrome", "tablet_mode"},
		Pre:          arc.Booted(),
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

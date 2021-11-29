// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"time"

	"chromiumos/tast/common/android/ui"
	"chromiumos/tast/errors"
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
		Func:         WMNonresizableConversion,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Verifies that Window Manager non-resizable/conversion use-cases behaves as described in go/arc-wm-r",
		Contacts:     []string{"armenk@google.com", "takise@google.com", "arc-framework+tast@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"android_vm", "chrome"},
		HardwareDeps: hwdep.D(hwdep.InternalDisplay()),
		Fixture:      "arcBooted",
		Timeout:      8 * time.Minute,
	})
}

func WMNonresizableConversion(ctx context.Context, s *testing.State) {
	wm.SetupAndRunTestCases(ctx, s, false, []wm.TestCase{
		{
			// non-resizable/conversion: landscape
			Name: "NV_conversion_landscape",
			Func: wmNV19,
		},
		{
			// non-resizable/conversion: portrait
			Name: "NV_conversion_portrait",
			Func: wmNV20,
		},
		{
			// non-resizable/conversion: undefined orientation
			Name: "NV_conversion_undefined_orientation",
			Func: wmNV21,
		},
	})
}

// wmNV19 covers non-resizable/conversion behavior in landscape mode.
// Expected behavior is defined in: go/arc-wm-r NV19 non-resizable/conversion: landscape.
func wmNV19(ctx context.Context, tconn *chrome.TestConn, a *arc.ARC, d *ui.Device) error {
	return runNVConversionByOrientation(ctx, tconn, a, d, wm.NonResizableLandscapeActivity, display.OrientationLandscapePrimary)
}

// wmNV20 covers non-resizable/conversion behavior in portrait mode.
// Expected behavior is defined in: go/arc-wm-r NV20 non-resizable/conversion: portrait.
func wmNV20(ctx context.Context, tconn *chrome.TestConn, a *arc.ARC, d *ui.Device) error {
	return runNVConversionByOrientation(ctx, tconn, a, d, wm.NonResizablePortraitActivity, display.OrientationPortraitPrimary)
}

func wmNV21(ctx context.Context, tconn *chrome.TestConn, a *arc.ARC, d *ui.Device) error {
	// Start an unspecified activity.
	act, err := arc.NewActivity(a, wm.Pkg24, wm.NonResizableUnspecifiedActivity)
	if err != nil {
		return err
	}
	defer act.Close()

	if err := act.StartWithDefaultOptions(ctx, tconn); err != nil {
		return err
	}
	defer act.Stop(ctx, tconn)

	if err := wm.WaitUntilActivityIsReady(ctx, tconn, act, d); err != nil {
		return err
	}

	originalWindowInfo, err := ash.GetARCAppWindowInfo(ctx, tconn, wm.Pkg24)
	if err != nil {
		return err
	}
	windowID := originalWindowInfo.ID

	// Enable tablet mode.
	cleanupRoundOne, err := ash.EnsureTabletModeEnabled(ctx, tconn, true)
	if err != nil {
		return errors.Wrap(err, "failed to ensure if tablet mode is enabled")
	}
	defer cleanupRoundOne(ctx)

	if err := wm.WaitForARCAppWindowState(ctx, tconn, ash.WindowStateMaximized, windowID, false); err != nil {
		return err
	}

	// Compare activity bounds to make sure it covers the primary display work area.
	if err := wm.CheckMaximizeWindowInTabletMode(ctx, tconn, wm.Pkg24); err != nil {
		return err
	}

	// Get display orientation in tablet mode.
	tabletModeDO, err := display.GetOrientation(ctx, tconn)
	if err != nil {
		return err
	}

	// Disable tablet mode.
	if err := ash.SetTabletModeEnabled(ctx, tconn, false); err != nil {
		return errors.Wrap(err, "failed to disable tablet mode")
	}

	if err := wm.WaitForARCAppWindowState(ctx, tconn, ash.WindowStateMaximized, windowID, true); err != nil {
		return err
	}

	if err := testing.Poll(ctx, func(ctx context.Context) error {
		// Get activity's window info after switching back from tablet mode.
		currentWindowInfo, err := ash.GetARCAppWindowInfo(ctx, tconn, wm.Pkg24)
		if err != nil {
			return testing.PollBreak(err)
		}

		// Activity bounds should be equal to the original bounds.
		if originalWindowInfo.BoundsInRoot != currentWindowInfo.BoundsInRoot {
			return errors.Errorf("invalid window bounds after switching back from tablet mode in original orientation: got %q; want %q", currentWindowInfo.BoundsInRoot, originalWindowInfo.BoundsInRoot)
		}
		return nil

	}, &testing.PollOptions{Timeout: 10 * time.Second}); err != nil {
		return err
	}

	// Enable tablet mode.
	cleanupRoundTwo, err := ash.EnsureTabletModeEnabled(ctx, tconn, true)
	if err != nil {
		return errors.Wrap(err, "failed to ensure if tablet mode is enabled")
	}
	defer cleanupRoundTwo(ctx)

	if err := wm.WaitForARCAppWindowState(ctx, tconn, ash.WindowStateMaximized, windowID, false); err != nil {
		return err
	}

	// Rotate the screen 270 degree.
	cleanupRotation, err := wm.RotateDisplay(ctx, tconn, display.Rotate270)
	if err != nil {
		return err
	}
	defer cleanupRotation()

	// Display should be portrait.
	// Wait until display rotates to portrait-primary orientation.
	info, err := display.GetPrimaryInfo(ctx, tconn)
	if err != nil {
		return errors.Wrap(err, "failed to get the display info")
	}
	if err := display.WaitForDisplayRotation(ctx, tconn, info.ID, display.Rotate270); err != nil {
		return err
	}

	// Compare activity bounds to make sure it covers the primary display work area.
	if err := wm.CheckMaximizeWindowInTabletMode(ctx, tconn, wm.Pkg24); err != nil {
		return err
	}

	// Disable tablet mode.
	if err := ash.SetTabletModeEnabled(ctx, tconn, false); err != nil {
		return errors.Wrap(err, "failed to disable tablet mode")
	}

	if err := wm.WaitForARCAppWindowState(ctx, tconn, ash.WindowStateMaximized, windowID, true); err != nil {
		return err
	}

	// Display should be in original orientation.
	// Wait until display rotates to original orientation.
	if err := wm.WaitForDisplayOrientation(ctx, tconn, tabletModeDO.Type); err != nil {
		return err
	}
	if err := wm.CheckMaximizeNonResizable(ctx, tconn, act, d); err != nil {
		return err
	}
	if err := ash.WaitWindowFinishAnimating(ctx, tconn, windowID); err != nil {
		return err
	}
	return testing.Poll(ctx, func(ctx context.Context) error {
		// Get activity's window info after switching back from tablet mode.
		currentWindowInfo, err := ash.GetARCAppWindowInfo(ctx, tconn, wm.Pkg24)
		if err != nil {
			return testing.PollBreak(err)
		}

		// Activity bounds should be equal to the original bounds.
		if originalWindowInfo.BoundsInRoot != currentWindowInfo.BoundsInRoot {
			return errors.Errorf("invalid window bounds after switching back from portrait tablet mode: got %q; want %q", currentWindowInfo.BoundsInRoot, originalWindowInfo.BoundsInRoot)
		}
		return nil
	}, &testing.PollOptions{Timeout: 5 * time.Second})
}

func runNVConversionByOrientation(ctx context.Context, tconn *chrome.TestConn, a *arc.ARC, d *ui.Device, actName string, desiredOrientationInTabletMode display.OrientationType) error {
	// Store original display orientation.
	originalDO, err := display.GetOrientation(ctx, tconn)
	if err != nil {
		return err
	}

	// Start a new activity.
	act, err := arc.NewActivity(a, wm.Pkg24, actName)
	if err != nil {
		return err
	}
	defer act.Close()

	if err := act.StartWithDefaultOptions(ctx, tconn); err != nil {
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

	if desiredOrientationInTabletMode == display.OrientationLandscapePrimary {
		if err := wm.CheckMaximizeNonResizable(ctx, tconn, act, d); err != nil {
			return err
		}
	} else {
		if err := wm.CheckRestoreNonResizable(ctx, tconn, act, d); err != nil {
			return err
		}
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

	// Compare activity's window TargetBounds to primary display work area.
	if err := wm.CheckMaximizeWindowInTabletMode(ctx, tconn, wm.Pkg24); err != nil {
		return err
	}

	if err := testing.Poll(ctx, func(ctx context.Context) error {
		// Get display orientation in tablet mode.
		tabletModeDO, err := display.GetOrientation(ctx, tconn)
		if err != nil {
			return testing.PollBreak(err)
		}
		if tabletModeDO.Type != desiredOrientationInTabletMode {
			return errors.Errorf("invalid display orientation in tablet mode, got: %q, want: %q", tabletModeDO.Type, desiredOrientationInTabletMode)
		}
		return nil
	}, &testing.PollOptions{Timeout: 5 * time.Second}); err != nil {
		return err
	}

	// Disable tablet mode.
	if err := ash.SetTabletModeEnabled(ctx, tconn, false); err != nil {
		return errors.Wrap(err, "failed to disable tablet mode")
	}
	if err := ash.WaitForCondition(ctx, tconn, func(w *ash.Window) bool {
		return w.ID == windowID && w.IsFrameVisible == true
	}, &testing.PollOptions{Timeout: 5 * time.Second}); err != nil {
		return errors.Wrap(err, "failed to wait for frame to become visible")
	}
	if err := ash.WaitForARCAppWindowState(ctx, tconn, wm.Pkg24, ash.WindowStateMaximized); err != nil {
		return err
	}
	if err := ash.WaitWindowFinishAnimating(ctx, tconn, windowID); err != nil {
		return err
	}

	if err := testing.Poll(ctx, func(ctx context.Context) error {
		// Get display orientaiton after switching to clamshell mode.
		clamshellDO, err := display.GetOrientation(ctx, tconn)
		if err != nil {
			return testing.PollBreak(err)
		}
		if clamshellDO.Type != originalDO.Type {
			return errors.Errorf("invalid display orientation after switching back to clamshell, got: %q, want: %q", clamshellDO.Type, originalDO.Type)
		}
		return nil
	}, &testing.PollOptions{Timeout: 5 * time.Second}); err != nil {
		return err
	}

	return testing.Poll(ctx, func(ctx context.Context) error {
		windowInfoAfterTabletMode, err := ash.GetARCAppWindowInfo(ctx, tconn, wm.Pkg24)
		if err != nil {
			return testing.PollBreak(err)
		}
		if desiredOrientationInTabletMode == display.OrientationLandscapePrimary {
			if err := wm.CheckMaximizeNonResizable(ctx, tconn, act, d); err != nil {
				return err
			}
		} else {
			if err := wm.CheckRestoreNonResizable(ctx, tconn, act, d); err != nil {
				return err
			}
		}
		// Activity should have same TargetBounds that it had before enabling tablet mode.
		if windowInfoBeforeTabletMode.TargetBounds != windowInfoAfterTabletMode.TargetBounds {
			return errors.Errorf("failed to retrieve original window bounds after switching back from tablet mode, got: %s, want: %s", windowInfoAfterTabletMode.TargetBounds, windowInfoBeforeTabletMode.TargetBounds)
		}
		return nil
	}, &testing.PollOptions{Timeout: 5 * time.Second})
}

// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/arc/ui"
	"chromiumos/tast/local/bundles/cros/arc/wm"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         WMNonresizableConversion,
		Desc:         "Verifies that Window Manager NV use-cases behaves as described in go/arc-wm-r",
		Contacts:     []string{"armenk@google.com", "arc-framework+tast@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"android_vm", "chrome"},
		Pre:          arc.Booted(),
		Timeout:      8 * time.Minute,
	})
}

func WMNonresizableConversion(ctx context.Context, s *testing.State) {
	wm.SetupAndRunTestCases(ctx, s, []wm.TestCase{
		wm.TestCase{
			// non-resizable/conversion: landscape
			Name: "NV_conversion_landscape",
			Func: wmNV19,
		},
	})

}

// wmNV19 covers non-resizable/conversion behavior in landscape mode.
// Expected behavior is defined in: go/arc-wm-r NV19 non-resizable/conversion: landscape.
func wmNV19(ctx context.Context, tconn *chrome.TestConn, a *arc.ARC, d *ui.Device) error {
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
	windowInfoBeforeTableMode, err := ash.GetARCAppWindowInfo(ctx, tconn, wm.Pkg24)
	if err != nil {
		return nil
	}

	// Enable tablet mode, the activity should go to Maximized state.
	cleanup, err := ash.EnsureTabletModeEnabled(ctx, tconn, true)
	if err != nil {
		return errors.Wrap(err, "failed to ensure if tablet mode is enabled")
	}

	if err := ash.WaitForARCAppWindowState(ctx, tconn, wm.Pkg24, ash.WindowStateMaximized); err != nil {
		return err
	}

	// Store activity's window info when tablet mode is enabled to make sure it is in Maximized state.
	windowInfoAtTableMode, err := ash.GetARCAppWindowInfo(ctx, tconn, wm.Pkg24)
	if err != nil {
		return err
	}

	// Compare activity's window TargetBounds to primary display work area.
	if err := wm.CheckMaximizeWindowInTabletMode(ctx, tconn, windowInfoAtTableMode.TargetBounds); err != nil {
		return err
	}

	// Disable tablet mode.
	cleanup(ctx)

	// Wait for activity to go back to Normal state after disabling tablet mode.
	if err := ash.WaitForARCAppWindowState(ctx, tconn, wm.Pkg24, ash.WindowStateNormal); err != nil {
		return err
	}

	windowInfoAfterTableMode, err := ash.GetARCAppWindowInfo(ctx, tconn, wm.Pkg24)
	if err != nil {
		return err
	}

	// Activity should have same TargetBounds that it had before enabling tablet mode.
	if windowInfoBeforeTableMode.TargetBounds != windowInfoAfterTableMode.TargetBounds {
		return errors.Errorf("failed to retrieve original window bounds after switching back from tablet mode, got: %s, want: %s", windowInfoAfterTableMode.TargetBounds, windowInfoBeforeTableMode.TargetBounds)
	}

	return nil
}

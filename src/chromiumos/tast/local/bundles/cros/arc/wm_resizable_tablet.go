// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"time"

	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/arc/ui"
	"chromiumos/tast/local/bundles/cros/arc/wm"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/display"
	"chromiumos/tast/testing"
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
			// resizable/tablet: hide Shelf behavior
			Name: "RT_hide_Shelf_behavior",
			Func: wmRT12,
		},
		wm.TestCase{
			// resizable/tablet: display size change
			Name: "RT_display_size_change",
			Func: wmRT15,
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

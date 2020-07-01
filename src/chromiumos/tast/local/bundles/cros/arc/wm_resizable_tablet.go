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
	"chromiumos/tast/local/chrome/display"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         WMResizableTablet,
		Desc:         "Verifies that Window Manager resizable tablet use-cases behave as described in go/arc-wm-r",
		Contacts:     []string{"armenk@google.com", "arc-framework+tast@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"android_vm", "chrome"},
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
	})
}

// wmRT01 covers resizable/tablet: default launch behavior.
// Expected behavior is defined in: go/arc-wm-r RT01: resizable/tablet: default launch behavior.
func wmRT01(ctx context.Context, tconn *chrome.TestConn, a *arc.ARC, d *ui.Device) error {
	for _, tc := range []struct {
		// Test-case activity name
		activityName string
		// The orientation that device must be in so test-case can run
		displayOrientationType display.OrientationType
	}{
		{
			activityName:           wm.ResizableLandscapeActivity,
			displayOrientationType: display.OrientationLandscapePrimary,
		},
		{
			activityName:           wm.ResizablePortraitActivity,
			displayOrientationType: display.OrientationPortraitPrimary,
		},
	} {
		if err := func() error {
			orientation, err := display.GetOrientation(ctx, tconn)
			if err != nil {
				return err
			}
			// Compare device's display orientation with the test-case orientation.
			// If they are not equal, the display should rotate 270 degree so landscape will become protirate and vice versa.
			// After the display is in correct orientation that the activity must have, then the activty can start.
			if tc.displayOrientationType != orientation.Type {
				resetRot, err := wm.RotateDisplay(ctx, tconn, display.Rotate270)
				if err != nil {
					return err
				}
				defer resetRot()
			}

			act, err := arc.NewActivity(a, wm.Pkg24, tc.activityName)
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
			windowInfo, err := ash.GetARCAppWindowInfo(ctx, tconn, wm.Pkg24)
			if err != nil {
				return err
			}

			return wm.CheckMaximizeWindowInTabletMode(ctx, tconn, *windowInfo)
		}(); err != nil {
			return errors.Wrapf(err, "%q test failed", tc.activityName)
		}
	}
	return nil
}

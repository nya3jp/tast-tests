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
	"chromiumos/tast/local/coords"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         WMNonresizableTablet,
		Desc:         "Verifies that Window Manager non-resizable tablet use-cases behave as described in go/arc-wm-r",
		Contacts:     []string{"armenk@google.com", "arc-framework+tast@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"android_vm", "chrome"},
		Pre:          arc.Booted(),
		Timeout:      8 * time.Minute,
	})
}

func WMNonresizableTablet(ctx context.Context, s *testing.State) {
	wm.SetupAndRunTestCases(ctx, s, true, []wm.TestCase{
		wm.TestCase{
			// non-resizable/tablet: default launch behavior
			Name: "NT_default_launch_behavior",
			Func: wmNT01,
		},
	})
}

// wmNT01 covers non-resizable/tablet: default launch behavior.
// Expected behavior is defined in: go/arc-wm-r NT01: non-resizable/tablet: default launch behavior.
func wmNT01(ctx context.Context, tconn *chrome.TestConn, a *arc.ARC, d *ui.Device) error {
	// Get the default display orientation and set it back after all test-cases are completed.
	defaultOrientation, err := display.GetOrientation(ctx, tconn)
	if err != nil {
		return err
	}
	defer setDisplayOrientation(ctx, tconn, defaultOrientation.Type)

	// 2 Test-cases for activities with specified orientation.
	for _, tc := range []struct {
		// Test-case activity name.
		activityName string
		// Activity's desired orientation.
		desiredDO display.OrientationType
	}{
		{
			activityName: wm.NonResizableLandscapeActivity,
			desiredDO:    display.OrientationLandscapePrimary,
		},
		{
			activityName: wm.NonResizablePortraitActivity,
			desiredDO:    display.OrientationPortraitPrimary,
		},
	} {
		if err := func() error {
			// Set the display to the opposite orientation, so when activity starts, the display should adjust itself to match with activity's desired orientation.
			if err := setDisplayOrientation(ctx, tconn, getOppositeDisplayOrientation(tc.desiredDO)); err != nil {
				return err
			}

			// Start the activity.
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

			if err := wm.CheckMaximizeWindowInTabletMode(ctx, tconn, *windowInfo); err != nil {
				return err
			}

			newDO, err := display.GetOrientation(ctx, tconn)
			if err != nil {
				return err
			}

			// Compare display orientation after activity is ready, it should be equal to activity's desired orientation.
			if tc.desiredDO != newDO.Type {
				return errors.Errorf("invalid display orientation, want: %q, got: %q", tc.desiredDO, newDO.Type)
			}

			return nil
		}(); err != nil {
			return errors.Wrapf(err, "%q test failed", tc.activityName)
		}
	}

	// Unspecified activity orientation.
	// Set the display to an orientation, then start Unspecified activity.
	// Unspecified activity shouldn't change the display orientation.
	for _, displayOrientation := range []display.OrientationType{
		display.OrientationPortraitPrimary,
		display.OrientationLandscapePrimary,
	} {
		if err := checkUnspecifiedActivityInTabletMode(ctx, tconn, a, d, displayOrientation); err != nil {
			return errors.Wrapf(err, "%q test failed", wm.NonResizableUnspecifiedActivity)
		}
	}

	return nil
}

// getOppositeDisplayOrientation returns Portrait for Landscape orientation and vice versa.
func getOppositeDisplayOrientation(orientation display.OrientationType) display.OrientationType {
	if orientation == display.OrientationPortraitPrimary {
		return display.OrientationLandscapePrimary
	}
	return display.OrientationPortraitPrimary
}

// checkUnspecifiedActivityInTabletMode makes sure that the display orientation won't change for an activity with unspecified orientation.
func checkUnspecifiedActivityInTabletMode(ctx context.Context, tconn *chrome.TestConn, a *arc.ARC, d *ui.Device, orientation display.OrientationType) error {
	// Set the display orientation.
	if err := setDisplayOrientation(ctx, tconn, orientation); err != nil {
		return err
	}

	// Start undefined activity.
	act, err := arc.NewActivity(a, wm.Pkg24, wm.NonResizableUnspecifiedActivity)
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

	newDO, err := display.GetOrientation(ctx, tconn)
	if err != nil {
		return err
	}

	// Compare display orientation after the activity is ready, it should be equal to the initial display orientation.
	if orientation != newDO.Type {
		return errors.Errorf("invalid display orientation for unspecified activity, want: %q, got: %q", orientation, newDO.Type)
	}

	windowInfo, err := ash.GetARCAppWindowInfo(ctx, tconn, wm.Pkg24)
	if err != nil {
		return err
	}

	if isPortraitRect(windowInfo.BoundsInRoot) {
		// If app is portrait but the display is not, then return error.
		if orientation != display.OrientationPortraitPrimary {
			return errors.New("invalid unspecified activity orientation, want: Landscape, got: Portrait")
		}
	} else { // App is Landscape
		// If app is Landscape but the display is not, then return error.
		if orientation != display.OrientationLandscapePrimary {
			return errors.New("invalid unspecified activity orientation, want: Portrait, got: Landscape")
		}
	}

	return nil
}

// setDisplayOrientation sets the display orientation by OrientationType.
func setDisplayOrientation(ctx context.Context, tconn *chrome.TestConn, desiredOrientation display.OrientationType) error {
	// Get display orientation
	initialDO, err := display.GetOrientation(ctx, tconn)
	if err != nil {
		return err
	}

	if initialDO.Type != desiredOrientation {
		rotationAngle := display.Rotate0
		if desiredOrientation == display.OrientationPortraitPrimary {
			rotationAngle = display.Rotate270
		}

		_, err := wm.RotateDisplay(ctx, tconn, rotationAngle)
		if err != nil {
			return err
		}
	}

	return nil
}

// isPortraitRect returns true if width is greater than height.
func isPortraitRect(rect coords.Rect) bool {
	if rect.Width < rect.Height {
		return true
	}
	return false
}

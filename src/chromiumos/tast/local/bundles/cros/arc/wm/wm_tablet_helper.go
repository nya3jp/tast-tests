// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package wm provides Window Manager Helper functions.
package wm

import (
	"context"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/arc/ui"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/display"
	"chromiumos/tast/local/coords"
	"chromiumos/tast/testing"
)

// TabletLaunchActivityInfo holds activity info.
type TabletLaunchActivityInfo struct {
	// Test-case activity name.
	ActivityName string
	// Activity's desired orientation.
	DesiredDO display.OrientationType
}

// TabletDefaultLaunchHelper runs tablet default lunch test-cases by given activity names.
func TabletDefaultLaunchHelper(ctx context.Context, tconn *chrome.TestConn, a *arc.ARC, d *ui.Device, activityInfo []TabletLaunchActivityInfo, isResizable bool) error {
	// Get the default display orientation and set it back after all test-cases are completed.
	defaultOrientation, err := display.GetOrientation(ctx, tconn)
	if err != nil {
		return err
	}
	defer setDisplayOrientation(ctx, tconn, defaultOrientation.Type)

	// 2 Test-cases for activities with specified orientation.
	for _, tc := range activityInfo {
		if err := func() error {
			// Set the display to the opposite orientation, so when activity starts, the display should adjust itself to match with activity's desired orientation.
			if err := setDisplayOrientation(ctx, tconn, getOppositeDisplayOrientation(tc.DesiredDO)); err != nil {
				return err
			}

			// Start the activity.
			act, err := arc.NewActivity(a, Pkg24, tc.ActivityName)
			if err != nil {
				return err
			}
			defer act.Close()

			if err := act.Start(ctx, tconn); err != nil {
				return err
			}
			defer act.Stop(ctx, tconn)

			if err := WaitUntilActivityIsReady(ctx, tconn, act, d); err != nil {
				return err
			}

			windowInfo, err := ash.GetARCAppWindowInfo(ctx, tconn, Pkg24)
			if err != nil {
				return err
			}

			if err := CheckMaximizeWindowInTabletMode(ctx, tconn, *windowInfo); err != nil {
				return err
			}

			newDO, err := display.GetOrientation(ctx, tconn)
			if err != nil {
				return err
			}

			// Compare display orientation after activity is ready, it should be equal to activity's desired orientation.
			if tc.DesiredDO != newDO.Type {
				return errors.Errorf("invalid display orientation, want: %q, got: %q", tc.DesiredDO, newDO.Type)
			}

			return nil
		}(); err != nil {
			return errors.Wrapf(err, "%q test failed", tc.ActivityName)
		}
	}

	// Unspecified activity orientation.
	// Set the display to an orientation, then start Unspecified activity.
	// Unspecified activity shouldn't change the display orientation.
	for _, displayOrientation := range []display.OrientationType{
		display.OrientationPortraitPrimary,
		display.OrientationLandscapePrimary,
	} {
		unActName := NonResizableUnspecifiedActivity
		if isResizable {
			unActName = ResizableUnspecifiedActivity
		}
		if err := checkUnspecifiedActivityInTabletMode(ctx, tconn, a, d, displayOrientation, unActName); err != nil {
			return errors.Wrapf(err, "%q test failed", NonResizableUnspecifiedActivity)
		}
	}

	return nil
}

// TabletShelfHideShowHelper runs tablet test-cases that hide and show the shelf.
func TabletShelfHideShowHelper(ctx context.Context, tconn *chrome.TestConn, a *arc.ARC, d *ui.Device, activityInfo []TabletLaunchActivityInfo) error {
	// Get primary display info to set shelf behavior.
	primaryDisplayInfo, err := display.GetPrimaryInfo(ctx, tconn)
	if err != nil {
		return err
	}
	if primaryDisplayInfo == nil {
		return errors.New("failed to find primary display info")
	}
	// Get the default display orientation and set it back after all test-cases are completed.
	defaultOrientation, err := display.GetOrientation(ctx, tconn)
	if err != nil {
		return err
	}
	defer setDisplayOrientation(ctx, tconn, defaultOrientation.Type)

	for _, tc := range activityInfo {
		if err := func() error {
			if err := showHideShelfHelper(ctx, tconn, a, d, tc, primaryDisplayInfo.ID); err != nil {
				return err
			}
			return nil
		}(); err != nil {
			return errors.Wrapf(err, "%q test failed", tc)
		}
	}

	return nil
}

// showHideShelfHelper runs shelf show/hide scenarios per activity on tablet.
func showHideShelfHelper(ctx context.Context, tconn *chrome.TestConn, a *arc.ARC, d *ui.Device, activityInfo TabletLaunchActivityInfo, pdID string) error {
	// Get initial shelf behavior to make sure it is never hide.
	initSB, err := ash.GetShelfBehavior(ctx, tconn, pdID)
	if err != nil {
		return err
	}
	if initSB != ash.ShelfBehaviorNeverAutoHide {
		// Set shelf behavior to never auto hide for test's initial state.
		if err := ash.SetShelfBehavior(ctx, tconn, pdID, ash.ShelfBehaviorNeverAutoHide); err != nil {
			return err
		}
	}

	// Start the activity.
	act, err := arc.NewActivity(a, Pkg24, activityInfo.ActivityName)
	if err != nil {
		return err
	}
	defer act.Close()

	if err := act.Start(ctx, tconn); err != nil {
		return err
	}
	defer act.Stop(ctx, tconn)

	if err := WaitUntilActivityIsReady(ctx, tconn, act, d); err != nil {
		return err
	}
	if err := CheckMaximizeNonResizable(ctx, tconn, act, d); err != nil {
		return err
	}

	// Check the display orientation.
	displayOrientation, err := display.GetOrientation(ctx, tconn)
	if err != nil {
		return err
	}

	// Compare display orientation after activity is ready, it should be equal to activity's desired orientation.
	if activityInfo.DesiredDO != displayOrientation.Type {
		return errors.Errorf("invalid display orientation, want: %q, got: %q", activityInfo.DesiredDO, displayOrientation.Type)
	}

	// Store initial window info to compare with after hiding and showing the shelf.
	winInfoInitialState, err := ash.GetARCAppWindowInfo(ctx, tconn, Pkg24)
	if err != nil {
		return err
	}

	// Set shelf behavior to auto hide.
	if err := ash.SetShelfBehavior(ctx, tconn, pdID, ash.ShelfBehaviorAlwaysAutoHide); err != nil {
		return err
	}

	// Wait for shelf animation to complete.
	if err := WaitForShelfAnimationComplete(ctx, tconn); err != nil {
		return errors.Wrap(err, "failed to wait for shelf animation to complete")
	}

	// Compare window bounds before and after hiding the shelf. It should be larger when shelf is hidden.
	testing.Poll(ctx, func(ctx context.Context) error {
		winInfoShelfHidden, err := ash.GetARCAppWindowInfo(ctx, tconn, Pkg24)
		if err != nil {
			return err
		}
		if winInfoShelfHidden.BoundsInRoot.Height <= winInfoInitialState.BoundsInRoot.Height {
			return errors.Errorf("invalid window bounds when shelf is shown, got: %s, want smaller than: %s", winInfoInitialState.BoundsInRoot, winInfoShelfHidden.BoundsInRoot)
		}
		return nil
	}, &testing.PollOptions{Timeout: 5 * time.Second})

	// Show the shelf.
	if err := ash.SetShelfBehavior(ctx, tconn, pdID, ash.ShelfBehaviorNeverAutoHide); err != nil {
		return err
	}

	// Wait for shelf animation to complete.
	if err := WaitForShelfAnimationComplete(ctx, tconn); err != nil {
		return errors.Wrap(err, "failed to wait for shelf animation to complete")
	}

	if err := CheckMaximizeNonResizable(ctx, tconn, act, d); err != nil {
		return err
	}

	// Compare window bounds after showing the shelf with initial bounds. They should be equal.
	return testing.Poll(ctx, func(ctx context.Context) error {
		winInfoShelfReShown, err := ash.GetARCAppWindowInfo(ctx, tconn, Pkg24)
		if err != nil {
			return err
		}

		if winInfoInitialState.BoundsInRoot != winInfoShelfReShown.BoundsInRoot {
			return errors.Errorf("invalid window bounds after hiding and showing the shelf, got: %s, want: %s", winInfoShelfReShown.BoundsInRoot, winInfoInitialState.BoundsInRoot)
		}
		return nil
	}, &testing.PollOptions{Timeout: 5 * time.Second})
}

// getOppositeDisplayOrientation returns Portrait for Landscape orientation and vice versa.
func getOppositeDisplayOrientation(orientation display.OrientationType) display.OrientationType {
	if orientation == display.OrientationPortraitPrimary {
		return display.OrientationLandscapePrimary
	}
	return display.OrientationPortraitPrimary
}

// checkUnspecifiedActivityInTabletMode makes sure that the display orientation won't change for an activity with unspecified orientation.
func checkUnspecifiedActivityInTabletMode(ctx context.Context, tconn *chrome.TestConn, a *arc.ARC, d *ui.Device, orientation display.OrientationType, unActName string) error {
	// Set the display orientation.
	if err := setDisplayOrientation(ctx, tconn, orientation); err != nil {
		return err
	}

	// Start undefined activity.
	act, err := arc.NewActivity(a, Pkg24, unActName)
	if err != nil {
		return err
	}
	defer act.Close()

	if err := act.Start(ctx, tconn); err != nil {
		return err
	}
	defer act.Stop(ctx, tconn)

	if err := WaitUntilActivityIsReady(ctx, tconn, act, d); err != nil {
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

	windowInfo, err := ash.GetARCAppWindowInfo(ctx, tconn, Pkg24)
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

		_, err := RotateDisplay(ctx, tconn, rotationAngle)
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

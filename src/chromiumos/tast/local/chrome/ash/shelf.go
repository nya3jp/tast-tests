// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package ash implements a library used for communication with Chrome Ash.
package ash

import (
	"context"
	"fmt"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/display"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

// ShelfBehavior represents the different Chrome OS shelf behaviors.
type ShelfBehavior string

// As defined in ShelfAutoHideBehavior here:
// https://cs.chromium.org/chromium/src/ash/public/cpp/shelf_types.h
const (
	// ShelfBehaviorAlwaysAutoHide represents always auto-hide.
	ShelfBehaviorAlwaysAutoHide ShelfBehavior = "always"
	//ShelfBehaviorNeverAutoHide represents never auto-hide, meaning that it is always visible.
	ShelfBehaviorNeverAutoHide = "never"
	// ShelfBehaviorHidden represents always hidden, used for debugging, since this state is not exposed to the user.
	ShelfBehaviorHidden = "hidden"
	// ShelfBehaviorInvalid represents an invalid state.
	ShelfBehaviorInvalid = "invalid"
)

// SetShelfBehavior sets the shelf visibility behavior.
// displayID is the display that contains the shelf.
func SetShelfBehavior(ctx context.Context, c *chrome.Conn, displayID string, b ShelfBehavior) error {
	expr := fmt.Sprintf(
		`new Promise(function(resolve, reject) {
		  chrome.autotestPrivate.setShelfAutoHideBehavior(%q, %q, function() {
		    if (chrome.runtime.lastError) {
		      reject(new Error(chrome.runtime.lastError.message));
		    } else {
		      resolve();
		    }
		  });
		})`, displayID, b)
	return c.EvalPromise(ctx, expr, nil)
}

// GetShelfBehavior returns the shelf visibility behavior.
// displayID is the display that contains the shelf.
func GetShelfBehavior(ctx context.Context, c *chrome.Conn, displayID string) (ShelfBehavior, error) {
	var b ShelfBehavior
	expr := fmt.Sprintf(
		`new Promise(function(resolve, reject) {
		  chrome.autotestPrivate.getShelfAutoHideBehavior(%q, function(behavior) {
		    if (chrome.runtime.lastError) {
		      reject(new Error(chrome.runtime.lastError.message));
		    } else {
		      resolve(behavior);
		    }
		  });
		})`, displayID)
	if err := c.EvalPromise(ctx, expr, &b); err != nil {
		return ShelfBehaviorInvalid, err
	}
	switch b {
	case ShelfBehaviorAlwaysAutoHide, ShelfBehaviorNeverAutoHide, ShelfBehaviorHidden:
	default:
		return ShelfBehaviorInvalid, errors.Errorf("invalid shelf behavior %q", b)
	}
	return b, nil
}

// ShelfAlignment represents the different Chrome OS shelf alignments.
type ShelfAlignment string

// As defined in ShelfAlignment here:
// https://cs.chromium.org/chromium/src/ash/public/cpp/shelf_types.h
const (
	ShelfAlignmentBottom       ShelfAlignment = "Bottom"
	ShelfAlignmentLeft                        = "Left"
	ShelfAlignmentRight                       = "Right"
	ShelfAlignmentBottomLocked                = "BottomLocked"
	ShelfAlignmentInvalid                     = "Invalid"
)

// SetShelfAlignment sets the shelf alignment.
// displayID is the display that contains the shelf.
func SetShelfAlignment(ctx context.Context, c *chrome.Conn, displayID string, a ShelfAlignment) error {
	expr := fmt.Sprintf(
		`new Promise(function(resolve, reject) {
		  chrome.autotestPrivate.setShelfAlignment(%q, %q, function() {
		    if (chrome.runtime.lastError) {
		      reject(new Error(chrome.runtime.lastError.message));
		    } else {
		      resolve();
		    }
		  });
		})`, displayID, a)
	return c.EvalPromise(ctx, expr, nil)
}

// GetShelfAlignment returns the shelf alignment.
// displayID is the display that contains the shelf.
func GetShelfAlignment(ctx context.Context, c *chrome.Conn, displayID string) (ShelfAlignment, error) {
	var a ShelfAlignment
	expr := fmt.Sprintf(
		`new Promise(function(resolve, reject) {
		  chrome.autotestPrivate.getShelfAlignment(%q, function(alignment) {
		    if (chrome.runtime.lastError) {
		      reject(new Error(chrome.runtime.lastError.message));
		    } else {
		      resolve(alignment);
		    }
		  });
		})`, displayID)
	if err := c.EvalPromise(ctx, expr, &a); err != nil {
		return ShelfAlignmentInvalid, err
	}
	switch a {
	case ShelfAlignmentBottom, ShelfAlignmentLeft, ShelfAlignmentRight, ShelfAlignmentBottomLocked:
	default:
		return ShelfAlignmentInvalid, errors.Errorf("invalid shelf alignment %q", a)
	}
	return a, nil
}

// ShelfItem corresponds to the "ShelfItem" defined in autotest_private.idl.
type ShelfItem struct {
	AppID           string `json:"appId"`
	LaunchID        string `json:"launchId"`
	Title           string `json:"title"`
	Type            string `json:"type"`
	Status          string `json:"status"`
	ShowsToolTip    bool   `json:"showsTooltip"`
	PinnedByPolicy  bool   `json:"pinnedByPolicy"`
	HasNotification bool   `json:"hasNotification"`
}

// ShelfItems returns the list of apps in the shelf.
func ShelfItems(ctx context.Context, c *chrome.Conn) ([]*ShelfItem, error) {
	var s []*ShelfItem
	shelfQuery := fmt.Sprintf("tast.promisify(chrome.autotestPrivate.getShelfItems)()")
	if err := c.EvalPromise(ctx, shelfQuery, &s); err != nil {
		return nil, errors.Wrap(err, "failed to call getShelfItems")
	}
	return s, nil
}

// AppShown checks if an app specified by appID is shown in the shelf.
func AppShown(ctx context.Context, c *chrome.Conn, appID string) (bool, error) {
	var appShown bool
	shownQuery := fmt.Sprintf("tast.promisify(chrome.autotestPrivate.isAppShown)(%q)", appID)
	if err := c.EvalPromise(ctx, shownQuery, &appShown); err != nil {
		errors.Errorf("Running autotestPrivate.isAppShown failed for %v", appID)
		return false, err
	}
	return appShown, nil
}

// WaitForApp waits for the app specifed by appID to appear in the shelf.
func WaitForApp(ctx context.Context, c *chrome.Conn, appID string) error {
	return testing.Poll(ctx, func(ctx context.Context) error {
		if visible, err := AppShown(ctx, c, appID); err != nil {
			return testing.PollBreak(err)
		} else if !visible {
			return errors.New("app is not shown yet")
		}
		return nil
	}, &testing.PollOptions{Timeout: time.Minute})
}

// HotseatStateType corresponds to the "HotseatState" defined in autotest_private.idl.
type HotseatStateType string

const (
	// Hidden means that hotseat is shown off screen.
	Hidden HotseatStateType = "Hidden"
	// ShownClamShell means that hotseat is shown within the shelf in clamshell mode.
	ShownClamShell HotseatStateType = "ShownClamShell"
	// ShownHomeLauncher means that hotseat is shown in the tablet mode home launcher's shelf.
	ShownHomeLauncher HotseatStateType = "ShownHomeLauncher"
	// Extended means that hotseat is shown above the shelf.
	Extended HotseatStateType = "Extended"
)

// HotseatSwipeDescriptor corresponds to the "HotseatSwipeDescriptor" defined in autotest_private.idl.
type HotseatSwipeDescriptor struct {
	SwipeStartLocation Location `json:"swipeStartLocation"`
	SwipeEndLocation   Location `json:"swipeEndLocation"`
}

// HotseatInfo corresonds to the "HotseatInfo" defined in autotest_private.idl.
type HotseatInfo struct {
	SwipeUp      HotseatSwipeDescriptor `json:"swipeUp"`
	HotseatState HotseatStateType       `json:"state"`
	IsAnimating  bool                   `json:"isAnimating"`
}

// FetchHotseatInfo does
func FetchHotseatInfo(ctx context.Context, c *chrome.Conn) (*HotseatInfo, error) {
	var s *HotseatInfo
	fetchQuery := fmt.Sprintf("tast.promisify(chrome.autotestPrivate.getHotseatInfo)()")
	if err := c.EvalPromise(ctx, fetchQuery, &s); err != nil {
		errors.Wrap(err, "Running autotestPrivate.getHotseatInfo failed")
		return nil, err
	}

	return s, nil
}

// SwipeUpHotseatAndWaitForCompletion swipes the hotseat up, changing the hotseat state from hidden to extended. The function does not end until the hotseat animation completes.
func SwipeUpHotseatAndWaitForCompletion(ctx context.Context, c *chrome.Conn) error {
	info, err := FetchHotseatInfo(ctx, c)
	if err != nil {
		return errors.Wrap(err, "failed to fetch hotseat info")
	}

	if info.HotseatState != Hidden {
		return errors.Errorf("The hotseat state is unexpected: expected hotseat state is hidden; actual hotseat state is %v", info.HotseatState)
	}

	// Obtain the suitable touch screen writer.
	tsw, err := input.Touchscreen(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to create touch screen event writer")
	}
	orientation, err := display.GetOrientation(ctx, c)
	if err != nil {
		return errors.Wrap(err, "failed to obtain the orientation info")
	}
	tsw.SetRotation(-orientation.Angle)

	// Obtain the coordinate converter from the touch screen writer.
	tcc, err := NewTouchCoordConverter(ctx, c, tsw)
	if err != nil {
		return errors.Wrap(err, "failed to create touch coord converter")
	}

	// Convert the gesture locations from screen coordinates to touch screen coordinates.
	startX, startY := tcc.ConvertLocation(info.SwipeUp.SwipeStartLocation)
	endX, endY := tcc.ConvertLocation(info.SwipeUp.SwipeEndLocation)

	// Instead of dragging the hotseat to the final place directly, leave some space to apply hotseat animation.
	endY = (startY + endY) / 2

	stw, err := tsw.NewSingleTouchWriter()
	if err != nil {
		return errors.Wrap(err, "failed to get single touch writer")
	}
	if err := stw.Swipe(ctx, startX, startY, endX, endY, time.Millisecond); err != nil {
		return errors.Wrap(err, "failed swipe up the hotseat")
	}

	stw.End()

	// Wait for the hotseat animation to finish.
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		info, err := FetchHotseatInfo(ctx, c)
		if err != nil {
			return errors.Wrap(err, "failed to fetch hotseat info")
		}

		if info.IsAnimating || info.HotseatState != Extended {
			return errors.Wrapf(err, "expected hotseat state: Extended; expected hotseat animating state: false; actual hotseat state: %v; actual animating state: %t", info.HotseatState, info.IsAnimating)
		}

		return nil
	}, &testing.PollOptions{Timeout: 2 * time.Second}); err != nil {
		return errors.Wrap(err, "failed to wait for hotseat to reach the expected: ")
	}

	return nil
}

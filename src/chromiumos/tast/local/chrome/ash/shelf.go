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

// PinApp pins the shelf icon for the app specified by |appID|.
func PinApp(ctx context.Context, tconn *chrome.Conn, appID string) error {
	query := fmt.Sprintf("tast.promisify(chrome.autotestPrivate.pinShelfIcon)(%q)", appID)
	return tconn.EvalPromise(ctx, query, nil)
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

// ScrollableShelfInfo corresponds to the "ScrollableShelfInfo" defined in autotest_private.idl
type ScrollableShelfInfo struct {
	MainAxisOffset       float32 `json:"mainAxisOffset"`
	PageOffset           float32 `json:"pageOffset"`
	TargetMainAxisOffset float32 `json:"targetMainAxisOffset"`
	LeftArrowBounds      Rect    `json:"leftArrowBounds"`
	RightArrowBounds     Rect    `json:"rightArrowBounds"`
	IsAnimating          bool    `json:"isAnimating"`
}

// AppType defines the types of available apps.
type AppType string

// Corresponds to the definition in autotest_private.idl.
const (
	Arc       AppType = "Arc"
	BuiltIn   AppType = "BuiltIn"
	Crostini  AppType = "Crostini"
	Extension AppType = "Extension"
	Web       AppType = "Web"
	MacNative AppType = "MacNative"
)

// AppReadiness maps apps::mojom::Readiness.
type AppReadiness string

// Corresponds to the definition in autotest_private.idl
const (
	Ready               AppReadiness = "Ready"
	DisabledByBlacklist AppReadiness = "DisabledByBlacklist"
	DisabledByPolicy    AppReadiness = "DisabledByPolicy"
	DisabledByUser      AppReadiness = "DisabledByUser"
	Terminated          AppReadiness = "Terminated"
	UninstalledByUser   AppReadiness = "UninstalledByUser"
)

// ChromeApp corresponds to the "App" defined in autotest_private.idl.
type ChromeApp struct {
	AppID                 string       `json:"appId"`
	Name                  string       `json:"name"`
	ShortName             string       `json:"shortName"`
	Type                  AppType      `json:"type"`
	Readiness             AppReadiness `json:"readiness"`
	AdditionalSearchTerms []string     `json:"additionalSearchTerms"`
	ShowInLauncher        bool         `json:"showInLauncher"`
	ShowInSearch          bool         `json:"showInSearch"`
}

// ChromeApps returns all of the installed apps.
func ChromeApps(ctx context.Context, c *chrome.Conn) ([]*ChromeApp, error) {
	var s []*ChromeApp
	chromeQuery := fmt.Sprintf("tast.promisify(chrome.autotestPrivate.getAllInstalledApps)()")
	if err := c.EvalPromise(ctx, chromeQuery, &s); err != nil {
		return nil, errors.Wrap(err, "failed to call getAllInstalledApps")
	}
	return s, nil
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

// FetchScrollableShelfInfo returns the scrollable shelf's ui related information.
func FetchScrollableShelfInfo(ctx context.Context, c *chrome.TestConn, scrollDistance float32) (*ScrollableShelfInfo, error) {
	var s *ScrollableShelfInfo
	ScrollableShelfQuery := fmt.Sprintf("tast.promisify(chrome.autotestPrivate.getScrollableShelfInfo)(%f)", scrollDistance)
	if err := c.EvalPromise(ctx, ScrollableShelfQuery, &s); err != nil {
		return nil, errors.Wrap(err, "failed to call getScrollableShelfInfo")
	}
	return s, nil
}

// PressShelfArrowButtonAndWaitForCompletion triggers the scroll animation by mouse click then waits the animation to finish.
func PressShelfArrowButtonAndWaitForCompletion(ctx context.Context, tconn *chrome.TestConn, buttonBounds Rect, targetOffset float32) error {
	var err error

	// Before pressing the arrow button, wait scrollable shelf to be idle.
	err = testing.Poll(ctx, func(ctx context.Context) error {
		info, err := FetchScrollableShelfInfo(ctx, tconn, 0)
		if err != nil {
			return errors.Wrap(err, "failed to fetch scrollable shelf's information when waiting for scroll animation")
		}
		if info.IsAnimating {
			return errors.Errorf("unexpected scroll animation status: got %t; want %t", true, false)
		}
		return nil
	}, &testing.PollOptions{Timeout: 2 * time.Second})
	if err != nil {
		return errors.Wrap(err, "failed to wait scrollable shelf to be idle before starting the scroll animation")
	}

	// Press the arrow button.
	err = MouseClick(ctx, tconn, Location{X: buttonBounds.Left + buttonBounds.Width/2, Y: buttonBounds.Top + buttonBounds.Height/2}, LeftButton)
	if err != nil {
		return errors.Wrap(err, "failed to trigger the scroll animation by clicking at the arrow button")
	}

	// Wait the scroll animation to finish.
	err = testing.Poll(ctx, func(ctx context.Context) error {
		info, err := FetchScrollableShelfInfo(ctx, tconn, 0)
		if err != nil {
			return errors.Wrap(err, "failed to fetch scrollable shelf's information when waiting for scroll animation")
		}
		if info.MainAxisOffset != targetOffset || info.IsAnimating {
			return errors.Errorf("unexpected scrollable shelf status; actual offset: %f, actual animation status: %t, target offset: %f, target animation status: %t", info.MainAxisOffset, info.IsAnimating, targetOffset, false)
		}
		return nil
	}, &testing.PollOptions{Timeout: 2 * time.Second})
	if err != nil {
		return errors.Wrap(err, "failed to wait scrollable shelf to finish scroll animation")
	}

	return nil
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

// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package cuj

import (
	"context"
	"regexp"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/touch"
	"chromiumos/tast/testing"
)

// CloseAllWindows closes all currently open windows by iterating over
// the shelf icons and calling apps.closeApp on each one.
func CloseAllWindows(ctx context.Context, tconn *chrome.TestConn) error {
	shelfItems, err := ash.ShelfItems(ctx, tconn)
	if err != nil {
		return errors.Wrap(err, "failed to get shelf items")
	}
	for _, shelfItem := range shelfItems {
		if shelfItem.Status != ash.ShelfItemClosed {
			if err := apps.Close(ctx, tconn, shelfItem.AppID); err != nil {
				return errors.Wrapf(err, "failed to close the app %v", shelfItem.AppID)
			}
		}
	}
	return nil
}

// GetBrowserStartTime opens chrome browser and returns the browser start time.
func GetBrowserStartTime(ctx context.Context, cr *chrome.Chrome, tconn *chrome.TestConn, tabletMode bool) (time.Duration, error) {
	var browserStartTime time.Duration
	var startTime time.Time
	var launchErr error

	// Get the expected browser.
	chromeApp, err := apps.ChromeOrChromium(ctx, tconn)
	if err != nil {
		return browserStartTime, errors.Wrap(err, "could not find the Chrome app")
	}

	// Make sure the browser hasn't been opened.
	shown, err := ash.AppShown(ctx, tconn, chromeApp.ID)
	if err != nil {
		return 0, errors.Wrap(err, "failed to check if the browser window is shown or not")
	}
	if shown {
		return 0, errors.New("browser is already shown")
	}

	if tabletMode {
		testing.ContextLog(ctx, "Launch Google Chrome (could be Chrome or Chromium) from hotseat")
		startTime, launchErr = LaunchAppFromHotseat(ctx, tconn, "Chrome", "Chromium")
	} else {
		testing.ContextLog(ctx, "Launch Google Chrome (could be Chrome or Chromium) from shelf")
		startTime, launchErr = LaunchAppFromShelf(ctx, tconn, "Chrome", "Chromium")
	}

	if launchErr != nil {
		testing.ContextLog(ctx, "Failed to open Chrome")
		return browserStartTime, errors.Wrap(launchErr, "failed to open Chrome")
	}
	// Make sure app is launched.
	if err := ash.WaitForApp(ctx, tconn, chromeApp.ID, 30*time.Second); err != nil {
		return browserStartTime, errors.Wrap(err, "failed to wait for the app to be launched")
	}
	endTime := time.Now()
	browserStartTime = endTime.Sub(startTime)

	conn, err := cr.NewConnForTarget(ctx, chrome.MatchTargetURL("chrome://newtab/"))
	if err != nil {
		return browserStartTime, errors.Wrap(err, "failed to connect to chrome")
	}
	conn.CloseTarget(ctx)

	return browserStartTime, nil
}

// LaunchAppFromShelf opens an app by name which is currently pinned to the shelf.
// Which it is also support multiple names for a single app (e.g. "Chrome"||"Chromium" for Google Chrome, the browser).
// It returns the time when a mouse click event is injected to the app icon.
func LaunchAppFromShelf(ctx context.Context, tconn *chrome.TestConn, appName string, appOtherPossibleNames ...string) (time.Time, error) {
	params := nodewith.Name(appName).ClassName("ash/ShelfAppButton")
	if len(appOtherPossibleNames) > 0 {
		params = paramsOfAppNames(append(appOtherPossibleNames, appName))
	}

	ac := uiauto.New(tconn)
	if err := ac.WithTimeout(10 * time.Second).WaitForLocation(params)(ctx); err != nil {
		return time.Time{}, errors.Wrapf(err, "failed to find app %q", appName)
	}

	// Click mouse to launch app.
	startTime := time.Now()
	if err := ac.LeftClick(params)(ctx); err != nil {
		return startTime, errors.Wrapf(err, "failed to launch app %q", appName)
	}
	return startTime, nil
}

// LaunchAppFromHotseat opens an app by name which is currently pinned to the hotseat.
// Which it is also support multiple names for a single app (e.g. "Chrome"||"Chromium" for Google Chrome, the browser).
// It returns the time when a touch event is injected to the app icon.
func LaunchAppFromHotseat(ctx context.Context, tconn *chrome.TestConn, appName string, appOtherPossibleNames ...string) (time.Time, error) {
	var startTime time.Time
	// Get touch controller for tablet.
	tc, err := touch.New(ctx, tconn)
	if err != nil {
		return startTime, errors.Wrap(err, "failed to create the touch controller")
	}
	defer tc.Close()
	tsew, tcc, err := touch.NewTouchscreenAndConverter(ctx, tconn)
	if err != nil {
		return startTime, errors.Wrap(err, "failed to access to the touch screen")
	}
	defer tsew.Close()
	stw, err := tsew.NewSingleTouchWriter()
	if err != nil {
		return startTime, errors.Wrap(err, "failed to create a new single touch writer")
	}
	defer stw.Close()

	// Make sure hotseat is shown.
	if err := ash.SwipeUpHotseatAndWaitForCompletion(ctx, tconn, stw, tcc); err != nil {
		return startTime, errors.Wrap(err, "failed to show hotseat")
	}

	params := nodewith.Name(appName).ClassName("ash/ShelfAppButton")
	if len(appOtherPossibleNames) > 0 {
		params = paramsOfAppNames(append(appOtherPossibleNames, appName))
	}

	ac := uiauto.New(tconn)
	if err := ac.WithTimeout(10 * time.Second).WaitForLocation(params)(ctx); err != nil {
		return startTime, errors.Wrapf(err, "failed to find app %q", appName)
	}

	// Press button to launch app.
	startTime = time.Now()
	if err := tc.Tap(params)(ctx); err != nil {
		return startTime, errors.Wrapf(err, "failed to tap %q", appName)
	}
	return startTime, nil
}

// paramsOfAppNames combine all possible app names as a ui.FindParams
func paramsOfAppNames(names []string) *nodewith.Finder {
	pattern := "("
	for idx, name := range names {
		pattern += regexp.QuoteMeta(name)
		if idx != len(names)-1 {
			pattern += "|"
		}
	}
	pattern += ")"

	return nodewith.NameRegex(regexp.MustCompile(pattern)).ClassName("ash/ShelfAppButton")
}

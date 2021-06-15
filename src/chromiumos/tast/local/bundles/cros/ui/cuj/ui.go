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
	"chromiumos/tast/local/chrome/ui"
	"chromiumos/tast/local/chrome/ui/pointer"
	"chromiumos/tast/testing"
)

// Click executes the click action of the first node found with the
// given params. If the node doesn't exist in a second, an error is returned.
func Click(ctx context.Context, tconn *chrome.TestConn, params ui.FindParams) error {
	return WaitAndClick(ctx, tconn, params, time.Second)
}

// WaitAndClick executes the click action of a node found with the
// given params. If the timeout is hit, an error is returned.
func WaitAndClick(ctx context.Context, tconn *chrome.TestConn, params ui.FindParams, timeout time.Duration) error {
	node, err := ui.FindWithTimeout(ctx, tconn, params, timeout)
	if err != nil {
		return err
	}
	defer node.Release(ctx)
	return node.StableLeftClick(ctx, &testing.PollOptions{Interval: time.Second, Timeout: 10 * time.Second})
}

// ClickDescendant finds the first descendant of the parent node using the params
// and clicks it. If the node doesn't exist in a second, an error is returned.
func ClickDescendant(ctx context.Context, parent *ui.Node, params ui.FindParams) error {
	return WaitAndClickDescendant(ctx, parent, params, 5*time.Second)
}

// WaitAndClickDescendant finds a descendant of the parent node using the params
// and clicks it. If the timeout is hit, an error is returned.
func WaitAndClickDescendant(ctx context.Context, parent *ui.Node, params ui.FindParams, timeout time.Duration) error {
	node, err := parent.DescendantWithTimeout(ctx, params, timeout)
	if err != nil {
		return err
	}
	defer node.Release(ctx)
	return node.StableLeftClick(ctx, &testing.PollOptions{Interval: time.Second, Timeout: 10 * time.Second})
}

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
	// Get the expected browser.
	chromeApp, err := apps.ChromeOrChromium(ctx, tconn)
	if err != nil {
		return -1, errors.Wrap(err, "could not find the Chrome app")
	}

	// Make sure the browser hasn't been opened.
	shown, err := ash.AppShown(ctx, tconn, chromeApp.ID)
	if err != nil {
		return -1, errors.Wrap(err, "failed to check if the browser window is shown or not")
	}
	if shown {
		return -1, errors.New("browser is already shown")
	}

	msg := "shelf"
	launchFunc := LaunchAppFromShelf
	if tabletMode {
		msg = "hotseat"
		launchFunc = LaunchAppFromHotseat
	}
	testing.ContextLog(ctx, "Launch Google Chrome (could be Chrome or Chromium) from "+msg)

	startTime, launchErr := launchFunc(ctx, tconn, "Chrome", "Chromium")
	if launchErr != nil {
		return -1, errors.Wrap(launchErr, "failed to open Chrome")
	}
	// Make sure app is launched.
	if err := ash.WaitForApp(ctx, tconn, chromeApp.ID, 30*time.Second); err != nil {
		return -1, errors.Wrap(err, "failed to wait for the app to be launched")
	}
	browserStartTime := time.Since(startTime)

	// Depending on the settings, Chrome might open all left-off pages automatically from last session.
	// Close all existing tabs and test can open new pages in the browser.
	if err := CloseBrowserTabs(ctx, tconn); err != nil {
		return -1, errors.Wrap(err, "failed to close all Chrome tabs")
	}

	return browserStartTime, nil
}

// CloseBrowserTabs closes all browser tabs through chrome.tabs API.
func CloseBrowserTabs(ctx context.Context, tconn *chrome.TestConn) error {
	return tconn.Eval(ctx, `(async () => {
		const tabs = await tast.promisify(chrome.tabs.query)({});
		await tast.promisify(chrome.tabs.remove)(tabs.filter((tab) => tab.id).map((tab) => tab.id));
	})()`, nil)
}

// LaunchAppFromShelf opens an app by name which is currently pinned to the shelf.
// Which it is also support multiple names for a single app (e.g. "Chrome"||"Chromium" for Google Chrome, the browser).
// It returns the time when a mouse click event is injected to the app icon.
func LaunchAppFromShelf(ctx context.Context, tconn *chrome.TestConn, appName string, appOtherPossibleNames ...string) (time.Time, error) {
	params := ui.FindParams{Name: appName, ClassName: "ash/ShelfAppButton"}
	if len(appOtherPossibleNames) > 0 {
		params = (*paramsOfAppNames(append(appOtherPossibleNames, appName)))
	}

	icon, err := ui.FindWithTimeout(ctx, tconn, params, 10*time.Second)
	if err != nil {
		return time.Time{}, errors.Wrapf(err, "failed to find app %q", appName)
	}
	defer icon.Release(ctx)

	// Click mouse to launch app.
	startTime := time.Now()
	if err := icon.LeftClick(ctx); err != nil {
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
	tc, err := pointer.NewTouchController(ctx, tconn)
	if err != nil {
		return startTime, errors.Wrap(err, "failed to create the touch controller")
	}
	defer tc.Close()
	stw := tc.EventWriter()
	tcc := tc.TouchCoordConverter()

	// Make sure hotseat is shown.
	if err := ash.SwipeUpHotseatAndWaitForCompletion(ctx, tconn, stw, tcc); err != nil {
		return startTime, errors.Wrap(err, "failed to show hotseat")
	}

	params := ui.FindParams{Name: appName, ClassName: "ash/ShelfAppButton"}
	if len(appOtherPossibleNames) > 0 {
		params = (*paramsOfAppNames(append(appOtherPossibleNames, appName)))
	}

	icon, err := ui.FindWithTimeout(ctx, tconn, params, 10*time.Second)
	if err != nil {
		return startTime, errors.Wrapf(err, "failed to find app %q", appName)
	}
	defer icon.Release(ctx)

	// Press button to launch app.
	startTime = time.Now()
	x, y := tcc.ConvertLocation(icon.Location.CenterPoint())
	if err := stw.Move(x, y); err != nil {
		return startTime, errors.Wrapf(err, "failed to press icon %q", appName)
	}
	if err := stw.End(); err != nil {
		return startTime, errors.Wrapf(err, "failed to release pressed icon %q", appName)
	}
	return startTime, nil
}

// paramsOfAppNames combine all possible app names as a ui.FindParams
func paramsOfAppNames(names []string) *ui.FindParams {
	pattern := "("
	for idx, name := range names {
		pattern += regexp.QuoteMeta(name)
		if idx != len(names)-1 {
			pattern += "|"
		}
	}
	pattern += ")"

	return &ui.FindParams{
		Attributes: map[string]interface{}{"name": regexp.MustCompile(pattern)},
		ClassName:  "ash/ShelfAppButton",
	}
}

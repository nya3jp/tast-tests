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
	"chromiumos/tast/local/chrome/uiauto/ossettings"
	"chromiumos/tast/local/chrome/uiauto/role"
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

// CloseChrome closes Chrome browser application properly.
// If there exist unsave changes on web page, e.g. media content is playing or online document is editing,
// "leave site" prompt will prevent the tab from closing.
// This function confirms the "leave site" prompts so browser can be closed.
func CloseChrome(ctx context.Context, tconn *chrome.TestConn) error {
	if err := apps.Close(ctx, tconn, apps.Chrome.ID); err != nil {
		return errors.Wrap(err, "failed to close Chrome")
	}

	ui := uiauto.New(tconn)
	leaveWin := nodewith.Name("Leave site?").Role(role.Window).First()
	leaveBtn := nodewith.Name("Leave").Role(role.Button).Ancestor(leaveWin)
	if err := ui.WithTimeout(time.Second).WaitUntilExists(leaveWin)(ctx); err != nil {
		return nil
	}

	return ui.RetryUntil(ui.LeftClick(leaveBtn), ui.WithTimeout(time.Second).WaitUntilGone(leaveWin))(ctx)
}

// LaunchAppFromShelf opens an app by name which is currently pinned to the shelf.
// Which it is also support multiple names for a single app (e.g. "Chrome"||"Chromium" for Google Chrome, the browser).
// It returns the time when a mouse click event is injected to the app icon.
func LaunchAppFromShelf(ctx context.Context, tconn *chrome.TestConn, appName string, appOtherPossibleNames ...string) (time.Time, error) {
	params := appIconFinder(appName, appOtherPossibleNames...)

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

	params := appIconFinder(appName, appOtherPossibleNames...)

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

// appIconFinder returns the finder of app icon with input app name(s).
// It will only look for the icon locate on native display.
func appIconFinder(appName string, appOtherPossibleNames ...string) *nodewith.Finder {
	nativeDisplay := nodewith.ClassName("RootWindow-0").Role(role.Window)
	finder := nodewith.ClassName("ash/ShelfAppButton").Ancestor(nativeDisplay)

	if len(appOtherPossibleNames) == 0 {
		return finder.Name(appName)
	}

	pattern := "(" + appName
	for _, name := range appOtherPossibleNames {
		pattern += "|"
		pattern += regexp.QuoteMeta(name)
	}
	pattern += ")"

	return finder.NameRegex(regexp.MustCompile(pattern))
}

// UnsetMirrorDisplay unsets the mirror display settings.
func UnsetMirrorDisplay(ctx context.Context, tconn *chrome.TestConn) error {
	ui := uiauto.New(tconn)

	testing.ContextLog(ctx, "Launch os-settings to disable mirror")
	settings, err := ossettings.LaunchAtPage(ctx, tconn, nodewith.Name("Device").Role(role.Link))
	if err != nil {
		return errors.Wrap(err, "failed to launch os-settings Device page")
	}

	displayFinder := nodewith.Name("Displays").Role(role.Link).Ancestor(ossettings.WindowFinder)
	if err := ui.LeftClickUntil(displayFinder, ui.WithTimeout(3*time.Second).WaitUntilGone(displayFinder))(ctx); err != nil {
		return errors.Wrap(err, "failed to launch display page")
	}

	mirrorFinder := nodewith.Name("Mirror Built-in display").Role(role.CheckBox).Ancestor(ossettings.WindowFinder)
	// Find the node info for the mirror checkbox.
	nodeInfo, err := ui.Info(ctx, mirrorFinder)
	if err != nil {
		return errors.Wrap(err, "failed to get info for the mirror checkbox")
	}
	if nodeInfo.Checked == "true" {
		testing.ContextLog(ctx, "Click 'Mirror Built-in display' checkbox")
		if err := ui.LeftClick(mirrorFinder)(ctx); err != nil {
			return errors.Wrap(err, "failed to click mirror display")
		}
	}

	return settings.Close(ctx)
}

// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package cuj

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"time"

	"chromiumos/tast/common/action"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/lacros"
	"chromiumos/tast/local/chrome/lacros/lacrosfixt"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/ossettings"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/chrome/uiauto/touch"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

// Tab holds information obtained from the chrome.tabs API.
// See https://developer.chrome.com/docs/extensions/reference/tabs/#type-Tab
type Tab struct {
	ID     int    `json:"ID"`
	Index  int    `json:"index"`
	Title  string `json:"title"`
	URL    string `json:"url"`
	Active bool   `json:"active"`
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
// If lfixtVal is given, it will open the lacros-Chrome, and return the lacros instance.
func GetBrowserStartTime(ctx context.Context, tconn *chrome.TestConn,
	lFixtVal lacrosfixt.FixtValue, closeTabs, tabletMode bool) (*lacros.Lacros, time.Duration, error) {
	var l *lacros.Lacros
	chromeApp, err := apps.PrimaryBrowser(ctx, tconn)
	if err != nil {
		return nil, -1, errors.Wrap(err, "could not find the Chrome app")
	}

	// Make sure the browser hasn't been opened.
	shown, err := ash.AppShown(ctx, tconn, chromeApp.ID)
	if err != nil {
		return nil, -1, errors.Wrap(err, "failed to check if the browser window is shown or not")
	}
	if shown {
		// Close the browser if it is aready opened.
		if err := apps.Close(ctx, tconn, chromeApp.ID); err != nil {
			return nil, -1, errors.Wrap(err, "failed to close the opened browser")
		}
	}

	msg := "shelf"
	launchFunc := LaunchAppFromShelf
	if tabletMode {
		msg = "hotseat"
		launchFunc = LaunchAppFromHotseat
	}
	testing.ContextLog(ctx, "Launch Google Chrome from "+msg)

	var startTime time.Time
	launchChromeApp := func(ctx context.Context) error {
		startTime, err = launchFunc(ctx, tconn, "Chrome", "Chromium", "Lacros")
		if err != nil {
			return errors.Wrap(err, "failed to open Chrome")
		}
		// Make sure app is launched.
		if err := ash.WaitForApp(ctx, tconn, chromeApp.ID, 30*time.Second); err != nil {
			return errors.Wrap(err, "failed to wait for the app to be launched")
		}
		return nil
	}
	ui := uiauto.New(tconn)
	if err := ui.Retry(3, launchChromeApp)(ctx); err != nil {
		return nil, -1, errors.Wrap(err, "failed to launch the Chrome app after 3 retries")
	}
	browserStartTime := time.Since(startTime)

	// If it's ash-Chrome, we will close all existing tabs so the test case will start with a
	// clean Chrome.
	closeTabsFunc := CloseBrowserTabs
	bTconn := tconn
	if lFixtVal != nil {
		// Connect to lacros-Chrome started from UI.
		l, err = lacros.Connect(ctx, tconn)
		if err != nil {
			return nil, -1, errors.Wrap(err, "failed to get lacros instance")
		}
		bTconn, err = l.TestAPIConn(ctx)
		if err != nil {
			return nil, -1, errors.Wrap(err, "failed to create test API conn")
		}
		// For lacros-Chrome, we will close all existing tabs but leave a new tab to keep the Chrome
		// process alive.
		closeTabsFunc = KeepNewTab
	}

	// Depending on the settings, Chrome might open all left-off pages automatically from last session.
	// Close all existing tabs and test can open new pages in the browser.
	if closeTabs {
		if err := closeTabsFunc(ctx, bTconn); err != nil {
			return nil, -1, errors.Wrap(err, "failed to close extra Chrome tabs")
		}
	}

	return l, browserStartTime, nil
}

// GetBrowserTabs gets all browser tabs through chrome.tabs API.
func GetBrowserTabs(ctx context.Context, tconn *chrome.TestConn) ([]Tab, error) {
	var tabs []Tab
	if err := tconn.Eval(ctx, `(async () => {
		const tabs = await tast.promisify(chrome.tabs.query)({currentWindow: true});
		return tabs;
	})()`, &tabs); err != nil {
		return nil, err
	}
	return tabs, nil
}

// CloseBrowserTabsByID closes browser tabs by ID through chrome.tabs API.
func CloseBrowserTabsByID(ctx context.Context, tconn *chrome.TestConn, tabIDs []int) error {
	str := "["
	for i, id := range tabIDs {
		str += strconv.Itoa(id)
		if i != len(tabIDs)-1 {
			str += ", "
		}
	}
	str += "]"
	return tconn.Eval(ctx, fmt.Sprintf(`(async () => {
		await tast.promisify(chrome.tabs.remove)(%s);
	})()`, str), nil)
}

// CloseBrowserTabs closes all browser tabs through chrome.tabs API.
func CloseBrowserTabs(ctx context.Context, tconn *chrome.TestConn) error {
	return tconn.Eval(ctx, `(async () => {
		const tabs = await tast.promisify(chrome.tabs.query)({});
		await tast.promisify(chrome.tabs.remove)(tabs.filter((tab) => tab.id).map((tab) => tab.id));
	})()`, nil)
}

// KeepNewTab closes all other browser tabs and leave only one new tab.
// Leaving a new tab is critical to keep lacros-Chrome process running.
func KeepNewTab(ctx context.Context, tconn *chrome.TestConn) error {
	tabs, err := GetBrowserTabs(ctx, tconn)
	if err != nil {
		return errors.Wrap(err, "failed to get browser tabs")
	}
	if len(tabs) == 0 {
		return errors.New("browser has no tabs")
	}
	newTab := "chrome://newtab/"
	if len(tabs) == 1 && tabs[0].URL == newTab {
		return nil
	}
	// Just simply reate a new tab and close all other tabs.
	if err := tconn.Eval(ctx, `(async () => {
		await tast.promisify(chrome.tabs.create)({});
	})()`, nil); err != nil {
		return errors.Wrap(err, "failed to create new tab")
	}
	var tabsToClose []int
	for _, t := range tabs {
		tabsToClose = append(tabsToClose, t.ID)
	}
	if err := CloseBrowserTabsByID(ctx, tconn, tabsToClose); err != nil {
		return errors.Wrap(err, "failed to close other browser tabs")
	}
	return nil
}

// CloseChrome closes Chrome browser application properly.
// If there exist unsave changes on web page, e.g. media content is playing or online document is editing,
// "leave site" prompt will prevent the tab from closing.
// This function confirms the "leave site" prompts so browser can be closed.
func CloseChrome(ctx context.Context, tconn *chrome.TestConn) error {
	chromeApp, err := apps.PrimaryBrowser(ctx, tconn)
	if err != nil {
		return errors.Wrap(err, "could not find the Chrome app")
	}

	if err := apps.Close(ctx, tconn, chromeApp.ID); err != nil {
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
// It will only look for the icon locate on internal display.
func appIconFinder(appName string, appOtherPossibleNames ...string) *nodewith.Finder {
	internalDisplay := nodewith.ClassName("RootWindow-0").Role(role.Window)
	finder := nodewith.ClassName("ash/ShelfAppButton").Ancestor(internalDisplay)

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

// extendedDisplayWindowClassName obtains the class name of the root window on the extended display.
// If multiple display windows are present, the first one will be returned.
func extendedDisplayWindowClassName(ctx context.Context, tconn *chrome.TestConn) (string, error) {
	ui := uiauto.New(tconn)

	// Root window on extended display has the class name in RootWindow-<id> format.
	// We found extended display window could be RootWindow-1, or RootWindow-2.
	// Here we try 1 to 10.
	for i := 1; i <= 10; i++ {
		className := fmt.Sprintf("RootWindow-%d", i)
		win := nodewith.ClassName(className).Role(role.Window)
		if err := ui.Exists(win)(ctx); err == nil {
			return className, nil
		}
	}
	return "", errors.New("failed to find any window with class name RootWindow-1 to RootWindow-10")
}

// SwitchWindowToDisplay switches current window to expected display.
func SwitchWindowToDisplay(ctx context.Context, tconn *chrome.TestConn, kb *input.KeyboardEventWriter, externalDisplay bool) action.Action {
	return func(ctx context.Context) error {
		var expectedRootWindow *nodewith.Finder
		var display string
		ui := uiauto.New(tconn)
		w, err := ash.FindWindow(ctx, tconn, func(w *ash.Window) bool {
			return w.IsActive && w.IsFrameVisible
		})
		if err != nil {
			return errors.Wrap(err, "failed to get current active window")
		}
		if externalDisplay {
			display = "external display"
			extendedWinClassName, err := extendedDisplayWindowClassName(ctx, tconn)
			if err != nil {
				return errors.Wrap(err, "failed to find root window on external display")
			}
			expectedRootWindow = nodewith.ClassName(extendedWinClassName).Role(role.Window)
		} else {
			display = "internal display"
			// Root window on built-in display.
			expectedRootWindow = nodewith.ClassName("RootWindow-0").Role(role.Window)
		}
		currentWindow := nodewith.Name(w.Title).Role(role.Window)
		expectedWindow := currentWindow.Ancestor(expectedRootWindow).First()
		if err := ui.Exists(expectedWindow)(ctx); err != nil {
			testing.ContextLog(ctx, "Expected window not found: ", err)
			testing.ContextLogf(ctx, "Switch window %q to %s", w.Title, display)
			return uiauto.Combine("switch window to "+display,
				kb.AccelAction("Search+Alt+M"),
				ui.WithTimeout(3*time.Second).WaitUntilExists(expectedWindow),
			)(ctx)
		}
		return nil
	}
}

// DismissMobilePrompt dismisses the prompt of "This app is designed for mobile".
func DismissMobilePrompt(ctx context.Context, tconn *chrome.TestConn) error {
	ui := uiauto.New(tconn)

	prompt := nodewith.Name("This app is designed for mobile").Role(role.Window)
	if err := ui.WithTimeout(5 * time.Second).WaitUntilExists(prompt)(ctx); err == nil {
		testing.ContextLog(ctx, "Dismiss the app prompt")
		gotIt := nodewith.Name("Got it").Role(role.Button).Ancestor(prompt)
		if err := ui.LeftClickUntil(gotIt, ui.WithTimeout(time.Second).WaitUntilGone(gotIt))(ctx); err != nil {
			return errors.Wrap(err, "failed to click 'Got it' button")
		}
	}
	return nil
}

// ExpandMenu returns a function that clicks the button and waits for the menu to expand to the given height.
// This function is useful when the target menu will expand to its full size with animation. On Low end DUTs
// the expansion animation might stuck for some time. The node might have returned a stable location if
// checking with a fixed interval before the animiation completes. This function ensures animation completes
// by checking the menu height.
func ExpandMenu(tconn *chrome.TestConn, button, menu *nodewith.Finder, height int) action.Action {
	ui := uiauto.New(tconn)
	startTime := time.Now()
	return func(ctx context.Context) error {
		if err := ui.LeftClick(button)(ctx); err != nil {
			return errors.Wrap(err, "failed to click button")
		}
		return testing.Poll(ctx, func(ctx context.Context) error {
			menuInfo, err := ui.Info(ctx, menu)
			if err != nil {
				return errors.Wrap(err, "failed to get menu info")
			}
			if menuInfo.Location.Height < height {
				return errors.Errorf("got menu height %d, want %d", menuInfo.Location.Height, height)
			}
			// Examine this log regularly to see how fast the menu is expanded and determine if
			// we still need to keep this ExpandMenu() function.
			testing.ContextLog(ctx, "Menu expanded to full height in ", time.Now().Sub(startTime))
			return nil
		}, &testing.PollOptions{Timeout: time.Minute, Interval: time.Second})
	}
}

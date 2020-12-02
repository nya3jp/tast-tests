// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package cuj

import (
	"context"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/ui"
	"chromiumos/tast/local/chrome/ui/pointer"
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

// GetClipboardTextData return clipboard text data
func GetClipboardTextData(ctx context.Context, tconn *chrome.TestConn) (string, error) {
	var data string

	if err := tconn.Call(ctx, &data, `tast.proimsify(chrome.autotestPrivate.getClipboardTextData)`); err != nil {
		return "", err
	}

	return data, nil
}

// SetClipboardDataAndWait forcibly sets the clipboard to data. Unlike setClipboardTextData, it
// checks the clipboard data and if it's still not yet the specified one, it repeats the
// setClipboardTextData.
func SetClipboardDataAndWait(ctx context.Context, tconn *chrome.TestConn, data string) error {
	if err := tconn.Call(ctx, nil, `tast.promisify(chrome.autotestPrivate.setClipboardTextData)`, data); err != nil {
		return err
	}
	return testing.Poll(ctx, func(ctx context.Context) error {
		var clipData string
		if err := tconn.Call(ctx, &clipData, `tast.promisify(chrome.autotestPrivate.getClipboardTextData)`); err != nil {
			return err
		}
		if clipData != data {
			return errors.Errorf("clipboard data missmatch: got %q, want %q", clipData, data)
		}
		return nil
	}, &testing.PollOptions{Timeout: 20 * time.Second})
}

// GetOpenBrowserStartTime open chrome browser and return the browser start time.
func GetOpenBrowserStartTime(ctx context.Context, cr *chrome.Chrome, tconn *chrome.TestConn, tabletMode bool) (time.Duration, error) {
	var browserStartTime time.Duration
	var startTime time.Time
	var launchErr error

	// Get the expected browser.
	chromeApp, err := apps.ChromeOrChromium(ctx, tconn)
	if err != nil {
		return browserStartTime, errors.Wrap(err, "could not find the Chrome app")
	}

	if tabletMode {
		testing.ContextLogf(ctx, "Launch %q from hotseat", chromeApp.Name)
		startTime, launchErr = LaunchAppFromHotseat(ctx, tconn, chromeApp)
	} else {
		testing.ContextLogf(ctx, "Launch %q from shelf", chromeApp.Name)
		startTime, launchErr = LaunchAppFromShelf(ctx, tconn, chromeApp)
	}

	if launchErr != nil {
		testing.ContextLogf(ctx, "Failed to open %q", chromeApp.Name)
		return browserStartTime, errors.Wrapf(launchErr, "failed to open %q", chromeApp.Name)
	}
	// Make sure app is launched.
	if err := ash.WaitForApp(ctx, tconn, chromeApp.ID); err != nil {
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
// It returns the time a mouse click event is injected to the app icon.
func LaunchAppFromShelf(ctx context.Context, tconn *chrome.TestConn, app apps.App) (time.Time, error) {
	params := ui.FindParams{Name: app.Name, ClassName: "ash/ShelfAppButton"}
	icon, err := ui.FindWithTimeout(ctx, tconn, params, 10*time.Second)
	if err != nil {
		return time.Time{}, errors.Wrapf(err, "failed to find app %q", app.Name)
	}
	defer icon.Release(ctx)

	// Click mouse to launch app.
	startTime := time.Now()
	if err := icon.LeftClick(ctx); err != nil {
		return startTime, errors.Wrapf(err, "failed to launch app %q", app.Name)
	}
	return startTime, nil
}

// LaunchAppFromHotseat opens an app by name which is currently pinned to the hotseat.
// It returns the time a touch event is injected to the app icon.
func LaunchAppFromHotseat(ctx context.Context, tconn *chrome.TestConn, app apps.App) (time.Time, error) {
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

	params := ui.FindParams{Name: app.Name, ClassName: "ash/ShelfAppButton"}
	icon, err := ui.FindWithTimeout(ctx, tconn, params, 10*time.Second)
	if err != nil {
		return startTime, errors.Wrapf(err, "failed to find app %q", app.Name)
	}
	defer icon.Release(ctx)

	// Press button to launch app.
	startTime = time.Now()
	x, y := tcc.ConvertLocation(icon.Location.CenterPoint())
	if err := stw.Move(x, y); err != nil {
		return startTime, errors.Wrapf(err, "failed to press icon %q", app.Name)
	}
	if err := stw.End(); err != nil {
		return startTime, errors.Wrapf(err, "failed to release pressed icon %q", app.Name)
	}
	return startTime, nil
}

// WaitUntilAllTabsLoaded waits all the tabs to finish loading.
func WaitUntilAllTabsLoaded(ctx context.Context, tconn *chrome.TestConn, timeout time.Duration) error {
	query := map[string]interface{}{
		"status":        "loading",
		"currentWindow": true,
	}
	return testing.Poll(ctx, func(ctx context.Context) error {
		var tabs []map[string]interface{}
		if err := tconn.Call(ctx, &tabs, `tast.promisify(chrome.tabs.query)`, query); err != nil {
			return testing.PollBreak(err)
		}
		if len(tabs) != 0 {
			return errors.Errorf("still %d tabs are loading", len(tabs))
		}
		return nil
	}, &testing.PollOptions{Timeout: timeout})
}

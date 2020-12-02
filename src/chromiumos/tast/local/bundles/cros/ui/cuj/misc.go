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
// the shelf icons and calling autotestPrivate.closeApp on each one.
func CloseAllWindows(ctx context.Context, tconn *chrome.TestConn) error {
	return tconn.Call(ctx, nil, `(async () => {
		  let items = await tast.promisify(chrome.autotestPrivate.getShelfItems)();
		  await Promise.all(items.map(item =>
		      tast.promisify(chrome.autotestPrivate.closeApp)(
		          item.appId.toString())));
		})()`, nil)
}

// GetClipboardTextData return clipboard text data
func GetClipboardTextData(ctx context.Context, tconn *chrome.TestConn) (string, error) {
	var data string

	if err := tconn.Call(ctx, &data, `tast.proimsify(chrome.autotestPrivate.getClipboardTextData)`); err != nil {
		return "", err
	}

	return data, nil
}

// ForceClipboard forcibly sets the clipboard to data. Unlike setClipboardTextData, it
// checks the clipboard data and if it's still not yet the specified one, it repeats the
// setClipboardTextData.
func ForceClipboard(ctx context.Context, tconn *chrome.TestConn, data string) error {
	return testing.Poll(ctx, func(ctx context.Context) error {
		if err := tconn.Call(ctx, nil, `tast.promisify(chrome.autotestPrivate.setClipboardTextData)`, data); err != nil {
			return err
		}
		var clipData string
		if err := tconn.Eval(ctx, `tast.promisify(chrome.autotestPrivate.getClipboardTextData)()`, &clipData); err != nil {
			return err
		}
		if clipData != data {
			return errors.Errorf("clipboard data missmatch: got %q, want %q", clipData, data)
		}
		return nil
	}, &testing.PollOptions{Timeout: 20 * time.Second})
}

// GetOpenBrowserStartTime open chrome browser and return the browser start time.
func GetOpenBrowserStartTime(ctx context.Context, tconn *chrome.TestConn, tabletMode bool) (int64, error) {
	var browserStartTime int64
	var startTime time.Time
	var launchErr error
	if tabletMode {
		testing.ContextLogf(ctx, "Launch %q from hotseat", apps.Chrome.Name)
		startTime, launchErr = LaunchAppFromHotseat(ctx, tconn, apps.Chrome)
	} else {
		testing.ContextLogf(ctx, "Launch %q from shelf", apps.Chrome.Name)
		startTime, launchErr = LaunchAppFromShelf(ctx, tconn, apps.Chrome)
	}
	defer apps.Close(ctx, tconn, apps.Chrome.ID)
	if launchErr != nil {
		testing.ContextLog(ctx, "Failed to open Google Chrome")
		return browserStartTime, errors.Wrap(launchErr, "failed to open Google Chrome")
	}
	// Make sure app is launched.
	if err := ash.WaitForApp(ctx, tconn, apps.Chrome.ID); err != nil {
		return browserStartTime, errors.Wrap(err, "failed to wait for the app to be launched")
	}
	endTime := time.Now()
	browserStartTime = endTime.Sub(startTime).Milliseconds()
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

// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package wmputils contains utility functions for wmp tests.
package wmputils

import (
	"context"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/browser"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/testing"
)

const (
	timeout  = 30 * time.Second
	interval = time.Second
)

// EnsureOnlyBrowserWindowOpen ensures that there is only one open window that is the primary browser, and waits for the browser to be visible to avoid a race that may cause test flakiness.
// If there is no or more than one browser window(s) open, it throws an error.
func EnsureOnlyBrowserWindowOpen(ctx context.Context, tconn *chrome.TestConn, bt browser.Type) (*ash.Window, error) {
	var w *ash.Window
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		// Check if there is only one open window.
		ws, err := ash.GetAllWindows(ctx, tconn)
		if err != nil {
			testing.PollBreak(errors.Wrap(err, "failed to get the window list"))
		}
		if len(ws) != 1 {
			return errors.Errorf("expected 1 window, got %v", len(ws))
		}

		// Check if that is the browser window and visible (!IsAnimating also used as heuristic criteria for readiness to accept inputs).
		w = ws[0]
		if !w.IsVisible || w.IsAnimating || !IsBrowserWindow(w, bt) {
			return errors.Errorf("expected %v browser window to become visible, State: %v", bt, w.State)
		}
		return nil
	}, &testing.PollOptions{Timeout: timeout, Interval: interval}); err != nil {
		return nil, errors.Wrap(err, "expected 1 visible browser window")
	}
	return w, nil
}

// IsBrowserWindow returns true if it's a browser window either ash-chrome or lacros-chrome.
func IsBrowserWindow(w *ash.Window, bt browser.Type) bool {
	return (bt == browser.TypeAsh && w.WindowType == ash.WindowTypeBrowser) ||
		(bt == browser.TypeLacros && w.WindowType == ash.WindowTypeLacros)
}

// WaitforAppLaunch waits for the app to launch.
func WaitforAppLaunch(ctx context.Context, tconn *chrome.TestConn, ac *uiauto.Context, appsList []apps.App) error {
	for _, app := range appsList {
		if err := ash.WaitForApp(ctx, tconn, app.ID, time.Minute); err != nil {
			return errors.Wrapf(err, "%s did not appear in shelf after launch", app.Name)
		}
	}

	if err := VerifyWindowCount(ctx, tconn, len(appsList)); err != nil {
		return errors.Wrap(err, "failed to verify window count")
	}

	return nil
}

// VerifyWindowCount verifies that there are `windowCount` the app windows.
func VerifyWindowCount(ctx context.Context, tconn *chrome.TestConn, windowCount int) error {
	ws, err := ash.GetAllWindows(ctx, tconn)
	if err != nil {
		return errors.Wrap(err, "failed to get all open windows")
	}
	if len(ws) != windowCount {
		return errors.Wrapf(err, "found inconsistent number of window(s): got %v, want %v", len(ws), windowCount)
	}

	return nil
}

// OpenApps opens a list of apps.
func OpenApps(ctx context.Context, tconn *chrome.TestConn, appsList []apps.App) error {
	for _, app := range appsList {
		if err := apps.Launch(ctx, tconn, app.ID); err != nil {
			return errors.Wrapf(err, "failed to open %s", app.Name)
		}
		if err := ash.WaitForApp(ctx, tconn, app.ID, time.Minute); err != nil {
			return errors.Wrapf(err, "%s did not appear in shelf after launch", app.Name)
		}
	}

	return nil
}

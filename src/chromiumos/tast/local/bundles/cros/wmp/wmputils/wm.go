// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package wmputils contains utility functions for wmp tests.
package wmputils

import (
	"context"
	"strings"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/browser"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/event"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
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
		if !w.IsVisible || w.IsAnimating || !ash.BrowserTypeMatch(bt)(w) {
			return errors.Errorf("expected %v browser window to become visible, State: %v", bt, w.State)
		}
		return nil
	}, &testing.PollOptions{Timeout: timeout, Interval: interval}); err != nil {
		return nil, errors.Wrap(err, "expected 1 visible browser window")
	}
	return w, nil
}

// VerifyWindowCount verifies that there are `windowCount` app windows.
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

// WaitforAppsToLaunch waits for the given apps to launch.
func WaitforAppsToLaunch(ctx context.Context, tconn *chrome.TestConn, ac *uiauto.Context, appsList []apps.App) error {
	for _, app := range appsList {
		if err := ash.WaitForApp(ctx, tconn, app.ID, time.Minute); err != nil {
			return errors.Wrapf(err, "%s did not appear in shelf after launch", app.Name)
		}

		// Some apps may take a long time to load such as Play Store. Wait for launch event to be completed.
		if err := ac.WithInterval(2*time.Second).WaitUntilNoEvent(nodewith.Root(), event.LocationChanged)(ctx); err != nil {
			return errors.Wrap(err, "failed to wait for the app launch event to be completed")
		}
	}

	if err := VerifyWindowCount(ctx, tconn, len(appsList)); err != nil {
		return errors.Wrap(err, "failed to verify window count")
	}

	return nil
}

// WaitforAppsToBeVisible waits for the windows of the given apps to be visible.
func WaitforAppsToBeVisible(ctx context.Context, tconn *chrome.TestConn, ac *uiauto.Context, appsList []apps.App) error {
	for _, app := range appsList {
		// Wait for the launched app window to become visible.
		if err := ash.WaitForCondition(ctx, tconn, func(w *ash.Window) bool {
			if !w.IsVisible {
				return false
			}
			// The window title of Lacros is suffixed with "Chrome", not "Lacros".
			if w.WindowType == ash.WindowTypeLacros {
				return strings.Contains(w.Title, "Chrome")
			}
			return strings.Contains(w.Title, app.Name)
		}, &testing.PollOptions{Timeout: 30 * time.Second}); err != nil {
			return errors.Wrapf(err, "%s app window not visible after launching", app.Name)
		}
	}

	return nil
}

// OpenApps opens the given apps, waits for them to launch and their windows to appear..
func OpenApps(ctx context.Context, tconn *chrome.TestConn, ac *uiauto.Context, appsList []apps.App) error {
	for _, app := range appsList {
		if err := apps.Launch(ctx, tconn, app.ID); err != nil {
			return errors.Wrapf(err, "failed to open %s", app.Name)
		}
	}

	if err := WaitforAppsToLaunch(ctx, tconn, ac, appsList); err != nil {
		return errors.Wrap(err, "failed to wait for app launch")
	}

	return nil
}

// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package wmputils contains utility functions for wmp tests.
package wmputils

import (
	"context"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/browser"
	"chromiumos/tast/testing"
)

const (
	timeout  = 30 * time.Second
	interval = time.Second
)

// CloseAllWindows closes all open windows and waits until gone.
func CloseAllWindows(ctx context.Context, tconn *chrome.TestConn) error {
	ws, err := ash.GetAllWindows(ctx, tconn)
	if err != nil {
		return errors.Wrap(err, "failed to get all open windows")
	}
	for _, w := range ws {
		if err := w.CloseWindow(ctx, tconn); err != nil {
			return errors.Wrapf(err, "failed to close window (%+v): %v", w, err)
		}
	}
	return nil
}

// EnsureOnlyBrowserWindowOpen ensures that there is only one open window that is the primary browser. Wait for the browser to be visible to avoid a race that may cause test flakiness.
// If there is no or more than one browser window(s) open, it throws an error.
func EnsureOnlyBrowserWindowOpen(ctx context.Context, tconn *chrome.TestConn, bt browser.Type) (*ash.Window, error) {
	var ws []*ash.Window
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		var err error
		ws, err = ash.GetAllWindows(ctx, tconn)
		if err != nil {
			testing.PollBreak(errors.Wrap(err, "failed to get the window list"))
		}
		if len(ws) != 1 {
			return errors.Errorf("expected 1 window, got %v", len(ws))
		}
		return nil
	}, &testing.PollOptions{Timeout: timeout, Interval: interval}); err != nil {
		return nil, errors.Wrapf(err, "expected 1 window, got %v", len(ws))
	}

	if err := ash.WaitForCondition(ctx, tconn, func(w *ash.Window) bool {
		if bt == browser.TypeAsh {
			return w.ID == ws[0].ID && w.IsVisible && !w.IsAnimating && w.WindowType == ash.WindowTypeBrowser
		}
		if bt == browser.TypeLacros {
			return w.ID == ws[0].ID && w.IsVisible && !w.IsAnimating && w.WindowType == ash.WindowTypeLacros
		}
		return false
	}, &testing.PollOptions{Timeout: timeout, Interval: interval}); err != nil {
		return nil, errors.Wrapf(err, "failed to wait for %v browser window to become visible, State: %v", bt, ws[0].State)
	}
	return ws[0], nil
}

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

// EnsureOnlyBrowserWindowOpen ensures that there is only one open window that is the primary browser, and waits for the browser to be visible to avoid a race that may cause test flakiness.
// If there is no or more than one browser window(s) open, it throws an error.
func EnsureOnlyBrowserWindowOpen(ctx context.Context, tconn *chrome.TestConn, bt browser.Type) (*ash.Window, error) {
	var ws []*ash.Window
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		// Check if there is only one open window.
		var err error
		ws, err = ash.GetAllWindows(ctx, tconn)
		if err != nil {
			testing.PollBreak(errors.Wrap(err, "failed to get the window list"))
		}
		if len(ws) != 1 {
			return errors.Errorf("expected 1 window, got %v", len(ws))
		}

		// Check if that is the browser window and visible (!IsAnimating also used as heuristic criteria for readiness to accept inputs).
		w := ws[0]
		if !w.IsVisible || w.IsAnimating ||
			(bt == browser.TypeAsh && w.WindowType != ash.WindowTypeBrowser) ||
			(bt == browser.TypeLacros && w.WindowType != ash.WindowTypeLacros) {
			return errors.Errorf("expected %v browser window to become visible, State: %v", bt, w.State)
		}
		return nil
	}, &testing.PollOptions{Timeout: timeout, Interval: interval}); err != nil {
		return nil, errors.Wrapf(err, "expected 1 visible browser window, got %v", len(ws))
	}
	return ws[0], nil
}

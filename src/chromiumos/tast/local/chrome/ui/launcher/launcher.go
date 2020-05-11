// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package launcher is used for controlling the launcher directly through the UI.
package launcher

import (
	"context"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ui"
	"chromiumos/tast/local/input"
)

const defaultTimeout = 15 * time.Second

// SearchAndLaunch searches a query in the launcher and executes it.
func SearchAndLaunch(ctx context.Context, tconn *chrome.TestConn, query string) error {
	if err := OpenLauncher(ctx, tconn); err != nil {
		return errors.Wrap(err, "failed to open launcher")
	}

	appNode, err := SearchAndWaitForApp(ctx, tconn, query, defaultTimeout)
	if err != nil {
		return errors.Wrap(err, "failed to find app")
	}

	if err := appNode.LeftClick(ctx); err != nil {
		return errors.Wrap(err, "failed to launch app")
	}
	return nil
}

// OpenLauncher opens the launcher.
func OpenLauncher(ctx context.Context, tconn *chrome.TestConn) error {
	keyboard, err := input.Keyboard(ctx)
	if err != nil {
		return err
	}
	defer keyboard.Close()

	return keyboard.Accel(ctx, "Search")
}

func search(ctx context.Context, tconn *chrome.TestConn, query string) error {
	// Click the search box.
	searchBox, err := ui.FindWithTimeout(ctx, tconn, ui.FindParams{ClassName: "SearchBoxView"}, defaultTimeout)
	if err != nil {
		return errors.Wrap(err, "failed to wait for launcher searchbox")
	}
	defer searchBox.Release(ctx)
	if err := searchBox.LeftClick(ctx); err != nil {
		return errors.Wrap(err, "failed to click launcher searchbox")
	}

	// Set up keyboard.
	kb, err := input.Keyboard(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to get keyboard")
	}
	defer kb.Close()

	// Search for anything by typing query string.
	if err := kb.Type(ctx, query); err != nil {
		return errors.Wrapf(err, "failed to type %q", query)
	}
	return nil
}

// SearchAndWaitForApp searches for APP name and wait for it to appear.
// Launcher should be opened already.
// timeout only applies to waiting for the presense of the app.
func SearchAndWaitForApp(ctx context.Context, tconn *chrome.TestConn, appName string, timeout time.Duration) (*ui.Node, error) {
	if err := search(ctx, tconn, appName); err != nil {
		return nil, errors.Wrapf(err, "failed to search app: %s", appName)
	}

	searchResultView, err := ui.FindWithTimeout(ctx, tconn, ui.FindParams{ClassName: "SearchResultPageView"}, time.Second)
	if err != nil {
		return nil, errors.Wrap(err, "failed to find Search Result Container")
	}

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	appNode, err := searchResultView.DescendantWithTimeout(ctx, ui.FindParams{Name: appName + ", Installed App"}, timeout)
	if err != nil {
		return nil, errors.Wrapf(err, "%s app does not exist in search result", appName)
	}
	return appNode, err
}

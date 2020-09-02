// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package launcher is used for controlling the launcher directly through the UI.
package launcher

import (
	"context"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ui"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
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

// OpenExpandedView opens the Launcher and go to Apps list page.
func OpenExpandedView(ctx context.Context, tconn *chrome.TestConn) error {
	if err := OpenLauncher(ctx, tconn); err != nil {
		return errors.Wrap(err, "failed to open Launcher")
	}
	params := ui.FindParams{Name: "Expand to all apps", ClassName: "ExpandArrowView"}
	expandArrowView, err := ui.FindWithTimeout(ctx, tconn, params, 10*time.Second)
	if err != nil {
		return errors.Wrap(err, "failed to find ExpandArrowView")
	}
	defer expandArrowView.Release(ctx)

	if err := expandArrowView.LeftClick(ctx); err != nil {
		return errors.Wrap(err, "failed to open expanded application list view")
	}
	return nil
}

// FindAppFromItemView finds the node handle of an application from the application the expanded launcher.
// This function assumes the expanded launcher is opened.
func FindAppFromItemView(ctx context.Context, tconn *chrome.TestConn, app apps.App) (*ui.Node, error) {
	params := ui.FindParams{Name: app.Name, ClassName: "ui/app_list/AppListItemView"}
	icon, err := ui.Find(ctx, tconn, params)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to find app %q", app.Name)
	}
	return icon, nil
}

// LaunchFromListView runs an app from the expanded launcher.
// This function assumes the expanded launcher is opened.
func LaunchFromListView(ctx context.Context, tconn *chrome.TestConn, app apps.App) error {
	icon, err := FindAppFromItemView(ctx, tconn, app)
	if err != nil {
		return err
	}
	defer icon.Release(ctx)
	if err := icon.LeftClick(ctx); err != nil {
		return errors.Wrap(err, "failed to run app")
	}
	// Make sure all items on the shelf are done moving.
	ui.WaitForLocationChangeCompleted(ctx, tconn)
	return nil
}

// PinAppToShelf pins an app from the expanded launcher to shelf.
// This function assumes the expanded launcher is opened.
func PinAppToShelf(ctx context.Context, tconn *chrome.TestConn, app apps.App) error {
	// Find the icon from the the expanded launcher.
	icon, err := FindAppFromItemView(ctx, tconn, app)
	if err != nil {
		return err
	}
	defer icon.Release(ctx)
	// Open context menu.
	if err := icon.RightClick(ctx); err != nil {
		return errors.Wrap(err, "failed to open context menu")
	}
	// Find option to pin app to shelf.
	params := ui.FindParams{Name: "Pin to shelf"}
	option, err := ui.FindWithTimeout(ctx, tconn, params, 10*time.Second)
	if err != nil {
		// The option pin to shelf is not available for this icon
		return errors.Wrap(err, `option "Pin to shelf" is not available`)
	}
	defer option.Release(ctx)
	// Pin app to shelf.
	if err := option.LeftClick(ctx); err != nil {
		return errors.Wrap(err, `failed to select option "Pin to shelf"`)
	}
	// Make sure all items on the shelf are done moving.
	ui.WaitForLocationChangeCompleted(ctx, tconn)
	return nil
}

func search(ctx context.Context, tconn *chrome.TestConn, query string) error {
	// Click the search box.
	searchBox, err := ui.FindWithTimeout(ctx, tconn, ui.FindParams{ClassName: "SearchBoxView"}, defaultTimeout)
	if err != nil {
		return errors.Wrap(err, "failed to wait for launcher searchbox")
	}
	defer searchBox.Release(ctx)

	condition := func(ctx context.Context) (bool, error) {
		return ui.Exists(ctx, tconn, ui.FindParams{ClassName: "SearchResultPageView"})
	}
	opts := testing.PollOptions{Timeout: defaultTimeout, Interval: 500 * time.Millisecond}
	if err := searchBox.LeftClickUntil(ctx, condition, &opts); err != nil {
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

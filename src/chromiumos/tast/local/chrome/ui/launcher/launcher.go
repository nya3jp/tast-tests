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

// Search executes a search query.
// Launcher should be open already.
func Search(ctx context.Context, tconn *chrome.TestConn, query string) error {
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

// WaitForAppResult waits for an app to appear as a search result.
// Launcher should be opened already and search done already.
// timeout only applies to waiting for the presence of the app.
func WaitForAppResult(ctx context.Context, tconn *chrome.TestConn, appName string, timeout time.Duration) (*ui.Node, error) {
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

// SearchAndWaitForApp searches for APP name and wait for it to appear.
// Launcher should be opened already.
// timeout only applies to waiting for the presence of the app.
func SearchAndWaitForApp(ctx context.Context, tconn *chrome.TestConn, appName string, timeout time.Duration) (*ui.Node, error) {
	if err := Search(ctx, tconn, appName); err != nil {
		return nil, errors.Wrapf(err, "failed to search app: %s", appName)
	}

	appNode, err := WaitForAppResult(ctx, tconn, appName, defaultTimeout)
	if err != nil {
		return nil, errors.Wrapf(err, "%s app does not exist in search result", appName)
	}
	return appNode, err
}

// OpenExpandedView opens the Launcher to the Apps list page.
func OpenExpandedView(ctx context.Context, tconn *chrome.TestConn) error {
	// TODO: Call autotestPrivate API instead after https://bugs.chromium.org/p/chromium/issues/detail?id=1127384 is implemented.
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
	// Make sure items are done moving.
	ui.WaitForLocationChangeCompleted(ctx, tconn)
	return nil
}

// RenameFolder renames a folder to a new name.
func RenameFolder(ctx context.Context, tconn *chrome.TestConn, from, to string) error {
	// Chrome add prefix "Folder " to all folder names in ui/app_list/AppListItemView.
	fromFolderSearchName := "Folder " + from
	toFolderSearchName := "Folder " + to

	// Find folder icon.
	params := ui.FindParams{Name: fromFolderSearchName, ClassName: "ui/app_list/AppListItemView"}
	folder, err := ui.Find(ctx, tconn, params)
	if err != nil {
		return errors.Wrap(err, "failed to find folder")
	}
	defer folder.Release(ctx)

	// Select folder icon to open text field.
	condition := func(ctx context.Context) (bool, error) {
		return ui.Exists(ctx, tconn, ui.FindParams{Name: from, ClassName: "Textfield"})
	}
	opts := testing.PollOptions{Timeout: 5 * time.Second, Interval: 500 * time.Millisecond}
	if err := folder.LeftClickUntil(ctx, condition, &opts); err != nil {
		return errors.Wrap(err, "failed to left click folder")
	}
	ui.WaitForLocationChangeCompleted(ctx, tconn)

	// Select text field.
	params = ui.FindParams{Name: from, ClassName: "Textfield"}
	textField, err := ui.Find(ctx, tconn, params)
	if err != nil {
		return errors.Wrap(err, "failed to find text field ")
	}
	defer textField.Release(ctx)
	if err := textField.LeftClick(ctx); err != nil {
		return errors.Wrap(err, "failed to select text field")
	}

	// Use keyboard to change text field name.
	kb, err := input.Keyboard(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to get keyboard")
	}
	defer kb.Close()
	if err := kb.Type(ctx, to+"\n"); err != nil {
		return errors.Wrap(err, "failed to input new folder name")
	}

	// Make sure folder has the new name.
	params = ui.FindParams{Name: toFolderSearchName, ClassName: "ui/app_list/AppListItemView"}
	if err := ui.WaitUntilExists(ctx, tconn, params, 10*time.Second); err != nil {
		return errors.Wrap(err, "failed to verify name of renamed folder")
	}

	return nil
}

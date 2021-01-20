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
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/ui"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

const defaultTimeout = 15 * time.Second

// ExpandedItemsClass define the class name of the expanded launcher view which is used as search parameters in ui.
const ExpandedItemsClass = "AppListItemView"

// SearchAndLaunchWithQuery searches a query in the launcher and executes an app from the list.
func SearchAndLaunchWithQuery(ctx context.Context, tconn *chrome.TestConn, query, appName string) error {
	if err := OpenLauncher(ctx, tconn); err != nil {
		return errors.Wrap(err, "failed to open launcher")
	}

	appNode, err := SearchAndWaitForApp(ctx, tconn, query, appName, defaultTimeout)
	if err != nil {
		return errors.Wrap(err, "failed to find app")
	}
	defer appNode.Release(ctx)

	if err := appNode.StableLeftClick(ctx, &testing.PollOptions{Timeout: defaultTimeout}); err != nil {
		return errors.Wrap(err, "failed to launch app")
	}
	return nil
}

// SearchAndLaunch searches an app in the launcher and executes it.
func SearchAndLaunch(ctx context.Context, tconn *chrome.TestConn, appName string) error {
	return SearchAndLaunchWithQuery(ctx, tconn, appName, appName)
}

// OpenLauncher opens the launcher.
func OpenLauncher(ctx context.Context, tconn *chrome.TestConn) error {
	return OpenExpandedView(ctx, tconn)
}

// Search executes a search query.
// Launcher should be open already.
func Search(ctx context.Context, tconn *chrome.TestConn, query string) error {
	// Click the search box.
	if err := ui.StableFindAndClick(ctx, tconn, ui.FindParams{ClassName: "SearchBoxView"}, &testing.PollOptions{Timeout: defaultTimeout}); err != nil {
		return errors.Wrap(err, "failed to find and click launcher searchbox")
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
// Launcher search result window is refreshed sometimes.
// Regularly check search result window and node until it is positioned.
// For more details, refer to b/169542037.
func WaitForAppResult(ctx context.Context, tconn *chrome.TestConn, appName string, timeout time.Duration) (*ui.Node, error) {
	var appNode *ui.Node
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		searchResultView, err := ui.FindWithTimeout(ctx, tconn, ui.FindParams{ClassName: "SearchResultPageView"}, time.Second)
		if err != nil {
			return errors.Wrap(err, "failed to find Search Result Container")
		}
		defer searchResultView.Release(ctx)

		appNode, err = searchResultView.DescendantWithTimeout(ctx, ui.FindParams{Name: appName + ", Installed App"}, time.Second)
		if err != nil {
			// Try blocked apps as well.
			appNode, err = searchResultView.DescendantWithTimeout(ctx, ui.FindParams{Name: appName + ", Installed App, Blocked"}, time.Second)
			if err != nil {
				return errors.Wrapf(err, "%s app does not exist in search result", appName)
			}
		}
		if err := appNode.WaitLocationStable(ctx, &testing.PollOptions{Interval: 1 * time.Second, Timeout: 3 * time.Second}); err != nil {
			appNode.Release(ctx)
			return errors.Wrap(err, "failed to wait for search result window positioned")
		}
		return nil
	}, &testing.PollOptions{Timeout: timeout}); err != nil {
		return nil, errors.Wrap(err, "failed to wait for search result")
	}

	return appNode, nil
}

// SearchAndWaitForApp searches for a query and waits for the app to appear.
// Launcher should be opened already.
// timeout only applies to waiting for the presence of the app.
func SearchAndWaitForApp(ctx context.Context, tconn *chrome.TestConn, query, appName string, timeout time.Duration) (*ui.Node, error) {
	if err := Search(ctx, tconn, query); err != nil {
		return nil, errors.Wrapf(err, "failed to search %s for app %s", query, appName)
	}

	appNode, err := WaitForAppResult(ctx, tconn, appName, defaultTimeout)
	if err != nil {
		return nil, errors.Wrapf(err, "%s app does not exist in search result", appName)
	}
	return appNode, err
}

// OpenExpandedView opens the Launcher to the Apps list page.
func OpenExpandedView(ctx context.Context, tconn *chrome.TestConn) error {
	// TODO: Call autotestPrivate API instead after http://crbug.com/1127384 is implemented.
	params := ui.FindParams{ClassName: ExpandedItemsClass}
	exist, err := ui.Exists(ctx, tconn, params)
	if err != nil {
		return errors.Wrap(err, "cannot check if AppListItemView exists")
	}
	if exist {
		// Even if it exist, active window may cover it in tablet mode. Check for active windows.
		windows, err := ash.FindAllWindows(ctx, tconn, func(window *ash.Window) bool {
			return window.IsVisible
		})
		if err != nil {
			return errors.Wrap(err, "failed to get all windows")
		}
		// Do nothing if there are no windows. Homescreen should be there already.
		if len(windows) == 0 {
			return nil
		}
	}

	if err := ash.TriggerLauncherStateChange(ctx, tconn, ash.AccelShiftSearch); err != nil {
		return errors.Wrap(err, "failed to switch to fullscreen")
	}
	if err := ash.WaitForLauncherState(ctx, tconn, ash.FullscreenAllApps); err != nil {
		return errors.Wrap(err, "failed to switch the state to 'FullscreenAllApps'")
	}
	return nil
}

// FindAppFromItemView finds the node handle of an application from the expanded launcher.
func FindAppFromItemView(ctx context.Context, tconn *chrome.TestConn, app apps.App) (*ui.Node, error) {
	if err := OpenExpandedView(ctx, tconn); err != nil {
		return nil, errors.Wrapf(err, "failed to expand launcher while looking for app %q", app.Name)
	}
	params := ui.FindParams{Name: app.Name, ClassName: ExpandedItemsClass}
	icon, err := ui.FindWithTimeout(ctx, tconn, params, defaultTimeout)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to find app %q", app.Name)
	}
	return icon, nil
}

// LaunchApp runs an app from the expanded launcher.
func LaunchApp(ctx context.Context, tconn *chrome.TestConn, app apps.App) error {
	icon, err := FindAppFromItemView(ctx, tconn, app)
	if err != nil {
		return err
	}
	defer icon.Release(ctx)

	// To handle the case of icon is off screen, we first make sure the icon is in focus first.
	if err := icon.FocusAndWait(ctx, 10*time.Second); err != nil {
		return errors.Wrap(err, "failed to focus on icon")
	}

	if err := icon.LeftClick(ctx); err != nil {
		return errors.Wrap(err, "failed to run app")
	}
	// Make sure app appears on the shelf.
	if err := ash.WaitForApp(ctx, tconn, app.ID); err != nil {
		return errors.Wrapf(err, "failed to wait for app %q: ", app.Name)
	}
	// Make sure all items on the shelf are done moving.
	if err := ui.WaitForLocationChangeCompleted(ctx, tconn); err != nil {
		return errors.Wrap(err, "failed to wait for location change to be completed")
	}
	return nil
}

// PinAppToShelf pins an app from the expanded launcher to shelf.
func PinAppToShelf(ctx context.Context, tconn *chrome.TestConn, app apps.App) error {
	// Find the icon from the the expanded launcher.
	icon, err := FindAppFromItemView(ctx, tconn, app)
	if err != nil {
		return err
	}
	defer icon.Release(ctx)

	// To handle the case of icon is off screen, we first make sure the icon is in focus first.
	if err := icon.FocusAndWait(ctx, 10*time.Second); err != nil {
		return errors.Wrap(err, "failed to focus on icon")
	}

	// Open context menu.
	if err := icon.RightClick(ctx); err != nil {
		return errors.Wrap(err, "failed to open context menu")
	}
	// Find option to pin app to shelf.
	params := ui.FindParams{Name: "Pin to shelf", ClassName: "MenuItemView"}
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
	if err := ui.WaitForLocationChangeCompleted(ctx, tconn); err != nil {
		return errors.Wrap(err, "failed to wait for location change to be completed")
	}
	return nil
}

// RenameFolder renames a folder to a new name.
func RenameFolder(ctx context.Context, tconn *chrome.TestConn, from, to string, kb *input.KeyboardEventWriter) error {
	// Make sure expanded launcher view is open.
	if err := OpenExpandedView(ctx, tconn); err != nil {
		return errors.Wrap(err, "failed to open expanded launcher before renaming folder")
	}

	// Chrome add prefix "Folder " to all folder names in AppListItemView.
	fromFolderSearchName := "Folder " + from
	toFolderSearchName := "Folder " + to

	// Find and click folder icon.
	params := ui.FindParams{Name: fromFolderSearchName, ClassName: ExpandedItemsClass}
	if err := ui.StableFindAndClick(ctx, tconn, params, &testing.PollOptions{Timeout: 10 * time.Second}); err != nil {
		return errors.Wrapf(err, "failed to find folder %q", from)
	}
	// Make sure the folder is fully opened before we do the next UI operation.
	if err := ui.WaitForLocationChangeCompleted(ctx, tconn); err != nil {
		return errors.Wrap(err, "failed to wait for location change to be completed")
	}

	// Click label.
	params = ui.FindParams{Name: from, ClassName: "Label"}
	if err := ui.StableFindAndClick(ctx, tconn, params, &testing.PollOptions{Timeout: 10 * time.Second}); err != nil {
		return errors.Wrap(err, "failed to find and click label")
	}

	// Use keyboard to change text field name.
	if err := kb.Type(ctx, to+"\n"); err != nil {
		return errors.Wrap(err, "failed to input new folder name")
	}

	// Make sure folder has the new name.
	params = ui.FindParams{Name: toFolderSearchName, ClassName: ExpandedItemsClass}
	if err := ui.WaitUntilExists(ctx, tconn, params, 10*time.Second); err != nil {
		return errors.Wrap(err, "failed to verify name of renamed folder")
	}

	return nil
}

// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package launcher is used for controlling the launcher directly through the UI.
package launcher

import (
	"context"
	"fmt"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/input"
)

// ExpandedItemsClass define the class name of the expanded launcher view which is used as search parameters in ui.
const ExpandedItemsClass = "AppListItemView"

// SearchResultListItemFinder is the finder of the list items in launcher search result.
var SearchResultListItemFinder = nodewith.ClassName("ui/app_list/SearchResultView")

// SearchAndWaitForAppOpen return a function that searches for an app, launches it, and waits for it to be open.
func SearchAndWaitForAppOpen(tconn *chrome.TestConn, kb *input.KeyboardEventWriter, app apps.App) uiauto.Action {
	return uiauto.Combine(fmt.Sprintf("SearchAndWaitForAppOpen(%+q)", app),
		SearchAndLaunch(tconn, kb, app.Name),
		func(ctx context.Context) error {
			return ash.WaitForApp(ctx, tconn, app.ID, time.Minute)
		},
	)
}

// SearchAndLaunch return a function that searches an app in the launcher and executes it.
func SearchAndLaunch(tconn *chrome.TestConn, kb *input.KeyboardEventWriter, appName string) uiauto.Action {
	return SearchAndLaunchWithQuery(tconn, kb, appName, appName)
}

// SearchAndLaunchWithQuery return a function that searches a query in the launcher and executes an app from the list.
func SearchAndLaunchWithQuery(tconn *chrome.TestConn, kb *input.KeyboardEventWriter, query, appName string) uiauto.Action {
	ui := uiauto.New(tconn)
	return uiauto.Combine(fmt.Sprintf("SearchAndLaunchWithQuery(%s, %s)", query, appName),
		Open(tconn),
		Search(tconn, kb, query),
		ui.WithInterval(1*time.Second).LeftClick(AppSearchFinder(appName)),
	)
}

// SearchAndRightClick returns a function that searches a query in the launcher and right click the app from the list.
// It right clicks the app until a menu item is displayed.
func SearchAndRightClick(tconn *chrome.TestConn, kb *input.KeyboardEventWriter, query, appName string) uiauto.Action {
	ui := uiauto.New(tconn)
	menuItem := nodewith.Role(role.MenuItem).First()
	app := AppSearchFinder(appName)
	return uiauto.Combine(fmt.Sprintf("SearchAndRightClick(%s, %s)", query, appName),
		Open(tconn),
		Search(tconn, kb, query),
		ui.WithTimeout(time.Minute).WithInterval(10*time.Second).RightClickUntil(app, ui.WaitUntilExists(menuItem)),
	)
}

// Open return a function that opens the launcher.
func Open(tconn *chrome.TestConn) uiauto.Action {
	return OpenExpandedView(tconn)
}

// OpenExpandedView return a function that opens the Launcher to the Apps list page.
func OpenExpandedView(tconn *chrome.TestConn) uiauto.Action {
	return func(ctx context.Context) error {
		// TODO: Call autotestPrivate API instead after http://crbug.com/1127384 is implemented.
		ui := uiauto.New(tconn)
		if err := ui.Exists(nodewith.ClassName(ExpandedItemsClass).First())(ctx); err == nil {
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
}

// AppSearchFinder returns a Finder to find the specified app in an open launcher's search results.
func AppSearchFinder(appName string) *nodewith.Finder {
	searchResultView := nodewith.ClassName("SearchResultPageView")
	return nodewith.NameStartingWith(appName + ", Installed App").Ancestor(searchResultView)
}

// AppItemViewFinder returns a Finder to find the specified app in an open launcher's item view.
func AppItemViewFinder(appName string) *nodewith.Finder {
	return nodewith.Name(appName).ClassName(ExpandedItemsClass)
}

// Search return a function that executes a search query.
// Launcher should be open already.
func Search(tconn *chrome.TestConn, kb *input.KeyboardEventWriter, query string) uiauto.Action {
	return func(ctx context.Context) error {
		// Click the search box.
		ui := uiauto.New(tconn)
		if err := ui.LeftClick(nodewith.ClassName("SearchBoxView").Role(role.Group))(ctx); err != nil {
			return errors.Wrap(err, "failed to click launcher searchbox")
		}

		// Search for anything by typing query string.
		if err := kb.Type(ctx, query); err != nil {
			return errors.Wrapf(err, "failed to type %q", query)
		}
		return nil
	}
}

// LaunchAndWaitForAppOpen return a function that launches an app from the expanded launcher and waits for it to be open.
func LaunchAndWaitForAppOpen(tconn *chrome.TestConn, app apps.App) uiauto.Action {
	return uiauto.Combine(fmt.Sprintf("LaunchAndWaitForAppOpen(%+q)", app),
		LaunchApp(tconn, app.Name),
		func(ctx context.Context) error {
			return ash.WaitForApp(ctx, tconn, app.ID, time.Minute)
		},
	)
}

// LaunchApp return a function that launches an app from the expanded launcher.
func LaunchApp(tconn *chrome.TestConn, appName string) uiauto.Action {
	ui := uiauto.New(tconn)
	return uiauto.Combine(fmt.Sprintf("LaunchApp(%s)", appName),
		OpenExpandedView(tconn),
		ui.FocusAndWait(AppItemViewFinder(appName)),
		ui.LeftClick(AppItemViewFinder(appName)),
	)
}

// PinAppToShelf return a function that pins an app from the expanded launcher to shelf.
func PinAppToShelf(tconn *chrome.TestConn, app apps.App) uiauto.Action {
	ui := uiauto.New(tconn)
	return uiauto.Combine(fmt.Sprintf("PinAppToShelf(%+q)", app),
		OpenExpandedView(tconn),
		ui.FocusAndWait(AppItemViewFinder(app.Name)),
		ui.RightClick(AppItemViewFinder(app.Name)),
		ui.LeftClick(nodewith.Name("Pin to shelf").ClassName("MenuItemView")),
	)
}

// RenameFolder return a function that renames a folder to a new name.
func RenameFolder(tconn *chrome.TestConn, kb *input.KeyboardEventWriter, from, to string) uiauto.Action {
	// Chrome add prefix "Folder " to all folder names in AppListItemView.
	fromFolder := nodewith.Name("Folder " + from).ClassName(ExpandedItemsClass)
	toFolder := nodewith.Name("Folder " + to).ClassName(ExpandedItemsClass)
	ui := uiauto.New(tconn)
	return uiauto.Combine(fmt.Sprintf("RenameFolder(%s, %s)", from, to),
		OpenExpandedView(tconn),
		ui.LeftClick(fromFolder),
		ui.FocusAndWait(nodewith.Name(from).ClassName("Textfield")),
		func(ctx context.Context) error {
			return kb.Type(ctx, to+"\n")
		},
		ui.WaitUntilExists(toFolder),
	)
}

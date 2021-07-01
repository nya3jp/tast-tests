// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package launcher is used for controlling the launcher directly through the UI.
package launcher

import (
	"context"
	"fmt"
	"regexp"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/event"
	"chromiumos/tast/local/chrome/uiauto/mouse"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/coords"
	"chromiumos/tast/local/input"
)

// ExpandedItemsClass define the class name of the expanded launcher view which is used as search parameters in ui.
const ExpandedItemsClass = "AppListItemView"

// UnnamedFolderFinder is the finder of a newly created folder with the default name.
var UnnamedFolderFinder = nodewith.Name("Folder Unnamed").ClassName(ExpandedItemsClass)

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

// CreateFolder is a helper function to create a folder by dragging the first icon on top of the second icon.
func CreateFolder(ctx context.Context, tconn *chrome.TestConn) uiauto.Action {
	// Create a folder in launcher by dragging the first icon on top of the second icon.
	ui := uiauto.New(tconn)
	return uiauto.Combine("createFolder",
		DragIconToIcon(tconn, 1, 0),
		ui.WaitUntilExists(UnnamedFolderFinder),
	)
}

// DragIconToIcon drags from one icon to another icon.
func DragIconToIcon(tconn *chrome.TestConn, srcIndex, destIndex int) uiauto.Action {
	src := nodewith.ClassName(ExpandedItemsClass).Nth(srcIndex)
	dest := nodewith.ClassName(ExpandedItemsClass).Nth(destIndex)
	return DragItemToItem(tconn, src, dest)
}

// DragItemToItem drags from the src item to the dest item.
// We cannot use simple mouse.Drag because the default UI behavior is now that the icon location changes after you start a drag.
// This function will delay the calculation of the destination point until after mouse press.
func DragItemToItem(tconn *chrome.TestConn, src, dest *nodewith.Finder) uiauto.Action {
	const duration = time.Second
	return func(ctx context.Context) error {
		ui := uiauto.New(tconn)
		start, err := ui.Location(ctx, src)
		if err != nil {
			return errors.Wrap(err, "failed to get location for first icon")
		}
		if err := mouse.Move(tconn, start.CenterPoint(), 0)(ctx); err != nil {
			return errors.Wrap(err, "failed to move to the start location")
		}
		if err := mouse.Press(tconn, mouse.LeftButton)(ctx); err != nil {
			return errors.Wrap(err, "failed to press the button")
		}

		// Move a little bit first to trigger launcher-app-paging.
		if err := mouse.Move(tconn, start.CenterPoint().Add(coords.Point{X: 1, Y: 1}), time.Second)(ctx); err != nil {
			return errors.Wrap(err, "failed to move the mouse")
		}

		// Get destination location during drag.
		end, err := ui.Location(ctx, dest)
		if err != nil {
			return errors.Wrap(err, "failed to get location for second icon")
		}
		if err := mouse.Move(tconn, end.CenterPoint(), 200*time.Millisecond)(ctx); err != nil {
			return errors.Wrap(err, "failed to move the mouse")
		}
		return mouse.Release(tconn, mouse.LeftButton)(ctx)
	}
}

// RemoveIconFromFolder opens a folder and drags an icon out of the folder.
func RemoveIconFromFolder(tconn *chrome.TestConn) uiauto.Action {
	return func(ctx context.Context) error {
		ui := uiauto.New(tconn)
		// Click to open the folder.
		if err := ui.LeftClick(UnnamedFolderFinder)(ctx); err != nil {
			return errors.Wrap(err, "failed to click the folder")
		}

		// Get the location for the first item in the folder.
		folderItems := nodewith.ClassName(ExpandedItemsClass).Ancestor(nodewith.ClassName("AppListFolderView"))
		start, err := ui.Location(ctx, folderItems.Nth(0))
		if err != nil {
			return errors.Wrap(err, "failed to get the location of the first folder item")
		}

		// Get a point outside of the folder view.
		folderView := nodewith.ClassName("AppListFolderView")
		folderViewLocation, err := ui.Location(ctx, folderView)
		if err != nil {
			return errors.Wrap(err, "failed to get folderViewLocation")
		}
		pointOutsideFolder := coords.NewPoint(folderViewLocation.Right()+50, folderViewLocation.CenterY()+50)

		// Drag the first folder item outside of the folder.
		mouse.Move(tconn, start.CenterPoint(), 0)(ctx)
		mouse.Press(tconn, mouse.LeftButton)(ctx)
		mouse.Move(tconn, pointOutsideFolder, time.Second)(ctx)

		// Get the location of the folder during the drag.
		folderLocation, err := ui.Location(ctx, UnnamedFolderFinder)
		if err != nil {
			return errors.Wrap(err, "failed to get location for folder")
		}

		// Drag app to the right of the folder.
		mouse.Move(tconn, folderLocation.CenterPoint().Add(coords.Point{X: folderLocation.Width, Y: 0}), time.Second)(ctx)

		// Release the mouse, ending the drag.
		mouse.Release(tconn, mouse.LeftButton)(ctx)

		// Make sure that the folder has closed.
		err = ui.Gone(folderView)(ctx)
		if err != nil {
			errors.Wrap(err, "folderView is not gone")
		}

		return nil
	}
}

// AddItemsToFolder adds non-folder items to the specified folder.
// Assumes that the folder is on the current page.
// If the next available item to add is not on the current page, then the folder will be moved to the next page.
// If the folder is full, then the attempt will still be made to add the item to the folder, and no error will be returned for that case.
func AddItemsToFolder(ctx context.Context, tconn *chrome.TestConn, folder *nodewith.Finder, numItemsToAdd int) error {
	numItemsInFolder, err := GetFolderSize(ctx, tconn, folder)
	if err != nil {
		return errors.Wrap(err, "failed to get folder size")
	}
	itemToAddIndex := 0
	targetTotalItems := numItemsInFolder + numItemsToAdd

	for numItemsInFolder < targetTotalItems {
		item := nodewith.ClassName(ExpandedItemsClass).Nth(itemToAddIndex)

		// Check that item is on the current page, otherwise move the folder to the next page.
		onPage, err := IsItemOnCurrentPage(ctx, tconn, item)
		if err != nil {
			return errors.Wrap(err, "failed to check if the item is on the current page")
		}
		if !onPage {
			err = DragIconToNextPage(tconn)(ctx)
			if err != nil {
				return errors.Wrap(err, "failed to drag icon to the next page")
			}
		}

		// Cannot add folders to a folder so skip over folder items.
		isFolder, err := IsFolderItem(ctx, tconn, item)
		if err != nil {
			return errors.Wrap(err, "failed to check if item is a folder")
		}
		if isFolder {
			itemToAddIndex++
			continue
		}

		// Add the item to the folder.
		err = DragItemToItem(tconn, item, folder)(ctx)
		if err != nil {
			return errors.Wrap(err, "failed to drag icon into a folder")
		}
		numItemsInFolder++
	}
	return nil
}

// DragIconToNextPage drags an icon to the next page of the app list.
func DragIconToNextPage(tconn *chrome.TestConn) uiauto.Action {
	return func(ctx context.Context) error {
		ui := uiauto.New(tconn)
		src := nodewith.ClassName(ExpandedItemsClass).Nth(0)
		// Move and press the mouse on the source icon.
		start, err := ui.Location(ctx, src)
		if err != nil {
			return errors.Wrap(err, "failed to get location for first icon")
		}
		if err := mouse.Move(tconn, start.CenterPoint(), 0)(ctx); err != nil {
			return errors.Wrap(err, "failed to move to the start location")
		}
		if err := mouse.Press(tconn, mouse.LeftButton)(ctx); err != nil {
			return errors.Wrap(err, "failed to press the icon")
		}

		// Move a little bit first to trigger launcher-app-paging.
		if err := mouse.Move(tconn, start.CenterPoint().Add(coords.Point{X: 1, Y: 1}), time.Second)(ctx); err != nil {
			return errors.Wrap(err, "failed to move the mouse")
		}

		// Get destination location during drag.
		end, err := ui.Location(ctx, nodewith.ClassName("AppsGridView"))
		if err != nil {
			return errors.Wrap(err, "failed to get location for AppsGridView")
		}
		endPoint := coords.NewPoint(end.CenterPoint().X, end.Bottom())

		// Drag icon to the bottom of the AppsGridView.
		if err := mouse.Move(tconn, endPoint, 200*time.Millisecond)(ctx); err != nil {
			return errors.Wrap(err, "failed to move the mouse")
		}

		// Move a little bit and wait for page change.
		pageSwitcher := nodewith.ClassName("PageSwitcher")
		if err := ui.WaitForEvent(pageSwitcher, event.Alert, mouse.Move(tconn, endPoint.Add(coords.Point{X: 1, Y: 0}), time.Second))(ctx); err != nil {
			return errors.Wrap(err, "failed to wait for page change event")
		}

		// Move icon to near the top left of the AppsGridView and release to place the icon at the start of the page.
		if err := mouse.Move(tconn, end.TopLeft().Add(coords.Point{X: 10, Y: 10}), 200*time.Millisecond)(ctx); err != nil {
			return errors.Wrap(err, "failed to move to near the top left of the AppsGridView")
		}
		return mouse.Release(tconn, mouse.LeftButton)(ctx)
	}
}

// GetFolderSize opens the given folder, gets the number of apps inside, and then closes the folder.
// This assumes that there is no folder open and may not work if a folder is already opened.
func GetFolderSize(ctx context.Context, tconn *chrome.TestConn, folder *nodewith.Finder) (int, error) {
	ui := uiauto.New(tconn)
	// Click to open the folder.
	if err := ui.LeftClick(folder)(ctx); err != nil {
		return 0, errors.Wrap(err, "failed to click the folder")
	}

	// Get |folderItemsInfo| which is used to get the size of the folder.
	folderView := nodewith.ClassName("AppListFolderView")
	folderItems := nodewith.ClassName("AppListItemView").Ancestor(folderView)
	folderItemsInfo, err := ui.NodesInfo(ctx, folderItems)
	if err != nil {
		return 0, errors.Wrap(err, "failed to find folderItemsInfo")
	}

	// Get and click on a location outside of the folder to close it.
	folderViewLocation, err := ui.Location(ctx, folderView)
	if err != nil {
		return 0, errors.Wrap(err, "failed to get folderViewLocation")
	}
	pointOutsideFolder := coords.NewPoint(folderViewLocation.Right()+50, folderViewLocation.CenterY()+50)
	// Click to close the folder.
	if err := mouse.Click(tconn, pointOutsideFolder, mouse.LeftButton)(ctx); err != nil {
		return 0, errors.Wrap(err, "failed to click outside of the folder")
	}

	return len(folderItemsInfo), nil
}

// IsItemOnCurrentPage will return whether the item is shown on the current launcher page.
// Assumes that there is no folder open, and may not work if a folder is opened.
func IsItemOnCurrentPage(ctx context.Context, tconn *chrome.TestConn, item *nodewith.Finder) (bool, error) {
	ui := uiauto.New(tconn)
	itemLocation, err := ui.Location(ctx, item)
	if err != nil {
		return false, errors.Wrap(err, "failed to get location for the item")
	}

	gridLocation, err := ui.Location(ctx, nodewith.ClassName("AppsGridView"))
	if err != nil {
		return false, errors.Wrap(err, "failed to get location for the AppsGridView")
	}

	return gridLocation.Contains(*itemLocation), nil
}

// IsFolderItem returns whether the item is a folder. Assumes that there is no folder open.
func IsFolderItem(ctx context.Context, tconn *chrome.TestConn, item *nodewith.Finder) (bool, error) {
	ui := uiauto.New(tconn)
	i, err := ui.Info(ctx, item)
	if err != nil {
		return false, errors.Wrap(err, "failed to get location for the item")
	}

	// If the name of the item begins with Folder, then it is considered a folder.
	match, err := regexp.MatchString("^Folder ", i.Name)
	if err != nil {
		return false, errors.Wrap(err, "failed to match name with regexp")
	}
	return match, nil
}

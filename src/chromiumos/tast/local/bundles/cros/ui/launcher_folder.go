// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"
	"time"

	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/ui/mouse"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/coords"

	// "chromiumos/tast/local/chrome/uiauto/mouse"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/power"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         LauncherFolder,
		Desc:         "Test foldering actions in the launcher",
		Contacts:     []string{"mmourgos@chromium.org"},
		Attr:         []string{"group:crosbolt", "crosbolt_perbuild"},
		SoftwareDeps: []string{"chrome"},
		HardwareDeps: hwdep.D(hwdep.InternalDisplay()),
		Fixture:      "chromeLoggedInWith100FakeApps",
	})
}

func LauncherFolder(ctx context.Context, s *testing.State) {
	// Ensure display on to record ui performance correctly.
	if err := power.TurnOnDisplay(ctx); err != nil {
		s.Fatal("Failed to turn on display: ", err)
	}

	cr := s.FixtValue().(*chrome.Chrome)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect to test API: ", err)
	}

	cleanup, err := ash.EnsureTabletModeEnabled(ctx, tconn, false)
	if err != nil {
		s.Fatal("Failed to ensure in clamshell mode: ", err)
	}
	defer cleanup(ctx)

	// Currently tast test may show a couple of notifications like "sign-in error"
	// and they may overlap with UI components of launcher. This prevents intended
	// actions on certain devices and causes test failures. Open and close the
	// quick settings to dismiss those notification popups. See
	// https://crbug.com/1084185.
	if err := ash.CloseNotifications(ctx, tconn); err != nil {
		s.Fatal("Failed to open/close the quick settings: ", err)
	}

	// Search or Shift-Search key to show the apps grid in fullscreen.
	kw, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to obtain the keyboard")
	}
	defer kw.Close()

	accel := "Shift+Search"
	if err := kw.Accel(ctx, accel); err != nil {
		s.Fatalf("Failed to type %s: %v", accel, err)
	}

	// Wait for the launcher state change.
	if err := ash.WaitForLauncherState(ctx, tconn, ash.FullscreenAllApps); err != nil {
		s.Fatal("Failed to wait: ", err)
	}

	ac := uiauto.New(tconn)
	appListItemViews := nodewith.ClassName("AppListItemView").Ancestor(nodewith.ClassName("AppsGridView"))

	firstItemLocation, err := ac.Location(ctx, appListItemViews.Nth(0))
	if err != nil {
		s.Fatal("Failed to get the location of app list item view: ", err)
	}
	firstItemPoint := firstItemLocation.CenterPoint()
	secondItemLocation, err := ac.Location(ctx, appListItemViews.Nth(1))
	if err != nil {
		s.Fatal("Failed to get the location of app list item veiw: ", err)
	}
	secondItemPoint := secondItemLocation.CenterPoint()

	// Setup complete~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~

	// Move the mouse and press on the second app list item view.
	if err := mouse.Move(ctx, tconn, secondItemPoint, time.Second); err != nil {
		s.Fatal("Failed to move 1: ", err)
	}
	if err := mouse.Press(ctx, tconn, mouse.LeftButton); err != nil {
		s.Fatal("Failed to press 1: ", err)
	}

	// Drag the mouse to the left slighly to trigger a drag and drop.
	dragLeftPoint := coords.NewPoint(secondItemPoint.X, secondItemPoint.Y-5)
	mouse.Move(ctx, tconn, dragLeftPoint, time.Second)

	// With a drag in progress, get the location of the first item while the app
	// list is cardified.
	firstItemRectCardified, err := ac.Location(ctx, appListItemViews.Nth(0))
	if err != nil {
		s.Fatal("Failed to get the location of app list item view: ", err)
	}
	firstItemPointCardified := firstItemRectCardified.CenterPoint()

	// Drag the first app item view over the second item item and release to create
	// a folder.
	mouse.Move(ctx, tconn, firstItemPointCardified, time.Second)
	mouse.Release(ctx, tconn, mouse.LeftButton)

	addItemsToFolder(ctx, tconn, ac, s, firstItemPointCardified, secondItemPoint, 18)

	// Remove some items from the folder.
	removeItemsFromFolder(ctx, tconn, ac, s, firstItemPoint, 5)

	// Add the removed items back.
	addItemsToFolder(ctx, tconn, ac, s, firstItemPointCardified, secondItemPoint, 5)

	// Move the folder to the next page and fill with apps completely.
	moveItemToNextPage(ctx, tconn, ac, s, firstItemPoint, firstItemPointCardified)
	addItemsToFolder(ctx, tconn, ac, s, firstItemPointCardified, secondItemPoint, 19)
	moveItemToNextPage(ctx, tconn, ac, s, firstItemPoint, firstItemPointCardified)
	addItemsToFolder(ctx, tconn, ac, s, firstItemPointCardified, secondItemPoint, 9)

	s.Log("Done adding items to max folder limit: ")
}

func addItemsToFolder(ctx context.Context, tconn *chrome.TestConn, ac *uiauto.Context, s *testing.State, folderPointCardified, itemToAddPoint coords.Point, numItemsToAdd int) {
	dragLeftPoint := coords.NewPoint(itemToAddPoint.X, itemToAddPoint.Y-5)

	// Add all items on the current page to the folder.
	for i := 0; i < numItemsToAdd; i++ {
		s.Log("Foldering item i = :", i)
		mouse.Move(ctx, tconn, itemToAddPoint, 200*time.Millisecond)
		mouse.Press(ctx, tconn, mouse.LeftButton)
		mouse.Move(ctx, tconn, dragLeftPoint, time.Second)
		mouse.Move(ctx, tconn, folderPointCardified, 200*time.Millisecond)
		mouse.Release(ctx, tconn, mouse.LeftButton)
	}
}

func removeItemsFromFolder(ctx context.Context, tconn *chrome.TestConn, ac *uiauto.Context, s *testing.State, folderPoint coords.Point, numItemsToRemove int) {

	for i := 0; i < numItemsToRemove; i++ {
		// Click to open the folder
		mouse.Move(ctx, tconn, folderPoint, time.Second)
		mouse.Click(ctx, tconn, folderPoint, mouse.LeftButton)

		// Get the location of the first item in the folder
		folderItems := nodewith.ClassName("AppListItemView").Ancestor(nodewith.ClassName("AppListFolderView"))

		firstItemInFolderLocation, err := ac.Location(ctx, folderItems.Nth(0))
		if err != nil {
			s.Fatal("Failed to get the location of the first folder item: ", err)
		}
		firstItemInFolderPoint := firstItemInFolderLocation.CenterPoint()

		// Drag the first item out of the folder
		mouse.Move(ctx, tconn, firstItemInFolderPoint, time.Second)
		mouse.Press(ctx, tconn, mouse.LeftButton)

		folderView := nodewith.ClassName("AppListFolderView")
		folderViewLocation, err := ac.Location(ctx, folderView)
		if err != nil {
			s.Fatal("Failed to get folderViewLocation: ", err)
		}
		pointOutsideFolder := coords.NewPoint(folderViewLocation.Right()+50, folderViewLocation.CenterY()+50)
		mouse.Move(ctx, tconn, pointOutsideFolder, time.Second)
		mouse.Release(ctx, tconn, mouse.LeftButton)
	}

}

func moveItemToNextPage(ctx context.Context, tconn *chrome.TestConn, ac *uiauto.Context, s *testing.State, initialItemPoint, itemDropPoint coords.Point) {
	dragDownPoint := coords.NewPoint(initialItemPoint.X, initialItemPoint.Y-5)

	appsGridViewInfo, err := ac.Location(ctx, nodewith.ClassName("AppsGridView"))
	if err != nil {
		s.Fatal("Failed to get appsGridViewInfo: ", err)
	}
	nextPageLocation := coords.NewPoint(appsGridViewInfo.CenterPoint().X, appsGridViewInfo.Bottom())
	nextPageLocationTwo := coords.NewPoint(nextPageLocation.X+5, nextPageLocation.Y)

	// Move the folder to the beginning of the next page
	mouse.Move(ctx, tconn, initialItemPoint, time.Second)
	mouse.Press(ctx, tconn, mouse.LeftButton)
	mouse.Move(ctx, tconn, dragDownPoint, time.Second)
	mouse.Move(ctx, tconn, nextPageLocation, time.Second)
	mouse.Move(ctx, tconn, nextPageLocationTwo, time.Second)
	mouse.Move(ctx, tconn, itemDropPoint, time.Second)
	mouse.Release(ctx, tconn, mouse.LeftButton)
	mouse.Move(ctx, tconn, dragDownPoint, time.Second)
}

func getFolderItemCount(ctx context.Context, tconn *chrome.TestConn, ac *uiauto.Context, s *testing.State, kw *input.KeyboardEventWriter, folderPoint coords.Point) int {
	// Move the mouse and click on the folder to open it.
	mouse.Move(ctx, tconn, folderPoint, time.Second)
	mouse.Click(ctx, tconn, folderPoint, mouse.LeftButton)

	// Check that a folder exists.
	folderItems := nodewith.ClassName("AppListItemView").Ancestor(nodewith.ClassName("AppListFolderView"))
	folderItemsInfo, err := ac.NodesInfo(ctx, folderItems)
	if err != nil {
		s.Fatal("Failed to find folderItemsInfo: ", err)
	}

	// Click outside the folder to close it.
	folderView := nodewith.ClassName("AppListFolderView")
	folderViewLocation, err := ac.Location(ctx, folderView)
	if err != nil {
		s.Fatal("Failed to get folderViewLocation: ", err)
	}
	pointOutsideFolder := coords.NewPoint(folderViewLocation.Right()+1, folderViewLocation.CenterY())
	mouse.Move(ctx, tconn, folderPoint, time.Second)
	mouse.Click(ctx, tconn, pointOutsideFolder, mouse.LeftButton)

	s.Log("getFolderItemCount: ", len(folderItemsInfo))
	// test commen
	return len(folderItemsInfo)
}

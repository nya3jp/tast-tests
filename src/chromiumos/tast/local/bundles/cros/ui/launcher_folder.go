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

	// Find the apps grid view bounds.
	ac := uiauto.New(tconn)
	appsGridView := nodewith.ClassName("AppsGridView")
	appsGridViewInfo, err := ac.Location(ctx, appsGridView)
	if err != nil {
		s.Fatal("Failed to get appsGridViewInfo: ", err)
	}

	appListItemViews := nodewith.ClassName("AppListItemView").Ancestor(appsGridView)

	secondItemLocation, err := ac.Location(ctx, appListItemViews.Nth(1))
	if err != nil {
		s.Fatal("Failed to get the location of app list item veiw: ", err)
	}
	dragStart := secondItemLocation.CenterPoint()

	// s.Log(dragStart)

	// Setup complete~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~

	// Move the mouse and press on the second app list item view.
	if err := mouse.Move(ctx, tconn, dragStart, time.Second); err != nil {
		s.Fatal("Failed to move 1: ", err)
	}
	if err := mouse.Press(ctx, tconn, mouse.LeftButton); err != nil {
		s.Fatal("Failed to press 1: ", err)
	}

	// Drag the mouse to the left slighly to trigger a drag and drop.
	dragRightEnd := coords.NewPoint(dragStart.X, dragStart.Y-5)
	if err := mouse.Move(ctx, tconn, dragRightEnd, time.Second); err != nil {
		s.Fatal("Failed to move 2: ", err)
	}

	firstItemLocationCardified, err := ac.Location(ctx, appListItemViews.Nth(0))
	if err != nil {
		s.Fatal("Failed to get the location of app list item view: ", err)
	}
	dragEnd := firstItemLocationCardified.CenterPoint()

	// Drag the first app item view over the second item item and release to create
	// a folder.
	if err := mouse.Move(ctx, tconn, dragEnd, time.Second); err != nil {
		// s.Log(err)
		s.Fatal("Failed to move 2: ", err)
	}
	mouse.Release(ctx, tconn, mouse.LeftButton)

	// Add all items on the current page to a folder.
	for i := 0; i < 18; i++ {
		// s.Log("foldering item i = ", i)
		mouse.Move(ctx, tconn, dragStart, 100*time.Millisecond)
		mouse.Press(ctx, tconn, mouse.LeftButton)
		mouse.Move(ctx, tconn, dragRightEnd, time.Second)
		mouse.Move(ctx, tconn, dragEnd, 100*time.Millisecond)
		mouse.Release(ctx, tconn, mouse.LeftButton)
	}

	firstItemLocation, err := ac.Location(ctx, appListItemViews.Nth(0))
	if err != nil {
		s.Fatal("Failed to get the location of app list item view: ", err)
	}
	firstItemMouseTarget := firstItemLocation.CenterPoint()
	// ui.waituntilexists

	nextPageLocation := coords.NewPoint(appsGridViewInfo.CenterPoint().X, appsGridViewInfo.Bottom())
	nextPageLocationTwo := coords.NewPoint(appsGridViewInfo.CenterPoint().X+5, appsGridViewInfo.Bottom())
	// s.Log("Bottom of apps grid view ", nextPageLocation)

	// Move the folder to the beginning of the next page
	mouse.Move(ctx, tconn, firstItemMouseTarget, time.Second)
	mouse.Press(ctx, tconn, mouse.LeftButton)
	mouse.Move(ctx, tconn, dragRightEnd, time.Second)
	mouse.Move(ctx, tconn, nextPageLocation, time.Second)
	mouse.Move(ctx, tconn, nextPageLocationTwo, time.Second)
	mouse.Move(ctx, tconn, firstItemMouseTarget, time.Second)
	mouse.Release(ctx, tconn, mouse.LeftButton)

	// Add all items on the current page to the folder.
	for i := 0; i < 20; i++ {
		// s.Log("foldering item i = ", i)
		mouse.Move(ctx, tconn, dragStart, 200*time.Millisecond)
		mouse.Press(ctx, tconn, mouse.LeftButton)
		mouse.Move(ctx, tconn, dragRightEnd, time.Second)
		mouse.Move(ctx, tconn, dragEnd, 200*time.Millisecond)
		mouse.Release(ctx, tconn, mouse.LeftButton)
	}

	// Pickup and move the folder to the next page in the app list.
	mouse.Move(ctx, tconn, firstItemMouseTarget, time.Second)
	mouse.Press(ctx, tconn, mouse.LeftButton)
	mouse.Move(ctx, tconn, dragRightEnd, time.Second)
	mouse.Move(ctx, tconn, nextPageLocation, time.Second)
	mouse.Move(ctx, tconn, nextPageLocationTwo, time.Second)
	mouse.Move(ctx, tconn, firstItemMouseTarget, time.Second)
	mouse.Release(ctx, tconn, mouse.LeftButton)

	// Add all items on the current page to the folder.
	for i := 0; i < 9; i++ {
		// s.Log("foldering item i = ", i)
		mouse.Move(ctx, tconn, dragStart, 200*time.Millisecond)
		mouse.Press(ctx, tconn, mouse.LeftButton)
		mouse.Move(ctx, tconn, dragRightEnd, time.Second)
		mouse.Move(ctx, tconn, dragEnd, 200*time.Millisecond)
		mouse.Release(ctx, tconn, mouse.LeftButton)
	}

	//Now the folder should be full

	//check that the folder has 48 items
	getFolderItemCount(ctx, tconn, ac, s, kw, firstItemMouseTarget)

	//attemps to drag one more item into the folder

	//check that the folder still has only 48 items
	// getFolderItemCount(ctx, tconn, s, firstItemMouseTarget)
	//

	//

	//

	// Check that the folder was created~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~
	// Check that the folder was created~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~END
}

func getFolderItemCount(ctx context.Context, tconn *chrome.TestConn, ac *uiauto.Context, s *testing.State, kw *input.KeyboardEventWriter, firstItemMouseTarget coords.Point) {
	// s.Log("Now in GetFolderItemCount: ")
	// s.Log(firstItemMouseTarget)

	// Move the mouse and click on the folder to open it.
	if err := mouse.Move(ctx, tconn, firstItemMouseTarget, time.Second); err != nil {
		s.Fatal("Failed to move mouse to firstItemMouseTarget: ", err)
	}
	if err := mouse.Click(ctx, tconn, firstItemMouseTarget, mouse.LeftButton); err != nil {
		s.Fatal("Failed to click on folder: ", err)
	}

	// Check that a folder has been created.
	folderView := nodewith.ClassName("AppListFolderView")
	// if folderView == nil {
	// s.Log("FolderView is nil")
	// }
	folderViewInfo, err := ac.NodesInfo(ctx, folderView)
	if err != nil {
		s.Fatal("Failed to find foldeVieInfo: ", err)
	}
	// s.Log(len(folderViewInfo))

	// Check that the foler has 48 items in it.
	folderItems := nodewith.ClassName("AppListItemView").Ancestor(nodewith.ClassName("AppListFolderView"))
	// if folderItems == nil {
	// 	s.Log("Failed to find folderItems: ", err)
	// }
	folderItemsInfo, err := ac.NodesInfo(ctx, folderItems)
	if err != nil {
		s.Fatal("Failed to find folderItemsInfo: ", err)
	}
	if len(folderItemsInfo) != 48 {
		s.Fatal("Unexpected number of items in Folder, expected 48 got: ", len(folderItemsInfo))
	}
	// s.Log(len(folderItemsInfo))

	// Click outside the folder to close it.
	folderViewLocation, err := ac.Location(ctx, folderView)
	if err != nil {
		s.Fatal("Failed to get folderViewLocation: ", err)
	}
	pointOutsideFolder := coords.NewPoint(folderViewLocation.Right()+1, folderViewLocation.CenterY())
	mouse.Click(ctx, tconn, pointOutsideFolder, mouse.LeftButton)

}

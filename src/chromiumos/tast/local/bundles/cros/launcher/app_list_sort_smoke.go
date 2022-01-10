// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package launcher

import (
	"context"
	"io/ioutil"
	"sort"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/launcher"
	"chromiumos/tast/local/chrome/uiauto/mouse"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/coords"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         AppListSortSmoke,
		LacrosStatus: testing.LacrosVariantUnknown,
		Desc:         "Basic smoke tests for the app list sorting",
		Contacts: []string{
			"cros-system-ui-eng@google.com",
			"chromeos-sw-engprod@google.com",
			"andrewxu@chromium.org",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Fixture:      "chromeLoggedInWithLauncherSort",
	})
}

func AppListSortSmoke(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(chrome.HasChrome).Chrome()
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect Test API: ", err)
	}
	defer faillog.DumpUITreeWithScreenshotOnError(ctx, s.OutDir(), s.HasError, cr, "ui_tree")

	// Bubble launcher requires clamshell mode.
	cleanup, err := ash.EnsureTabletModeEnabled(ctx, tconn, false)
	if err != nil {
		s.Fatal("Failed to ensure clamshell mode: ", err)
	}
	defer cleanup(ctx)

	extDirBase, err := ioutil.TempDir("", "")
	if err != nil {
		s.Fatal("Failed to create a temporary directory for loading extensions: ", err)
	}

	// ...
	appNames := []string{"a", "b", "c", "d", "e"}
	paths, err := ash.PrepareFakeAppsWithSharedIcon(extDirBase, appNames, true)
	if err != nil {
		s.Fatal("Failed to prepare for fake apps: ", err)
	}

	var m chrome.ExtensionLoadManager
	defer m.Close(ctx, tconn)
	if err := m.LoadExtensions(ctx, tconn, paths); err != nil {
		s.Fatal("Could not install fake extensions: ", err)
	}

	ui := uiauto.New(tconn)
	bubble := nodewith.ClassName(ash.AppListBubbleClassName)

	// When a DUT switches from tablet mode to clamshell mode, sometimes it
	// takes a while to settle down. Wait for the transition to finish.
	if err := ui.WaitForLocation(nodewith.Root())(ctx); err != nil {
		s.Fatal("Failed to wait for location changes: ", err)
	}

	if err := uiauto.Combine("open bubble by clicking home button",
		ui.LeftClick(nodewith.ClassName("ash/HomeButton")),
		ui.WaitUntilExists(bubble),
	)(ctx); err != nil {
		s.Fatal("Could not open bubble by clicking home button: ", err)
	}

	// ...
	appsGrid := nodewith.ClassName("ScrollableAppsGridView")
	appListItems, err := ui.NodesInfo(ctx, nodewith.ClassName(launcher.ExpandedItemsClass).Ancestor(appsGrid))
	if err != nil {
		s.Fatal("Failed to get the list of app list items: ", err)
	}
	expectedOrder := []string{"e", "d", "c", "b", "a"}
	inOrder, actualOrder, err := verifyAppListItemOrder(appListItems, expectedOrder)
	if err != nil {
		s.Fatal("Failed to check the item order: ", err)
	}
	if !inOrder {
		s.Fatalf("App list items are out of order: the actual order is %s; the expected order is %s", actualOrder, expectedOrder)
	}

	src := nodewith.ClassName("ScrollableAppsGridView").ClassName(launcher.ExpandedItemsClass).Ancestor(appsGrid).Nth(0)
	firstIconLocation, err := ui.Location(ctx, src)
	if err != nil {
		s.Fatal("Failed to get location for the first icon: ", err)
	}
	firstIconCenter := firstIconLocation.CenterPoint()

	dest := nodewith.ClassName("ScrollableAppsGridView").ClassName(launcher.ExpandedItemsClass).Ancestor(appsGrid).Nth(1)
	secondIconLocation, err := ui.Location(ctx, dest)
	if err != nil {
		s.Fatal("Failed to get location for the second icon: ", err)
	}
	secondIconCenter := secondIconLocation.CenterPoint()
	rightClickLocation := coords.Point{(firstIconCenter.X + secondIconCenter.X) / 2, (firstIconCenter.Y + secondIconCenter.Y) / 2}

	nameSortContextMenuItem := nodewith.Name("Name").ClassName("MenuItemView")
	if err := uiauto.Combine("sort app list items through the context menu",
		ui.MouseClickAtLocation(1, rightClickLocation),
		ui.WaitUntilExists(nameSortContextMenuItem),
		ui.LeftClick(nameSortContextMenuItem),
	)(ctx); err != nil {
		s.Fatal("Failed to sort app list items: ", err)
	}

	// Verify that after name sorting the fake apps are placed in the alphabetical order.
	appListItems, err = ui.NodesInfo(ctx, nodewith.ClassName(launcher.ExpandedItemsClass).Ancestor(appsGrid))
	if err != nil {
		s.Fatal("Failed to get the list of app list items: ", err)
	}
	expectedOrder = []string{"a", "b", "c", "d", "e"}
	inOrder, actualOrder, err = verifyAppListItemOrder(appListItems, expectedOrder)
	if err != nil {
		s.Fatal("Failed to check the item order: ", err)
	}
	if !inOrder {
		s.Fatalf("App list items are out of order: the actual order is %s; the expected order is %s", actualOrder, expectedOrder)
	}

	redoButton := nodewith.Name("Redo").ClassName("PillButton")
	if err := uiauto.Combine("click at the redo button",
		ui.WaitUntilExists(redoButton),
		ui.LeftClick(redoButton))(ctx); err != nil {
		s.Fatal("Failed to redo temporary sorting: ", err)
	}

	// Verify that after clicking at the redo button the fake apps are placed in the reverse alphabetical order.
	appListItems, err = ui.NodesInfo(ctx, nodewith.ClassName(launcher.ExpandedItemsClass).Ancestor(appsGrid))
	if err != nil {
		s.Fatal("Failed to get the list of app list items: ", err)
	}
	expectedOrder = []string{"e", "d", "c", "b", "a"}
	inOrder, actualOrder, err = verifyAppListItemOrder(appListItems, expectedOrder)
	if err != nil {
		s.Fatal("Failed to check the item order: ", err)
	}
	if !inOrder {
		s.Fatalf("App list items are out of order: the actual order is %s; the expected order is %s", actualOrder, expectedOrder)
	}

	if err := uiauto.Combine("sort through the context menu again",
		ui.MouseClickAtLocation(1, rightClickLocation),
		ui.WaitUntilExists(nameSortContextMenuItem),
		ui.LeftClick(nameSortContextMenuItem),
	)(ctx); err != nil {
		s.Fatal("Failed to sort app list items: ", err)
	}

	if err := uiauto.Combine("close bubble by clicking in screen corner",
		mouse.Click(tconn, coords.Point{X: 0, Y: 0}, mouse.LeftButton),
		ui.WaitUntilGone(bubble),
	)(ctx); err != nil {
		s.Fatal("Could not close bubble by clicking in screen corner: ", err)
	}

	if err := uiauto.Combine("open bubble by clicking home button",
		ui.LeftClick(nodewith.ClassName("ash/HomeButton")),
		ui.WaitUntilExists(bubble),
	)(ctx); err != nil {
		s.Fatal("Could not open bubble by clicking home button: ", err)
	}

	// Verify that after reshowing the bubble launcher the fake apps are still placed in the alphabetical order.
	appListItems, err = ui.NodesInfo(ctx, nodewith.ClassName(launcher.ExpandedItemsClass).Ancestor(appsGrid))
	if err != nil {
		s.Fatal("Failed to get the list of app list items: ", err)
	}
	expectedOrder = []string{"a", "b", "c", "d", "e"}
	inOrder, actualOrder, err = verifyAppListItemOrder(appListItems, expectedOrder)
	if err != nil {
		s.Fatal("Failed to check the item order: ", err)
	}
	if !inOrder {
		s.Fatalf("App list items are out of order: the actual order is %s; the expected order is %s", actualOrder, expectedOrder)
	}

	if err := uiauto.Combine("close bubble by clicking in screen corner",
		mouse.Click(tconn, coords.Point{X: 0, Y: 0}, mouse.LeftButton),
		ui.WaitUntilGone(bubble),
	)(ctx); err != nil {
		s.Fatal("Could not close bubble by clicking in screen corner: ", err)
	}
}

func verifyAppListItemOrder(items []uiauto.NodeInfo, expectedAppNames []string) (bool, []string, error) {
	itemSize := len(items)
	if itemSize <= 0 {
		return false, nil, errors.Errorf("expect item count to be greater than 0; the actual count is %d", itemSize)
	}

	expectedMapping := map[string]int{}
	for index, name := range expectedAppNames {
		expectedMapping[name] = index
	}

	actualMapping := map[int]string{}
	var indexArray []int
	for index, item := range items {
		if _, found := expectedMapping[item.Name]; found {
			actualMapping[index] = item.Name
			indexArray = append(indexArray, index)
		}
	}

	sort.Ints(indexArray)
	var actualAppNames []string
	isOrdered := true
	for index, key := range indexArray {
		val, _ := actualMapping[key]
		actualAppNames = append(actualAppNames, val)
		if val != expectedAppNames[index] {
			isOrdered = false
		}
	}

	return isOrdered, actualAppNames, nil
}

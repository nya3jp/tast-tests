// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package launcher

import (
	"context"

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
		Fixture:      "chromeLoggedInWith5FakeAppsLauncherSort",
	})
}

type appOrder string

// An enum type indicating the app ordering.
const (
	increasing appOrder = "increasing"
	decreasing appOrder = "decreasing"
)

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

	// Verify that by default the fake apps are placed in the reverse-alphabetical order. Note that when launcher sorting is enabled, a new app is placed at the front when the sorting order is not specified.
	gridIndices, err := launcher.GetTopLevelGridIndicesForNames(ctx, tconn, ash.AlphabeticalFakeAppNames)
	if err != nil {
		s.Fatal("Could not fetch the top level item indices: ", err)
	}
	if !verifyGridIndicesOrder(decreasing, gridIndices) {
		s.Fatal("Fake apps are not placed in the decreasing order and the actual grid indices are ", gridIndices)
	}

	appsGrid := nodewith.ClassName("ScrollableAppsGridView")
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
	s.Log(rightClickLocation)

	nameSortContextMenuItem := nodewith.Name("Name").ClassName("MenuItemView")
	if err := uiauto.Combine("sort app list items through the context menu",
		ui.MouseClickAtLocation(1, rightClickLocation),
		ui.WaitUntilExists(nameSortContextMenuItem),
		ui.LeftClick(nameSortContextMenuItem),
	)(ctx); err != nil {
		s.Fatal("Failed to sort app list items: ", err)
	}

	// Verify that after name sorting the fake apps are placed in the alphabetical order.
	gridIndices, err = launcher.GetTopLevelGridIndicesForNames(ctx, tconn, ash.AlphabeticalFakeAppNames)
	if err != nil {
		s.Fatal("Could not fetch the top level item indices: ", err)
	}
	if !verifyGridIndicesOrder(increasing, gridIndices) {
		s.Fatal("Fake apps are not placed in the increasing order and the actual grid indices are ", gridIndices)
	}

	redoButton := nodewith.Name("Redo").ClassName("PillButton")
	if err := uiauto.Combine("click at the redo button",
		ui.WaitUntilExists(redoButton),
		ui.LeftClick(redoButton))(ctx); err != nil {
		s.Fatal("Failed to redo temporary sorting: ", err)
	}

	// Verify that after clicking at the redo button the fake apps are placed in the reverse alphabetical order.
	gridIndices, err = launcher.GetTopLevelGridIndicesForNames(ctx, tconn, ash.AlphabeticalFakeAppNames)
	if err != nil {
		s.Fatal("Could not fetch the top level item indices: ", err)
	}
	if !verifyGridIndicesOrder(decreasing, gridIndices) {
		s.Fatal("Fake apps are not placed in the decreasing order and the actual grid indices are ", gridIndices)
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
	gridIndices, err = launcher.GetTopLevelGridIndicesForNames(ctx, tconn, ash.AlphabeticalFakeAppNames)
	if err != nil {
		s.Fatal("Could not fetch the top level item indices: ", err)
	}
	if !verifyGridIndicesOrder(increasing, gridIndices) {
		s.Fatal("Fake apps are not placed in the increasing order and the actual grid indices are ", gridIndices)
	}

	if err := uiauto.Combine("close bubble by clicking in screen corner",
		mouse.Click(tconn, coords.Point{X: 0, Y: 0}, mouse.LeftButton),
		ui.WaitUntilGone(bubble),
	)(ctx); err != nil {
		s.Fatal("Could not close bubble by clicking in screen corner: ", err)
	}
}

func verifyGridIndicesOrder(expectedOrder appOrder, gridIndices []launcher.GridIndex) bool {
	for idx, gridIndex := range gridIndices {
		if idx == len(gridIndices)-1 {
			break
		}

		comparison := launcher.CompareGridIndices(gridIndex, gridIndices[idx+1])

		switch expectedOrder {
		case increasing:
			if comparison {
				return false
			}
		case decreasing:
			if !comparison {
				return false
			}
		}
	}

	return true
}

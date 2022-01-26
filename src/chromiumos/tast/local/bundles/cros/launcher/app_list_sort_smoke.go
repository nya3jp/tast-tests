// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package launcher

import (
	"context"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/apps"
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

type appListSortTestType string

const (
	clamshell appListSortTestType = "clamshell"
	tablet    appListSortTestType = "tablet"
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
		Fixture:      "chromeLoggedInWithFakeAppsWithSpecifiedNamesProductivityLauncherAppSort",
		Params: []testing.Param{
			{
				Name: "clamshell",
				Val:  clamshell,
			},
			{
				Name: "tablet",
				Val:  tablet,
			},
		},
	})
}

func AppListSortSmoke(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(chrome.HasChrome).Chrome()
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect Test API: ", err)
	}
	defer faillog.DumpUITreeWithScreenshotOnError(ctx, s.OutDir(), s.HasError, cr, "ui_tree")

	testType := s.Param().(appListSortTestType)
	isTablet := testType == tablet

	cleanup, err := ash.EnsureTabletModeEnabled(ctx, tconn, isTablet)
	if err != nil {
		s.Fatal("Failed to ensure clamshell mode: ", err)
	}
	defer cleanup(ctx)

	ui := uiauto.New(tconn)

	// When a DUT switches from tablet mode to clamshell mode, sometimes it
	// takes a while to settle down. Wait for the transition to finish.
	if err := ui.WaitForLocation(nodewith.Root())(ctx); err != nil {
		s.Fatal("Failed to wait for location changes: ", err)
	}

	var appsGrid *nodewith.Finder
	if isTablet {
		appsGrid = nodewith.ClassName(launcher.PagedAppsGridViewClass)
	} else {
		appsGrid = nodewith.ClassName(launcher.BubbleAppsGridViewClass)
	}

	if err := openLauncherAndWait(ctx, tconn, ui, isTablet); err != nil {
		s.Fatal("Failed to open launcher and wait: ", err)
	}

	lastFakeAppName := ash.FakeAppAlphabeticalNames[len(ash.FakeAppAlphabeticalNames)-1]
	lastFakeApp := nodewith.ClassName(launcher.ExpandedItemsClass).Ancestor(appsGrid).Name(lastFakeAppName)
	if err := ui.WaitForLocation(lastFakeApp)(ctx); err != nil {
		s.Fatalf("Failed to wait for the fake app %v location to be idle: %v", lastFakeAppName, err)
	}

	indices, err := launcher.GetAppListItemIndices(ctx, tconn, []*nodewith.Finder{lastFakeApp}, appsGrid)
	srcIndex := indices[0]
	if srcIndex < 0 {
		s.Fatal("Failed to get the view index for the app ", lastFakeAppName)
	}

	// Move the fake app that should be placed at rear in the sorting order to
	// the front. It ensures that apps are out of order before sorting.
	if srcIndex != 0 {
		if err := launcher.DragIconAfterIcon(ctx, tconn, srcIndex, 0, appsGrid)(ctx); err != nil {
			s.Fatalf("Failed to drag the app %v from %v to the front: %v", lastFakeAppName, srcIndex, err)
		}
	}

	defaultFakeAppIndices, err := getFakeAppIndices(ctx, tconn, appsGrid)
	if err != nil {
		s.Fatal("Failed to get fake apps' default indices: ", err)
	}

	reorderContextMenuItem := nodewith.Name("Reorder by").ClassName("MenuItemView")
	nameSortContextMenuItem := nodewith.Name("Name").ClassName("MenuItemView")
	undoButton := nodewith.Name("Undo").ClassName("PillButton")
	if err := uiauto.Combine("sort app list items through the context menu with the alphabetical order",
		ui.RightClick(lastFakeApp),
		ui.WaitUntilExists(reorderContextMenuItem),
		ui.MouseMoveTo(reorderContextMenuItem, 0),
		ui.WaitUntilExists(nameSortContextMenuItem),
		ui.LeftClick(nameSortContextMenuItem),
		ui.WaitUntilExists(undoButton),
		ui.WaitForLocation(lastFakeApp),
	)(ctx); err != nil {
		s.Fatal("Failed to trigger alphabetical sorting: ", err)
	}

	ordered, err := areFakeAppsOrdered(ctx, tconn, appsGrid)
	if err != nil {
		s.Fatal("Failed to check the fake app order: ", err)
	}

	if !ordered {
		s.Fatal("Fake apps do not follow the alphabetical order")
	}

	if err := uiauto.Combine("undo alphabetical sorting",
		ui.LeftClick(undoButton),
		ui.WaitUntilGone(undoButton),
		ui.WaitForLocation(lastFakeApp),
	)(ctx); err != nil {
		s.Fatal("Failed to undo alphabetical sorting: ", err)
	}

	recoveredIndices, err := getFakeAppIndices(ctx, tconn, appsGrid)
	if err != nil {
		s.Fatal("Failed to get fake apps' indices after reverting sorting: ", err)
	}

	// Verify that after reverting sorting, fake apps' indices are the same with the default ones.
	for index := range defaultFakeAppIndices {
		if defaultFakeAppIndices[index] != recoveredIndices[index] {
			s.Fatalf("Failed to recover app order after sorting is reverted: default"+
				"fake app indices are: %v and the actual indices after reverting are: %v", defaultFakeAppIndices, recoveredIndices)
		}
	}

	if err := uiauto.Combine("sort app list items through the context menu with the alphabetical order",
		ui.RightClick(lastFakeApp),
		ui.WaitUntilExists(reorderContextMenuItem),
		ui.MouseMoveTo(reorderContextMenuItem, 0),
		ui.WaitUntilExists(nameSortContextMenuItem),
		ui.LeftClick(nameSortContextMenuItem),
		ui.WaitUntilExists(undoButton),
		ui.WaitForLocation(lastFakeApp),
	)(ctx); err != nil {
		s.Fatal("Failed to trigger alphabetical sorting: ", err)
	}

	if err := closeLauncherAndWait(ctx, tconn, ui, isTablet); err != nil {
		s.Fatalf("Failed to close laucher and wait in %v: %v", testType, err)
	}

	if err := openLauncherAndWait(ctx, tconn, ui, isTablet); err != nil {
		s.Fatal("Failed to open launcher and wait: ", err)
	}

	ordered, err = areFakeAppsOrdered(ctx, tconn, appsGrid)
	if err != nil {
		s.Fatal("Failed to check the fake app order: ", err)
	}

	if !ordered {
		s.Fatal("Fake apps do not follow the alphabetical order")
	}
}

func openLauncherAndWait(ctx context.Context, tconn *chrome.TestConn, ui *uiauto.Context, isTablet bool) error {
	if isTablet {
		if err := launcher.Open(tconn)(ctx); err != nil {
			return errors.Wrap(err, "failed to open the launcher")
		}

		if err := ash.WaitForHotseatAnimatingToIdealState(ctx, tconn, ash.ShelfShownHomeLauncher); err != nil {
			return errors.Wrap(err, "failed to wait until the shelf is shown")
		}

		return nil
	}

	if err := uiauto.Combine("open bubble by clicking home button",
		ui.LeftClick(nodewith.ClassName("ash/HomeButton")),
		ui.WaitUntilExists(nodewith.ClassName(launcher.BubbleAppsGridViewClass)),
	)(ctx); err != nil {
		return errors.Wrap(err, "failed to open bubble by clicking home button")
	}

	return nil
}

func closeLauncherAndWait(ctx context.Context, tconn *chrome.TestConn, ui *uiauto.Context, isTablet bool) error {
	if isTablet {
		if err := uiauto.Combine("close launcher in tablet by activating the browser",
			launcher.LaunchApp(tconn, apps.Chrome.Name),
		)(ctx); err != nil {
			return errors.Wrap(err, "failed to close launcher in tablet by activating the browser")
		}

		if err := ash.WaitForHotseatAnimatingToIdealState(ctx, tconn, ash.ShelfHidden); err != nil {
			return errors.Wrap(err, "failed to wait until the shelf is hidden")
		}

		return nil
	}

	appsGrid := nodewith.ClassName(launcher.BubbleAppsGridViewClass)
	if err := uiauto.Combine("close bubble by clicking in screen corner",
		mouse.Click(tconn, coords.Point{X: 0, Y: 0}, mouse.LeftButton),
		ui.WaitUntilGone(appsGrid),
	)(ctx); err != nil {
		return errors.Wrap(err, "failed to close launcher in clamshell by clicking in screen corner")
	}

	return nil
}

func getFakeAppIndices(ctx context.Context, tconn *chrome.TestConn, appsGrid *nodewith.Finder) ([]int, error) {
	finderArray := make([]*nodewith.Finder, len(ash.FakeAppAlphabeticalNames))
	for index, name := range ash.FakeAppAlphabeticalNames {
		finderArray[index] = nodewith.ClassName(launcher.ExpandedItemsClass).Ancestor(appsGrid).Name(name)
	}

	indices, err := launcher.GetAppListItemIndices(ctx, tconn, finderArray, appsGrid)
	if err != nil {
		return indices, errors.Wrap(err, "failed to get fake app indices")
	}

	if len(indices) < 2 {
		return indices, errors.Wrapf(err, "expect indices length is greater than 1; the actual length is %v", len(indices))
	}

	return indices, nil
}

func areFakeAppsOrdered(ctx context.Context, tconn *chrome.TestConn, appsGrid *nodewith.Finder) (bool, error) {
	indices, err := getFakeAppIndices(ctx, tconn, appsGrid)
	if err != nil {
		return false, errors.Wrap(err, "failed to get fake app indices")
	}

	for idx := 1; idx < len(indices); idx++ {
		if indices[idx] <= indices[idx-1] {
			return false, nil
		}
	}

	return true, nil
}

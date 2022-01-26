// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package launcher

import (
	"context"
	"os"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/launcher"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/testing"
)

type testType struct {
	TabletMode bool              // Whether the test runs in tablet mode
	SortManner launcher.SortType // Indicates the sort order used for testing
}

// An array of strings sorted in alphabetical order. These strings are used as app names when installing fake apps.
var fakeAppAlphabeticalNames = []string{"a", "A", "b", "B", "C"}

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
		Params: []testing.Param{
			{
				Name: "clamshell_alphabetical",
				Val:  testType{TabletMode: false, SortManner: launcher.AlphabeticalSort},
			},
			{
				Name: "tablet_alphabetical",
				Val:  testType{TabletMode: true, SortManner: launcher.AlphabeticalSort},
			},
		},
	})
}

func AppListSortSmoke(ctx context.Context, s *testing.State) {
	// Create the fake apps with the specified names.
	opts, extDirBase, err := ash.GetPrepareFakeAppsWithNamesOptions(fakeAppAlphabeticalNames)
	if err != nil {
		s.Fatal("Failed to create the fake apps with the specified names")
	}
	defer os.RemoveAll(extDirBase)

	// Enable the app list sort.
	opts = append(opts, chrome.EnableFeatures("ProductivityLauncher", "LauncherAppSort"))

	cr, err := chrome.New(ctx, opts...)
	if err != nil {
		s.Fatal("Chrome login failed: ", err)
	}
	defer cr.Close(ctx)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect Test API: ", err)
	}
	defer faillog.DumpUITreeWithScreenshotOnError(ctx, s.OutDir(), s.HasError, cr, "ui_tree")

	tabletMode := s.Param().(testType).TabletMode
	originallyEnabled, err := ash.TabletModeEnabled(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to check if DUT is in tablet mode: ", err)
	}
	cleanup, err := ash.EnsureTabletModeEnabled(ctx, tconn, tabletMode)
	if err != nil {
		s.Fatal("Failed to ensure clamshell/tablet mode: ", err)
	}
	defer cleanup(ctx)

	// When a DUT switches from tablet mode to clamshell mode, sometimes it takes a while to settle down.
	if originallyEnabled && !tabletMode {
		if err := ash.WaitForLauncherState(ctx, tconn, ash.Closed); err != nil {
			s.Fatal("Launcher not closed after transition to clamshell mode: ", err)
		}
	}

	var appsGrid *nodewith.Finder
	if tabletMode {
		appsGrid = nodewith.ClassName(launcher.PagedAppsGridViewClass)
	} else {
		appsGrid = nodewith.ClassName(launcher.BubbleAppsGridViewClass)
	}

	// Ensure that the launcher shows.
	if tabletMode {
		if err := launcher.OpenExpandedView(tconn)(ctx); err != nil {
			s.Fatal("Failed to open Expanded Application list view: ", err)
		}
	} else {
		if err := launcher.OpenBubbleLauncher(tconn)(ctx); err != nil {
			s.Fatal("Failed to open bubble launcher: ", err)
		}
	}

	if err := launcher.WaitForStableNumberOfApps(ctx, tconn); err != nil {
		s.Fatal("Failed to wait for the stable number of apps: ", err)
	}

	lastFakeAppName := fakeAppAlphabeticalNames[len(fakeAppAlphabeticalNames)-1]
	lastFakeApp := nodewith.ClassName(launcher.ExpandedItemsClass).Ancestor(appsGrid).Name(lastFakeAppName)
	ui := uiauto.New(tconn)
	if err := ui.WaitForLocation(lastFakeApp)(ctx); err != nil {
		s.Fatalf("Failed to wait for the fake app %v location to be idle: %v", lastFakeAppName, err)
	}

	indices, err := getFakeAppIndices(ctx, ui, []string{lastFakeAppName}, appsGrid)
	if err != nil {
		s.Fatalf("Failed to get the view index of the app %v: %v", lastFakeApp.Name, err)
	}
	srcIndex := indices[0]

	// Move the fake app that should be placed at rear in the sorting order to
	// the front. It ensures that apps are out of order before sorting.
	if srcIndex != 0 {
		if err := launcher.DragIconAfterIcon(ctx, tconn, srcIndex, 0, appsGrid)(ctx); err != nil {
			s.Fatalf("Failed to drag the app %v from %v to the front: %v", lastFakeAppName, srcIndex, err)
		}
	}

	if err := ui.WaitForLocation(lastFakeApp)(ctx); err != nil {
		s.Fatal("Failed to wait for the dragged item bounds to become stable: ", err)
	}

	defaultFakeAppIndices, err := getFakeAppIndices(ctx, ui, fakeAppAlphabeticalNames, appsGrid)
	if err != nil {
		s.Fatal("Failed to get the indices of the fake apps: ", err)
	}

	if err := launcher.TriggerAppListSortAndWait(ctx, ui, launcher.AlphabeticalSort, lastFakeApp); err != nil {
		s.Fatal("Failed to trigger alphabetical sorting: ", err)
	}

	fakeAppIndices, err := getFakeAppIndices(ctx, ui, fakeAppAlphabeticalNames, appsGrid)
	if err != nil {
		s.Fatal("Failed to get view indices of fake apps: ", err)
	}

	if err := verifyFakeAppsOrdered(fakeAppIndices); err != nil {
		s.Fatal("Failed to verify fake apps order: ", err)
	}

	undoButton := nodewith.Name("Undo").ClassName("PillButton")
	if err := uiauto.Combine("undo alphabetical sorting",
		ui.LeftClick(undoButton),
		ui.WaitUntilGone(undoButton),
		ui.WaitForLocation(lastFakeApp),
	)(ctx); err != nil {
		s.Fatal("Failed to undo alphabetical sorting: ", err)
	}

	recoveredIndices, err := getFakeAppIndices(ctx, ui, fakeAppAlphabeticalNames, appsGrid)
	if err != nil {
		s.Fatal("Failed to get fake apps' indices after reverting sorting: ", err)
	}

	// Verify that after reverting sorting, fake apps' indices are the same with the default ones.
	if len(defaultFakeAppIndices) != len(recoveredIndices) {
		s.Fatalf("Unexpected number of fake apps in the grid after sorting is reverted: "+
			"the number before reversion is %v while the number after reversion is %v", len(defaultFakeAppIndices), len(recoveredIndices))
	}
	for index := range defaultFakeAppIndices {
		if defaultFakeAppIndices[index] != recoveredIndices[index] {
			s.Fatalf("Failed to recover app order after sorting is reverted: default"+
				"fake app indices are: %v and the actual indices after reverting are: %v", defaultFakeAppIndices, recoveredIndices)
		}
	}

	if err := launcher.TriggerAppListSortAndWait(ctx, ui, launcher.AlphabeticalSort, lastFakeApp); err != nil {
		s.Fatal("Failed to trigger alphabetical sorting: ", err)
	}

	if tabletMode {
		if err := launcher.HideTabletModeLauncher(tconn)(ctx); err != nil {
			s.Fatal("Failed to hide the launcher in tablet: ", err)
		}
	} else if err := launcher.CloseBubbleLauncher(tconn)(ctx); err != nil {
		s.Fatal("Failed to close the bubble launcher: ", err)
	}

	// Show the launcher again.
	if tabletMode {
		if err := launcher.OpenExpandedView(tconn)(ctx); err != nil {
			s.Fatal("Failed to open Expanded Application list view: ", err)
		}
	} else {
		if err := launcher.OpenBubbleLauncher(tconn)(ctx); err != nil {
			s.Fatal("Failed to open bubble launcher: ", err)
		}
	}

	fakeAppIndices, err = getFakeAppIndices(ctx, ui, fakeAppAlphabeticalNames, appsGrid)
	if err != nil {
		s.Fatal("Failed to get view indices of fake apps: ", err)
	}

	if err := verifyFakeAppsOrdered(fakeAppIndices); err != nil {
		s.Fatal("Failed to verify fake apps order: ", err)
	}
}

func getFakeAppIndices(ctx context.Context, ui *uiauto.Context, appNames []string, appsGrid *nodewith.Finder) ([]int, error) {
	viewIndices := make([]int, len(appNames))
	for idx := range viewIndices {
		viewIndices[idx] = -1
	}

	nameIndexMapping := make(map[string]int)
	for index, name := range appNames {
		nameIndexMapping[name] = index
	}

	// Get the node information of all app list items.
	appListItems, err := ui.NodesInfo(ctx, nodewith.ClassName(launcher.ExpandedItemsClass).Ancestor(appsGrid))
	if err != nil {
		return viewIndices, errors.Wrap(err, "failed to get the node information of all app list items")
	}

	for viewIndex, item := range appListItems {
		if nameIndex, found := nameIndexMapping[item.Name]; found {
			viewIndices[nameIndex] = viewIndex
		}
	}

	for index, viewIndex := range viewIndices {
		if viewIndex == -1 {
			return viewIndices, errors.Errorf("failed to find the view index for the app %v", appNames[index])
		}
	}

	return viewIndices, nil
}

func verifyFakeAppsOrdered(viewIndices []int) error {
	for index := 1; index < len(viewIndices); index++ {
		if viewIndices[index] <= viewIndices[index-1] {
			return errors.Errorf("fake apps are out of order: the actual view indices are %v", viewIndices)
		}
	}

	return nil
}

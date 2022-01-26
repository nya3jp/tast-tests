// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package launcher

import (
	"context"
	"os"

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

// An array of strings sorted in alphabetical order. These strings are used as app names when installing fake apps.
var fakeAppAlphabeticalNames = []string{"A", "B", "C", "D", "E"}

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
		appsGrid = nodewith.ClassName(launcher.TabletAppsGridViewClass)
	} else {
		appsGrid = nodewith.ClassName(launcher.ClamshellAppsGridViewClass)
	}

	// Ensure that the launcher shows.
	if isTablet {
		if err := launcher.OpenExpandedView(tconn)(ctx); err != nil {
			s.Fatal("Failed to open Expanded Application list view: ", err)
		}
	} else {
		if err := launcher.OpenBubbleLauncher(tconn)(ctx); err != nil {
			s.Fatal("Failed to open bubble launcher: ", err)
		}
	}

	lastFakeAppName := fakeAppAlphabeticalNames[len(fakeAppAlphabeticalNames)-1]
	lastFakeApp := nodewith.ClassName(launcher.ExpandedItemsClass).Ancestor(appsGrid).Name(lastFakeAppName)
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
	same := true
	if len(defaultFakeAppIndices) != len(recoveredIndices) {
		same = false
	}
	if same {
		for index := range defaultFakeAppIndices {
			if defaultFakeAppIndices[index] != recoveredIndices[index] {
				same = false
				break
			}
		}
	}
	if !same {
		s.Fatalf("Failed to recover app order after sorting is reverted: default"+
			"fake app indices are: %v and the actual indices after reverting are: %v", defaultFakeAppIndices, recoveredIndices)
	}

	if err := launcher.TriggerAppListSortAndWait(ctx, ui, launcher.AlphabeticalSort, lastFakeApp); err != nil {
		s.Fatal("Failed to trigger alphabetical sorting: ", err)
	}

	if err := closeLauncherAndWait(ctx, tconn, ui, isTablet); err != nil {
		s.Fatalf("Failed to close laucher and wait in %v: %v", testType, err)
	}

	// Ensure that the launcher shows.
	if isTablet {
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

func closeLauncherAndWait(ctx context.Context, tconn *chrome.TestConn, ui *uiauto.Context, isTablet bool) error {
	if isTablet {
		if err := uiauto.Combine("close launcher in tablet by activating the browser",
			launcher.LaunchApp(tconn, apps.Chrome.Name),
		)(ctx); err != nil {
			return errors.Wrap(err, "failed to close launcher in tablet by activating the browser")
		}

		ws, err := ash.GetAllWindows(ctx, tconn)
		if err != nil {
			return errors.Wrap(err, "failed to get the window list")
		}
		if len(ws) != 1 {
			return errors.Errorf("expected 1 window, got %v", len(ws))
		}

		// Wait for the window to finish animating before activating.
		if err := ash.WaitWindowFinishAnimating(ctx, tconn, ws[0].ID); err != nil {
			return errors.Wrap(err, "failed to wait for the window animation")
		}

		return nil
	}

	appsGrid := nodewith.ClassName(launcher.ClamshellAppsGridViewClass)
	if err := uiauto.Combine("close bubble by clicking in screen corner",
		mouse.Click(tconn, coords.Point{X: 0, Y: 0}, mouse.LeftButton),
		ui.WaitUntilGone(appsGrid),
	)(ctx); err != nil {
		return errors.Wrap(err, "failed to close launcher in clamshell by clicking in screen corner")
	}

	return nil
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

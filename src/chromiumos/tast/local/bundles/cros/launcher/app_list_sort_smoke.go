// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package launcher

import (
	"context"
	"io/ioutil"
	"os"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/launcher"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/testing"
)

var fakeAppInfoForSortSmokeTest = launcher.FakeAppInfoForSort{
	AlphabeticalNames: []string{"a", "B", "c", "d", "E"},
	ColorOrderNames:   []string{"white", "red", "yellow", "cyan", "blue", "purple", "black"},
	IconFileNames: []string{"app_list_sort_smoke_white.png", "app_list_sort_smoke_red.png", "app_list_sort_smoke_yellow.png", "app_list_sort_smoke_cyan.png",
		"app_list_sort_smoke_blue.png", "app_list_sort_smoke_purple.png", "app_list_sort_smoke_black.png"}}

func init() {
	testing.AddTest(&testing.Test{
		Func:         AppListSortSmoke,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Basic smoke tests for the app list sorting",
		Contacts: []string{
			"cros-system-ui-eng@google.com",
			"chromeos-sw-engprod@google.com",
			"andrewxu@chromium.org",
			"tbarzic@chromium.org",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Data:         fakeAppInfoForSortSmokeTest.IconFileNames,
		Params: []testing.Param{
			{
				Name: "clamshell_alphabetical",
				Val:  launcher.SortTestType{TabletMode: false, SortMethod: launcher.AlphabeticalSort, OrderedAppNames: fakeAppInfoForSortSmokeTest.AlphabeticalNames},
			},
			{
				Name: "tablet_alphabetical",
				Val:  launcher.SortTestType{TabletMode: true, SortMethod: launcher.AlphabeticalSort, OrderedAppNames: fakeAppInfoForSortSmokeTest.AlphabeticalNames},
			},
			{
				Name: "clamshell_color",
				Val:  launcher.SortTestType{TabletMode: false, SortMethod: launcher.ColorSort, OrderedAppNames: fakeAppInfoForSortSmokeTest.ColorOrderNames},
			},
			{
				Name: "tablet_color",
				Val:  launcher.SortTestType{TabletMode: true, SortMethod: launcher.ColorSort, OrderedAppNames: fakeAppInfoForSortSmokeTest.ColorOrderNames},
			},
		},
	})
}

// AppListSortSmoke verifies that the app order after sort is expected.
func AppListSortSmoke(ctx context.Context, s *testing.State) {
	var opts []chrome.Option

	testParam := s.Param().(launcher.SortTestType)
	fakeAppNamesInOrder := testParam.OrderedAppNames

	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	extDirBase, err := ioutil.TempDir("", "")
	if err != nil {
		s.Fatal("Failed to create a temporary directory: ", err)
	}
	defer os.RemoveAll(extDirBase)

	// Prepare fake apps based on the sort method to be verified.
	switch testParam.SortMethod {
	case launcher.AlphabeticalSort:
		opts, err = ash.GeneratePrepareFakeAppsWithNamesOptions(extDirBase, fakeAppNamesInOrder)
	case launcher.ColorSort:
		iconFileNames := fakeAppInfoForSortSmokeTest.IconFileNames
		iconData := make([][]byte, len(iconFileNames))
		for index, imageName := range iconFileNames {
			imageBytes, err := launcher.ReadImageBytesFromFilePath(s.DataPath(imageName))
			if err != nil {
				s.Fatalf("Failed to read image byte data from %q: %v", imageName, err)
			}
			iconData[index] = imageBytes
		}
		opts, err = ash.GeneratePrepareFakeAppsWithIconDataOptions(extDirBase, fakeAppNamesInOrder, iconData)
	}

	if err != nil {
		s.Fatalf("Failed to create the fake apps for verifying %v: %v", testParam.SortMethod, err)
	}

	// Enable the app list sort.
	opts = append(opts, chrome.EnableFeatures("LauncherAppSort"))

	cr, err := chrome.New(ctx, opts...)
	if err != nil {
		s.Fatal("Chrome login failed: ", err)
	}
	defer cr.Close(cleanupCtx)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect Test API: ", err)
	}

	tabletMode := testParam.TabletMode

	cleanup, err := launcher.SetUpLauncherTest(ctx, tconn, tabletMode, true /*stabilizeAppCount*/)
	if err != nil {
		s.Fatal("Failed to set up launcher test case: ", err)
	}
	defer cleanup(cleanupCtx)

	defer faillog.DumpUITreeWithScreenshotOnError(cleanupCtx, s.OutDir(), s.HasError, cr, "ui_tree")

	ui := uiauto.New(tconn)
	if err := ui.WaitUntilExists(launcher.ReorderEducationNudgeFinder)(ctx); err != nil {
		s.Fatal("Failed to wait for the reorder education nudge to show: ", err)
	}

	appsGrid := nodewith.ClassName(launcher.BubbleAppsGridViewClass)
	if tabletMode {
		appsGrid = nodewith.ClassName(launcher.PagedAppsGridViewClass)
	}

	lastFakeAppName := fakeAppNamesInOrder[len(fakeAppNamesInOrder)-1]
	lastFakeApp := nodewith.ClassName(launcher.ExpandedItemsClass).Ancestor(appsGrid).Name(lastFakeAppName)
	if err := ui.WaitForLocation(lastFakeApp)(ctx); err != nil {
		s.Fatalf("Failed to wait for the fake app %q location to be idle: %v", lastFakeAppName, err)
	}

	indices, err := launcher.FetchItemIndicesByName(ctx, ui, []string{lastFakeAppName}, appsGrid)
	if err != nil {
		s.Fatalf("Failed to get the view index of the app %q: %v", lastFakeAppName, err)
	}
	srcIndex := indices[0]

	// Move the fake app that should be placed at rear in the sorting order to
	// the front. It ensures that apps are out of order before sorting.
	if srcIndex != 0 {
		if err := launcher.DragIconAfterIcon(ctx, tconn, srcIndex, 0, appsGrid)(ctx); err != nil {
			s.Fatalf("Failed to drag the app %q from %d to the front: %v", lastFakeAppName, srcIndex, err)
		}
	}

	if err := ui.WaitForLocation(lastFakeApp)(ctx); err != nil {
		s.Fatal("Failed to wait for the dragged item bounds to become stable: ", err)
	}

	defaultFakeAppIndices, err := launcher.FetchItemIndicesByName(ctx, ui, fakeAppNamesInOrder, appsGrid)
	if err != nil {
		s.Fatal("Failed to get the indices of the fake apps: ", err)
	}

	if err := launcher.TriggerAppListSortAndWaitForUndoButtonExist(ctx, ui, testParam.SortMethod, lastFakeApp); err != nil {
		s.Fatalf("Failed to trigger %v: %v", testParam.SortMethod, err)
	}

	// App items not on the first launcher page get hidden temporarily during sort animation. Wait
	// for them to reappear before proceeding.
	if err := launcher.VerifyFakeAppsOrdered(ctx, ui, appsGrid, fakeAppNamesInOrder, true /*wait=*/); err != nil {
		s.Fatal("Failed to verify fake apps order: ", err)
	}

	undoButton := nodewith.Name(launcher.GetUndoButtonNameForSortType(testParam.SortMethod)).ClassName("PillButton")
	if err := uiauto.Combine("undo alphabetical sorting",
		ui.LeftClick(undoButton),
		ui.WaitUntilGone(undoButton),
		ui.WaitForLocation(lastFakeApp),
	)(ctx); err != nil {
		s.Fatal("Failed to undo alphabetical sorting: ", err)
	}

	// App items not on the first launcher page get hidden temporarily during sort revert animation.
	// Wait for them to reappear before proceeding.
	for _, name := range fakeAppNamesInOrder {
		if err := ui.WaitUntilExists(nodewith.ClassName(launcher.ExpandedItemsClass).Name(name).Ancestor(appsGrid))(ctx); err != nil {
			s.Fatalf("Failed to find app %q after sort revert: %v", name, err)
		}
	}

	recoveredIndices, err := launcher.FetchItemIndicesByName(ctx, ui, fakeAppNamesInOrder, appsGrid)
	if err != nil {
		s.Fatal("Failed to get fake apps' indices after reverting sorting: ", err)
	}

	// Verify that after reverting sorting, fake apps' indices are the same with the default ones.
	if len(defaultFakeAppIndices) != len(recoveredIndices) {
		s.Fatalf("App count mismatch after sort revert: got %d, expecting %d", len(recoveredIndices), len(defaultFakeAppIndices))
	}
	for i := range defaultFakeAppIndices {
		if defaultFakeAppIndices[i] != recoveredIndices[i] {
			s.Fatalf("Unexpected app order after sort revert: got %v, expecting: %v", recoveredIndices, defaultFakeAppIndices)
		}
	}

	if err := launcher.TriggerAppListSortAndWaitForUndoButtonExist(ctx, ui, testParam.SortMethod, lastFakeApp); err != nil {
		s.Fatalf("Failed to trigger %v after reverting: %v", testParam.SortMethod, err)
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
			s.Fatal("Failed to open Expanded Application list view for reshowing: ", err)
		}
	} else {
		if err := launcher.OpenBubbleLauncher(tconn)(ctx); err != nil {
			s.Fatal("Failed to open bubble launcher for reshowing: ", err)
		}
	}

	if err := ui.Gone(undoButton)(ctx); err != nil {
		s.Fatal("Didn't expect to find undo button: ", err)
	}

	if err := launcher.VerifyFakeAppsOrdered(ctx, ui, appsGrid, fakeAppNamesInOrder, false /*wait=*/); err != nil {
		s.Fatal("Failed to verify fake apps order after reshowing the launcher: ", err)
	}
}

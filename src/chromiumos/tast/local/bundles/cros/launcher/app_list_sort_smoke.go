// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package launcher

import (
	"bytes"
	"context"
	"image"
	"image/png"
	"os"
	"sort"

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
	SortMethod launcher.SortType // Indicates the sort order used for testing
}

// An array of strings sorted in alphabetical order. These strings are used as app names when installing fake apps.
var fakeAppAlphabeticalNames = []string{"a", "B", "c", "d", "E"}

// The app names whose corresponding icons follow the color order.
var fakeAppColorOrderNames = []string{"white", "red", "yellow", "cyan", "blue", "purple", "black"}

func getIconImageNames() []string {
	prefix := "app_list_sort_smoke_"
	suffix := ".png"

	names := make([]string, len(fakeAppColorOrderNames))
	for index, color := range fakeAppColorOrderNames {
		names[index] = prefix + color + suffix
	}

	return names
}

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
		Data:         getIconImageNames(),
		Params: []testing.Param{
			{
				Name: "clamshell_alphabetical",
				Val:  testType{TabletMode: false, SortMethod: launcher.AlphabeticalSort},
			},
			{
				Name: "tablet_alphabetical",
				Val:  testType{TabletMode: true, SortMethod: launcher.AlphabeticalSort},
			},
			{
				Name: "clamshell_color",
				Val:  testType{TabletMode: false, SortMethod: launcher.ColorSort},
			},
			{
				Name: "tablet_color",
				Val:  testType{TabletMode: true, SortMethod: launcher.ColorSort},
			},
		},
	})
}

func AppListSortSmoke(ctx context.Context, s *testing.State) {
	var opts []chrome.Option
	var extDirBase string
	var err error

	testParam := s.Param().(testType)
	var fakeAppNamesInOrder []string

	// Prepare fake apps based on the sort method.
	switch testParam.SortMethod {
	case launcher.AlphabeticalSort:
		fakeAppNamesInOrder = fakeAppAlphabeticalNames
		opts, extDirBase, err = ash.GetPrepareFakeAppsWithNamesOptions(fakeAppNamesInOrder)
	case launcher.ColorSort:
		fakeAppNamesInOrder = fakeAppColorOrderNames
		iconNames := getIconImageNames()
		iconData := make([][]byte, len(iconNames))
		for index, imageName := range iconNames {
			imageBytes, err := getImgBytesFromFilePath(s.DataPath(imageName))
			if err != nil {
				s.Fatalf("Failed to read icon data from %s: %v", imageName, err)
			}
			iconData[index] = imageBytes
		}
		opts, extDirBase, err = ash.GetPrepareFakeAppsWithIconDataOptions(fakeAppNamesInOrder, iconData)
		if err != nil {
			s.Fatal("Failed to create the fake apps with the specified names: ", err)
		}
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

	tabletMode := testParam.TabletMode
	originallyEnabled, err := ash.TabletModeEnabled(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to check if DUT is in tablet mode: ", err)
	}
	cleanup, err := ash.EnsureTabletModeEnabled(ctx, tconn, tabletMode)
	if err != nil {
		s.Fatal("Failed to ensure clamshell/tablet mode: ", err)
	}
	defer cleanup(ctx)

	// Ensure that the tablet launcher is closed before opening a launcher instance for test in clamshell.
	if originallyEnabled && !tabletMode {
		if err := ash.WaitForLauncherState(ctx, tconn, ash.Closed); err != nil {
			s.Fatal("Launcher not closed after transition to clamshell mode: ", err)
		}
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
		s.Fatal("Failed to wait for item count in app list to stabilize: ", err)
	}

	var appsGrid *nodewith.Finder
	if tabletMode {
		appsGrid = nodewith.ClassName(launcher.PagedAppsGridViewClass)
	} else {
		appsGrid = nodewith.ClassName(launcher.BubbleAppsGridViewClass)
	}

	lastFakeAppName := fakeAppNamesInOrder[len(fakeAppNamesInOrder)-1]
	lastFakeApp := nodewith.ClassName(launcher.ExpandedItemsClass).Ancestor(appsGrid).Name(lastFakeAppName)
	ui := uiauto.New(tconn)
	if err := ui.WaitForLocation(lastFakeApp)(ctx); err != nil {
		s.Fatalf("Failed to wait for the fake app %s location to be idle: %v", lastFakeAppName, err)
	}

	indices, err := launcher.GetItemIndicesByName(ctx, ui, []string{lastFakeAppName}, appsGrid)
	if err != nil {
		s.Fatalf("Failed to get the view index of the app %s: %v", lastFakeAppName, err)
	}
	srcIndex := indices[0]

	// Move the fake app that should be placed at rear in the sorting order to
	// the front. It ensures that apps are out of order before sorting.
	if srcIndex != 0 {
		if err := launcher.DragIconAfterIcon(ctx, tconn, srcIndex, 0, appsGrid)(ctx); err != nil {
			s.Fatalf("Failed to drag the app %s from %d to the front: %v", lastFakeAppName, srcIndex, err)
		}
	}

	if err := ui.WaitForLocation(lastFakeApp)(ctx); err != nil {
		s.Fatal("Failed to wait for the dragged item bounds to become stable: ", err)
	}

	defaultFakeAppIndices, err := launcher.GetItemIndicesByName(ctx, ui, fakeAppNamesInOrder, appsGrid)
	if err != nil {
		s.Fatal("Failed to get the indices of the fake apps: ", err)
	}

	if err := launcher.TriggerAppListSortAndWaitForUndoButtonExist(ctx, ui, testParam.SortMethod, lastFakeApp); err != nil {
		s.Fatalf("Failed to trigger %v: %v", testParam.SortMethod, err)
	}

	fakeAppIndices, err := launcher.GetItemIndicesByName(ctx, ui, fakeAppNamesInOrder, appsGrid)
	if err != nil {
		s.Fatal("Failed to get view indices of fake apps: ", err)
	}

	if err := verifyFakeAppsOrdered(fakeAppIndices, fakeAppNamesInOrder); err != nil {
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

	recoveredIndices, err := launcher.GetItemIndicesByName(ctx, ui, fakeAppNamesInOrder, appsGrid)
	if err != nil {
		s.Fatal("Failed to get fake apps' indices after reverting sorting: ", err)
	}

	// Verify that after reverting sorting, fake apps' indices are the same with the default ones.
	if len(defaultFakeAppIndices) != len(recoveredIndices) {
		s.Fatalf("App count mismatch after sort revert: got %d, expecting %d", len(recoveredIndices), len(defaultFakeAppIndices))
	}
	for index := range defaultFakeAppIndices {
		if defaultFakeAppIndices[index] != recoveredIndices[index] {
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
			s.Fatal("Failed to open Expanded Application list view: ", err)
		}
	} else {
		if err := launcher.OpenBubbleLauncher(tconn)(ctx); err != nil {
			s.Fatal("Failed to open bubble launcher: ", err)
		}
	}

	if err := ui.Gone(undoButton)(ctx); err != nil {
		s.Fatal("Didn't expect to find undo button: ", err)
	}

	fakeAppIndices, err = launcher.GetItemIndicesByName(ctx, ui, fakeAppNamesInOrder, appsGrid)
	if err != nil {
		s.Fatal("Failed to get view indices of fake apps: ", err)
	}

	if err := verifyFakeAppsOrdered(fakeAppIndices, fakeAppNamesInOrder); err != nil {
		s.Fatal("Failed to verify fake apps order: ", err)
	}
}

func getImgBytesFromFilePath(filePath string) ([]byte, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	image, _, err := image.Decode(f)
	if err != nil {
		return nil, err
	}
	buf := new(bytes.Buffer)
	err = png.Encode(buf, image)
	if err != nil {
		return nil, err
	}
	imgBytes := buf.Bytes()
	return imgBytes, nil
}

type indexNamePair struct {
	ViewIndex int
	AppName   string
}

type byViewIndex []indexNamePair

func (data byViewIndex) Len() int           { return len(data) }
func (data byViewIndex) Swap(i, j int)      { data[i], data[j] = data[j], data[i] }
func (data byViewIndex) Less(i, j int) bool { return data[i].ViewIndex < data[j].ViewIndex }
func (data byViewIndex) NameList() []string {
	names := make([]string, data.Len())
	for index, pair := range data {
		names[index] = pair.AppName
	}
	return names
}

func verifyFakeAppsOrdered(viewIndices []int, namesInOrder []string) error {
	if len(viewIndices) != len(namesInOrder) {
		return errors.Errorf("unexpected view indices count: got %d, expecting %d", len(namesInOrder), len(viewIndices))
	}

	for index := 1; index < len(viewIndices); index++ {
		if viewIndices[index] <= viewIndices[index-1] {
			// The code below calculates names in the view index order.
			actualNames := make([]indexNamePair, len(viewIndices))
			for indexInArray, viewIndex := range viewIndices {
				actualNames[indexInArray] = indexNamePair{ViewIndex: viewIndex, AppName: namesInOrder[indexInArray]}
			}

			data := byViewIndex(actualNames)
			sort.Sort(data)
			return errors.Errorf("unexpected fake app order: got %v, expecting %v", data.NameList(), namesInOrder)
		}
	}

	return nil
}

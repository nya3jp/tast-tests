// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package launcher

import (
	"context"
	"io/ioutil"
	"os"
	"time"

	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/launcher"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/testing"
)

var fakeAppInfoForAppInstallWithAppListSortedTest = launcher.FakeAppInfoForSort{
	AlphabeticalNames: []string{"B", "e", "H", "j", "m"},
	ColorOrderNames:   []string{"white", "red", "cyan", "blue", "purple", "black"},
	IconFileNames: []string{"app_list_sort_smoke_white.png", "app_list_sort_smoke_red.png", "app_list_sort_smoke_cyan.png",
		"app_list_sort_smoke_blue.png", "app_list_sort_smoke_purple.png", "app_list_sort_smoke_black.png"},
	AlphabeticalNamesAfterAppInstall: []string{"B", "e", "H", "InstallAppWithAppListSortedMockApp", "j", "m"},
	ColorOrderNamesAfterAppInstall:   []string{"white", "red", "InstallAppWithAppListSortedMockApp", "cyan", "blue", "purple", "black"},
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         InstallArcAppWithAppListSorted,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Tests to verify app installation with app list sorted",
		Contacts: []string{
			"cros-system-ui-eng@google.com",
			"chromeos-sw-engprod@google.com",
			"andrewxu@chromium.org",
			"tbarzic@chromium.org",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Data:         fakeAppInfoForAppInstallWithAppListSortedTest.IconFileNames,
		Params: []testing.Param{
			{
				Name: "clamshell_alphabetical_androidp",
				Val: launcher.SortTestType{TabletMode: false, SortMethod: launcher.AlphabeticalSort,
					OrderedAppNames:             fakeAppInfoForAppInstallWithAppListSortedTest.AlphabeticalNames,
					OrderedAppNamesAfterInstall: fakeAppInfoForAppInstallWithAppListSortedTest.AlphabeticalNamesAfterAppInstall},
				ExtraSoftwareDeps: []string{"android_p"},
			},
			{
				Name: "clamshell_alphabetical_androidvm",
				Val: launcher.SortTestType{TabletMode: false, SortMethod: launcher.AlphabeticalSort,
					OrderedAppNames:             fakeAppInfoForAppInstallWithAppListSortedTest.AlphabeticalNames,
					OrderedAppNamesAfterInstall: fakeAppInfoForAppInstallWithAppListSortedTest.AlphabeticalNamesAfterAppInstall},
				ExtraSoftwareDeps: []string{"android_vm"},
			},
			{
				Name: "tablet_alphabetical_androidp",
				Val: launcher.SortTestType{TabletMode: true, SortMethod: launcher.AlphabeticalSort,
					OrderedAppNames:             fakeAppInfoForAppInstallWithAppListSortedTest.AlphabeticalNames,
					OrderedAppNamesAfterInstall: fakeAppInfoForAppInstallWithAppListSortedTest.AlphabeticalNamesAfterAppInstall},
				ExtraSoftwareDeps: []string{"android_p"},
			},
			{
				Name: "tablet_alphabetical_androidvm",
				Val: launcher.SortTestType{TabletMode: true, SortMethod: launcher.AlphabeticalSort,
					OrderedAppNames:             fakeAppInfoForAppInstallWithAppListSortedTest.AlphabeticalNames,
					OrderedAppNamesAfterInstall: fakeAppInfoForAppInstallWithAppListSortedTest.AlphabeticalNamesAfterAppInstall},
				ExtraSoftwareDeps: []string{"android_vm"},
			},
			{
				Name: "clamshell_color_androidp",
				Val: launcher.SortTestType{TabletMode: false, SortMethod: launcher.ColorSort,
					OrderedAppNames:             fakeAppInfoForAppInstallWithAppListSortedTest.ColorOrderNames,
					OrderedAppNamesAfterInstall: fakeAppInfoForAppInstallWithAppListSortedTest.ColorOrderNamesAfterAppInstall},
				ExtraSoftwareDeps: []string{"android_p"},
			},
			{
				Name: "clamshell_color_androidvm",
				Val: launcher.SortTestType{TabletMode: false, SortMethod: launcher.ColorSort,
					OrderedAppNames:             fakeAppInfoForAppInstallWithAppListSortedTest.ColorOrderNames,
					OrderedAppNamesAfterInstall: fakeAppInfoForAppInstallWithAppListSortedTest.ColorOrderNamesAfterAppInstall},
				ExtraSoftwareDeps: []string{"android_vm"},
			},
			{
				Name: "tablet_color_androidp",
				Val: launcher.SortTestType{TabletMode: true, SortMethod: launcher.ColorSort,
					OrderedAppNames:             fakeAppInfoForAppInstallWithAppListSortedTest.ColorOrderNames,
					OrderedAppNamesAfterInstall: fakeAppInfoForAppInstallWithAppListSortedTest.ColorOrderNamesAfterAppInstall},
				ExtraSoftwareDeps: []string{"android_p"},
			},
			{
				Name: "tablet_color_androidvm",
				Val: launcher.SortTestType{TabletMode: true, SortMethod: launcher.ColorSort,
					OrderedAppNames:             fakeAppInfoForAppInstallWithAppListSortedTest.ColorOrderNames,
					OrderedAppNamesAfterInstall: fakeAppInfoForAppInstallWithAppListSortedTest.ColorOrderNamesAfterAppInstall},
				ExtraSoftwareDeps: []string{"android_vm"},
			},
		},
		Timeout: chrome.GAIALoginTimeout + arc.BootTimeout + 120*time.Second,
	})
}

// InstallArcAppWithAppListSorted verifies that a new app's location after installation
// maintains the app list sort order. The installations under both temporary sort and permanent
// sort get verified.
func InstallArcAppWithAppListSorted(ctx context.Context, s *testing.State) {
	var opts []chrome.Option
	testParam := s.Param().(launcher.SortTestType)

	tmpDir, err := ioutil.TempDir("", "")
	if err != nil {
		s.Fatal("Failed to create a temporary directory: ", err)
	}
	defer os.RemoveAll(tmpDir)

	fakeAppNames := testParam.OrderedAppNames

	// Prepare fake apps based on the sort method to be verified.
	switch testParam.SortMethod {
	case launcher.AlphabeticalSort:
		opts, err = ash.GeneratePrepareFakeAppsWithNamesOptions(tmpDir, fakeAppNames)
	case launcher.ColorSort:
		iconFileNames := fakeAppInfoForAppInstallWithAppListSortedTest.IconFileNames
		iconData := make([][]byte, len(iconFileNames))
		for index, imageName := range iconFileNames {
			imageBytes, err := launcher.ReadImageBytesFromFilePath(s.DataPath(imageName))
			if err != nil {
				s.Fatalf("Failed to read image byte data from %q: %v", imageName, err)
			}
			iconData[index] = imageBytes
		}
		opts, err = ash.GeneratePrepareFakeAppsWithIconDataOptions(tmpDir, fakeAppNames, iconData)
	}

	if err != nil {
		s.Fatalf("Failed to create fake apps for verifying %v: %v", testParam.SortMethod, err)
	}

	// Turn on both the App List Sort and ARC options.
	opts = append(opts, chrome.EnableFeatures("ProductivityLauncher", "LauncherAppSort"), chrome.ARCEnabled(),
		chrome.UnRestrictARCCPU(), chrome.ExtraArgs(arc.DisableSyncFlags()...))

	cr, err := chrome.New(ctx, opts...)
	if err != nil {
		s.Fatal("Chrome login failed: ", err)
	}
	defer cr.Close(ctx)

	defer faillog.DumpUITreeWithScreenshotOnError(ctx, s.OutDir(), s.HasError, cr, "ui_tree")

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}

	// Setup ARC.
	a, err := arc.New(ctx, s.OutDir())
	if err != nil {
		s.Fatal("Failed to start ARC: ", err)
	}
	defer a.Close(ctx)

	// Enter the tablet/clamshell mode depending on the test param.
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
			s.Fatal("Failed to open expanded Application list view: ", err)
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

	var lastFakeAppName string
	if testParam.SortMethod == launcher.AlphabeticalSort {
		// According to AlphabeticalNames specified by the test parameter, the last app name in alphabetical order
		// should be "m".
		lastFakeAppName = "m"
	} else {
		// Under color sorting order, the black icon should be placed at the rear.
		lastFakeAppName = "black"
	}
	lastFakeApp := nodewith.ClassName(launcher.ExpandedItemsClass).Ancestor(appsGrid).Name(lastFakeAppName)
	ui := uiauto.New(tconn)
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

	// Sort app list to enter temporary sort mode before the Android app installation.
	if err := launcher.TriggerAppListSortAndWaitForUndoButtonExist(ctx, ui, testParam.SortMethod, lastFakeApp); err != nil {
		s.Fatalf("Failed to trigger %v: %v", testParam.SortMethod, err)
	}

	if err := launcher.VerifyFakeAppsOrdered(ctx, ui, appsGrid, fakeAppNames, true /*wait*/); err != nil {
		s.Fatal("Failed to verify fake apps order after first sort: ", err)
	}

	// Install a mock Android app under temporary sort.
	const apk = "ArcInstallAppWithAppListSortedTest.apk"
	if err := a.Install(ctx, arc.APKPath(apk)); err != nil {
		s.Fatal("Failed installing app under temporary sort: ", err)
	}

	// Verify that app installation under temporary sort should commit the sort order, which means:
	// 1. App list items are sorted in order
	// 2. The undo button disappears
	undoButton := nodewith.Name("Undo").ClassName("PillButton")
	installedApp := nodewith.ClassName(launcher.ExpandedItemsClass).Ancestor(appsGrid).Name("InstallAppWithAppListSortedMockApp")
	if err := uiauto.Combine("undo alphabetical sorting",
		ui.WaitUntilGone(undoButton),
		ui.WaitForLocation(lastFakeApp),
	)(ctx); err != nil {
		s.Fatal("Failed to wait app installation with app list sorted to finish: ", err)
	}

	if err := launcher.VerifyFakeAppsOrdered(ctx, ui, appsGrid, testParam.OrderedAppNamesAfterInstall, true /*wait*/); err != nil {
		s.Fatal("Failed to verify fake apps order after installation: ", err)
	}

	if err := a.Uninstall(ctx, "org.chromium.arc.testapp.installappwithapplistsorted"); err != nil {
		s.Fatal("Failed uninstalling app: ", err)
	}

	if err := ui.WaitUntilGone(installedApp)(ctx); err != nil {
		s.Fatal("Failed to wait for the app icon of the uninstalled app to disappear: ", err)
	}

	// Install the mock Android app under permanent sort.
	if err := a.Install(ctx, arc.APKPath(apk)); err != nil {
		s.Fatal("Failed installing app under permanent sort: ", err)
	}

	if err := ui.WaitUntilExists(installedApp)(ctx); err != nil {
		s.Fatal("Failed to wait for the app icon of the installed app to show: ", err)
	}

	if err := launcher.VerifyFakeAppsOrdered(ctx, ui, appsGrid, testParam.OrderedAppNamesAfterInstall, false /*wait*/); err != nil {
		s.Fatal("Failed to verify fake apps order after re-installation: ", err)
	}
}

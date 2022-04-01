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

type installAppWithAppListSortedTestType struct {
	tabletMode                  bool
	sortMethod                  launcher.SortType
	fakeAppNames                []string
	orderedAppNamesAfterInstall []string
}

// The names of the fake apps installed when chrome starts. Used in the tests that verify app installation under alphabetical sort.
var fakeAppAlphabeticalOrderBeforeInstall = []string{"B", "e", "H", "j", "m"}

// The ordered app names after app installation under alphabetical order.
var orderedAppNamesAfterInstallAlphabetical = []string{"B", "e", "H", "InstallAppWithAppListSortedMockApp", "j", "m"}

// Similar to fakeAppAlphabeticalOrderBeforeInstall with one difference: these strings are used in the tests that verify app installation under color sort.
var fakeAppColorOrderBeforeInstall = []string{"white", "red", "cyan", "blue", "purple", "black"}

// The paths to the icons used by fake apps in the tests that verify color sort.
var fakeAppColorOrderIconPaths = []string{"app_list_sort_smoke_white.png", "app_list_sort_smoke_red.png", "app_list_sort_smoke_cyan.png",
	"app_list_sort_smoke_blue.png", "app_list_sort_smoke_purple.png", "app_list_sort_smoke_black.png"}

// The ordered app names after app installation under color order.
var orderedAppNamesAfterInstallColor = []string{"white", "red", "InstallAppWithAppListSortedMockApp", "cyan", "blue", "purple", "black"}

var androidP = "android_p"
var androidVM = "android_vm"

func init() {
	testing.AddTest(&testing.Test{
		Func:         InstallAppWithAppListSorted,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Basic smoke tests for the app list sorting",
		Contacts: []string{
			"cros-system-ui-eng@google.com",
			"chromeos-sw-engprod@google.com",
			"andrewxu@chromium.org",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Data:         fakeAppColorOrderIconPaths,
		Params: []testing.Param{
			{
				Name: "clamshell_alphabetical_androidp",
				Val: installAppWithAppListSortedTestType{tabletMode: false, sortMethod: launcher.AlphabeticalSort,
					fakeAppNames: fakeAppAlphabeticalOrderBeforeInstall, orderedAppNamesAfterInstall: orderedAppNamesAfterInstallAlphabetical},
				ExtraSoftwareDeps: []string{"android_p"},
			},
			{
				Name: "clamshell_alphabetical_androidvm",
				Val: installAppWithAppListSortedTestType{tabletMode: false, sortMethod: launcher.AlphabeticalSort,
					fakeAppNames: fakeAppAlphabeticalOrderBeforeInstall, orderedAppNamesAfterInstall: orderedAppNamesAfterInstallAlphabetical},
				ExtraSoftwareDeps: []string{"android_vm"},
			},
			{
				Name: "tablet_alphabetical_androidp",
				Val: installAppWithAppListSortedTestType{tabletMode: true, sortMethod: launcher.AlphabeticalSort,
					fakeAppNames: fakeAppAlphabeticalOrderBeforeInstall, orderedAppNamesAfterInstall: orderedAppNamesAfterInstallAlphabetical},
				ExtraSoftwareDeps: []string{"android_p"},
			},
			{
				Name: "tablet_alphabetical_androidvm",
				Val: installAppWithAppListSortedTestType{tabletMode: true, sortMethod: launcher.AlphabeticalSort,
					fakeAppNames: fakeAppAlphabeticalOrderBeforeInstall, orderedAppNamesAfterInstall: orderedAppNamesAfterInstallAlphabetical},
				ExtraSoftwareDeps: []string{"android_vm"},
			},
			{
				Name: "clamshell_color_androidp",
				Val: installAppWithAppListSortedTestType{tabletMode: false, sortMethod: launcher.ColorSort,
					fakeAppNames: fakeAppColorOrderBeforeInstall, orderedAppNamesAfterInstall: orderedAppNamesAfterInstallColor},
				ExtraSoftwareDeps: []string{"android_p"},
			},
			{
				Name: "clamshell_color_androidvm",
				Val: installAppWithAppListSortedTestType{tabletMode: false, sortMethod: launcher.ColorSort,
					fakeAppNames: fakeAppColorOrderBeforeInstall, orderedAppNamesAfterInstall: orderedAppNamesAfterInstallColor},
				ExtraSoftwareDeps: []string{"android_vm"},
			},
			{
				Name: "tablet_color_androidp",
				Val: installAppWithAppListSortedTestType{tabletMode: true, sortMethod: launcher.ColorSort,
					fakeAppNames: fakeAppColorOrderBeforeInstall, orderedAppNamesAfterInstall: orderedAppNamesAfterInstallColor},
				ExtraSoftwareDeps: []string{"android_p"},
			},
			{
				Name: "tablet_color_androidvm",
				Val: installAppWithAppListSortedTestType{tabletMode: true, sortMethod: launcher.ColorSort,
					fakeAppNames: fakeAppColorOrderBeforeInstall, orderedAppNamesAfterInstall: orderedAppNamesAfterInstallColor},
				ExtraSoftwareDeps: []string{"android_vm"},
			},
		},
		Timeout: chrome.GAIALoginTimeout + arc.BootTimeout + 120*time.Second,
	})
}

func InstallAppWithAppListSorted(ctx context.Context, s *testing.State) {
	var opts []chrome.Option
	testParam := s.Param().(installAppWithAppListSortedTestType)

	extDirBase, err := ioutil.TempDir("", "")
	if err != nil {
		s.Fatal("Failed to create a temporary directory: ", err)
	}
	defer os.RemoveAll(extDirBase)

	fakeAppNames := testParam.fakeAppNames

	// Prepare fake apps based on the sort method to be verified.
	switch testParam.sortMethod {
	case launcher.AlphabeticalSort:
		opts, err = ash.GeneratePrepareFakeAppsWithNamesOptions(extDirBase, fakeAppNames)
	case launcher.ColorSort:
		iconData := make([][]byte, len(fakeAppColorOrderIconPaths))
		for index, imageName := range fakeAppColorOrderIconPaths {
			imageBytes, err := launcher.ReadImageBytesFromFilePath(s.DataPath(imageName))
			if err != nil {
				s.Fatalf("Failed to read image byte data from %q: %v", imageName, err)
			}
			iconData[index] = imageBytes
		}
		opts, err = ash.GeneratePrepareFakeAppsWithIconDataOptions(extDirBase, fakeAppNames, iconData)
	}

	if err != nil {
		s.Fatalf("Failed to create the fake apps for verifying %v: %v", testParam.sortMethod, err)
	}

	// Enable the app list sort and ARC.
	opts = append(opts, chrome.EnableFeatures("ProductivityLauncher", "LauncherAppSort"), chrome.ARCEnabled(), chrome.ExtraArgs(arc.DisableSyncFlags()...))

	cr, err := chrome.New(ctx, opts...)
	if err != nil {
		s.Fatal("Chrome login failed: ", err)
	}
	defer cr.Close(ctx)

	defer faillog.DumpUITreeWithScreenshotOnError(ctx, s.OutDir(), s.HasError, cr, "ui_tree")

	s.Log("Creating Test API connection")
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

	tabletMode := testParam.tabletMode
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

	appsGrid := nodewith.ClassName(launcher.BubbleAppsGridViewClass)
	if tabletMode {
		appsGrid = nodewith.ClassName(launcher.PagedAppsGridViewClass)
	}

	var lastFakeAppName string
	if testParam.sortMethod == launcher.AlphabeticalSort {
		lastFakeAppName = "m"
	} else {
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

	if err := launcher.TriggerAppListSortAndWaitForUndoButtonExist(ctx, ui, testParam.sortMethod, lastFakeApp); err != nil {
		s.Fatalf("Failed to trigger %v: %v", testParam.sortMethod, err)
	}

	if err := launcher.VerifyFakeAppsOrdered(ctx, ui, appsGrid, fakeAppNames); err != nil {
		s.Fatal("Failed to verify fake apps order: ", err)
	}

	apk := "ArcInstallAppWithAppListSortedTest.apk"
	if err := a.Install(ctx, arc.APKPath(apk)); err != nil {
		s.Fatal("Failed installing app: ", err)
	}

	undoButton := nodewith.Name("Undo").ClassName("PillButton")
	installedApp := nodewith.ClassName(launcher.ExpandedItemsClass).Ancestor(appsGrid).Name("InstallAppWithAppListSortedMockApp")
	if err := uiauto.Combine("undo alphabetical sorting",
		ui.WaitUntilGone(undoButton),
		ui.WaitForLocation(lastFakeApp),
		ui.WaitUntilExists(installedApp),
	)(ctx); err != nil {
		s.Fatal("Failed to wait app installation with app list sorted to finish: ", err)
	}

	if err := launcher.VerifyFakeAppsOrdered(ctx, ui, appsGrid, testParam.orderedAppNamesAfterInstall); err != nil {
		s.Fatal("Failed to verify fake apps order after installation: ", err)
	}

	if err := a.Uninstall(ctx, "org.chromium.arc.testapp.installappwithapplistsorted"); err != nil {
		s.Fatal("Failed uninstalling app: ", err)
	}

	if err := ui.WaitUntilGone(installedApp)(ctx); err != nil {
		s.Fatal("Failed to wait for the app icon of the uninstalled app to disappear: ", err)
	}

	if err := a.Install(ctx, arc.APKPath(apk)); err != nil {
		s.Fatal("Failed installing app again: ", err)
	}

	if err := ui.WaitUntilExists(installedApp)(ctx); err != nil {
		s.Fatal("Failed to wait for the app icon of the installed app to show: ", err)
	}

	if err := launcher.VerifyFakeAppsOrdered(ctx, ui, appsGrid, testParam.orderedAppNamesAfterInstall); err != nil {
		s.Fatal("Failed to verify fake apps order after re-installation: ", err)
	}
}

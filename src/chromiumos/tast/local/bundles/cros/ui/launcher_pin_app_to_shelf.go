// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"
	"time"

	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/bundles/cros/ui/pointer"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/ui"
	"chromiumos/tast/local/chrome/ui/faillog"
	"chromiumos/tast/local/chrome/ui/launcher"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

// init adds the test LauncherPinAppToShelf.
func init() {
	testing.AddTest(&testing.Test{
		Func: LauncherPinAppToShelf,
		Desc: "Using Launcher To Pin Application to Shelf",
		Contacts: []string{
			"seewaifu@chromium.org",
			"kyleshima@chromium.org",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Params: []testing.Param{
			{
				Name: "clamshell_mode",
				Val:  false,
			},
			{
				Name:              "tablet_mode",
				Val:               true,
				ExtraSoftwareDeps: []string{"tablet_mode"},
			},
		},
	})
}

// LauncherPinAppToShelf tests if pinning application onto shelf behaves correctly.
func LauncherPinAppToShelf(ctx context.Context, s *testing.State) {
	tabletMode := s.Param().(bool)

	cr, err := chrome.New(ctx)
	if err != nil {
		s.Fatal("Chrome login failed: ", err)
	}
	defer cr.Close(ctx)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}
	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)

	cleanup, err := ash.EnsureTabletModeEnabled(ctx, tconn, tabletMode)
	if err != nil {
		s.Fatal("Failed to ensure clamshell/tablet mode: ", err)
	}
	defer cleanup(ctx)

	// When a DUT switching from tablet mode to clamshell mode, sometimes it takes a while to settle down.
	// Added a delay here to let all events finishing up.
	if err := testing.Sleep(ctx, 2*time.Second); err != nil {
		s.Fatal("Failed to sleep: ", err)
	}

	app1 := apps.WebStore
	app2 := apps.Camera
	app3 := apps.Settings
	app4 := apps.Help
	app5 := apps.Files

	// Open the Launcher and go to Apps list page.
	if err := launcher.OpenExpandedView(ctx, tconn); err != nil {
		// Open the Launcher and go to Apps list page again for clamshell mode.
		s.Fatal("Failed to open Expanded Application list view: ", err)
	}

	// Pin three apps from the launcher by select "Pin to shelf" on the context menu but do not open them.
	// Then, verify all three applications is on the shelf and existing pinned apps go to the left.
	pinApps(ctx, s, tconn, []apps.App{app1, app2, app3})

	// Launch another app that is not pinned.
	if err := launcher.LaunchApp(ctx, tconn, app4); err != nil {
		s.Fatalf("Failed to run application %v from application list view: %v", app4.Name, err)
	}

	// Verify the newly launched app is the rightmost button.
	rightmostButton(ctx, s, tconn, app4.Name)

	// Launch another app that is not pinned.
	if err := launcher.LaunchApp(ctx, tconn, app5); err != nil {
		s.Fatalf("Failed to run application %v from application list view: %v", app5.Name, err)
	}

	// Verify the newly launched app is the rightmost button.
	rightmostButton(ctx, s, tconn, app5.Name)

	// Pin the app that was just launched and verify swap places with the other unpinned app launched earlier.

	// Save the current position of each app on the shelf.
	itemsBefore, err := ash.ShelfItems(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to obtain the shelf items: ", err)
	}

	var tc *pointer.TouchController
	var stw *input.SingleTouchEventWriter
	var tcc *input.TouchCoordConverter

	if tabletMode {
		// If it is in table mode, we need to event writer and touch converter.
		tc, err = pointer.NewTouchController(ctx, tconn)
		if err != nil {
			s.Fatal("Failed to create the touch controller: ", err)
		}
		stw = tc.EventWriter()
		tcc = tc.TouchCoordConverter()
		defer tc.Close()
	}

	// Pin the app that was just launched.
	if err := ash.PinAppByName(ctx, tconn, stw, tcc, app5.Name); err != nil {
		s.Fatalf("Failed to pin app %q to shelf: %v", app5.Name, err)
	}
	// Save the latest position of each app on the shelf.
	itemsAfter, err := ash.ShelfItems(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to obtain the shelf items: ", err)
	}
	// Make sure the last two applications swap places on shelf after one of them was pinned.
	last := len(itemsBefore) - 1
	if itemsBefore[last].Title != itemsAfter[last-1].Title ||
		itemsBefore[last-1].Title != itemsAfter[last].Title {
		s.Fatalf("%v and %v did not swap place after %v was pinned",
			itemsBefore[last-1].Title, itemsBefore[last].Title, itemsBefore[last].Title)
	}

	// Run any inactive pinned app.
	if err := ash.LaunchAppByName(ctx, tconn, stw, tcc, app1.Name, app1.ID); err != nil {
		s.Fatalf("Failed to run app %v: %v", app1.Name, err)
	}
}

// pinApps pins a list of applications onto the shelf.
func pinApps(ctx context.Context, s *testing.State, tconn *chrome.TestConn, apps []apps.App) {
	prevLocations := buttonLocations(ctx, s, tconn)
	for _, app := range apps {
		if err := launcher.PinAppToShelf(ctx, tconn, app); err != nil {
			s.Fatalf("Fail to pin app %q to shelf: %v", app.Name, err)
		}

		//  Verify that pinned Application appears on the Shelf.
		params := ui.FindParams{Name: app.Name, ClassName: "ash/ShelfAppButton"}
		if err := ui.WaitUntilExists(ctx, tconn, params, 10*time.Second); err != nil {
			s.Fatalf("Failed to find app %v on shelf", app.Name)
		}

		//  Verify that existing pinned apps go to the left after a new app is pinned.
		if !buttonsShiftLeft(ctx, s, tconn, prevLocations) {
			s.Fatal("Buttons were not left shifted after pinning new applications")
		}
		prevLocations = buttonLocations(ctx, s, tconn)
	}
}

// buttonLocations returns the locations of all buttons on the shelf.
func buttonLocations(ctx context.Context, s *testing.State, tconn *chrome.TestConn) map[string]int {
	button2Loc := make(map[string]int)
	params := ui.FindParams{ClassName: "ash/ShelfAppButton"}
	appButtons, err := ui.FindAll(ctx, tconn, params)
	if err != nil {
		s.Fatal("Failed to get all button on shelf: ", err)
	}
	defer appButtons.Release(ctx)
	for _, b := range appButtons {
		button2Loc[b.Name] = b.Location.Left
	}
	return button2Loc
}

// buttonsShiftLeft makes sure all the existing buttons on the shelf shifting to the left after a new item is added to the shelf.
func buttonsShiftLeft(ctx context.Context, s *testing.State, tconn *chrome.TestConn, prevLocations map[string]int) bool {
	params := ui.FindParams{ClassName: "ash/ShelfAppButton"}
	appButtons, err := ui.FindAll(ctx, tconn, params)
	if err != nil {
		s.Fatal("Failed to get all button on shelf: ", err)
	}
	defer appButtons.Release(ctx)
	if len(appButtons) != len(prevLocations)+1 {
		s.Fatalf("Have %v buttons on the shelf; want %v", len(appButtons), len(prevLocations)+1)
	}
	for _, b := range appButtons {
		leftCoord, ok := prevLocations[b.Name]
		if ok && leftCoord <= b.Location.Left {
			s.Fatal("Button: ", b.Name, " Old Location ", leftCoord, " New Location ", b.Location.Left)
		}
	}
	return true
}

// rightmostButton checks if the given app is the rightmost app on the shelf.
func rightmostButton(ctx context.Context, s *testing.State, tconn *chrome.TestConn, appName string) {
	params := ui.FindParams{ClassName: "ash/ShelfAppButton"}
	appButtons, err := ui.FindAll(ctx, tconn, params)
	if err != nil {
		s.Fatal("Failed to get all buttons on shelf: ", err)
	}
	defer appButtons.Release(ctx)
	rightMostButton := appButtons[0]
	for _, b := range appButtons[1:] {
		if b.Location.Left > rightMostButton.Location.Left {
			rightMostButton = b
		}
	}
	if rightMostButton.Name != appName {
		s.Fatalf("The active unpinned application %v is not the rightmost button on the shelf", appName)
	}
}

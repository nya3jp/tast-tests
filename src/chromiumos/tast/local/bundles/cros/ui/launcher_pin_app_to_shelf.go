// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"
	"time"

	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/ui"
	"chromiumos/tast/local/chrome/ui/faillog"
	"chromiumos/tast/local/chrome/ui/launcher"
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
		Pre:          chrome.LoggedIn(),
	})
}

// LauncherPinAppToShelf tests if pinning application onto shelf behaves correctly.
func LauncherPinAppToShelf(ctx context.Context, s *testing.State) {
	cr := s.PreValue().(*chrome.Chrome)
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}
	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)

	app1 := apps.WebStore
	app2 := apps.Camera
	app3 := apps.Settings
	app4 := apps.Help
	app5 := apps.Files

	// Open the Launcher and go to Apps list page.
	if err := launcher.OpenExpandedView(ctx, tconn); err != nil {
		s.Fatal("Failed to open Expanded Application list view: ", err)
	}

	// Pin three apps but do not open them.
	// Steps:
	//     1. Open the Launcher.
	//     2. Right click on any application.
	//     3. Select "Pin to Shelf".
	//     4. Pin three apps but do not open them.
	// Verifications:
	//     1. It takes you to Apps list page.
	//     3a. Verify that pinned Application appears on the Shelf.
	//     3b. Verify that pinned apps go to the left.
	pinApps(ctx, s, tconn, []string{app1.Name, app2.Name, app3.Name})

	// Launch an app that is not pinned.
	// Steps:
	//     5. Launch an app that is not pinned.
	// Verifications:
	//     5. Verify the app that was launched is to the right of the pinned and non-active apps.
	runAppFromListView(ctx, s, tconn, app4.Name)

	// Open the Launcher and go to Apps list page again.
	if err := launcher.OpenExpandedView(ctx, tconn); err != nil {
		s.Fatal("Failed to open Expanded Application list view: ", err)
	}

	// Launch another app that is not pinned.
	// Steps:
	//     6. Launch a new app which is also not pinned.
	runAppFromListView(ctx, s, tconn, app5.Name)

	// Steps:
	//     7. Pin the app you launched in 6.
	// Verifications:
	//     7. It should swap places with the app launched in (5).
	// Save the current position of each app on the shelf.
	oldPosForApps := appPositionOnLauncher(ctx, s, tconn)
	// Pin the app that was just launched.
	pinAppFromShelf(ctx, s, tconn, app5.Name)
	// Save the latest position of each app on the shelf.
	newPosForApps := appPositionOnLauncher(ctx, s, tconn)
	// Make sure the last two applications swap places on shelf after one of them was pinned.
	// Verification
	if oldPosForApps[app4.Name] != newPosForApps[app5.Name] ||
		oldPosForApps[app5.Name] != newPosForApps[app4.Name] {
		s.Fatalf("%v and %v did not swap place after %v was pinned", app4.Name, app5.Name, app5.Name)
	}

	// Run any inactive pinned app.
	// Steps:
	//     8. Run any inactive pinned app.
	if err := ash.LaunchByAppName(ctx, tconn, app1.Name); err != nil {
		s.Fatalf("Failed to run app %v: %v", app1.Name, err)
	}
}

// appPositionOnLauncher return a mapping of application name and its position on the shelf.
func appPositionOnLauncher(ctx context.Context, s *testing.State, tconn *chrome.TestConn) map[string]int {
	positions := make(map[string]int)
	items, err := ash.ShelfItems(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to obtain the shelf items: ", err)
	}
	for i, item := range items {
		positions[item.Title] = i
	}
	return positions
}

// pinApps pins a list of applications onto the shelf.
func pinApps(ctx context.Context, s *testing.State, tconn *chrome.TestConn, appNames []string) {
	prevLocations := buttonLocations(ctx, s, tconn)
	for _, appName := range appNames {
		app, err := launcher.FindAppFromItemView(ctx, tconn, appName)
		if err != nil {
			s.Fatalf("Failed to find application %v from application list view: %v", appName, err)
		}
		selectOptionFromContext(ctx, s, tconn, app, "Pin to shelf")
		//  Verify that pinned Application appears on the Shelf.
		button := findButtonFromShelf(ctx, s, tconn, app.Name)
		button.Release(ctx)
		app.Release(ctx)
		//  Verify that existing pinned apps go to the left after a new app is pinned.
		if !buttonsShiftLeft(ctx, s, tconn, prevLocations) {
			s.Fatal("Buttons were not left shifted after pinning new applications")
		}
		prevLocations = buttonLocations(ctx, s, tconn)
	}
}

// pinAppFromShelf pinned an active app to the shelf.
func pinAppFromShelf(ctx context.Context, s *testing.State, tconn *chrome.TestConn, appName string) {
	button := findButtonFromShelf(ctx, s, tconn, appName)
	defer button.Release(ctx)
	selectOptionFromContext(ctx, s, tconn, button, "Pin")
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
	for _, b := range appButtons {
		left, ok := prevLocations[b.Name]
		if ok && left <= b.Location.Left {
			s.Fatal("Button: ", b.Name, " Old Location ", left, " New Location ", b.Location.Left)
			return false
		}
	}
	return true
}

// runAppFromListView opens an app that is not currently pinned to the shelf.
// This function
func runAppFromListView(ctx context.Context, s *testing.State, tconn *chrome.TestConn, appName string) {
	if err := launcher.LaunchFromListView(ctx, tconn, appName); err != nil {
		s.Fatalf("Failed to run application %v from application list view: %v", appName, err)
	}
	// Verify app is on the shelf
	button := findButtonFromShelf(ctx, s, tconn, appName)
	defer button.Release(ctx)
	// Verify the app that was launched is to the right of the pinned and non-active apps.
	if !rightmostButton(ctx, s, tconn, button) {
		s.Fatalf("The active unpinned application %v is not the rightmost button on the shelf", appName)
	}
}

// selectOptionFromContext selects an option from the menu of a button.
func selectOptionFromContext(ctx context.Context, s *testing.State, tconn *chrome.TestConn, app *ui.Node, optionName string) {
	if err := app.RightClick(ctx); err != nil {
		s.Fatalf("Failed to open menu for application %v: %v", app.Name, err)
	}
	params := ui.FindParams{Name: optionName}
	option, err := ui.FindWithTimeout(ctx, tconn, params, 10*time.Second)
	if err != nil {
		// The option pin to shelf is not available for this icon
		s.Fatalf("Option %q is not available for app %v: %v", optionName, app.Name, err)
	}
	defer option.Release(ctx)
	if err := option.LeftClick(ctx); err != nil {
		s.Fatalf("Failed to select option %q for app %v: %v", optionName, app.Name, err)
	}
	// Make sure all items on the shelf are done moving.
	ui.WaitForLocationChangeCompleted(ctx, tconn)
}

// findButtonFromShelf finds the button on the shelf based on application name.
func findButtonFromShelf(ctx context.Context, s *testing.State, tconn *chrome.TestConn, name string) *ui.Node {
	params := ui.FindParams{Name: name, ClassName: "ash/ShelfAppButton"}
	button, err := ui.FindWithTimeout(ctx, tconn, params, 10*time.Second)
	if err != nil {
		s.Fatalf("Failed to find app %v on shelf", name)
	}
	return button
}

// rightmostButton checks if the given button is the rightmost apps on the shelf.
func rightmostButton(ctx context.Context, s *testing.State, tconn *chrome.TestConn, button *ui.Node) bool {
	params := ui.FindParams{ClassName: "ash/ShelfAppButton"}
	appButtons, err := ui.FindAll(ctx, tconn, params)
	defer appButtons.Release(ctx)
	if err != nil {
		s.Fatal("Failed to get all button on shelf: ", err)
	}
	for _, b := range appButtons {
		if b.Name != button.Name && b.Location.Left >= button.Location.Left {
			return false
		}
	}
	return true
}

// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"
	"time"

	"chromiumos/tast/local/chrome"
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
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Pre:          chrome.LoggedIn(),
	})
}

// LauncherPinAppToShelf tests if pinning application onto shelf behaves correctly.
// Here is the procedure of testing.
//     1. Open the Launcher.
//     2. Right click on any application.
//     3. Select "Pin to Shelf".
//     4. Pin three apps but do not open them.
//     5. Launch an app that is not pinned.
//     6. Launch a new app which is also not pinned.
//     7. Pin the app you launched in 6.
//     8. Run any inactive pinned app.
// Here is a set of verification needed to be down for the above procedure.
//     1. It takes you to Apps list page.
//     3a. Verify that pinned Application appears on the Shelf.
//     3b. Verify that pinned apps go to the left.
//     5. Verify the app that was launched is to the right of the pinned and non-active apps.
//     7. It should swap places with the app launched in (5).
func LauncherPinAppToShelf(ctx context.Context, s *testing.State) {
	cr := s.PreValue().(*chrome.Chrome)
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}
	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)

	appName1 := "Web Store"
	appName2 := "Camera"
	appName3 := "Settings"
	appName4 := "Explore"
	appName5 := "Files"

	// Open the Launcher and go to Apps list page.
	openExpandedView(ctx, s, tconn)

	// Pin three apps but do not open them.
	pinApps(ctx, s, tconn, []string{appName1, appName2, appName3})

	// Launch an app that is not pinned.
	runUnpinnedApp(ctx, s, tconn, appName4)

	// Open the Launcher and go to Apps list page again.
	openExpandedView(ctx, s, tconn)

	// Launch another app that is not pinned.
	runUnpinnedApp(ctx, s, tconn, appName5)

	// Save the current position of each app on the shelf.
	oldPosForApps := appPosOnLauncher(ctx, s, tconn)

	// Pin the app that was just launched.
	pinActiveApp(ctx, s, tconn, appName5)

	// Save the latest position of each app on the shelf.
	newPosForApps := appPosOnLauncher(ctx, s, tconn)

	// Make sure the last two applications swap places on shelf after one of them was pinned.
	if oldPosForApps[appName4] != newPosForApps[appName5] ||
		oldPosForApps[appName5] != newPosForApps[appName4] {
		s.Fatalf("%v and %v did not swap place after %v was pinned", appName4, appName5, appName5)
	}

	// Run any inactive pinned app.
	runPinnedApp(ctx, s, tconn, appName1)
}

// openExpandedView opens the Launcher and go to Apps list page.
func openExpandedView(ctx context.Context, s *testing.State, tconn *chrome.TestConn) {
	if err := launcher.OpenLauncher(ctx, tconn); err != nil {
		s.Fatal("Failed to open Launcher: ", err)
	}
	params := ui.FindParams{ClassName: "ExpandArrowView"}
	expandArrowView, err := ui.FindWithTimeout(ctx, tconn, params, 10*time.Second)
	if err != nil {
		s.Fatal("Failed to find ExpandArrowView: ", err)
	}
	defer expandArrowView.Release(ctx)

	// Keep clicking it until the click is received.
	condition := func(ctx context.Context) (bool, error) {
		exists, err := ui.Exists(ctx, tconn, params)
		return !exists, err
	}
	opts := testing.PollOptions{Timeout: 10 * time.Second, Interval: 500 * time.Millisecond}
	if err := expandArrowView.LeftClickUntil(ctx, condition, &opts); err != nil {
		s.Fatal("Failed to open expanded application list view: ", err)
	}
}

// findAppFromItemView finds the node handle of an application from the application list item view.
func findAppFromItemView(ctx context.Context, s *testing.State, tconn *chrome.TestConn, appName string) *ui.Node {
	params := ui.FindParams{Name: appName, ClassName: "ui/app_list/AppListItemView"}
	app, err := ui.Find(ctx, tconn, params)
	if err != nil {
		s.Logf("Failed to find application %v: %v", appName, err)
		return nil
	}
	return app
}

// appPosOnLauncher return a mapping of application name and its position on the shelf.
func appPosOnLauncher(ctx context.Context, s *testing.State, tconn *chrome.TestConn) map[string]int {
	positions := make(map[string]int)
	params := ui.FindParams{ClassName: "ash/ShelfAppButton"}
	appButtons, err := ui.FindAll(ctx, tconn, params)
	if err != nil {
		s.Fatal("Failed to get all button on shelf: ", err)
	}
	for i, b := range appButtons {
		positions[b.Name] = i
	}
	return positions
}

// pinApps pins a list of applications onto the shelf.
func pinApps(ctx context.Context, s *testing.State, tconn *chrome.TestConn, appNames []string) {
	prevLocations := buttonLocations(ctx, s, tconn)
	for _, appName := range appNames {
		app := findAppFromItemView(ctx, s, tconn, appName)
		if app == nil {
			s.Fatal("Failed to find application ", appName)
		}
		if !pinApp(ctx, s, tconn, app) {
			continue
		}
		//  Verify that existing pinned apps go to the left after a new app is pinned.
		if !buttonsShiftLeft(ctx, s, tconn, prevLocations) {
			s.Fatal("Buttons were not left shifted after pinning new applications")
		}
		prevLocations = buttonLocations(ctx, s, tconn)
	}
}

// pinActiveApp pinned an active app to the shelf.
func pinActiveApp(ctx context.Context, s *testing.State, tconn *chrome.TestConn, appName string) {
	button := findButtonFromShelf(ctx, s, tconn, appName)
	if button == nil {
		s.Fatalf("Failed to find active app %v on shelf", appName)
	}
	pinToShelf := findPinToShelf(ctx, s, tconn, button, "Pin")
	if pinToShelf == nil {
		s.Fatalf(`Failed to find "Pin to shelf" menu for app %v on shelf`, appName)
	}
	defer pinToShelf.Release(ctx)
	if err := pinToShelf.LeftClick(ctx); err != nil {
		s.Fatalf("Failed to pin app %v to shelf: %v", appName, err)
	}
	// Make sure all items on the shelf are done moving.
	ui.WaitForLocationChangeCompleted(ctx, tconn)
}

// buttonLocations returns the locations of all buttons on the shelf.
func buttonLocations(ctx context.Context, s *testing.State, tconn *chrome.TestConn) map[string]int {
	button2Loc := make(map[string]int)
	params := ui.FindParams{ClassName: "ash/ShelfAppButton"}
	appButtons, err := ui.FindAll(ctx, tconn, params)
	if err != nil {
		s.Fatal("Failed to get all button on shelf: ", err)
	}
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
	for _, b := range appButtons {
		left, ok := prevLocations[b.Name]
		if ok && left <= b.Location.Left {
			s.Fatal("Button: ", b.Name, " Old Location ", left, " New Location ", b.Location.Left)
			return false
		}
	}
	return true
}

// runUnpinnedApp opens an app that is not currently pinned to the shelf.
func runUnpinnedApp(ctx context.Context, s *testing.State, tconn *chrome.TestConn, appName string) {
	app := findAppFromItemView(ctx, s, tconn, appName)
	if app == nil {
		s.Fatal("Failed to find application ", appName)
	}
	runApp(ctx, s, tconn, app)
	button := findButtonFromShelf(ctx, s, tconn, appName)
	if button == nil {
		s.Fatalf("Failed to find newly run app %v on shelf", appName)
	}
	// Verify the app that was launched is to the right of the pinned and non-active apps.
	if !rightmostButton(ctx, s, tconn, button) {
		s.Fatalf("The active unpinned application %v is not the rightmost button on the shelf", appName)
	}
}

// runPinnedApp opens an app that is currently pinned to the shelf.
func runPinnedApp(ctx context.Context, s *testing.State, tconn *chrome.TestConn, appName string) {
	app := findButtonFromShelf(ctx, s, tconn, appName)
	if app == nil {
		s.Fatalf("Failed to find application %v on launcher", appName)
	}
	runApp(ctx, s, tconn, app)
}

// pinApp pins an app to the shelf and makes sure it is on the shelf at the end of this function.
func pinApp(ctx context.Context, s *testing.State, tconn *chrome.TestConn, app *ui.Node) bool {
	pinToShelf := findPinToShelf(ctx, s, tconn, app, "Pin to shelf")
	if pinToShelf == nil {
		return false
	}
	defer pinToShelf.Release(ctx)
	if err := pinToShelf.LeftClick(ctx); err != nil {
		s.Fatalf("Failed to pin app %v to shelf: %v", app.Name, err)
	}
	defer ui.WaitForLocationChangeCompleted(ctx, tconn)
	//  Verify that pinned Application appears on the Shelf.
	button := findButtonFromShelf(ctx, s, tconn, app.Name)
	if button == nil {
		s.Fatalf("Failed to find newly pinned app %v on shelf", app.Name)
	}
	return true
}

// findPinToShelf finds the menu for pinning an app to the shelf.
func findPinToShelf(ctx context.Context, s *testing.State, tconn *chrome.TestConn, app *ui.Node, pinMenuName string) *ui.Node {
	if err := app.RightClick(ctx); err != nil {
		s.Fatalf("Failed to open menu for application %v: %v", app.Name, err)
	}
	params := ui.FindParams{Name: pinMenuName}
	pinToShelf, err := ui.Find(ctx, tconn, params)
	if err != nil {
		// The option pin to shelf is not available for this icon
		if err := app.RightClick(ctx); err != nil {
			// Cancel RightClick by another RightClick
			s.Fatalf("Failed to cancel right click for app %v: %v", app.Name, err)
		}
		return nil
	}
	return pinToShelf
}

// runApp runs an application by left clicking the icon.
func runApp(ctx context.Context, s *testing.State, tconn *chrome.TestConn, app *ui.Node) {
	if err := app.LeftClick(ctx); err != nil {
		s.Fatalf("Failed to run app %v: %v", app.Name, err)
	}
	ui.WaitForLocationChangeCompleted(ctx, tconn)
}

// findButtonFromShelf finds the button on the shelf based on application name.
func findButtonFromShelf(ctx context.Context, s *testing.State, tconn *chrome.TestConn, name string) *ui.Node {
	params := ui.FindParams{Name: name, ClassName: "ash/ShelfAppButton"}
	button, _ := ui.Find(ctx, tconn, params)
	return button
}

// rightmostButton checks if the given button is the rightmost apps on the shelf.
func rightmostButton(ctx context.Context, s *testing.State, tconn *chrome.TestConn, button *ui.Node) bool {
	params := ui.FindParams{ClassName: "ash/ShelfAppButton"}
	appButtons, err := ui.FindAll(ctx, tconn, params)
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

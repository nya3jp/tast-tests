// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/ui"
	"chromiumos/tast/local/chrome/ui/faillog"
	"chromiumos/tast/local/chrome/ui/launcher"
	"chromiumos/tast/testing"
)

var shelfAppButton = "ash/ShelfAppButton"

// init adds the test LauncherPinAppToShelf.
func init() {
	testing.AddTest(&testing.Test{
		Func: LauncherPinAppToShelf,
		Desc: "Using Launcher To Pin Application to Shelf",
		Contacts: []string{
			"seewaifu@chromium.org",
			"chromeos-sw-engprod@google.com",
			"tbarzic@chromium.org",
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

	// This test needs to use New instead of precondition because this test
	// assume that the following icons are on the top level of the extended
	// launcher: WebStore, Camera, Settings, Help and Files.
	// To use chrome.New ensures that condition of the desktop is not affected
	// by the previous test of the same run.
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
	if err := ui.WaitForLocationChangeCompleted(ctx, tconn); err != nil {
		s.Fatal("Failed to wait for location changes: ", err)
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
	if err := pinApps(ctx, tconn, []apps.App{app1, app2, app3}); err != nil {
		s.Fatal("Pin apps to the shelf from the launcher: ", err)
	}

	// Launch another app that is not pinned.
	if err := launcher.LaunchApp(ctx, tconn, app4); err != nil {
		s.Fatalf("Failed to run application %v from application list view: %v", app4.Name, err)
	}

	// Verify the newly launched app is the rightmost button.

	if err := rightmostButton(ctx, tconn, app4.Name); err != nil {
		s.Fatalf("The active unpinned application %v is not the rightmost button on the shelf: %v", app4.Name, err)
	}

	// Launch another app that is not pinned.
	if err := launcher.LaunchApp(ctx, tconn, app5); err != nil {
		s.Fatalf("Failed to run application %v from application list view: %v", app5.Name, err)
	}

	// Verify the newly launched app is the rightmost button.
	if err := rightmostButton(ctx, tconn, app5.Name); err != nil {
		s.Fatalf("The active unpinned application %v is not the rightmost button on the shelf: %v", app5.Name, err)
	}

	// Pin the app that was just launched and verify swap places with the other unpinned app launched earlier.

	// Save the current position of each app on the shelf.
	itemsBefore, err := ash.ShelfItems(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to obtain the shelf items: ", err)
	}

	// Pin the app that was just launched.
	if tabletMode {
		err = ash.PinAppFromHotseat(ctx, tconn, app5.Name)
	} else {
		err = ash.PinAppFromShelf(ctx, tconn, app5.Name)
	}
	if err != nil {
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
	if tabletMode {
		err = ash.LaunchAppFromHotseat(ctx, tconn, app1.Name, app1.ID)
	} else {
		err = ash.LaunchAppFromShelf(ctx, tconn, app1.Name, app1.ID)
	}
	if err != nil {
		s.Fatalf("Failed to run app %v: %v", app1.Name, err)
	}
}

// pinApps pins a list of applications onto the shelf.
func pinApps(ctx context.Context, tconn *chrome.TestConn, apps []apps.App) error {
	prevLocations, err := buttonLocations(ctx, tconn)
	if err != nil {
		return errors.Wrap(err, "cannot get location for buttons")
	}
	for _, app := range apps {
		if err := launcher.PinAppToShelf(ctx, tconn, app); err != nil {
			return errors.Wrapf(err, "fail to pin app %q to shelf", app.Name)
		}

		//  Verify that pinned Application appears on the Shelf.
		params := ui.FindParams{Name: app.Name, ClassName: shelfAppButton}
		if err := ui.WaitUntilExists(ctx, tconn, params, 10*time.Second); err != nil {
			return errors.Wrapf(err, "failed to find app %v on shelf", app.Name)
		}

		//  Verify that existing pinned apps go to the left after a new app is pinned.
		if err := buttonsShiftLeft(ctx, tconn, prevLocations); err != nil {
			return errors.Wrap(err, "buttons were not left shifted after pinning new applications")
		}
		prevLocations, err = buttonLocations(ctx, tconn)
		if err != nil {
			return errors.Wrap(err, "cannot get location for buttons")
		}
	}
	return nil
}

// buttonLocations returns the left coordinates of the locations of all buttons on the shelf.
func buttonLocations(ctx context.Context, tconn *chrome.TestConn) (map[string]int, error) {
	button2Loc := make(map[string]int)
	params := ui.FindParams{ClassName: shelfAppButton}
	appButtons, err := ui.FindAll(ctx, tconn, params)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get all button on shelf")
	}
	defer appButtons.Release(ctx)
	for _, b := range appButtons {
		button2Loc[b.Name] = b.Location.Left
	}
	return button2Loc, nil
}

// buttonsShiftLeft makes sure all the existing buttons on the shelf shifting to the left after a new item is added to the shelf.
func buttonsShiftLeft(ctx context.Context, tconn *chrome.TestConn, prevLocations map[string]int) error {
	params := ui.FindParams{ClassName: shelfAppButton}
	appButtons, err := ui.FindAll(ctx, tconn, params)
	if err != nil {
		return errors.Wrap(err, "failed to get all button on shelf")
	}
	defer appButtons.Release(ctx)
	if len(appButtons) != len(prevLocations)+1 {
		return errors.Errorf("have %v buttons on the shelf; want %v", len(appButtons), len(prevLocations)+1)
	}
	for _, b := range appButtons {
		if leftCoord, ok := prevLocations[b.Name]; ok && leftCoord <= b.Location.Left {
			return errors.Errorf("button: %q  old Location %v new Location %v", b.Name, leftCoord, b.Location.Left)
		}
	}
	return nil
}

// rightmostButton checks if the given app is the rightmost app on the shelf.
func rightmostButton(ctx context.Context, tconn *chrome.TestConn, appName string) error {
	params := ui.FindParams{ClassName: shelfAppButton}
	appButtons, err := ui.FindAll(ctx, tconn, params)
	if err != nil {
		return errors.Wrap(err, "failed to get all buttons on shelf")
	}
	defer appButtons.Release(ctx)
	if len(appButtons) == 0 {
		return errors.New("no app buttons on the shelf")
	}
	rightMostButton := appButtons[0]
	for _, b := range appButtons[1:] {
		if b.Location.Left > rightMostButton.Location.Left {
			rightMostButton = b
		}
	}
	if rightMostButton.Name != appName {
		return errors.Errorf("%q is the rightmost button on the shelf; want %q", rightMostButton.Name, appName)
	}
	return nil
}

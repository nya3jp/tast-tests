// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package launcher

import (
	"context"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/launcher"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

var shelfAppButton = "ash/ShelfAppButton"

func init() {
	testing.AddTest(&testing.Test{
		Func:         PinAppToShelf,
		LacrosStatus: testing.LacrosVariantUnknown,
		Desc:         "Using Launcher To Pin Application to Shelf",
		Contacts: []string{
			"seewaifu@chromium.org",
			"chromeos-sw-engprod@google.com",
			"tbarzic@chromium.org",
			"cros-system-ui-eng@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Fixture:      "install2Apps",
		Params: []testing.Param{{
			Name: "productivity_launcher_clamshell_mode",
			Val:  launcher.TestCase{ProductivityLauncher: true, TabletMode: false},
		}, {
			Name: "clamshell_mode",
			Val:  launcher.TestCase{ProductivityLauncher: false, TabletMode: false},
		}, {
			Name:              "productivity_launcher_tablet_mode",
			Val:               launcher.TestCase{ProductivityLauncher: true, TabletMode: true},
			ExtraHardwareDeps: hwdep.D(hwdep.InternalDisplay()),
		}, {
			Name:              "tablet_mode",
			Val:               launcher.TestCase{ProductivityLauncher: false, TabletMode: true},
			ExtraHardwareDeps: hwdep.D(hwdep.InternalDisplay()),
		}},
	})
}

// PinAppToShelf tests if pinning application onto shelf behaves correctly.
func PinAppToShelf(ctx context.Context, s *testing.State) {
	testCase := s.Param().(launcher.TestCase)
	tabletMode := testCase.TabletMode
	productivityLauncher := testCase.ProductivityLauncher
	var opts = s.FixtValue().([]chrome.Option)
	if productivityLauncher {
		opts = append(opts, chrome.EnableFeatures("ProductivityLauncher"))
	} else {
		opts = append(opts, chrome.DisableFeatures("ProductivityLauncher"))
	}

	cr, err := chrome.New(ctx, opts...)
	if err != nil {
		s.Fatal("Failed to start chrome: ", err)
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

	if !tabletMode {
		if err := ash.WaitForLauncherState(ctx, tconn, ash.Closed); err != nil {
			s.Fatal("Launcher not closed after transition to clamshell mode: ", err)
		}
	}

	// Open the Launcher and go to Apps list page.
	if productivityLauncher && !tabletMode {
		if err := launcher.OpenBubbleLauncher(tconn)(ctx); err != nil {
			s.Fatal("Failed to open bubble launcher: ", err)
		}
	} else {
		if err := launcher.OpenExpandedView(tconn)(ctx); err != nil {
			s.Fatal("Failed to open Expanded Application list view: ", err)
		}
	}

	if err := launcher.WaitForStableNumberOfApps(ctx, tconn); err != nil {
		s.Fatal("Failed to wait for item count in app list to stabilize: ", err)
	}

	app1 := apps.WebStore
	app2 := apps.App{ID: "fake_0", Name: "fake app 0"}
	app3 := apps.App{ID: "fake_1", Name: "fake app 1"}
	app4 := apps.Settings
	app5 := apps.Help

	var container *nodewith.Finder
	if productivityLauncher && !tabletMode {
		container = nodewith.ClassName(launcher.BubbleAppsGridViewClass)
	} else {
		container = nodewith.ClassName(launcher.PagedAppsGridViewClass)
	}
	// Pin three apps from the launcher by select "Pin to shelf" on the context menu but do not open them.
	// Then, verify all three applications is on the shelf and existing pinned apps go to the left.
	if err := pinApps(ctx, tconn, []apps.App{app1, app2, app3}, container); err != nil {
		s.Fatal("Pin apps to the shelf from the launcher: ", err)
	}

	// Launch another app that is not pinned.
	if err := launcher.LaunchApp(tconn, app4.Name)(ctx); err != nil {
		s.Fatalf("Failed to run application %v from application list view: %v", app4.Name, err)
	}

	// Verify the newly launched app is the rightmost button.

	if err := rightmostButton(ctx, tconn, app4.Name); err != nil {
		s.Fatalf("The active unpinned application %v is not the rightmost button on the shelf: %v", app4.Name, err)
	}

	// Launch another app that is not pinned.
	if err := launcher.LaunchApp(tconn, app5.Name)(ctx); err != nil {
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
		err = ash.UpdateAppPinFromHotseat(ctx, tconn, app5.Name, true)
	} else {
		err = ash.UpdateAppPinFromShelf(ctx, tconn, app5.Name, true)
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
	if err = ash.LaunchAppFromShelf(ctx, tconn, app1.Name, app1.ID); err != nil {
		s.Fatalf("Failed to run app %v: %v", app1.Name, err)
	}

	// Unpin a running app, and verify the app remains in shelf, but gets shifted to the right.
	if tabletMode {
		err = ash.UpdateAppPinFromHotseat(ctx, tconn, app1.Name, false)
	} else {
		err = ash.UpdateAppPinFromShelf(ctx, tconn, app1.Name, false)
	}

	if err != nil {
		s.Fatalf("Failed to unpin app %q to shelf: %v", app1.Name, err)
	}

	// Save the latest position of each app on the shelf.
	items, err := ash.ShelfItems(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to obtain the shelf items: ", err)
	}
	// Make sure the unpinned running app is positioned on the right of the most recently pinned app.
	app1Index, err := getAppIndexInShelf(items, app1.Name)
	if err != nil {
		s.Fatalf("Unable to find %q, %v", app1.Name, err)
	}

	app5Index, err := getAppIndexInShelf(items, app5.Name)
	if app5Index == -1 {
		s.Fatalf("Unable to find %q, %v", app5.Name, err)
	}

	if app1Index < app5Index {
		s.Fatalf("%q not moved after %q after unpinning", app1.Name, app5.Name)
	}

	// Unpin an app that's not running, and verify it gets removed from shelf.
	if err := launcher.UnpinAppFromShelf(tconn, app3, container)(ctx); err != nil {
		s.Fatalf("Failed to unpin app %q from shelf", app3.Name)
	}

	items, err = ash.ShelfItems(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to obtain the shelf items: ", err)
	}

	app3Index, err := getAppIndexInShelf(items, app3.Name)

	if err == nil {
		s.Fatalf("Found app \"%q\" in shelf at %d after it was unpinned", app3.Name, app3Index)
	} else if err.Error() != "Not found" {
		s.Fatal("Failed to search for app in shelf items: ", err)
	}
}

// getAppIndexInShelf returns the index of an app with the provided name within shelf item list.
func getAppIndexInShelf(items []*ash.ShelfItem, name string) (int, error) {
	for index, item := range items {
		if item.Title == name {
			return index, nil
		}
	}

	return -1, errors.New("Not found")
}

// pinApps pins a list of applications onto the shelf.
func pinApps(ctx context.Context, tconn *chrome.TestConn, apps []apps.App, container *nodewith.Finder) error {
	prevLocations, err := buttonLocations(ctx, tconn)
	if err != nil {
		return errors.Wrap(err, "cannot get location for buttons")
	}
	for _, app := range apps {
		if err := launcher.PinAppToShelf(tconn, app, container)(ctx); err != nil {
			return errors.Wrapf(err, "fail to pin app %q to shelf", app.Name)
		}

		//  Verify that pinned Application appears on the Shelf.
		ui := uiauto.New(tconn)
		finder := nodewith.Name(app.Name).ClassName(shelfAppButton)
		if err := ui.WithTimeout(10 * time.Second).WaitUntilExists(finder)(ctx); err != nil {
			return errors.Wrapf(err, "failed to find app %v on shelf", app.Name)
		}

		if err := ui.WaitForLocation(finder)(ctx); err != nil {
			errors.Wrap(err, "failed to wait for location changes")
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
	finder := nodewith.ClassName(shelfAppButton)
	appButtons, err := uiauto.New(tconn).NodesInfo(ctx, finder)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get all button on shelf")
	}
	for _, b := range appButtons {
		button2Loc[b.Name] = b.Location.Left
	}
	return button2Loc, nil
}

// buttonsShiftLeft makes sure all the existing buttons on the shelf shifting to the left after a new item is added to the shelf.
func buttonsShiftLeft(ctx context.Context, tconn *chrome.TestConn, prevLocations map[string]int) error {
	finder := nodewith.ClassName(shelfAppButton)
	appButtons, err := uiauto.New(tconn).NodesInfo(ctx, finder)
	if err != nil {
		return errors.Wrap(err, "failed to get all button on shelf")
	}
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
	finder := nodewith.ClassName(shelfAppButton)
	appButtons, err := uiauto.New(tconn).NodesInfo(ctx, finder)
	if err != nil {
		return errors.Wrap(err, "failed to get all buttons on shelf")
	}
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

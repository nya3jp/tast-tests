// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package shelf

import (
	"context"
	"io/ioutil"
	"os"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/coords"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         RtlSmoke,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Tests the rearrangement of shelf app icons",
		Contacts: []string{
			"andrewxu@chromium.org",
			"tbarzic@chromium.org",
			"chromeos-sw-engprod@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Timeout:      3 * time.Minute,
	})
}

// RtlSmoke verifies shelf behaviors with a right-to-left language.
func RtlSmoke(ctx context.Context, s *testing.State) {
	dir, err := ioutil.TempDir("", "")
	if err != nil {
		s.Fatal("Failed to create a temporary directory: ", err)
	}
	defer os.RemoveAll(dir)

	const numFakeApps = 2
	opts, err := ash.GeneratePrepareFakeAppsOptions(dir, numFakeApps)
	opts = append(opts, chrome.ExtraArgs("--lang=ar"))
	cr, err := chrome.New(ctx, opts...)

	if err != nil {
		s.Fatal("Chrome login failed: ", err)
	}
	defer cr.Close(ctx)

	defer faillog.DumpUITreeWithScreenshotOnError(ctx, s.OutDir(), s.HasError, cr, "ui_tree")

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect Test API: ", err)
	}

	// Ensure that the device is in clamshell mode.
	cleanup, err := ash.EnsureTabletModeEnabled(ctx, tconn, false)
	if err != nil {
		s.Fatal("Failed to ensure clamshell/tablet mode: ", err)
	}
	defer cleanup(ctx)

	resetPinState, err := ash.ResetShelfPinState(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to get the function to reset pin states: ", err)
	}
	defer resetPinState(ctx)

	items, err := ash.ShelfItems(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to get shelf items: ", err)
	}

	// Get the expected browser.
	chromeApp, err := apps.ChromeOrChromium(ctx, tconn)
	if err != nil {
		s.Fatal("Could not find the Chrome app: ", err)
	}

	var itemsToUnpin []string

	// Unpin all apps except the browser.
	for _, item := range items {
		if item.AppID != chromeApp.ID {
			itemsToUnpin = append(itemsToUnpin, item.AppID)
		}
	}

	if err := ash.UnpinApps(ctx, tconn, itemsToUnpin); err != nil {
		s.Fatalf("Failed to unpin apps %v: %v", itemsToUnpin, err)
	}

	if err := ash.PinApps(ctx, tconn, []string{apps.Settings.ID}); err != nil {
		s.Fatal("Failed to pin Settings to shelf: ", err)
	}

	idArray := []string{apps.Settings.ID, chromeApp.ID}
	nameArray, err := ash.ShelfItemTitleFromID(ctx, tconn, idArray)
	if err != nil {
		s.Fatalf("Failed to get the shelf item names for %v after pinning Settings", idArray)
	}

	ui := uiauto.New(tconn)
	if err := ui.WaitForLocation(nodewith.ClassName("ash/ShelfAppButton").Name(nameArray[0]))(ctx); err != nil {
		s.Fatal("Failed to wait for the Settings app to be idle after pinning: ", err)
	}

	if err := verifyShelfAppOrder(ctx, tconn, ui, nameArray, true, s); err != nil {
		s.Fatalf("Failed to verify the shelf app bounds following %v: %v", nameArray, err)
	}

	if err := ash.PinApps(ctx, tconn, []string{apps.Files.ID}); err != nil {
		s.Fatal("Failed to pin Files to shelf: ", err)
	}

	idArray = []string{apps.Files.ID, apps.Settings.ID, chromeApp.ID}
	nameArray, err = ash.ShelfItemTitleFromID(ctx, tconn, idArray)
	if err != nil {
		s.Fatalf("Failed to get the shelf item names for %v after pinning Files", idArray)
	}

	if err := ui.WaitForLocation(nodewith.ClassName("ash/ShelfAppButton").Name(nameArray[0]))(ctx); err != nil {
		s.Fatal("Failed to wait for the Files app to be idle after pinning: ", err)
	}

	if err := verifyShelfAppOrder(ctx, tconn, ui, nameArray, true, s); err != nil {
		s.Fatalf("Failed to verify the shelf app bounds following %v after pinning Files: %v", nameArray, err)
	}

	const fakeAppName = "fake app 0"
	installedApps, err := ash.ChromeApps(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to obtain the list of the installed apps: ", err)
	}

	fakeAppID := ""

	// Some apps will disappear after pinned and make the shelf not overflow.
	// Choose fake apps to prevent the problem.
	for _, app := range installedApps {
		if app.Name == fakeAppName {
			fakeAppID = app.AppID
			break
		}
	}

	if fakeAppID == "" {
		s.Errorf("Failed to get the id for %s", fakeAppName)
	}

	if err := ash.PinApps(ctx, tconn, []string{fakeAppID}); err != nil {
		s.Fatal("Failed to pin Files to shelf: ", err)
	}

	if err := ui.WaitForLocation(nodewith.ClassName("ash/ShelfAppButton").Name(fakeAppName))(ctx); err != nil {
		s.Fatal("Failed to wait for the Files app to be idle after pinning: ", err)
	}

	nameArray = append([]string{fakeAppName}, nameArray...)
	if err := verifyShelfAppOrder(ctx, tconn, ui, nameArray, true, s); err != nil {
		s.Fatalf("Failed to verify the shelf app bounds following %v after pinning Files: %v", nameArray, err)
	}
}

func verifyShelfAppOrder(ctx context.Context, tconn *chrome.TestConn, ui *uiauto.Context, appNames []string, isHorizontal bool, s *testing.State) error {
	if len(appNames) == 1 {
		return nil
	}

	boundsArray := make([]*coords.Rect, len(appNames))
	for index, appName := range appNames {
		appButton := nodewith.ClassName("ash/ShelfAppButton").Name(appName)
		bounds, err := ui.Location(ctx, appButton)
		if err != nil {
			return errors.Wrapf(err, "failed to get the screen bounds of %s", appName)
		}
		boundsArray[index] = bounds
	}

	for index, bounds := range boundsArray {
		if index == 0 {
			continue
		}

		if isHorizontal && bounds.Left <= boundsArray[index-1].Right() {
			return errors.Errorf("got: %s is in front of %s; expect: %s is behind %s", appNames[index], appNames[index-1], appNames[index], appNames[index-1])
		}

		if !isHorizontal && bounds.Top <= boundsArray[index-1].Bottom() {
			return errors.Errorf("got: %s is below %s; expect: %s is above %s", appNames[index], appNames[index-1], appNames[index], appNames[index-1])
		}
	}

	return nil
}

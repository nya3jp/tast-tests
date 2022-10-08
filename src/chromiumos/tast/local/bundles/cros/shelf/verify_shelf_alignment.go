// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package shelf

import (
	"context"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/browser"
	"chromiumos/tast/local/chrome/browser/browserfixt"
	"chromiumos/tast/local/chrome/display"
	"chromiumos/tast/local/chrome/lacros/lacrosfixt"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/testing"
)

type testParam struct {
	isUnderRTL bool
	bt         browser.Type
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         VerifyShelfAlignment,
		LacrosStatus: testing.LacrosVariantExists,
		Desc:         "Tests the shelf alignment",
		Contacts: []string{
			"andrewxu@chromium.org",
			"tbarzic@chromium.org",
			"chromeos-sw-engprod@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Timeout:      3 * time.Minute,
		Params: []testing.Param{{
			Name: "normal",
			Val:  testParam{false, browser.TypeAsh},
		}, {
			Name: "rtl",
			Val:  testParam{true, browser.TypeAsh},
		}, {
			Name: "lacros",
			Val:  testParam{false, browser.TypeLacros},
		}, {
			Name: "rtl_lacros",
			Val:  testParam{true, browser.TypeLacros},
		}},
	})
}

// VerifyShelfAlignment verifies that changing the shelf alignment works as expected.
func VerifyShelfAlignment(ctx context.Context, s *testing.State) {
	// Reserve a few seconds for various cleanup.
	cleanUpCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	// If true, the system UI is adapted to right-to-left languages.
	isUnderRTL := s.Param().(testParam).isUnderRTL
	bt := s.Param().(testParam).bt

	// Set up the browser.
	var opts []chrome.Option
	if isUnderRTL {
		opts = append(opts, chrome.ExtraArgs("--force-ui-direction=rtl"))
	}
	cr, _, closeBrowser, err := browserfixt.SetUpWithNewChrome(ctx, bt, lacrosfixt.NewConfig(), opts...)
	if err != nil {
		s.Fatalf("Failed to start chrome (rtl? %v): %v", isUnderRTL, err)
	}
	defer cr.Close(cleanUpCtx)
	defer closeBrowser(cleanUpCtx)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect Test API: ", err)
	}

	// Ensure that the device is in clamshell mode.
	cleanup, err := ash.EnsureTabletModeEnabled(ctx, tconn, false)
	if err != nil {
		s.Fatal("Failed to ensure clamshell mode: ", err)
	}
	defer cleanup(cleanUpCtx)

	resetPinState, err := ash.ResetShelfPinState(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to get the function to reset pin states: ", err)
	}
	defer resetPinState(cleanUpCtx)

	items, err := ash.ShelfItems(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to get shelf items: ", err)
	}

	// Get the expected browser.
	browserApp, err := apps.PrimaryBrowser(ctx, tconn)
	if err != nil {
		s.Fatal("Could not find the Chrome app: ", err)
	}

	var itemsToUnpin []string

	// Unpin all apps except the browser.
	for _, item := range items {
		if item.AppID != browserApp.ID {
			itemsToUnpin = append(itemsToUnpin, item.AppID)
		}
	}

	if err := ash.UnpinApps(ctx, tconn, itemsToUnpin); err != nil {
		s.Fatalf("Failed to unpin apps %v: %v", itemsToUnpin, err)
	}

	// Pin the Settings app to create a more complex scenario for testing.
	if err := ash.PinApps(ctx, tconn, []string{apps.Settings.ID}); err != nil {
		s.Fatal("Failed to pin Settings to shelf")
	}

	// Get the primary display info.
	dispInfo, err := display.GetPrimaryInfo(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to get primary display info: ", err)
	}

	// Ensure that the shelf is placed at the bottom.
	originalAlignment, err := ash.GetShelfAlignment(ctx, tconn, dispInfo.ID)
	if originalAlignment != ash.ShelfAlignmentBottom {
		if err := ash.SetShelfAlignment(ctx, tconn, dispInfo.ID, ash.ShelfAlignmentBottom); err != nil {
			s.Fatal("Failed to place the shelf at the bottom: ", err)
		}
	}

	// Wait until the UI becomes idle.
	ui := uiauto.New(tconn)
	shelfInstance := nodewith.ClassName("ScrollableShelfView")
	if err := ui.WaitForLocation(shelfInstance)(ctx); err != nil {
		s.Fatal("Failed to wait for the shelf to be idle when the shelf alignment is ShelfAlignmentBottom: ", err)
	}
	appIcon := nodewith.ClassName(ash.ShelfIconClassName).Role(role.Button).Nth(0)
	if err := ui.WaitForLocation(appIcon)(ctx); err != nil {
		s.Fatal("Failed to wait for the first shelf app icon to be idle when the shelf alignment is ShelfAlignmentBottom: ", err)
	}

	homeButton := nodewith.ClassName("ash/HomeButton")
	homeButtonBounds, err := ui.Location(ctx, homeButton)
	if err != nil {
		s.Fatal("Failed to find the home button: ", err)
	}

	dispBounds := dispInfo.Bounds
	const gapUpperBound = 20

	systemTray := nodewith.ClassName("UnifiedSystemTray")
	systemTrayBounds, err := ui.Location(ctx, systemTray)
	if err != nil {
		s.Fatal("Failed to find the system tray view: ", err)
	}

	// Check the distance between the home button and the screen left side when isUnderRTL is false.
	if !isUnderRTL && homeButtonBounds.Left-dispBounds.Left > gapUpperBound {
		s.Fatalf("Expected the distance between homeButtonBounds.Left and dispBounds.Left is not greater than %q when the shelf alignment is ShelfAlignmentBottom; the actual gap is %q", gapUpperBound, homeButtonBounds.Left-dispBounds.Left)
	}

	// Check the distance between the home button and the screen right side when isUnderRTL is true.
	if isUnderRTL && dispBounds.Right()-homeButtonBounds.Right() > gapUpperBound {
		s.Fatalf("Expected the distance between dispBounds.Right() and homeButtonBounds.Right() under RTL is not greater than %q when the shelf alignment is ShelfAlignmentBottom; the actual gap is %q", gapUpperBound, dispBounds.Right()-homeButtonBounds.Right())
	}

	// Check the distance between the system tray view and the screen right side when isUnderRTL is false.
	if !isUnderRTL && dispBounds.Right()-systemTrayBounds.Right() > gapUpperBound {
		s.Fatalf("Expected the distance between dispBounds.Right() and systemTrayBounds.Right() is not greater than %q when the shelf alignment is ShelfAlignmentBottom; the actual gap is %q", gapUpperBound, dispBounds.Right()-systemTrayBounds.Right())
	}

	// Check the distance between the system tray view and the screen left side when isUnderRTL is true.
	if isUnderRTL && systemTrayBounds.Left-dispBounds.Left > gapUpperBound {
		s.Fatalf("Expected the distance between systemTrayBounds.Left and dispBounds.Left is not greater than %q when the shelf alignment is ShelfAlignmentBottom; the actual gap is %q", gapUpperBound, systemTrayBounds.Left-dispBounds.Left)
	}

	if err := ash.VerifyShelfAppAlignment(ctx, tconn, ash.ShelfAlignmentBottom); err != nil {
		s.Fatal("Failed to verify shelf app icon alignment when the shelf alignment is ShelfAlignmentBottom: ", err)
	}

	// Pin the Files app then verify the shelf app icon alignment again.
	if err := ash.PinApps(ctx, tconn, []string{apps.FilesSWA.ID}); err != nil {
		s.Fatal("Failed to pin Files to shelf when the shelf alignment is ShelfAlignmentBottom")
	}
	if err := ash.VerifyShelfAppAlignment(ctx, tconn, ash.ShelfAlignmentBottom); err != nil {
		s.Fatal("Failed to verify shelf app icon alignment after pinning the Files app when the shelf alignment is ShelfAlignmentBottom: ", err)
	}

	// Unpin the Files app.
	if err := ash.UnpinApps(ctx, tconn, []string{apps.FilesSWA.ID}); err != nil {
		s.Fatal("Failed to unpin Files when the shelf alignment is ShelfAlignmentBottom")
	}

	// Place the shelf at the left then wait until UI becomes idle.
	if err := ash.SetShelfAlignment(ctx, tconn, dispInfo.ID, ash.ShelfAlignmentLeft); err != nil {
		s.Fatal("Failed to place the shelf at the left: ", err)
	}
	if err := ui.WaitForLocation(shelfInstance)(ctx); err != nil {
		s.Fatal("Failed to wait for the shelf to be idle when the shelf alignment is ShelfAlignmentLeft: ", err)
	}
	if err := ui.WaitForLocation(appIcon)(ctx); err != nil {
		s.Fatal("Failed to wait for the first shelf app icon to be idle when the shelf alignment is ShelfAlignmentLeft: ", err)
	}

	// Check the distance between the home button and the screen top side.
	homeButtonBounds, err = ui.Location(ctx, homeButton)
	if err != nil {
		s.Fatal("Failed to find the home button with the left shelf: ", err)
	}
	if homeButtonBounds.Top-dispBounds.Top > gapUpperBound {
		s.Fatalf("Expected the distance between homeButtonBounds.Top and dispBounds.Top is not greater than %q when the shelf alignment is ShelfAlignmentLeft; the actual gap is %q", gapUpperBound, homeButtonBounds.Top-dispBounds.Top)
	}

	systemTrayBounds, err = ui.Location(ctx, systemTray)
	if err != nil {
		s.Fatal("Failed to find the system tray view with the left shelf: ", err)
	}

	if dispBounds.Bottom()-systemTrayBounds.Bottom() > gapUpperBound {
		s.Fatalf("Expected the distance between dispBounds.Bottom() and systemTrayBounds.Bottom() is not greater than %q when the shelf alignment is ShelfAlignmentLeft; the actual gap is %q", gapUpperBound, dispBounds.Bottom()-systemTrayBounds.Bottom())
	}

	if err := ash.VerifyShelfAppAlignment(ctx, tconn, ash.ShelfAlignmentLeft); err != nil {
		s.Fatal("Failed to verify shelf app icon alignment when the shelf alignment is ShelfAlignmentLeft: ", err)
	}

	// Pin the Files app then verify the shelf app icon alignment again.
	if err := ash.PinApps(ctx, tconn, []string{apps.Files.ID}); err != nil {
		s.Fatal("Failed to pin Files to shelf when the shelf alignment is ShelfAlignmentLeft")
	}
	if err := ash.VerifyShelfAppAlignment(ctx, tconn, ash.ShelfAlignmentLeft); err != nil {
		s.Fatal("Failed to verify shelf app icon alignment after pinning the Files app when the shelf alignment is ShelfAlignmentLeft: ", err)
	}

	// Unpin the Files app.
	if err := ash.UnpinApps(ctx, tconn, []string{apps.Files.ID}); err != nil {
		s.Fatal("Failed to unpin Files when the shelf alignment is ShelfAlignmentLeft")
	}

	// Place the shelf at the right then wait until UI becomes idle.
	if err := ash.SetShelfAlignment(ctx, tconn, dispInfo.ID, ash.ShelfAlignmentRight); err != nil {
		s.Fatal("Failed to place the shelf at the right: ", err)
	}
	if err := ui.WaitForLocation(shelfInstance)(ctx); err != nil {
		s.Fatal("Failed to wait for the shelf to be idle when the shelf alignment is ShelfAlignmentRight: ", err)
	}
	if err := ui.WaitForLocation(appIcon)(ctx); err != nil {
		s.Fatal("Failed to wait for the first shelf app icon to be idle when the shelf alignment is ShelfAlignmentRight: ", err)
	}

	homeButtonBounds, err = ui.Location(ctx, homeButton)
	if err != nil {
		s.Fatal("Failed to find the home button with the right shelf: ", err)
	}
	if homeButtonBounds.Top-dispBounds.Top > gapUpperBound {
		s.Fatalf("Expected the distance between homeButtonBounds.Top and dispBounds.Top is not greater than %q when the shelf alignment is ShelfAlignmentRight; the actual gap is %q", gapUpperBound, homeButtonBounds.Top-dispBounds.Top)
	}

	systemTrayBounds, err = ui.Location(ctx, systemTray)
	if err != nil {
		s.Fatal("Failed to find the system tray view with the right shelf: ", err)
	}
	if dispBounds.Bottom()-systemTrayBounds.Bottom() > gapUpperBound {
		s.Fatalf("Expected the distance between dispBounds.Bottom() and systemTrayBounds.Bottom() is not greater than %q when the shelf alignment is ShelfAlignmentRight; the actual gap is %q", gapUpperBound, dispBounds.Bottom()-systemTrayBounds.Bottom())
	}

	if err := ash.VerifyShelfAppAlignment(ctx, tconn, ash.ShelfAlignmentRight); err != nil {
		s.Fatal("Failed to verify shelf app icon alignment when the shelf alignment is ShelfAlignmentRight: ", err)
	}

	// Pin the Files app then verify the shelf app icon alignment again.
	if err := ash.PinApps(ctx, tconn, []string{apps.Files.ID}); err != nil {
		s.Fatal("Failed to pin Files to shelf when the shelf alignment is ShelfAlignmentRight: ", err)
	}
	if err := ash.VerifyShelfAppAlignment(ctx, tconn, ash.ShelfAlignmentRight); err != nil {
		s.Fatal("Failed to verify shelf app icon alignment after pinning the Files app when the shelf alignment is ShelfAlignmentRight: ", err)
	}

	// Unpin the Files app.
	if err := ash.UnpinApps(ctx, tconn, []string{apps.Files.ID}); err != nil {
		s.Fatal("Failed to unpin Files when the shelf alignment is ShelfAlignmentRight")
	}

	// Restore the shelf alignment.
	if err := ash.SetShelfAlignment(ctx, tconn, dispInfo.ID, originalAlignment); err != nil {
		s.Fatalf("Failed to restore the shelf alignment to %v: %v", originalAlignment, err)
	}
}

// Copyright 2022 The ChromiumOS Authors.
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
	"chromiumos/tast/local/chrome/display"
	"chromiumos/tast/local/chrome/uiauto/launcher"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/testing"
)

type overflowShelfSmokeTestType struct {
	isUnderRTL bool // If true, the system UI is adapted to right-to-left languages.
	inTablet   bool
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         OverflowShelfAlignment,
		LacrosStatus: testing.LacrosVariantNeeded,
		Desc:         "Tests the basic features of the overflow shelf",
		Contacts: []string{
			"andrewxu@chromium.org",
			"tbarzic@chromium.org",
			"chromeos-sw-engprod@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Timeout:      3 * time.Minute,
		Params: []testing.Param{{
			Name: "clamshell_mode_ltr",
			Val: overflowShelfSmokeTestType{
				isUnderRTL: false,
				inTablet:   false,
			},
			Fixture: "chromeLoggedInWith100FakeApps",
		},
			{
				Name: "clamshell_mode_rtl",
				Val: overflowShelfSmokeTestType{
					isUnderRTL: true,
					inTablet:   false,
				},
				Fixture: "install100Apps",
			},
			{
				Name: "tablet_mode_ltr",
				Val: overflowShelfSmokeTestType{
					isUnderRTL: false,
					inTablet:   true,
				},
				Fixture: "chromeLoggedInWith100FakeApps",
			},
			{
				Name: "tablet_mode_rtl",
				Val: overflowShelfSmokeTestType{
					isUnderRTL: true,
					inTablet:   true,
				},
				Fixture: "install100Apps",
			},
		},
	})
}

// OverflowShelfAlignment verifies the basic features of the overflow shelf, i.e. the
// shelf that accommodates so many shelf icons that some icons are hidden and the shelf arrow button(s) shows.
func OverflowShelfAlignment(ctx context.Context, s *testing.State) {
	testType := s.Param().(overflowShelfSmokeTestType)
	isUnderRTL := testType.isUnderRTL

	var cr *chrome.Chrome
	if isUnderRTL {
		var err error
		opts := s.FixtValue().([]chrome.Option)
		opts = append(opts, chrome.ExtraArgs("--force-ui-direction=rtl"))
		cr, err = chrome.New(ctx, opts...)
		if err != nil {
			s.Fatal("Failed to start chrome with rtl: ", err)
		}
	} else {
		cr = s.FixtValue().(*chrome.Chrome)
	}

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect Test API: ", err)
	}

	cleanUpCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	isInTablet := testType.inTablet
	originallyEnabled, err := ash.TabletModeEnabled(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to check if DUT is in tablet mode: ", err)
	}

	// Ensure that the device is in clamshell mode.
	cleanup, err := ash.EnsureTabletModeEnabled(ctx, tconn, isInTablet)
	if err != nil {
		s.Fatalf("Failed to ensure the tablet state %t: %v", isInTablet, err)
	}
	defer cleanup(cleanUpCtx)

	// Get the primary display info.
	dispInfo, err := display.GetPrimaryInfo(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to get primary display info: ", err)
	}

	// Ensure that the shelf is placed at the bottom when the test runs in the clamshell mode.
	if !isInTablet {
		var err error
		originalAlignment, err := ash.GetShelfAlignment(ctx, tconn, dispInfo.ID)
		if err != nil {
			s.Fatal("Failed to get the shelf alignment: ", err)
		}

		if originalAlignment != ash.ShelfAlignmentBottom {
			if err := ash.SetShelfAlignment(ctx, tconn, dispInfo.ID, ash.ShelfAlignmentBottom); err != nil {
				s.Fatal("Failed to place the shelf at the bottom: ", err)
			}

			// Restore the original alignment.
			defer ash.SetShelfAlignment(ctx, tconn, dispInfo.ID, originalAlignment)
		}
	}

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

	// Ensure that the tablet launcher is closed before opening a launcher instance for test in clamshell.
	if originallyEnabled && !isInTablet {
		if err := ash.WaitForLauncherState(ctx, tconn, ash.Closed); err != nil {
			s.Fatal("Launcher not closed after transition to clamshell mode: ", err)
		}
	}

	if isInTablet {
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

	// The code below verifies the overflow shelf placed at the display bottom by:
	// 1. Entering overflow mode then checking the arrow buttons
	// 2. Pinning the Settings app then checking the arrow buttons
	// 3. Scrolling the overflow to the end then checking the arrow buttons

	if err := ash.EnterShelfOverflow(ctx, tconn, isUnderRTL); err != nil {
		s.Fatal("Failed to enter overflow mode: ", err)
	}

	if err := ash.WaitUntilShelfIconAnimationFinishAction(tconn)(ctx); err != nil {
		s.Fatal("Failed to wait until the shelf icon animation finishes: ", err)
	}

	shelfInfo, err := ash.FetchScrollableShelfInfoForState(ctx, tconn, &ash.ShelfState{})
	if err != nil {
		s.Fatal("Failed to get the scrollable shelf info after entering overflow: ", err)
	}

	if !shelfInfo.LeftArrowBounds.Size().Empty() {
		s.Fatal("Failed to verify the visibility of the left arrow button: got true; expected false")
	}

	if shelfInfo.RightArrowBounds.Size().Empty() {
		s.Fatal("Failed to verify the visibility of the right arrow button: got false; expected true")
	}

	dispBounds := dispInfo.Bounds
	dispHalfWidth := dispBounds.Width / 2

	if !isUnderRTL && dispBounds.Right()-shelfInfo.RightArrowBounds.Right() > dispHalfWidth {
		s.Fatalf("Failed to verify that the right arrow button is closer to the display right side than to the display "+
			"left side: the actual right arrow bounds %v; the actual display bounds %v", shelfInfo.RightArrowBounds, dispBounds)
	}

	if isUnderRTL && shelfInfo.RightArrowBounds.Left-dispBounds.Left > dispHalfWidth {
		s.Fatalf("Failed to verify that the right arrow button is closer to the display right side than to the display "+
			"left side: the actual right arrow bounds %v; the actual display bounds %v", shelfInfo.RightArrowBounds, dispBounds)
	}

	appsGrid := nodewith.ClassName(launcher.BubbleAppsGridViewClass)
	if isInTablet {
		appsGrid = nodewith.ClassName(launcher.PagedAppsGridViewClass)
	}

	if err := launcher.PinAppToShelf(tconn, apps.Settings, appsGrid)(ctx); err != nil {
		s.Fatal("Failed to pin Settings to shelf through the app list icon context menu: ", err)
	}

	if err := ash.WaitForHotseatAnimationToFinish(ctx, tconn); err != nil {
		s.Fatal("Failed to wait until the shelf icon animation finishes after showing the settings app: ", err)
	}

	shelfInfo, err = ash.FetchScrollableShelfInfoForState(ctx, tconn, &ash.ShelfState{})
	if err != nil {
		s.Fatal("Failed to get the scrollable shelf info after pinning the Settings app to the bottom shelf: ", err)
	}

	if shelfInfo.LeftArrowBounds.Size().Empty() {
		s.Fatal("Failed to verify the visibility of the left arrow button after pinning the Settings app: got false; expected true")
	}

	if !isUnderRTL && shelfInfo.LeftArrowBounds.Left-dispBounds.Left > dispHalfWidth {
		s.Fatalf("Failed to verify that the left arrow button is closer to the display left side than to the display "+
			"right side: the actual right arrow bounds %v; the actual display bounds %v", shelfInfo.LeftArrowBounds, dispBounds)
	}

	if isUnderRTL && dispBounds.Right()-shelfInfo.LeftArrowBounds.Right() > dispHalfWidth {
		s.Fatalf("Failed to verify that the left arrow button is closer to the display right side than to the display "+
			"left side under RTL: the actual right arrow bounds %v; the actual display bounds %v", shelfInfo.LeftArrowBounds, dispBounds)
	}

	if !shelfInfo.RightArrowBounds.Size().Empty() {
		s.Fatal("Failed to verify the visibility of the right arrow button after pinning the Settings app: got true; expected false")
	}

	if err := ash.ScrollOverflowShelfToEnd(ctx, tconn, true); err != nil {
		s.Fatal("Failed to scroll the overflow shelf to the end by clicking at the left button: ", err)
	}

	shelfInfo, err = ash.FetchScrollableShelfInfoForState(ctx, tconn, &ash.ShelfState{})
	if err != nil {
		s.Fatal("Failed to get the scrollable shelf info after scrolling to the end through the left arrow button: ", err)
	}

	if !shelfInfo.LeftArrowBounds.Size().Empty() {
		s.Fatal("Failed to verify the visibility of the left arrow button after scrolling to the end: got true; expected false")
	}

	if shelfInfo.RightArrowBounds.Size().Empty() {
		s.Fatal("Failed to verify the visibility of the right arrow button after scrolling to the end: got false; expected true")
	}

	if !isUnderRTL && dispBounds.Right()-shelfInfo.RightArrowBounds.Right() > dispHalfWidth {
		s.Fatalf("Failed to verify that the right arrow button is closer to the display right side than to the display "+
			"left side after scrolling to the end through the left button: the actual right arrow bounds %v; the actual display bounds %v",
			shelfInfo.RightArrowBounds, dispBounds)
	}

	if isUnderRTL && shelfInfo.RightArrowBounds.Left-dispBounds.Left > dispHalfWidth {
		s.Fatalf("Failed to verify under RTL that the right arrow button is closer to the display right side than to the display "+
			"left side after scrolling to the end through the left button: the actual right arrow bounds %v; the actual display bounds %v",
			shelfInfo.RightArrowBounds, dispBounds)
	}

	if err := ash.UnpinApps(ctx, tconn, []string{apps.Settings.ID}); err != nil {
		s.Fatal("Failed to unpin the Settings app: ", err)
	}

	// In the tablet mode, shelf can only be placed at the display bottom.
	if isInTablet {
		return
	}

	// The code below verifies the overflow shelf placed at the display left by:
	// 1. Checking the arrow buttons right after the alignment becomes ShelfAlignmentLeft
	// 2. Pinning the Settings app then checking the arrow buttons
	// 3. Scrolling the overflow shelf to the end then checking the arrow button

	if err := ash.SetShelfAlignment(ctx, tconn, dispInfo.ID, ash.ShelfAlignmentLeft); err != nil {
		s.Fatal("Failed to place the shelf at the left: ", err)
	}

	shelfInfo, err = ash.FetchScrollableShelfInfoForState(ctx, tconn, &ash.ShelfState{})
	if err != nil {
		s.Fatal("Failed to get the scrollable shelf info after scrolling to the end through the left arrow button: ", err)
	}

	dispHalfHeight := dispBounds.Height / 2

	if !shelfInfo.LeftArrowBounds.Size().Empty() {
		s.Fatal("Failed to verify the visibility of the left arrow button for the left shelf: got true; expected false")
	}

	if shelfInfo.RightArrowBounds.Size().Empty() {
		s.Fatal("Failed to verify the visibility of the right arrow button for the right shelf: got false; expected true")
	}

	if dispBounds.Bottom()-shelfInfo.RightArrowBounds.Bottom() > dispHalfHeight {
		s.Fatalf("Failed to verify that the right arrow button is closer to the display bottom than to the display "+
			"top side when the shelf alignment is left: the actual right arrow bounds %v; the actual display bounds %v",
			shelfInfo.RightArrowBounds, dispBounds)
	}

	// Ensure that the launcher shows.
	if isInTablet {
		if err := launcher.OpenExpandedView(tconn)(ctx); err != nil {
			s.Fatal("Failed to open expanded Application list view with the left aligned shelf: ", err)
		}
	} else {
		if err := launcher.OpenBubbleLauncher(tconn)(ctx); err != nil {
			s.Fatal("Failed to open bubble launcher with the left aligned shelf: ", err)
		}
	}

	if err := launcher.WaitForStableNumberOfApps(ctx, tconn); err != nil {
		s.Fatal("Failed to wait for item count in app list to stabilize: ", err)
	}

	if err := launcher.PinAppToShelf(tconn, apps.Settings, appsGrid)(ctx); err != nil {
		s.Fatal("Failed to pin Settings to the left aligned shelf through the app list icon context menu: ", err)
	}

	if err := ash.WaitForHotseatAnimationToFinish(ctx, tconn); err != nil {
		s.Fatal("Failed to wait until the shelf icon animation finishes after pinning the settings app to the left aligned shelf: ", err)
	}

	shelfInfo, err = ash.FetchScrollableShelfInfoForState(ctx, tconn, &ash.ShelfState{})
	if err != nil {
		s.Fatal("Failed to get the scrollable shelf info after pinning the Settings app to the left aligned shelf: ", err)
	}

	if shelfInfo.LeftArrowBounds.Size().Empty() {
		s.Fatal("Failed to verify the visibility of the left arrow button after pinning the Settings app to the left aligned shelf: got false; expected true")
	}

	if !shelfInfo.RightArrowBounds.Size().Empty() {
		s.Fatal("Failed to verify the visibility of the right arrow button after pinning the Settings app to the left aligned shelf: got false; expected true")
	}

	if shelfInfo.LeftArrowBounds.Top-dispBounds.Top > dispHalfHeight {
		s.Fatalf("Failed to verify that the left arrow button is closer to the display top than to the display "+
			"bottom side when the shelf alignment is left: the actual right arrow bounds %v; the actual display bounds %v",
			shelfInfo.LeftArrowBounds, dispBounds)
	}

	if err := ash.ScrollOverflowShelfToEnd(ctx, tconn, true); err != nil {
		s.Fatal("Failed to scroll the overflow shelf to the end by clicking at the left button on the left aligned shelf: ", err)
	}

	shelfInfo, err = ash.FetchScrollableShelfInfoForState(ctx, tconn, &ash.ShelfState{})
	if err != nil {
		s.Fatal("Failed to get the scrollable shelf info after scrolling the left aligned shelf to the end: ", err)
	}

	if !shelfInfo.LeftArrowBounds.Size().Empty() {
		s.Fatal("Failed to verify the visibility of the left arrow button for the left shelf after scrolling to the end: got true; expected false")
	}

	if shelfInfo.RightArrowBounds.Size().Empty() {
		s.Fatal("Failed to verify the visibility of the right arrow button for the right shelf after scrolling to the end: got false; expected true")
	}

	if dispBounds.Bottom()-shelfInfo.RightArrowBounds.Bottom() > dispHalfHeight {
		s.Fatalf("Failed to verify that the right arrow button is closer to the display bottom than to the display "+
			"top side when the shelf alignment is left after scrolling to the end: the actual right arrow bounds %v; the actual display bounds %v",
			shelfInfo.RightArrowBounds, dispBounds)
	}

	if err := ash.UnpinApps(ctx, tconn, []string{apps.Settings.ID}); err != nil {
		s.Fatal("Failed to unpin the Settings app: ", err)
	}

	// The code below verifies the overflow shelf placed at the display right by:
	// 1. Checking the arrow buttons right after the alignment becomes ShelfAlignmentRight
	// 2. Pinning the Settings app then checking the arrow buttons
	// 3. Scrolling the overflow shelf to the end then checking the arrow button

	if err := ash.SetShelfAlignment(ctx, tconn, dispInfo.ID, ash.ShelfAlignmentRight); err != nil {
		s.Fatal("Failed to place the shelf at the right: ", err)
	}

	shelfInfo, err = ash.FetchScrollableShelfInfoForState(ctx, tconn, &ash.ShelfState{})
	if err != nil {
		s.Fatal("Failed to get the scrollable shelf info after setting the alignment to be ShelfAlignmentRight: ", err)
	}

	if !shelfInfo.LeftArrowBounds.Size().Empty() {
		s.Fatal("Failed to verify the visibility of the left arrow button for the right shelf: got true; expected false")
	}

	if shelfInfo.RightArrowBounds.Size().Empty() {
		s.Fatal("Failed to verify the visibility of the right arrow button for the right shelf: got false; expected true")
	}

	if dispBounds.Bottom()-shelfInfo.RightArrowBounds.Bottom() > dispHalfHeight {
		s.Fatalf("Failed to verify that the right arrow button is closer to the display bottom than to the display "+
			"top side when the alignment is ShelfAlignmentRight: the actual right arrow bounds %v; the actual display bounds %v",
			shelfInfo.RightArrowBounds, dispBounds)
	}

	// Ensure that the launcher shows.
	if isInTablet {
		if err := launcher.OpenExpandedView(tconn)(ctx); err != nil {
			s.Fatal("Failed to open expanded Application list view with the left aligned shelf: ", err)
		}
	} else {
		if err := launcher.OpenBubbleLauncher(tconn)(ctx); err != nil {
			s.Fatal("Failed to open bubble launcher with the left aligned shelf: ", err)
		}
	}

	if err := launcher.WaitForStableNumberOfApps(ctx, tconn); err != nil {
		s.Fatal("Failed to wait for item count in app list to stabilize: ", err)
	}

	if err := launcher.PinAppToShelf(tconn, apps.Settings, appsGrid)(ctx); err != nil {
		s.Fatal("Failed to pin Settings to the right aligned shelf through the app list icon context menu: ", err)
	}

	if err := ash.WaitForHotseatAnimationToFinish(ctx, tconn); err != nil {
		s.Fatal("Failed to wait until the shelf icon animation finishes after pinning the settings app to the right aligned shelf: ", err)
	}

	shelfInfo, err = ash.FetchScrollableShelfInfoForState(ctx, tconn, &ash.ShelfState{})
	if err != nil {
		s.Fatal("Failed to get the scrollable shelf info after pinning the Settings app to the right aligned shelf: ", err)
	}

	if shelfInfo.LeftArrowBounds.Size().Empty() {
		s.Fatal("Failed to verify the visibility of the left arrow button after pinning the Settings app to the right aligned shelf: got false; expected true")
	}

	if !shelfInfo.RightArrowBounds.Size().Empty() {
		s.Fatal("Failed to verify the visibility of the right arrow button after pinning the Settings app to the right aligned shelf: got false; expected true")
	}

	if shelfInfo.LeftArrowBounds.Top-dispBounds.Top > dispHalfHeight {
		s.Fatalf("Failed to verify that the left arrow button is closer to the display top than to the display "+
			"bottom side when the shelf alignment is right: the actual right arrow bounds %v; the actual display bounds %v",
			shelfInfo.LeftArrowBounds, dispBounds)
	}

	if err := ash.ScrollOverflowShelfToEnd(ctx, tconn, true); err != nil {
		s.Fatal("Failed to scroll the overflow shelf to the end by clicking at the left button on the right aligned shelf: ", err)
	}

	shelfInfo, err = ash.FetchScrollableShelfInfoForState(ctx, tconn, &ash.ShelfState{})
	if err != nil {
		s.Fatal("Failed to get the scrollable shelf info after scrolling the right aligned shelf to the end: ", err)
	}

	if !shelfInfo.LeftArrowBounds.Size().Empty() {
		s.Fatal("Failed to verify the visibility of the left arrow button for the right aligned shelf after scrolling to the end: got true; expected false")
	}

	if shelfInfo.RightArrowBounds.Size().Empty() {
		s.Fatal("Failed to verify the visibility of the right arrow button for the right aligned shelf after scrolling to the end: got false; expected true")
	}

	if dispBounds.Bottom()-shelfInfo.RightArrowBounds.Bottom() > dispHalfHeight {
		s.Fatalf("Failed to verify that the right arrow button is closer to the display bottom than to the display "+
			"top side when the shelf alignment is left after scrolling to the end: the actual right arrow bounds %v; the actual display bounds %v",
			shelfInfo.RightArrowBounds, dispBounds)
	}

	if err := ash.UnpinApps(ctx, tconn, []string{apps.Settings.ID}); err != nil {
		s.Fatal("Failed to unpin the Settings app: ", err)
	}
}

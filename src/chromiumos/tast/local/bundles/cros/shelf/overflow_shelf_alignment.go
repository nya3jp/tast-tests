// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package shelf

import (
	"context"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/browser"
	"chromiumos/tast/local/chrome/browser/browserfixt"
	"chromiumos/tast/local/chrome/display"
	"chromiumos/tast/local/chrome/lacros/lacrosfixt"
	"chromiumos/tast/local/chrome/uiauto/launcher"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/testing"
)

type overflowShelfSmokeTestType struct {
	isRTL      bool // If true, the system UI is adapted to right-to-left languages.
	tabletMode bool
	bt         browser.Type
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         OverflowShelfAlignment,
		LacrosStatus: testing.LacrosVariantExists,
		Desc:         "Verifies the overflow shelf by changing the shelf alignment",
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
				isRTL:      false,
				tabletMode: false,
				bt:         browser.TypeAsh,
			},
			Fixture: "install100Apps",
		}, {
			Name: "clamshell_mode_rtl",
			Val: overflowShelfSmokeTestType{
				isRTL:      true,
				tabletMode: false,
				bt:         browser.TypeAsh,
			},
			Fixture: "install100Apps",
		}, {
			Name: "tablet_mode_ltr",
			Val: overflowShelfSmokeTestType{
				isRTL:      false,
				tabletMode: true,
				bt:         browser.TypeAsh,
			},
			Fixture: "install100Apps",
		}, {
			Name: "tablet_mode_rtl",
			Val: overflowShelfSmokeTestType{
				isRTL:      true,
				tabletMode: true,
				bt:         browser.TypeAsh,
			},
			Fixture: "install100Apps",
		}, {
			Name: "clamshell_mode_ltr_lacros",
			Val: overflowShelfSmokeTestType{
				isRTL:      false,
				tabletMode: false,
				bt:         browser.TypeLacros,
			},
			Fixture:           "install100LacrosApps",
			ExtraSoftwareDeps: []string{"lacros"},
		}, {
			Name: "clamshell_mode_rtl_lacros",
			Val: overflowShelfSmokeTestType{
				isRTL:      true,
				tabletMode: false,
				bt:         browser.TypeLacros,
			},
			Fixture:           "install100LacrosApps",
			ExtraSoftwareDeps: []string{"lacros"},
		}, {
			Name: "tablet_mode_ltr_lacros",
			Val: overflowShelfSmokeTestType{
				isRTL:      false,
				tabletMode: true,
				bt:         browser.TypeLacros,
			},
			Fixture:           "install100LacrosApps",
			ExtraSoftwareDeps: []string{"lacros"},
		}, {
			Name: "tablet_mode_rtl_lacros",
			Val: overflowShelfSmokeTestType{
				isRTL:      true,
				tabletMode: true,
				bt:         browser.TypeLacros,
			},
			Fixture:           "install100LacrosApps",
			ExtraSoftwareDeps: []string{"lacros"},
		}},
	})
}

// OverflowShelfAlignment verifies the basic features of the overflow shelf, i.e. the
// shelf that hides excess icons and scroll buttons show to make them accessible,
// under different shelf alignments.
func OverflowShelfAlignment(ctx context.Context, s *testing.State) {
	// Reserve a few seconds for various cleanup.
	cleanUpCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	testType := s.Param().(overflowShelfSmokeTestType)
	isRTL := testType.isRTL
	bt := testType.bt

	// Set up browser.
	opts := s.FixtValue().([]chrome.Option)
	if isRTL {
		opts = append(opts, chrome.ExtraArgs("--force-ui-direction=rtl"))
	}
	cr, _, closeBrowser, err := browserfixt.SetUpWithNewChrome(ctx, bt, lacrosfixt.NewConfig(), opts...)
	if err != nil {
		s.Fatalf("Failed to start chrome (rtl? %v): %v", isRTL, err)
	}
	defer cr.Close(cleanUpCtx)
	defer closeBrowser(cleanUpCtx)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect Test API: ", err)
	}

	isInTablet := testType.tabletMode
	originallyEnabled, err := ash.TabletModeEnabled(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to check if DUT is in tablet mode: ", err)
	}

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

	if !isInTablet {
		// Ensure that the shelf is placed at the bottom when the test runs in the clamshell mode.
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

	// Ensure that the tablet launcher is closed before opening a launcher instance for test in clamshell.
	if originallyEnabled && !isInTablet {
		if err := ash.WaitForLauncherState(ctx, tconn, ash.Closed); err != nil {
			s.Fatal("Launcher not closed after transition to clamshell mode: ", err)
		}
	}

	// The code below verifies the overflow shelf placed at the display bottom by:
	// 1. Entering overflow mode then checking the arrow buttons
	// 2. Pinning the Settings app then checking the arrow buttons
	// 3. Scrolling the overflow to the end then checking the arrow buttons

	if err := ash.EnterShelfOverflowWithFakeApps(ctx, tconn, isRTL); err != nil {
		s.Fatal("Failed to enter overflow mode: ", err)
	}

	if err := ash.WaitUntilShelfIconAnimationFinishAction(tconn)(ctx); err != nil {
		s.Fatal("Failed to wait until the shelf icon animation finishes after entering overflow: ", err)
	}

	dispBounds := dispInfo.Bounds
	if err := ash.VerifyOverflowShelfScrollArrow(ctx, tconn, ash.RightOnlyVisible, isRTL, ash.ShelfAlignmentBottom, dispBounds); err != nil {
		s.Fatal("Failed to verify overflow shelf scroll arrows with the bottom shelf before pinning Settings app: ", err)
	}

	if err := pinSettingsAppThroughAppList(ctx, tconn, isInTablet); err != nil {
		s.Fatal("Failed to pin the Settings app to the bottom shelf from the app list item context menu: ", err)
	}

	if err := ash.VerifyOverflowShelfScrollArrow(ctx, tconn, ash.LeftOnlyVisible, isRTL, ash.ShelfAlignmentBottom, dispBounds); err != nil {
		s.Fatal("Failed to verify overflow shelf scroll arrows with the bottom shelf after pinning Settings app: ", err)
	}

	if err := ash.ScrollOverflowShelfToEnd(ctx, tconn, true /* leftArrowButton */); err != nil {
		s.Fatal("Failed to scroll the bottom shelf to the end by clicking at the left button: ", err)
	}

	if err := ash.VerifyOverflowShelfScrollArrow(ctx, tconn, ash.RightOnlyVisible, isRTL, ash.ShelfAlignmentBottom, dispBounds); err != nil {
		s.Fatal("Failed to verify overflow shelf scroll arrows with the bottom shelf after scrolling to the end: ", err)
	}

	if err := ash.UnpinApps(ctx, tconn, []string{apps.Settings.ID}); err != nil {
		s.Fatal("Failed to unpin the Settings app from the bottom shelf: ", err)
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
		s.Fatal("Failed to place the shelf at the left side: ", err)
	}

	if err := ash.VerifyOverflowShelfScrollArrow(ctx, tconn, ash.RightOnlyVisible, isRTL, ash.ShelfAlignmentLeft, dispBounds); err != nil {
		s.Fatal("Failed to verify overflow shelf scroll arrows with the left-aligned shelf: ", err)
	}

	if err := pinSettingsAppThroughAppList(ctx, tconn, isInTablet); err != nil {
		s.Fatal("Failed to pin the Settings app to the left aligned shelf from the app list item context menu: ", err)
	}

	if err := ash.VerifyOverflowShelfScrollArrow(ctx, tconn, ash.LeftOnlyVisible, isRTL, ash.ShelfAlignmentLeft, dispBounds); err != nil {
		s.Fatal("Failed to verify overflow shelf scroll arrows with the left-aligned shelf after pinning the Settings app: ", err)
	}

	if err := ash.ScrollOverflowShelfToEnd(ctx, tconn, true /* leftArrowButton */); err != nil {
		s.Fatal("Failed to scroll the overflow shelf to the end by clicking at the left button on the left aligned shelf: ", err)
	}

	if err := ash.VerifyOverflowShelfScrollArrow(ctx, tconn, ash.RightOnlyVisible, isRTL, ash.ShelfAlignmentLeft, dispBounds); err != nil {
		s.Fatal("Failed to verify overflow shelf scroll arrows with the left-aligned shelf after scrolling to the end: ", err)
	}

	if err := ash.UnpinApps(ctx, tconn, []string{apps.Settings.ID}); err != nil {
		s.Fatal("Failed to unpin the Settings app from the left-aligned shelf: ", err)
	}

	// The code below verifies the overflow shelf placed at the display right by:
	// 1. Checking the arrow buttons right after the alignment becomes ShelfAlignmentRight
	// 2. Pinning the Settings app then checking the arrow buttons
	// 3. Scrolling the overflow shelf to the end then checking the arrow button

	if err := ash.SetShelfAlignment(ctx, tconn, dispInfo.ID, ash.ShelfAlignmentRight); err != nil {
		s.Fatal("Failed to place the shelf at the right: ", err)
	}

	if err := ash.VerifyOverflowShelfScrollArrow(ctx, tconn, ash.RightOnlyVisible, isRTL, ash.ShelfAlignmentRight, dispBounds); err != nil {
		s.Fatal("Failed to verify overflow shelf scroll arrows with the right-aligned shelf: ", err)
	}

	if err := pinSettingsAppThroughAppList(ctx, tconn, isInTablet); err != nil {
		s.Fatal("Failed to pin the Settings app to the right aligned shelf from the app list item context menu: ", err)
	}

	if err := ash.VerifyOverflowShelfScrollArrow(ctx, tconn, ash.LeftOnlyVisible, isRTL, ash.ShelfAlignmentRight, dispBounds); err != nil {
		s.Fatal("Failed to verify overflow shelf scroll arrows with the right-aligned shelf after pinning the Settings app: ", err)
	}

	if err := ash.ScrollOverflowShelfToEnd(ctx, tconn, true /* leftArrowButton */); err != nil {
		s.Fatal("Failed to scroll the overflow shelf to the end by clicking at the left button on the right-aligned shelf: ", err)
	}

	if err := ash.VerifyOverflowShelfScrollArrow(ctx, tconn, ash.RightOnlyVisible, isRTL, ash.ShelfAlignmentRight, dispBounds); err != nil {
		s.Fatal("Failed to verify overflow shelf scroll arrows with the right-aligned shelf after scrolling to the end: ", err)
	}

	if err := ash.UnpinApps(ctx, tconn, []string{apps.Settings.ID}); err != nil {
		s.Fatal("Failed to unpin the Settings app from the right-aligned shelf: ", err)
	}
}

// pinSettingsAppThroughAppList pins the Settings app to the shelf through the app
// list item's context menu.
func pinSettingsAppThroughAppList(ctx context.Context, tconn *chrome.TestConn, isInTablet bool) error {
	var appsGrid *nodewith.Finder

	if isInTablet {
		appsGrid = nodewith.ClassName(launcher.PagedAppsGridViewClass)
		if err := launcher.OpenExpandedView(tconn)(ctx); err != nil {
			return errors.Wrap(err, "failed to open expanded Application list view")
		}
	} else {
		appsGrid = nodewith.ClassName(launcher.BubbleAppsGridViewClass)
		if err := launcher.OpenBubbleLauncher(tconn)(ctx); err != nil {
			return errors.Wrap(err, "failed to open bubble launcher")
		}
	}

	if err := launcher.WaitForStableNumberOfApps(ctx, tconn); err != nil {
		return errors.Wrap(err, "failed to wait for item count in app list to stabilize")
	}

	if err := launcher.PinAppToShelf(tconn, apps.Settings, appsGrid)(ctx); err != nil {
		return errors.Wrap(err, "failed to pin Settings to shelf through the app list icon context menu")
	}

	if err := ash.WaitForHotseatAnimationToFinish(ctx, tconn); err != nil {
		return errors.Wrap(err, "failed to wait until the shelf icon animation finishes after showing the settings app")
	}

	return nil
}

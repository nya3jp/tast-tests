// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package shelf

import (
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/testing"
	"context"
	"time"
)

type overflowShelfSmokeTestType struct {
	isUnderRTL bool // If true, the system UI is adapted to right-to-left languages.
	inTablet   bool
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         OverflowShelfSmoke,
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
					isUnderRTL: true,
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

// OverflowShelfSmoke verifies the basic features of the overflow shelf, i.e. the
// shelf that accommodates so many shelf icons that some icons are hidden and the shelf arrow button(s) shows.
func OverflowShelfSmoke(ctx context.Context, s *testing.State) {
	testType := s.Param().(overflowShelfSmokeTestType)
	isUnderRTL := testType.isUnderRTL

	var cr *chrome.Chrome
	if isUnderRTL {
		var err error
		opts := s.FixtValue().([]chrome.Option)
		cr, err = chrome.New(ctx, append(opts, chrome.ExtraArgs("--force-ui-direction=rtl"))...)
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

	if err := ash.EnterShelfOverflow(ctx, tconn); err != nil {
		s.Fatal("Failed to enter overflow mode: ", err)
	}

	if err := ash.WaitUntilShelfIconAnimationFinishAction(tconn)(ctx); err != nil {
		s.Fatal("Failed to wait until the shelf icon animation finishes: ", err)
	}

	if err := testing.Sleep(ctx, 2*time.Second); err != nil {
		s.Fatal("Failed to wait: ", err)
	}

	// Pin the Settings app to create a more complex scenario for testing.
	if err := ash.PinApps(ctx, tconn, []string{apps.Settings.ID}); err != nil {
		s.Fatal("Failed to pin Settings to shelf: ", err)
	}

	if err := testing.Sleep(ctx, 10*time.Second); err != nil {
		s.Fatal("Failed to wait: ", err)
	}
}

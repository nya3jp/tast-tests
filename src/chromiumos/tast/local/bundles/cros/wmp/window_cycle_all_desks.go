// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wmp

import (
	"context"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/bundles/cros/wmp/wmputils"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/filesapp"
	"chromiumos/tast/local/chrome/uiauto/ossettings"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         WindowCycleAllDesks,
		LacrosStatus: testing.LacrosVariantExists,
		Desc:         "Checks Alt+Tab and Alt+Shift+Tab functionality for cycling windows for all desks",
		Contacts: []string{
			"chromeos-sw-engprod@google.com",
			"chromeos-wmp@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome", "no_kernel_upstream"},
		Params: []testing.Param{{
			Fixture: "chromeLoggedIn",
		}, {
			Name:              "lacros",
			Fixture:           "lacros",
			ExtraAttr:         []string{"informational"},
			ExtraSoftwareDeps: []string{"lacros"},
		}},
	})
}

func WindowCycleAllDesks(ctx context.Context, s *testing.State) {
	// Reserve for various cleanup.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()

	cr := s.FixtValue().(chrome.HasChrome).Chrome()
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect to test API: ", err)
	}

	cleanup, err := ash.EnsureTabletModeEnabled(ctx, tconn, false)
	if err != nil {
		s.Fatal("Failed to ensure clamshell mode: ", err)
	}
	defer cleanup(cleanupCtx)

	defer faillog.DumpUITreeOnError(cleanupCtx, s.OutDir(), s.HasError, tconn)

	// Ensure there is no window open before test starts.
	if err := ash.CloseAllWindows(ctx, tconn); err != nil {
		s.Fatal("Failed to ensure no window is open: ", err)
	}

	// Launch the apps for the test. Different launch functions are used for Settings and Files,
	// since they include additional waiting for UI elements to load. This prevents the window
	// order from changing while cycling. If a window has not completely loaded, it may appear on
	// top once it finishes loading. If this happens while cycling windows, the test will likely fail.
	settings, err := ossettings.Launch(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to launch Settings: ", err)
	}
	if err := settings.WaitForSearchBox()(ctx); err != nil {
		s.Fatal("Failed waiting for Settings to load: ", err)
	}

	if _, err := filesapp.Launch(ctx, tconn); err != nil {
		s.Fatal("Failed to launch Files app: ", err)
	}

	ac := uiauto.New(tconn)
	// GetAllWindows returns windows in the order of most recent, matching the order they will appear in the cycle window.
	// We'll use this slice to find the expected window that should be brought to the top after each Alt+Tab cycle,
	// based on the number of times Tab was pressed. After successfully cycling windows, we'll update this to reflect
	// the expected order of the windows for the next Alt+Tab cycle.
	windows, err := ash.GetAllWindows(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to get all windows")
	}

	// 1. Tests that when there is only one desk with multiple browsers/apps, we can press alt-tab to cycle all windows.
	if err := wmputils.VerifyWindowsForCycleMenu(ctx, tconn, ac, windows); err != nil {
		s.Fatal(err, "Failed to cycle windows for the only one desk.")
	}

	// 2. Tests that when there are 8 desks and each desk has at least one browser/app, we can cycle all the apps/browsers
	// that are opened in all desks when the default status of the alt-tab window is "All desks".
	// Add 7 desks for a total of 8. Remove them at the end of the test.
	totalDesks := 8
	for i := 1; i < totalDesks; i++ {
		if err = ash.CreateNewDesk(ctx, tconn); err != nil {
			s.Error("Failed to create a new desk: ", err)
		}

		// Active the new created desk.
		if err = ash.ActivateDeskAtIndex(ctx, tconn, i); err != nil {
			s.Fatalf("Failed to activate desk with index %d: %v", i, err)
		}
		// Open one browser on the desk.
		browserApp, err := apps.PrimaryBrowser(ctx, tconn)
		if err != nil {
			s.Fatal("Could not find browser app info: ", err)
		}
		if err := apps.Launch(ctx, tconn, browserApp.ID); err != nil {
			s.Fatal("Failed to launch browser: ", err)
		}
		if err := ash.WaitForApp(ctx, tconn, browserApp.ID, time.Minute); err != nil {
			s.Fatal("Browser did not appear in shelf after launch: ", err)
		}
	}

	windows, err = ash.GetAllWindows(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to get all windows: ", err)
	}
	// Cycle windows for all 8 desks.
	if err := wmputils.VerifyWindowsForCycleMenu(ctx, tconn, ac, windows); err != nil {
		s.Fatal("Failed to cycle all desks windows: ", err)
	}
}

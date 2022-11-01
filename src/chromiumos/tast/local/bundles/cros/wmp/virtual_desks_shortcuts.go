// Copyright 2021 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wmp

import (
	"context"
	"fmt"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/browser"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         VirtualDesksShortcuts,
		LacrosStatus: testing.LacrosVariantExists,
		Desc:         "Checks that virtual desks shortcuts works correctly",
		Contacts: []string{
			"dandersson@chromium.org",
			"chromeos-wmp@google.com",
			"chromeos-sw-engprod@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		SearchFlags: []*testing.StringPair{{
			Key: "feature_id",
			// Switch desks.
			Value: "screenplay-353dbfd4-4666-4e1f-be6c-7a210f95069d",
		}},
		Params: []testing.Param{{
			Fixture: "chromeLoggedIn",
			Val:     browser.TypeAsh,
		}, {
			Name:              "lacros",
			Fixture:           "lacros",
			ExtraSoftwareDeps: []string{"lacros"},
			Val:               browser.TypeLacros,
		}},
	})
}

// deskFinder is used to wait until a given desk container window becomes visible.
func deskFinder(deskContainerName string) *nodewith.Finder {
	return nodewith.ClassName(deskContainerName).State("invisible", false)
}

// deskMiniViewFinder finds a desk mini view for a given desk.
func deskMiniViewFinder(deskName string) *nodewith.Finder {
	return nodewith.ClassName("DeskMiniView").Name(fmt.Sprintf("Desk: %s", deskName))
}

func findBrowserWindow(ctx context.Context, s *testing.State, tconn *chrome.TestConn, bt browser.Type) *ash.Window {
	window, err := ash.FindWindow(ctx, tconn, ash.BrowserTypeMatch(bt))
	if err != nil {
		s.Fatal("Failed to find browser window: ", err)
	}
	return window
}

func VirtualDesksShortcuts(ctx context.Context, s *testing.State) {
	// Reserve five seconds for various cleanup.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()

	bt := s.Param().(browser.Type)
	cr := s.FixtValue().(chrome.HasChrome).Chrome()
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}

	cleanup, err := ash.EnsureTabletModeEnabled(ctx, tconn, false)
	if err != nil {
		s.Fatal("Failed to ensure clamshell mode: ", err)
	}
	defer cleanup(cleanupCtx)

	defer ash.CleanUpDesks(cleanupCtx, tconn)
	defer faillog.DumpUITreeOnError(cleanupCtx, s.OutDir(), s.HasError, tconn)

	// Ensure there is no window open before test starts.
	// TODO(crbug.com/1311504): Ensure that chrome.ResetState can close lacros windows opened in the previous tests as well. Apply it to wmp test package.
	if err := ash.CloseAllWindows(ctx, tconn); err != nil {
		s.Fatal("Failed to ensure no window is open: ", err)
	}

	ac := uiauto.New(tconn)

	kb, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to create a keyboard: ", err)
	}

	// Internally, each virtual desk has a container for the windows on that desk. The container for
	// the currently active desk is visible while all other containers are invisible. When a desk is
	// activated, an animation plays out and the new desks's container window becomes visible. The
	// test uses this to know when a desk switch has completed.

	// First we go through shortcuts to create and switch desks.
	//   * Create a new desk. This places us on desk 2.
	//   * Switch back to desk 1.
	//   * Switch to desk 2.
	if err := uiauto.Combine(
		"create new virtual desk using keyboard shortcut",
		kb.AccelAction("Search+Shift+="),
		// This will wait until the container for desk 2 has become visible.
		ac.WaitForLocation(deskFinder("Desk_Container_B")),
	)(ctx); err != nil {
		s.Fatal("Failed to create desk 2: ", err)
	}

	if err := uiauto.Combine(
		"activate virtual desk on the left",
		kb.AccelAction("Search+["),
		ac.WaitForLocation(deskFinder("Desk_Container_A")),
	)(ctx); err != nil {
		s.Fatal("Failed to switch to desk 1: ", err)
	}

	if err := uiauto.Combine(
		"activate virtual desk on the right",
		kb.AccelAction("Search+]"),
		ac.WaitForLocation(deskFinder("Desk_Container_B")),
	)(ctx); err != nil {
		s.Fatal("Failed to switch to desk 2: ", err)
	}

	browserApp, err := apps.PrimaryBrowser(ctx, tconn)
	if err != nil {
		s.Fatal("Could not find browser app info: ", err)
	}

	// We then move the currently active window between desks. We're going to use a browser window for
	// these tests. We are on desk 2 when this sequence starts.
	//   * Open a browser and use a keyboard shortcut to move it to desk 1.
	//   * Verify that it no longer exists on desk 2.
	//   * Switch to desk 1 and verify that the browser is there.
	//   * Similar sequence to verify that the browser can be moved back to desk 2.
	if err := apps.Launch(ctx, tconn, browserApp.ID); err != nil {
		s.Fatal("Failed to open browser: ", err)
	}
	if err := ash.WaitForApp(ctx, tconn, browserApp.ID, time.Minute); err != nil {
		s.Fatal("Browser did not appear in shelf after launch: ", err)
	}
	if _, err := ash.WaitForAppWindow(ctx, tconn, browserApp.ID); err != nil {
		s.Fatal("Browser did not become visible: ", err)
	}
	if !findBrowserWindow(ctx, s, tconn, bt).OnActiveDesk {
		s.Fatal("Browser window is not on the current active desk (desk 2)")
	}

	if err := kb.Accel(ctx, "Search+Shift+["); err != nil {
		s.Fatal("Failed to move the current active window to the desk on the left: ", err)
	}
	// Wait for the window to finish animating and verify that it is no longer on the active desk.
	browserWindowID := findBrowserWindow(ctx, s, tconn, bt).ID
	ash.WaitWindowFinishAnimating(ctx, tconn, browserWindowID)
	if findBrowserWindow(ctx, s, tconn, bt).OnActiveDesk {
		s.Fatal("Browser window unexpectedly still on desk 2, expected it to be on desk 1")
	}

	// Change to the desk 1 and verify that the browser is on this desk.
	if err := ash.ActivateDeskAtIndex(ctx, tconn, 0); err != nil {
		s.Fatal("Failed to activate desk 1: ", err)
	}
	if !findBrowserWindow(ctx, s, tconn, bt).OnActiveDesk {
		s.Fatal("Browser window is not on desk 1")
	}

	// Move the browser back to desk 2 (on the right).
	if err := kb.Accel(ctx, "Search+Shift+]"); err != nil {
		s.Fatal("Failed to move app to desk: ", err)
	}
	// Wait for the window to finish animating and verify that it is no longer on the active desk.
	ash.WaitWindowFinishAnimating(ctx, tconn, browserWindowID)
	if findBrowserWindow(ctx, s, tconn, bt).OnActiveDesk {
		s.Fatal("Browser window unexpectedly still on desk 1, expected it to be on desk 2")
	}

	// Change to the desk 2 and verify that the browser is on this desk.
	if err := ash.ActivateDeskAtIndex(ctx, tconn, 1); err != nil {
		s.Fatal("Failed to activate desk 2: ", err)
	}
	if !findBrowserWindow(ctx, s, tconn, bt).OnActiveDesk {
		s.Fatal("Browser window is not on the currently active desk (desk 2)")
	}

	// We are now going to reorder desks in overview mode.
	if err := ash.SetOverviewModeAndWait(ctx, tconn, true); err != nil {
		s.Fatal("Failed to enter overview mode: ", err)
	}
	defer ash.SetOverviewModeAndWait(cleanupCtx, tconn, false)

	if err := uiauto.Combine(
		"reorder desk mini views",
		kb.AccelAction("Tab"),
		kb.AccelAction("Tab"),
		kb.AccelAction("Ctrl+Right"),
		ac.WaitForLocation(deskMiniViewFinder("Desk 1")),
		ac.WaitForLocation(deskMiniViewFinder("Desk 2")),
	)(ctx); err != nil {
		s.Fatal("Failed to reorder desks")
	}

	deskMiniViews, err := ash.FindDeskMiniViews(ctx, ac)
	if err != nil {
		s.Fatal("Failed to find desk mini views: ", err)
	}
	if deskMiniViews[0].Location.Left < deskMiniViews[1].Location.Left {
		s.Fatal("Failed to reorder desks")
	}

	if err := uiauto.Combine(
		"reorder desk mini views",
		kb.AccelAction("Ctrl+Left"),
		ac.WaitForLocation(deskMiniViewFinder("Desk 1")),
		ac.WaitForLocation(deskMiniViewFinder("Desk 2")),
	)(ctx); err != nil {
		s.Fatal("Failed to reorder desks")
	}

	deskMiniViews, err = ash.FindDeskMiniViews(ctx, ac)
	if err != nil {
		s.Fatal("Failed to find desk mini views: ", err)
	}
	if deskMiniViews[0].Location.Left > deskMiniViews[1].Location.Left {
		s.Fatal("Failed to reorder desks")
	}

	if err := ash.SetOverviewModeAndWait(ctx, tconn, false); err != nil {
		s.Fatal("Failed to exit overview mode: ", err)
	}

	// Finally, remove desk 2. We expect to be back at desk 1.
	if err := uiauto.Combine(
		"remove the current virtual desk",
		kb.AccelAction("Search+Shift+-"),
		ac.WaitForLocation(deskFinder("Desk_Container_A")),
	)(ctx); err != nil {
		s.Fatal("Failed to remove desk 2: ", err)
	}
}

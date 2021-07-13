// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package shelf

import (
	"context"
	"time"

	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/ui/mouse"
	"chromiumos/tast/local/chrome/ui/pointer"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/coords"
	"chromiumos/tast/local/ui"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: AutoHideSmoke,
		Desc: "Tests basic shelf behaviors",
		Contacts: []string{
			"yulunwu@chromium.org",
			"tbarzic@chromium.org",
			"chromeos-sw-engprod@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome", "tablet_mode"},
		Fixture:      "chromeLoggedIn",
		HardwareDeps: hwdep.D(hwdep.InternalDisplay()),
	})
}

// AutoHideSmoke tests basic shelf features.
func AutoHideSmoke(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(*chrome.Chrome)
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}

	//Begin test in clamshell mode.
	{
		cleanup, err := ash.EnsureTabletModeEnabled(ctx, tconn, false)
		if err != nil {
			s.Fatal("Failed to enter clamshell mode: ", err)
		}
		defer cleanup(ctx)

		if err := ash.WaitForHotseatAnimatingToIdealState(ctx, tconn, ash.ShelfShownClamShell); err != nil {
			s.Fatal("Failed to show clamshell shelf: ", err)
		}

		// Setup autohidden shelf.
		{
			ui := uiauto.New(tconn)
			SetAutoHiddenShelf := nodewith.Name("Autohide shelf").Role(role.MenuItem)
			if err := uiauto.Combine("set autohide shelf",
				// Right click on wallpaper.
				ui.RightClick(nodewith.ClassName("WallpaperView")),
				// Autohide shelf button takes some time before it becomes clickable.
				// Keep clicking it until the click is received and the menu closes.
				ui.WithInterval(500*time.Millisecond).LeftClickUntil(SetAutoHiddenShelf, ui.Gone(SetAutoHiddenShelf)),
			)(ctx); err != nil {
				s.Fatal("Failed to setup Autohide Shelf: ", err)
			}
		}

		dispMode, err := ash.InternalDisplayMode(ctx, tconn)
		if err != nil {
			s.Fatal("Failed to get display info: ", err)
		}

		//Open a single chrome browser window.
		const numWindows = 1
		if err := ash.CreateWindows(ctx, tconn, cr, ui.PerftestURL, numWindows); err != nil {
			s.Fatal("Failed to open browser window: ", err)
		}

		// Move the mouse out of the shelf area to the top left of the screen.
		if err := mouse.Move(ctx, tconn, coords.NewPoint(0, 0), time.Second); err != nil {
			s.Fatal("Failed to move the mouse to the top left of the screen: ", err)
		}

		// Move the mouse to the bottom center of the screen in the shelf area.
		if err := mouse.Move(ctx, tconn, coords.NewPoint(dispMode.WidthInNativePixels/2, dispMode.HeightInNativePixels), time.Second); err != nil {
			s.Fatal("Failed to move the mouse the bottom center of the screen: ", err)
		}

		if err := ash.WaitForHotseatAnimatingToIdealState(ctx, tconn, ash.ShelfShownClamShell); err != nil {
			s.Fatal("Autohidden shelf failed show animation: ", err)
		}
	}

	{
		// Enter tablet mode and verify that the shelf becomes hidden.
		cleanup, err := ash.EnsureTabletModeEnabled(ctx, tconn, true)
		if err != nil {
			s.Fatal("Failed to enter tablet mode: ", err)
		}
		defer cleanup(ctx)
		if err := ash.WaitForHotseatAnimatingToIdealState(ctx, tconn, ash.ShelfHidden); err != nil {
			s.Fatal("Shelf failed to autohide when entering tablet mode: ", err)
		}

		// Small swipe up from the bottom should cause the shelf to become visible.
		tc, err := pointer.NewTouchController(ctx, tconn)
		if err != nil {
			s.Fatal("Failed to create the touch controller: ", err)
		}
		defer tc.Close()
		if err := ash.SwipeUpHotseatAndWaitForCompletion(ctx, tconn, tc.EventWriter(), tc.TouchCoordConverter()); err != nil {
			s.Fatal("Failed to swipe up the hotseat to show extended shelf: ", err)
		}
	}
}

// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"

	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/ui/pointer"
	"chromiumos/tast/local/ui"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: HotseatSmoke,
		Desc: "Tests the basic features of hotseat",
		Contacts: []string{
			"andrewxu@chromium.org",
			"newcomer@chromium.org",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome", "tablet_mode"},
		Pre:          chrome.LoggedIn(),
	})
}

// HotseatSmoke tests the basic features of hotseat.
func HotseatSmoke(ctx context.Context, s *testing.State) {
	cr := s.PreValue().(*chrome.Chrome)
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect Test API: ", err)
	}

	// Verify that hotseat is shown after switching to tablet mode.
	{
		cleanup, err := ash.EnsureTabletModeEnabled(ctx, tconn, true)
		if err != nil {
			s.Fatal("Failed to enter tablet mode: ", err)
		}
		defer cleanup(ctx)

		if err := ash.WaitForHotseatAnimatingToIdealState(ctx, tconn, ash.ShelfShownHomeLauncher); err != nil {
			s.Fatal("Failed to show the shelf in tablet homelauncher: ", err)
		}
	}

	// Verify that hotseat is hidden after activating a window. Then it should be extended after gesture swipe.
	{
		const numWindows = 1
		conns, err := ash.CreateWindows(ctx, tconn, cr, ui.PerftestURL, numWindows)
		if err != nil {
			s.Fatal("Failed to open browser windows: ", err)
		}
		if err := conns.Close(); err != nil {
			s.Error("Failed to close the connection to a browser window")
		}

		tc, err := pointer.NewTouchController(ctx, tconn)
		if err != nil {
			s.Fatal("Failed to create the touch controller: ", err)
		}
		defer tc.Close()

		if err := ash.SwipeUpHotseatAndWaitForCompletion(ctx, tconn, tc.EventWriter(), tc.TouchCoordConverter()); err != nil {
			s.Fatal("Failed to swipe up the hotseat: ", err)
		}
	}

	// Verify that hotseat is shown after switching to clamshell mode.
	{
		if err := ash.SetTabletModeEnabled(ctx, tconn, false); err != nil {
			s.Fatal("Failed to enter clamshell mode: ", err)
		}

		if err := ash.WaitForHotseatAnimatingToIdealState(ctx, tconn, ash.ShelfShownClamShell); err != nil {
			s.Fatal("Failed to show the shelf after switching to clamshell: ", err)
		}
	}
}

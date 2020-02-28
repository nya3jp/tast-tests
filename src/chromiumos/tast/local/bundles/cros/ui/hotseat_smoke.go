// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"
	"time"

	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
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
		Attr:         []string{"group:crosbolt", "crosbolt_perbuild"},
		SoftwareDeps: []string{"chrome", "tablet_mode"},
		Pre:          chrome.LoggedIn(),
	})
}

// HotseatSmoke tests the basic features of hotseat, such as swiping up the in-app shelf,
// hiding the in-app shelf when activating the window and the transform from homepage to in-app shelf.
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

		info, err := ash.FetchHotseatInfo(ctx, tconn)
		if err != nil {
			s.Fatal("Failed to fetch the ui information of hotseat in tablet")
		}
		if info.HotseatState != ash.ShelfShownHomeLauncher {
			s.Fatal("Unexpected hotseat state: expected to be ShelfShownHomeLauncher; actual value is ", info.HotseatState)
		}
	}

	// Verify that hotseat is hidden after activating a window. Then it should be extended after gesture swipe.
	{
		const numWindows = 1
		conns, err := ash.CreateWindows(ctx, cr, ui.PerftestURL, numWindows)
		if err != nil {
			s.Fatal("Failed to open browser windows: ", err)
		}
		if err := conns.Close(); err != nil {
			s.Error("Failed to close the connection to a browser window")
		}

		if err := testing.Sleep(ctx, 2*time.Second); err != nil {
			s.Error("Failed to close the connection to a browser window")
		}

		if err := ash.SwipeUpHotseatAndWaitForCompletion(ctx, tconn); err != nil {
			s.Fatal("Failed to swipe up the hotseat: ", err)
		}
	}

	// Verify that hotseat is shown after switching to clamshell mode.
	{
		cleanup, err := ash.EnsureTabletModeEnabled(ctx, tconn, false)
		if err != nil {
			s.Fatal("Failed to enter tablet mode: ", err)
		}
		defer cleanup(ctx)

		info, err := ash.FetchHotseatInfo(ctx, tconn)
		if err != nil {
			s.Fatal("Failed to fetch the ui information of hotseat in clamshell")
		}
		if info.HotseatState != ash.ShelfShownClamShell {
			s.Fatal("Unexpected hotseat state: expected to be ShownClamShell; actual value is ", info.HotseatState)
		}
	}
}

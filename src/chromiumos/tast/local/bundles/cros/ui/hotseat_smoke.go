// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"

	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/ui"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: HotseatSmoke,
		Desc: "Tests the easily broken features of hotseat",
		Contacts: []string{
			"andrewxu@chromium.org",
			"newcomer@chromium.org",
		},
		Attr:         []string{"group:crosbolt", "crosbolt_perbuild"},
		SoftwareDeps: []string{"chrome", "tablet_mode"},
		Pre:          chrome.LoggedIn(),
	})
}

// HotseatSmoke tests the easily broken features of hotseat.
func HotseatSmoke(ctx context.Context, s *testing.State) {
	cr := s.PreValue().(*chrome.Chrome)
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect Test API: ", err)
	}

	cleanup, err := ash.EnsureTabletModeEnabled(ctx, tconn, true)
	if err != nil {
		s.Fatal("Failed to enter tablet mode: ", err)
	}
	defer cleanup(ctx)

	const numWindows = 1
	conns, err := ash.CreateWindows(ctx, cr, ui.PerftestURL, numWindows)
	if err != nil {
		s.Fatal("Failed to open browser windows: ", err)
	}
	if err := conns.Close(); err != nil {
		s.Error("Failed to close the connection to a browser window")
	}

	if err := ash.SwipeUpHotseatAndWaitForCompletion(ctx, tconn); err != nil {
		s.Fatal("Failed to swipe up the hotseat: ", err)
	}
}

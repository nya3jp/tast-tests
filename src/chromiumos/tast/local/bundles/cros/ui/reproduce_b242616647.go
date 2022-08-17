// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"

	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         ReproduceB242616647,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Reproduces https://buganizer.corp.google.com/issues/242616647 or verifies a fix",
		Contacts:     []string{"amusbach@chromium.org"},
		SoftwareDeps: []string{"chrome"},
		Fixture:      "chromeLoggedIn",
	})
}

func ReproduceB242616647(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(chrome.HasChrome).Chrome()

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect to test API: ", err)
	}

	conn, err := cr.NewConn(ctx, "about:blank")
	if err != nil {
		s.Fatal("Failed to open about:blank: ", err)
	}
	if err := conn.Close(); err != nil {
		s.Fatal("Failed to close connection to browser tab: ", err)
	}

	ws, err := ash.GetAllWindows(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to get windows: ", err)
	}
	if len(ws) != 1 {
		s.Fatalf("Unexpected number of windows; got %d, expected 1", len(ws))
	}
	wID := ws[0].ID

	state, err := ash.SetWindowState(ctx, tconn, wID, ash.WMEventMinimize, true)
	if err != nil {
		s.Fatal("Failed to minimize window: ", err)
	}
	if state != ash.WindowStateMinimized {
		s.Fatal("Tried to minimize the window, but now its state is: ", state)
	}

	w, err := ash.GetWindow(ctx, tconn, wID)
	if err != nil {
		s.Fatal("Failed to get window info: ", err)
	}

	if !w.IsAnimating {
		s.Fatal("Successfully reproduced https://buganizer.corp.google.com/issues/242616647")
	}
	s.Log("Failed to reproduce https://buganizer.corp.google.com/issues/242616647. Performing additional checks to more thoroughly test a fix")

	if err := ash.WaitWindowFinishAnimating(ctx, tconn, wID); err != nil {
		s.Fatal("Failed to wait for window minimize animation to finish: ", err)
	}
}

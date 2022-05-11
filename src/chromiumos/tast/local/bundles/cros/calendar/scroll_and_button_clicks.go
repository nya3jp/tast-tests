// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package calendar

import (
	"context"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/quicksettings"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         ScrollAndButtonClicks,
		LacrosStatus: testing.LacrosVariantExists,
		Desc:         "Checks the basic interacting with calendar view",
		Contacts: []string{
			"jiamingc@chromium.org",
			"cros-calendar@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Fixture:      "chromeLoggedIn",
	})
}

// ScrollAndButtonClicks verifies that we can open, scroll, click buttons on the Calendar view.
func ScrollAndButtonClicks(ctx context.Context, s *testing.State) {
	// cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()

	cr, err := chrome.New(ctx,
		chrome.EnableFeatures("CalendarView"))
	if err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}
	defer cr.Close(ctx)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}

	// Make sure the Quick Settings is expanded.
	if err := quicksettings.Expand(ctx, tconn); err != nil {
		s.Fatal("Failed to expand the Quick Settings: ", err)
	}

	ui := uiauto.New(tconn)

	// state := false
	const iterations = 20
	for i := 0; i < iterations; i++ {
		s.Logf("Opening Calendar view (iteration %d of %d)", i+1, iterations)

		if err := ui.LeftClick(quicksettings.DateView)(ctx); err != nil {
			s.Fatal("Failed to click the DateView in quick settings page: ", err)
		}
		// if err := bluetooth.PollForAdapterState(ctx, state); err != nil {
		// 	s.Fatal("Failed to toggle Bluetooth state: ", err)
		// }
		// state = !state
	}
}

// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package cellular

import (
	"context"

	"chromiumos/tast/local/cellular"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/quicksettings"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         ToggleCellularFromQuickSettings,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Checks that Cellular can be enabled and disabled from within the Quick Settings",
		Contacts: []string{
			"nikhilcn@chromium.org",
			"cros-connectivity@google.com",
		},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:cellular", "cellular_unstable", "cellular_sim_active"},
		Fixture:      "cellular",
	})
}

// ToggleCellularFromQuickSettings tests that a user can successfully toggle
// the Cellular state using the Quick Settings.
func ToggleCellularFromQuickSettings(ctx context.Context, s *testing.State) {
	cr, err := chrome.New(ctx)
	if err != nil {
		s.Fatal("Failed to create new chrome instance: ", err)
	}

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}

	helper, err := cellular.NewHelper(ctx)
	if err != nil {
		s.Fatal("Failed to create cellular.Helper: ", err)
	}

	ui := uiauto.New(tconn)

	if err := quicksettings.NavigateToNetworkDetailedView(ctx, tconn, true); err != nil {
		s.Fatal("Failed to navigate to the detailed Network view: ", err)
	}

	if _, err := helper.Enable(ctx); err != nil {
		s.Fatal("Failed to enable Cellular: ", err)
	}
	if err := uiauto.Combine("Wait until cellular is enabled in UI and not inhibited",
		ui.WaitUntilCheckedState(quicksettings.NetworkDetailedViewMobileDataToggle, true),
		ui.WaitUntilEnabled(quicksettings.NetworkDetailedViewMobileDataToggle),
	)(ctx); err != nil {
		s.Fatal("Failed: ", err)
	}

	state := false
	const iterations = 5

	for i := 0; i < iterations; i++ {
		s.Logf("Toggling Cellular (iteration %d of %d)", i+1, iterations)

		if err := ui.LeftClick(quicksettings.NetworkDetailedViewMobileDataToggle)(ctx); err != nil {
			s.Fatal("Failed to click on cellular toggle button")
		}

		if err := helper.WaitForEnabledState(ctx, state); err != nil {
			s.Fatal("Failed to toggle Cellular state: ", err)
		}

		if err := uiauto.Combine("Wait until cellular is in expected state in UI and not inhibited",
			ui.WaitUntilCheckedState(quicksettings.NetworkDetailedViewMobileDataToggle, state),
			ui.WaitUntilEnabled(quicksettings.NetworkDetailedViewMobileDataToggle),
		)(ctx); err != nil {
			s.Fatal("Failed: ", err)
		}
		state = !state
	}
}

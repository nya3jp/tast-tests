// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package cellular

import (
	"context"

	"chromiumos/tast/local/cellular"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/quicksettings"
	"chromiumos/tast/local/chrome/uiauto/role"
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
		Attr:         []string{"group:cellular"},
		Fixture:      "cellular",
	})
}

// ToggleCellularFromQuickSettings tests that a user can successfully toggle
// the Cellular state using the Quick Settings.
func ToggleCellularFromQuickSettings(ctx context.Context, s *testing.State) {
	cr, err := chrome.New(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}

	tconn, err := cr.TestAPIConn(ctx)

	helper, err := cellular.NewHelper(ctx)
	if err != nil {
		s.Fatal("Failed to create cellular.Helper: ", err)
	}

	ui := uiauto.New(tconn)

	if err := quicksettings.Expand(ctx, tconn); err != nil {
		s.Fatal("Fail to open quick settings")
	}

	networkFeaturePodLabelButton := nodewith.ClassName("FeaturePodLabelButton").NameContaining("network list")
	if err := ui.LeftClick(networkFeaturePodLabelButton)(ctx); err != nil {
		s.Fatal("Failed to open network quick settings")
	}

	toggleCellular := nodewith.Name("Mobile data").HasClass("TrayToggleButton").Role(role.Switch)

	if _, err := helper.Enable(ctx); err != nil {
		s.Fatal("Failed to enable Cellular: ", err)
	}

	state := false
	const iterations = 20

	for i := 0; i < iterations; i++ {
		s.Logf("Toggling Cellular (iteration %d of %d)", i+1, iterations)

		if err := ui.LeftClick(toggleCellular)(ctx); err != nil {
			s.Fatal("Failed to click on cellular toggle button")
		}

		if err := helper.WaitForEnabledState(ctx, state); err != nil {
			s.Fatal("Failed to toggle Cellular state: ", err)
		}
		state = !state
	}
}

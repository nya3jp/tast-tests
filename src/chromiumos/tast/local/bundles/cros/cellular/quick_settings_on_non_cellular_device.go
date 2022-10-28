// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package cellular

import (
	"context"

	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/quicksettings"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         QuickSettingsOnNonCellularDevice,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Checks that Cellular quick settings are not shown in non cellular devices",
		Contacts: []string{
			"nikhilcn@chromium.org",
			"cros-connectivity@google.com",
		},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:wificell"},
	})
}

// QuickSettingsOnNonCellularDevice tests that quick settings does not display
// mobile data settings on a non cellular device
func QuickSettingsOnNonCellularDevice(ctx context.Context, s *testing.State) {
	cr, err := chrome.New(ctx)
	if err != nil {
		s.Fatal("Failed to create new chrome instance: ", err)
	}

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}

	ui := uiauto.New(tconn)

	if err := quicksettings.NavigateToNetworkDetailedView(ctx, tconn, true); err != nil {
		s.Fatal("Failed to navigate to the detailed Network view: ", err)
	}

	if err := ui.Exists(quicksettings.NetworkDetailedViewMobileDataToggle)(ctx); err == nil {
		s.Fatal("Mobile data toggle is present on a non cellular device")
	}

	if err := ui.Exists(quicksettings.AddCellularButton)(ctx); err == nil {
		s.Fatal("Add cellular button is present on a non cellular device")
	}
}

// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package network

import (
	"context"

	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/quicksettings"
	"chromiumos/tast/local/shill"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         ToggleWifiFromNetworkQuickSettings,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Checks that WiFi can be enabled and disabled from within the Network Quick Settings",
		Contacts: []string{
			"tjohnsonkanu@chromium.org",
			"cros-connectivity@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Fixture:      "chromeLoggedInWithBluetoothAndNetworkRevampEnabled",
	})
}

// ToggleWifiFromNetworkQuickSettings tests that a user can successfully
// toggle the WiFi state using the toggle in the detailed Network view
// within the Quick Settings.
func ToggleWifiFromNetworkQuickSettings(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(*chrome.Chrome)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}

	// Enable Wifi in shill.
	wifiManager, err := shill.NewWifiManager(ctx, nil)
	if err != nil {
		s.Fatal("Failed to create shill Wi-Fi manager: ", err)
	}
	if err := wifiManager.Enable(ctx, false); err != nil {
		s.Fatal("Failed to enable Wi-Fi: ", err)
	}

	if err := quicksettings.NavigateToNetworkDetailedView(ctx, tconn, true); err != nil {
		s.Fatal("Failed to navigate to the detailed Network view: ", err)
	}

	ui := uiauto.New(tconn)

	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)

	state := true
	const iterations = 20
	for i := 0; i < iterations; i++ {
		s.Logf("Toggling WiFi (iteration %d of %d)", i+1, iterations)

		if err := ui.LeftClick(quicksettings.NetworkDetailedViewWifiToggleButton)(ctx); err != nil {
			s.Fatal("Failed to click the WiFi toggle: ", err)
		}

		if err := wifiManager.CheckWifiState(ctx, state); err != nil {
			s.Fatal("Failed to toggle WiFi state: ", err)
		}
		state = !state
	}
}

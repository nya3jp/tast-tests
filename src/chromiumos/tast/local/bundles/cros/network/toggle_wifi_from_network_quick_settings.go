// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package network

import (
	"context"
	"time"

	"chromiumos/tast/ctxutil"
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
		Fixture:      "chromeLoggedInWithNetworkRevampEnabled",
		Timeout:      3 * time.Minute,
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

	// Reserve ten seconds for cleanup.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()
	defer faillog.DumpUITreeOnError(cleanupCtx, s.OutDir(), s.HasError, tconn)

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

	state := true
	const iterations = 20
	for i := 0; i < iterations; i++ {
		s.Logf("Toggling WiFi (iteration %d of %d)", i+1, iterations)

		if err := ui.LeftClick(quicksettings.NetworkDetailedViewWifiToggleButtonRevamp)(ctx); err != nil {
			s.Fatal("Failed to click the WiFi toggle: ", err)
		}

		exp, err := wifiManager.IsWifiEnabled(ctx)
		if err != nil {
			s.Fatal("Failed to toggle WiFi state: ", err)
		}

		if state != exp {
			s.Errorf("WiFi has a different status than expected: got %t, want %t", state, exp)
		}

		state = !state
	}
}

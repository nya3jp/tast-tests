// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package cellular

import (
	"context"
	"time"

	"chromiumos/tast/local/cellular"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/ossettings"
	"chromiumos/tast/local/network/netconfig"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         ConnectToRoamingSim,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Checks the roaming label status on a roaming and non roaming SIM",
		Contacts: []string{
			"nikhilcn@chromium.org",
			"cros-connectivity@google.com",
		},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:cellular", "cellular_unstable", "cellular_sim_roaming"},
		Timeout:      3 * time.Minute,
	})
}

func ConnectToRoamingSim(ctx context.Context, s *testing.State) {
	cr, err := chrome.New(ctx)
	if err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}

	err = cellular.SetRoamingPolicy(ctx, true, false)
	if err != nil {
		s.Fatal("Failed to set roaming property: ", err)
	}

	networkName, err := cellular.GetCellularNetwork(ctx)
	if err != nil {
		s.Fatal("Failed to get a cellular network: ", err)
	}

	app, err := ossettings.OpenNetworkDetailPage(ctx, tconn, cr, networkName, netconfig.Cellular)
	if err != nil {
		s.Fatal("Failed to open network detail page: ", networkName)
	}
	defer app.Close(ctx)

	ui := uiauto.New(tconn).WithTimeout(60 * time.Second)

	if err := uiauto.Combine("connect to network and verify connected",
		ui.LeftClick(ossettings.ConnectButton),
		ui.WaitUntilExists(ossettings.ConnectedStatus),
	)(ctx); err != nil {
		s.Fatal("Failed to connect and verify connected: ", err)
	}

	err = cellular.SetRoamingPolicy(ctx, false, false)
	if err != nil {
		s.Fatal("Failed to set roaming property: ", err)
	}

	if err := ui.WaitUntilExists(ossettings.DisconnectedStatus)(ctx); err != nil {
		s.Fatal("Automatic disconnection failed: ", err)
	}

	if err := ui.LeftClick(ossettings.ConnectButton)(ctx); err != nil {
		s.Fatal("Failed to connect and verify connected: ", err)
	}

	const notificationTitle = "Network connection error"
	if _, err := ash.WaitForNotification(ctx, tconn, 30*time.Second, ash.WaitTitle(notificationTitle)); err != nil {
		s.Fatalf("Failed waiting for %v: %v", notificationTitle, err)
	}
}

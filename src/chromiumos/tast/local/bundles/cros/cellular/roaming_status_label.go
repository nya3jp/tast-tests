// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package cellular

import (
	"context"
	"time"

	"chromiumos/tast/local/cellular"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto/ossettings"
	"chromiumos/tast/local/network/netconfig"
	"chromiumos/tast/testing"
)

type testParameters struct {
	roamingSubLabel string
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         RoamingStatusLabel,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Checks the roaming label status on a roaming and non roaming SIM",
		Contacts: []string{
			"nikhilcn@chromium.org",
			"cros-connectivity@google.com",
		},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:cellular", "cellular_unstable"},
		Params: []testing.Param{
			{
				Name:      "on_roaming_sim",
				ExtraAttr: []string{"cellular_sim_roaming"},
				Val: testParameters{
					roamingSubLabel: "Currently roaming",
				},
			},
			{
				Name:      "on_non_roaming_sim",
				ExtraAttr: []string{"cellular_sim_prod_esim"},
				Val: testParameters{
					roamingSubLabel: "Not currently roaming",
				},
			},
		},
		Timeout: 3 * time.Minute,
	})
}

func RoamingStatusLabel(ctx context.Context, s *testing.State) {
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

	err = cellular.ConnectToCellularNetwork(ctx)
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

	roamingSubLabel, err := app.RoamingSubLabel(ctx, cr)
	if err != nil {
		s.Fatal("Failed to fetch sublabel: ", err)
	}

	if roamingSubLabel != s.Param().(testParameters).roamingSubLabel {
		s.Fatalf("Roaming sub-label is incorrect: got %q, want %q", roamingSubLabel, s.Param().(testParameters).roamingSubLabel)
	}
}

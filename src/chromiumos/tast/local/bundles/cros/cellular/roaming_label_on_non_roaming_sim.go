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
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         RoamingLabelOnNonRoamingSIM,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Checks the roaming label status on a non roaming SIM",
		Contacts: []string{
			"nikhilcn@chromium.org",
			"cros-connectivity@google.com",
		},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:cellular", "cellular_unstable", "cellular_sim_test_esim"},
		Timeout:      3 * time.Minute,
	})
}

func RoamingLabelOnNonRoamingSIM(ctx context.Context, s *testing.State) {
	cr, err := chrome.New(ctx)
	if err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}

	networkName, err := cellular.ConfigureRoamingNetwork(ctx, tconn, cr)
	if err != nil {
		s.Fatal("Failed to configure a roaming network: ", err)
	}

	app, err := ossettings.OpenNetworkDetailPage(ctx, tconn, cr, networkName)
	if err != nil {
		s.Fatal("Failed to open network detail page: ", networkName)
	}

	roamingSubLabel, err := app.RoamingSubLabel(ctx, cr)
	if err != nil {
		s.Fatal("Failed to fetch sublabel: ", err)
	}

	if roamingSubLabel != "Not currently roaming" {
		s.Fatal("Roaming sub label is not 'Not currently roaming'")
	}
}

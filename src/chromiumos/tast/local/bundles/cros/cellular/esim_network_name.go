// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package cellular

import (
	"context"
	"fmt"
	"strings"

	"chromiumos/tast/local/cellular"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto/ossettings"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         ESimNetworkName,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Tests the carrier name on connected esim",
		Contacts: []string{
			"nikhilcn@google.com",
			"cros-connectivity@google.com",
		},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:cellular", "cellular_unstable", "cellular_sim_prod_esim"},
		Fixture:      "cellular",
	})
}

func ESimNetworkName(ctx context.Context, s *testing.State) {
	cr, err := chrome.New(ctx)
	helper, err := cellular.NewHelper(ctx)
	if err != nil {
		s.Fatal("Failed to create cellular.Helper: ", err)
	}

	if err := helper.WaitForEnabledState(ctx, true); err != nil {
		s.Fatal("Failed to enable Cellular state: ", err)
	}

	if _, err := helper.Connect(ctx); err != nil {
		s.Fatal("Failed to connect to cellular service: ", err)
	}

	networkName, err := helper.GetCurrentNetworkName(ctx)
	if err != nil {
		s.Fatal("Error fetching the current network name: ", err)
	}

	tconn, err := cr.TestAPIConn(ctx)

	app, err := ossettings.OpenMobileDataSubpage(ctx, tconn, cr)
	if err != nil {
		s.Fatal("Failed to open mobile data subpage: ", err)
	}

	connectedESimNode := `div[id="itemTitle"][aria-hidden="true"]`
	expr := fmt.Sprintf(`
		var optionNode = shadowPiercingQuery(%q);
		if(optionNode == undefined){
			throw new Error("%s node not found.");
		}
		optionNode.innerText;
		`, connectedESimNode, "Title")

	var title string
	if err := app.EvalJSWithShadowPiercer(ctx, cr, expr, &title); err != nil {
		s.Fatal("Failed to fetch title: ", err)
	}

	if !strings.Contains(title, networkName) {
		s.Fatal("Network name is not present in the title")
	}
}

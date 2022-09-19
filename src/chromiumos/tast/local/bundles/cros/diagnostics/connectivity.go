// Copyright 2021 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package diagnostics

import (
	"context"
	"time"

	"chromiumos/tast/local/bundles/cros/diagnostics/utils"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/diagnosticsapp"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         Connectivity,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Can successfully navigate to the Connectivity page",
		Contacts: []string{
			"ashleydp@google.com",
			"zentaro@google.com",
			"menghuan@google.com",
			"cros-peripherals@google.com",
		},
		Attr:         []string{"group:mainline"},
		SoftwareDeps: []string{"chrome"},
		Fixture:      "diagnosticsPrep",
	})
}

// Connectivity verifies that the Connectivity page can be navigated to.
func Connectivity(ctx context.Context, s *testing.State) {
	tconn := s.FixtValue().(*utils.FixtureData).Tconn
	ui := uiauto.New(tconn).WithTimeout(20 * time.Second)

	// Open navigation if device is narrow view.
	if err := diagnosticsapp.ClickNavigationMenuButton(ctx, tconn); err != nil {
		s.Fatal("Could not click the menu button: ", err)
	}

	// Find the Connectivity navigation item.
	connectivityTab := diagnosticsapp.DxConnectivity.Ancestor(diagnosticsapp.DxRootNode)
	if err := ui.WaitUntilExists(connectivityTab)(ctx); err != nil {
		s.Fatal("Failed to find the Connectivity navigation item: ", err)
	}
	pollOpts := testing.PollOptions{Interval: time.Second, Timeout: 20 * time.Second}
	if err := ui.WithPollOpts(pollOpts).LeftClick(connectivityTab)(ctx); err != nil {
		s.Fatal("Could not click the Connectivity tab: ", err)
	}

	// Find the first routine action button.
	networkListContainer := diagnosticsapp.DxNetworkList.Ancestor(diagnosticsapp.DxRootNode)
	if err := ui.WaitUntilExists(networkListContainer)(ctx); err != nil {
		s.Fatal("Failed to find the network list: ", err)
	}
}

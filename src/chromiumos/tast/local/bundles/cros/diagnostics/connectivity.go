// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package diagnostics

import (
	"context"
	"time"

	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/diagnosticsapp"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         Connectivity,
		LacrosStatus: testing.LacrosVariantUnknown,
		Desc:         "Can successfully navigate to the Connectivity page",
		Contacts: []string{
			"ashleydp@google.com",
			"zentaro@google.com",
			"menghuan@google.com",
			"cros-peripherals@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
	})
}

// Connectivity verifies that the Connectivity page can be navigated to.
func Connectivity(ctx context.Context, s *testing.State) {
	cr, err := chrome.New(ctx, chrome.EnableFeatures("DiagnosticsAppNavigation", "EnableNetworkingInDiagnosticsApp"))
	if err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}
	defer cr.Close(ctx) // Close our own chrome instance

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect Test API: ", err)
	}
	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)

	dxRootnode, err := diagnosticsapp.Launch(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to launch diagnostics app: ", err)
	}

	// Find the Connectivity navigation item.
	ui := uiauto.New(tconn)
	connectivityTab := diagnosticsapp.DxConnectivity.Ancestor(dxRootnode)
	if err := ui.WithTimeout(20 * time.Second).WaitUntilExists(connectivityTab)(ctx); err != nil {
		s.Fatal("Failed to find the Connectivity navigation item: ", err)
	}
	pollOpts := testing.PollOptions{Interval: time.Second, Timeout: 20 * time.Second}
	if err := ui.WithPollOpts(pollOpts).LeftClick(connectivityTab)(ctx); err != nil {
		s.Fatal("Could not click the Connectivity tab: ", err)
	}

	// Find the first routine action button
	networkListContainer := diagnosticsapp.DxNetworkList.Ancestor(dxRootnode)
	if err := ui.WithTimeout(20 * time.Second).WaitUntilExists(networkListContainer)(ctx); err != nil {
		s.Fatal("Failed to find the network list: ", err)
	}
}

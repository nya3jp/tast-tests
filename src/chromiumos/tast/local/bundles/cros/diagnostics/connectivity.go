// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package diagnostics

import (
	"context"
	"time"

	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ui/diagnosticsapp"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: Connectivity,
		Desc: "Can successfully navigate to the Connectivity page",
		Contacts: []string{
			"michaelcheco@google.com",
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
	defer dxRootnode.Release(ctx)

	// Find the Connectivity navigation item.
	connectivityTab, err := dxRootnode.DescendantWithTimeout(ctx, diagnosticsapp.DxConnectivity, 20*time.Second)
	if err != nil {
		s.Fatal("Failed to find the Connectivity navigation item: ", err)
	}
	defer connectivityTab.Release(ctx)
	pollOpts := testing.PollOptions{Interval: time.Second, Timeout: 20 * time.Second}
	if err := connectivityTab.StableLeftClick(ctx, &pollOpts); err != nil {
		s.Fatal("Could not click the Connectivity tab: ", err)
	}

	// Find the first routine action button
	networkListContainer, err := dxRootnode.DescendantWithTimeout(ctx, diagnosticsapp.DxNetworkList, 20*time.Second)
	if err != nil {
		s.Fatal("Failed to find the network list: ", err)
	}
	defer networkListContainer.Release(ctx)
}

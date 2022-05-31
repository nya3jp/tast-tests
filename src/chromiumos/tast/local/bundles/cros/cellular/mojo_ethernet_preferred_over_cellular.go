// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package cellular

import (
	"context"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/cellular"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/network/netconfig"
	"chromiumos/tast/local/shill"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         MojoEthernetPreferredOverCellular,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Confirm Ethernet is preferred over cellular when both are enabled and wifi is disabled",
		Contacts:     []string{"shijinabraham@google.com", "cros-network-health@google.com", "chromeos-cellular-team@google.com"},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:cellular", "cellular_unstable", "cellular_sim_active"},
		Timeout:      10 * time.Minute,
		Fixture:      "cellular",
	})
}

// MojoEthernetPreferredOverCellular checks that ethernet is preferred over
// cellular when both are enabled and connected with WiFi disabled.
// Todo(b:222693784) Add test to check whether Wifi is preferred over Cellular.
func MojoEthernetPreferredOverCellular(ctx context.Context, s *testing.State) {
	cr, err := chrome.New(ctx)
	if err != nil {
		s.Fatal("Failed start Chrome: ", err)
	}
	defer cr.Close(ctx)

	// Enable cellular and connect to a network.
	cellularHelper, err := cellular.NewHelper(ctx)
	if err != nil {
		s.Fatal("Failed to create cellular.Helper: ", err)
	}

	service, err := cellularHelper.FindServiceForDevice(ctx)
	if err != nil {
		s.Fatal("Unable to find Cellular Service for Device: ", err)
	}
	if isConnected, err := service.IsConnected(ctx); err != nil {
		s.Fatal("Unable to get IsConnected for Service: ", err)
	} else if !isConnected {
		if _, err := cellularHelper.ConnectToDefault(ctx); err != nil {
			s.Fatal("Unable to Connect to Service: ", err)
		}
	}

	// Disable WiFi if present and defer re-enabling.
	if enableFunc, err := cellularHelper.Manager.DisableTechnologyForTesting(ctx, shill.TechnologyWifi); err != nil {
		s.Fatal("Unable to disable WiFi: ", err)
	} else if enableFunc != nil {
		newCtx, cancel := ctxutil.Shorten(ctx, shill.EnableWaitTime)
		defer cancel()
		defer enableFunc(ctx)
		ctx = newCtx
	}

	// Lab devices will have ethernet already enabled and connected.

	netConn, err := netconfig.CreateLoggedInCrosNetworkConfig(ctx, cr)
	if err != nil {
		s.Fatal("Failed to get network Mojo Object: ", err)
	}
	defer netConn.Close(ctx)

	// Get state of all active networks.
	filter := netconfig.NetworkFilter{
		Filter:      netconfig.ActiveFT,
		NetworkType: netconfig.All,
		Limit:       0}

	networkStates, err := netConn.GetNetworkStateList(ctx, filter)
	if err != nil {
		s.Fatal("Failed to get NetworkStateList: ", err)
	}

	// WiFi is disabled while cellular and ethernet is enabled and connected.
	// Since GetNetworkStateList returns networks in order of priority,
	// the first one should be ethernet and second one should be cellular.

	if len(networkStates) < 2 {
		s.Logf("NetworkStateList is %+v", networkStates)
		s.Fatal("Less than 2 networks in networkstatelist")
	}
	if networkStates[0].Type != netconfig.Ethernet {
		s.Fatal("Wrong network in the second position expected: ethernet, got: ", networkStates[0].Type)
	}
	if !netconfig.NetworkStateIsConnectedOrOnline(networkStates[0]) {
		s.Fatal("Ethernet not Online or Connected, got: ", networkStates[0].ConnectionState)
	}
	if networkStates[1].Type != netconfig.Cellular {
		s.Fatal("Wrong network in the second position expected: cellular, got: ", networkStates[1].Type)
	}
	if !netconfig.NetworkStateIsConnectedOrOnline(networkStates[1]) {
		s.Fatal("Cellular not Online or Connected, got: ", networkStates[1].ConnectionState)
	}
}

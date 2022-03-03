// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package cellular

import (
	"context"
	"time"

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

// MojoEthernetPreferredOverCellular checks that ethernet is preferred over cellular when both are enabled and connected with WiFi disabled
func MojoEthernetPreferredOverCellular(ctx context.Context, s *testing.State) {
	cr, err := chrome.New(ctx)
	if err != nil {
		s.Fatal("Failed start Chrome: ", err)
	}
	defer cr.Close(ctx)

	// Enable cellular and connect to a network
	cellularHelper, err := cellular.NewHelper(ctx)
	if err != nil {
		s.Fatal("Failed to create cellular.Helper: ", err)
	}

	if _, err = cellularHelper.Enable(ctx); err != nil {
		s.Fatal("Failed to enable cellular: ", err)
	}
	defer cellularHelper.Disable(ctx)

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

	// Disable Wifi
	wifiManager, err := shill.NewWifiManager(ctx, nil)
	if err != nil {
		s.Fatal("Failed to create shill Wi-Fi manager: ", err)
	}
	if err := wifiManager.Enable(ctx, false); err != nil {
		s.Fatal("Failed to disable Wi-Fi: ", err)
	}
	defer wifiManager.Enable(ctx, true)

	// Lab devices will have ethernet already enabled and connected

	netConn, err := netconfig.NewCrosNetworkConfig(ctx, cr)
	if err != nil {
		s.Fatal("Failed to get network Mojo Object: ", err)
	}
	defer netConn.Close(ctx)

	// Get state of all active networks
	filter := netconfig.NetworkFilter{
		Filter:      netconfig.ActiveFT,
		NetworkType: netconfig.All,
		Limit:       0}

	networkState, err := netConn.GetNetworkStateList(ctx, filter)
	if err != nil {
		s.Fatal("Failed to get NetworkStateList: ", err)
	}

	// WiFi is disabled while cellular and ethernet is enabled and connected
	// Since GetNetworkStateList returns networks in order of priority, the first one should be ethernet and second one should be cellular

	if networkState[0].Type != netconfig.Ethernet {
		s.Fatal("Wrong network in the second position expected: ethernet, got: ", networkState[0].Type)
	} else {
		if networkState[0].ConnectionState != netconfig.ConnectedCST && networkState[0].ConnectionState != netconfig.OnlineCST {
			s.Fatal("Unexpected Ethernet state, expected: Connected or Online got: ", networkState[0].ConnectionState)
		}
	}

	if networkState[1].Type != netconfig.Cellular {
		s.Fatal("Wrong network in the second position expected: cellular, got ", networkState[1].Type)
	} else {
		if networkState[1].ConnectionState != netconfig.ConnectedCST && networkState[1].ConnectionState != netconfig.OnlineCST {
			s.Fatal("Unexpected Cellular state, expected: Connected or Online got: ", networkState[1].ConnectionState)
		}
	}
}

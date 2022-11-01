// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package network

import (
	"context"
	//	"time"

	"chromiumos/tast/local/bundles/cros/network/health"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/shill"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         HealthGetNetworkList,
		LacrosStatus: testing.LacrosVariantNeeded,
		Desc:         "Validates that NetworkHealth API accurately gets networks",
		Contacts: []string{
			"khegde@chromium.org",                 // test maintainer
			"stevenjb@chromium.org",               // network-health tech lead
			"cros-network-health-team@google.com", // network-health team
		},
		SoftwareDeps: []string{"chrome", "no_qemu"},
		Attr:         []string{"group:mainline", "informational"},
	})
}

// HealthGetNetworkList validates that the NetworkHealth API correctly retrieves
// networks.
func HealthGetNetworkList(ctx context.Context, s *testing.State) {
	m, err := shill.NewManager(ctx)
	if err != nil {
		s.Fatal("Failed to create manager proxy: ", err)
	}

	services, err := m.Services(ctx)
	if err != nil {
		s.Fatal("Failed to get list of services: ", err)
	}
	if len(services) == 0 {
		s.Fatal("Failed to get non-zero list of services")
	}

	name, err := services[0].GetName(ctx)
	if err != nil {
		s.Fatal("Failed to get name for service: ", err)
	}

	connected, err := services[0].IsConnected(ctx)
	if err != nil {
		s.Fatal("Failed to get connected state for: ", name)
	}
	if !connected {
		s.Fatal("Service is not connected: ", name)
	}

	cr, err := chrome.New(ctx)
	if err != nil {
		s.Fatal("Failed start Chrome: ", err)
	}
	defer cr.Close(ctx)

	netConn, err := health.CreateLoggedInNetworkHealth(ctx, cr)
	if err != nil {
		s.Fatal("Failed to get network Mojo Object: ", err)
	}
	defer netConn.Close(ctx)

	networks, err := netConn.GetNetworkList(ctx, s)
	if err != nil {
		s.Fatal("Failed to run GetNetworkList: ", err)
	}

	if len(networks) == 0 {
		s.Fatal("Failed to get non-zero list of networks")
	}
	if networks[0].State != health.OnlineNS && networks[0].State != health.ConnectedNS {
		s.Fatalf("Network is not connected, GUID: %v, State: %v", networks[0].GUID, networks[0].State)
	}
}

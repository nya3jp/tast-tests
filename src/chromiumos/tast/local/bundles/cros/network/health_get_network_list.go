// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package network

import (
	"context"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
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
		Fixture:      "shillReset",
	})
}

// findMatchingNetwork searches the given list of networks to find one matching
// the shill type (sType) and guid (for non-Ethernet networks).
func findMatchingNetwork(networks []health.Network, sType, guid string) (*health.Network, error) {
	networkType, err := health.NetworkTypeFromShillType(sType)
	if err != nil {
		return nil, err
	}
	for _, n := range networks {
		if networkType != n.Type {
			continue
		}
		if n.Type == health.EthernetNT || guid == n.GUID {
			return &n, nil
		}
	}
	return nil, errors.New("failed to find a connected network in Network Health")
}

// HealthGetNetworkList validates that the NetworkHealth API correctly retrieves
// networks.
func HealthGetNetworkList(ctx context.Context, s *testing.State) {
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	cr, err := chrome.New(ctx)
	if err != nil {
		s.Fatal("Failed start Chrome: ", err)
	}
	defer cr.Close(cleanupCtx)

	netConn, err := health.CreateLoggedInNetworkHealth(ctx, cr)
	if err != nil {
		s.Fatal("Failed to get network Mojo Object: ", err)
	}
	defer netConn.Close(cleanupCtx)

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
	// The default service is guaranteed to to be first in Shill.
	defaultService := services[0]
	name, err := defaultService.GetName(ctx)
	if err != nil {
		s.Fatal("Failed to get name of default shill service: ", err)
	}
	sType, err := defaultService.GetType(ctx)
	if err != nil {
		s.Fatalf("Failed to get type for %v: %v", name, err)
	}
	guid, err := defaultService.GetGUID(ctx)
	if err != nil {
		s.Fatalf("Failed to get GUID for %v: %v", name, err)
	}

	networks, err := netConn.GetNetworkList(ctx, s)
	if err != nil {
		s.Fatal("Failed to run GetNetworkList: ", err)
	}
	network, err := findMatchingNetwork(networks, sType, guid)
	if err != nil {
		s.Fatalf("Network %s not found: %v", name, err)
	}
	if network.State != health.OnlineNS && network.State != health.ConnectedNS {
		s.Fatalf("Active network not connected, network: %v, State: %v", network.Name, network.State)
	}
}

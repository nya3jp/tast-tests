// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package network

import (
	"context"
	"time"

	"chromiumos/tast/common/shillconst"
	"chromiumos/tast/ctxutil"
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

func isMatchingType(sType string, nType health.NetworkType) bool {
	if sType == shillconst.TypeEthernet && nType == health.EthernetNT {
		return true
	}
	if sType == shillconst.TypeWifi && nType == health.WiFiNT {
		return true
	}
	if sType == shillconst.TypeCellular && nType == health.CellularNT {
		return true
	}
	if sType == shillconst.TypeVPN && nType == health.VPNNT {
		return true
	}
	return false
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

	networks, err := netConn.GetNetworkList(ctx, s)
	if err != nil {
		s.Fatal("Failed to run GetNetworkList: ", err)
	}
	if len(networks) == 0 {
		s.Fatal("Failed to get non-zero list of networks")
	}

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
	// The default service is guaranteed to to be first in Shill
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
	found := false
	for _, n := range networks {
		if n.State != health.OnlineNS && n.State != health.ConnectedNS {
			continue
		}
		if !isMatchingType(sType, n.Type) {
			continue
		}
		if n.Type == health.EthernetNT || guid == n.GUID {
			s.Logf("Matching active network found, name: %v, type: %v, guid %v", name, sType, guid)
			found = true
		}
	}
	if !found {
		s.Fatal("Failed to find a connected network in Network Health")
	}
}

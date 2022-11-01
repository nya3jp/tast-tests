// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package network

import (
	"context"

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
	s.Logf("Length of services is %v :", len(services))
	for i := 0; i < len(services); i++ {
		name, nerr := services[i].GetName(ctx)
		if nerr != nil {
			s.Fatal("Failed to get name ", nerr)
		}
		state, serr := services[i].GetState(ctx)
		if serr != nil {
			s.Fatal("Failed to get state ", serr)
		}
		s.Logf("Service name %v and state %v", name, state)
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

	networks, err := netConn.GetNetworkList(ctx)
	if err != nil {
		s.Fatal("Failed to run GetNetworkList: ", err)
	}

	s.Log("Length of networks is ", len(networks))
}

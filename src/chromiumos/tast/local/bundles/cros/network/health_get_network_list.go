// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package network

import (
	"context"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/bundles/cros/network/health"
	"chromiumos/tast/local/chrome"
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
	if networks[0].State != health.OnlineNS && networks[0].State != health.ConnectedNS {
		s.Fatalf("Network is not connected, GUID: %v, State: %v", networks[0].GUID, networks[0].State)
	}
	if networks[0].Type != health.EthernetNT {
		s.Fatalf("Wrong network type, got: %v, want: %v", networks[0].Type, health.EthernetNT)
	}
}

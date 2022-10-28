// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package network

import (
	"context"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/bundles/cros/wifi/hwsim"
	"chromiumos/tast/local/network/routing"
	"chromiumos/tast/local/network/virtualnet"
	"chromiumos/tast/local/network/virtualnet/subnet"
	"chromiumos/tast/local/shill"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:     VirtualWifi,
		Desc:     "Verify the routing on a virtual WiFi network",
		Contacts: []string{"jiejiang@google.com", "cros-networking@google.com"},
		// Not a real test.
		// Attr:         []string{"group:mainline", "informational"},
		LacrosStatus: testing.LacrosVariantUnneeded,
		Fixture:      "shillSimulatedWiFi",
		Timeout:      1 * time.Hour,
	})
}

func VirtualWifi(ctx context.Context, s *testing.State) {
	//Use a shortened context for test operations to reserve time for cleanup.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	m, _ := shill.NewManager(ctx)
	pool := subnet.NewPool()

	ifaces := s.FixtValue().(*hwsim.ShillSimulatedWiFi)

	wifi, err := virtualnet.CreateWifiRouterEnv(ctx, ifaces.AP[0], m, pool, virtualnet.EnvOptions{
		EnableDHCP: true,
		RAServer:   true,
	})
	if err != nil {
		s.Fatal("Failed to create router: ", err)
	}
	defer wifi.Cleanup(cleanupCtx)

	testing.ContextLog(ctx, "Connecting to the WiFi service")
	if err := wifi.Service.Connect(ctx); err != nil {
		s.Fatal("Failed to connect to the wifi service: ", err)
	}

	if err := wifi.Service.WaitForConnectedOrError(ctx); err != nil {
		s.Fatal("Failed to wait for service connected: ", err)
	}

	testing.ContextLog(ctx, "Verifying the WiFi service")
	gatewayAddrs, err := wifi.Router.WaitForVethInAddrs(ctx, true, true)
	if err != nil {
		s.Fatal("Failed to get gateway address: ", err)
	}
	if err := routing.ExpectPingSuccessWithTimeout(ctx, gatewayAddrs.IPv4Addr.String(), "chronos", 10*time.Second); err != nil {
		s.Error("Gateway IPv4 not reachable: ", err)
	}
	// b/235050937: Local IPv6 is not reachable on secondary networks.
	// if err := routing.ExpectPingSuccessWithTimeout(ctx, gatewayAddrs.IPv6Addrs[0].String(), "chronos", 10*time.Second); err != nil {
	// 	s.Error("Gateway IPv6 not reachable: ", err)
	// }

	// We have a connected WiFi network now. Have fun!
	testing.Sleep(ctx, 1*time.Hour)
}

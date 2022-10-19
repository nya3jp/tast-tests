// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package network

import (
	"context"
	"net"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/network/routing"
	"chromiumos/tast/local/network/virtualnet"
	"chromiumos/tast/local/network/virtualnet/dnsmasq"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         DHCPWebProxy,
		Desc:         "Verify that WPAD option (option 252) got from DHCP is reflected in the IPConfig object exposed by shill",
		Contacts:     []string{"jiejiang@google.com", "cros-networking@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		LacrosStatus: testing.LacrosVariantUnneeded,
	})
}

func DHCPWebProxy(ctx context.Context, s *testing.State) {
	// Use a shortened context for test operations to reserve time for cleanup.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	testEnv := routing.NewTestEnv()
	if err := testEnv.SetUp(ctx); err != nil {
		s.Fatal("Failed to set up routing test env: ", err)
	}
	defer func(ctx context.Context) {
		if err := testEnv.TearDown(ctx); err != nil {
			s.Error("Failed to tear down routing test env: ", err)
		}
	}(cleanupCtx)

	// Create the test network with higher priority and thus it should become the
	// default network.
	testNetworkOpts := virtualnet.EnvOptions{
		Priority:   routing.HighPriority,
		NameSuffix: routing.TestSuffix,
		EnableDHCP: false,
		RAServer:   false,
	}
	if err := testEnv.CreateNetworkEnvForTest(ctx, testNetworkOpts); err != nil {
		s.Fatal("Failed to create network environment for test: ", err)
	}

	subnet, err := testEnv.Pool.AllocNextIPv4Subnet()
	if err != nil {
		s.Fatal("Failed to allocate subnet for DHCP: ", err)
	}

	subnetIP := subnet.IP.To4()
	gateway := net.IPv4(subnetIP[0], subnetIP[1], subnetIP[2], 1)
	wpad := "http://" + gateway.String() + "/wpad.dat"

	dnsmasqServer := dnsmasq.New(
		dnsmasq.WithDHCPServer(subnet),
		dnsmasq.WithDHCPWPAD(wpad),
	)
	if err := testEnv.TestRouter.StartServer(ctx, "dnsmasq", dnsmasqServer); err != nil {
		s.Fatal("Failed to start dnsmasq in test router: ", err)
	}

	testing.ContextLog(ctx, "Waiting for service online")
	if err := testEnv.WaitForServiceOnline(ctx, testEnv.TestService); err != nil {
		s.Fatal("Failed to wait for service in test online: ", err)
	}

	ipconfigs, err := testEnv.TestService.GetIPConfigs(ctx)
	if err != nil {
		s.Fatal("Failed to get IPConfigs from test service: ", err)
	}

	if len(ipconfigs) != 1 {
		s.Fatalf("Number of IPConfigs does not match: len(ipconfigs) = %v. want 1", len(ipconfigs))
	}

	props, err := ipconfigs[0].GetIPProperties(ctx)
	if err != nil {
		s.Fatal("Failed to get IPConfig properties: ", err)
	}
	if props.WebProxyAutoDiscoveryURL != wpad {
		s.Fatalf("Value for WPAD does not match: got %v, want %v", props.WebProxyAutoDiscoveryURL, wpad)
	}
}

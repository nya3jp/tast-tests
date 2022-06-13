// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package network

import (
	"context"
	"time"

	"chromiumos/tast/local/bundles/cros/network/routing"
	"chromiumos/tast/local/bundles/cros/network/virtualnet"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         RoutingIPv6Only,
		Desc:         "Verify the shill behavior and routing semantics in an IPv6-only environment",
		Contacts:     []string{"jiejiang@google.com", "cros-networking@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		LacrosStatus: testing.LacrosVariantUnneeded,
	})
}

func RoutingIPv6Only(ctx context.Context, s *testing.State) {
	testEnv := routing.NewTestEnv()
	if err := testEnv.SetUp(ctx); err != nil {
		s.Fatal("Failed to set up routing test env: ", err)
	}
	defer func() {
		if err := testEnv.TearDown(ctx); err != nil {
			s.Error("Failed to tear down routing test env: ", err)
		}
	}()

	// Setup the IPv6-only test network, and verify its connectivity.
	testNetworkOpts := virtualnet.EnvOptions{
		Priority:   routing.HighPriority,
		NameSuffix: routing.TestSuffix,
		EnableDHCP: false,
		RAServer:   true,
	}
	if err := testEnv.CreateNetworkEnvForTest(ctx, testNetworkOpts); err != nil {
		s.Fatal("Failed to create network for test: ", err)
	}
	if err := testEnv.WaitForServiceOnline(ctx, testEnv.TestService); err != nil {
		s.Fatal("Failed to wait for the test service online: ", err)
	}
	if errs := testEnv.VerifyTestNetwork(ctx, routing.VerifyOptions{
		IPv4:      false,
		IPv6:      true,
		IsPrimary: true,
		Timeout:   5 * time.Second,
	}); len(errs) != 0 {
		for _, err := range errs {
			s.Fatal(ctx, "Failed to verify connectivity to the test network before DHCP timeout: ", err)
		}
		return
	}

	// Trigger the DHCP timeout event, and verify that the connectivty is not affected.
	testing.ContextLog(ctx, "Waiting for DHCP timeout event for ", routing.DHCPTimeout+time.Second)
	testing.Sleep(ctx, routing.DHCPTimeout+time.Second)
	testing.ContextLog(ctx, "DHCP timeout was triggered")
	if errs := testEnv.VerifyTestNetwork(ctx, routing.VerifyOptions{
		IPv4:      false,
		IPv6:      true,
		IsPrimary: true,
		Timeout:   0,
	}); len(errs) != 0 {
		for _, err := range errs {
			s.Fatal(ctx, "Failed to verify connectivity to the test network after DHCP timeout: ", err)
		}
		return
	}
}

// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.
package network

import (
	"chromiumos/tast/local/bundles/cros/network/routing"
	"chromiumos/tast/local/bundles/cros/network/virtualnet"
	"chromiumos/tast/testing"
	"context"
	"net"
	"time"
)

// This file is NOT for submission.
func init() {
	testing.AddTest(&testing.Test{
		Func:         RoutingDualstackNetwork,
		Desc:         "Verify that the basic routing functionalities on a dual-stack network",
		Contacts:     []string{"jiejiang@google.com", "cros-networking@google.com"},
		Attr:         []string{},
		LacrosStatus: testing.LacrosVariantUnneeded,
	})
}

func RoutingDualstackNetwork(ctx context.Context, s *testing.State) {
	testEnv := routing.NewTestEnv()
	if err := testEnv.SetUp(ctx); err != nil {
		s.Fatal("Failed to set up routing test env: ", err)
	}
	defer func() {
		if err := testEnv.TearDown(ctx); err != nil {
			s.Error("Failed to tear down routing test env: ", err)
		}
	}()

	for _, user := range []string{"root", "chronos"} {
		reachableAddrs := []net.IP{
			testEnv.BaseServer.VethInStaticIPv4Addr,
			testEnv.BaseServer.VethInStaticIPv6Addr,
		}
		for _, ip := range reachableAddrs {
			if err := routing.ExpectPingSuccessWithTimeout(ctx, ip.String(), user, 10*time.Second); err != nil {
				s.Fatalf("Failed to ping %s as user %s: %v", ip.String(), user, err)
			}
		}
	}

	testNetworkOpts := virtualnet.EnvOptions{
		Priority:   routing.HighPriority,
		NameSuffix: routing.TestPrefix,
		EnableDHCP: false,
		RAServer:   true,
	}
	if err := testEnv.CreateNetworkEnvForTest(ctx, testNetworkOpts); err != nil {
		s.Fatal("Failed to create network for test: ", err)
	}

	if err := testEnv.WaitForServiceOnline(ctx, testEnv.TestService); err != nil {
		s.Fatal("Failed to wait for service in test online: ", err)
	}

	testing.Sleep(ctx, 150*time.Second)

	for _, user := range []string{"root", "chronos"} {
		reachableAddrs := []net.IP{
			testEnv.TestServer.VethInStaticIPv4Addr,
			testEnv.TestServer.VethInStaticIPv6Addr,
		}
		for _, ip := range reachableAddrs {
			if err := routing.ExpectPingSuccessWithTimeout(ctx, ip.String(), user, 10*time.Second); err != nil {
				s.Fatalf("Failed to ping %s as user %s: %v", ip.String(), user, err)
			}
		}

		nonReachableAddrs := []net.IP{
			testEnv.BaseServer.VethInStaticIPv4Addr,
			testEnv.BaseServer.VethInStaticIPv6Addr,
		}
		for _, ip := range nonReachableAddrs {
			if err := routing.ExpectPingFailure(ctx, ip.String(), user); err != nil {
				s.Fatalf("Failed to ping %s as user %s: %v", ip.String(), user, err)
			}
		}
	}
}

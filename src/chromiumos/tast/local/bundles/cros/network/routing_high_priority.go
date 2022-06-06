// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package network

import (
	"context"
	"time"

	"chromiumos/tast/local/bundles/cros/network/routing"
	"chromiumos/tast/local/bundles/cros/network/virtualnet"
	"chromiumos/tast/local/bundles/cros/network/virtualnet/env"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         RoutingHighPriority,
		Desc:         "Verify the routing semantics in the case that there is a dual-stack network and then another network with higher priority shows up",
		Contacts:     []string{"jiejiang@google.com", "cros-networking@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		LacrosStatus: testing.LacrosVariantUnneeded,
	})
}

// RoutingHighPriority verifies that, when a new network with higher priority
// becomes Online, it should be the default network (primary network), but the
// local peers on the secondary network should still be reachable.
func RoutingHighPriority(ctx context.Context, s *testing.State) {
	testEnv := routing.NewTestEnv()
	if err := testEnv.SetUp(ctx); err != nil {
		s.Fatal("Failed to set up routing test env: ", err)
	}
	defer func() {
		if err := testEnv.TearDown(ctx); err != nil {
			s.Error("Failed to tear down routing test env: ", err)
		}
	}()

	// All the Envs in this test should have both IPv4 and IPv6 addresses.
	getEnvAddrs := func(env *env.Env) *env.IfaceAddrs {
		addrs, err := env.GetVethInAddrs(ctx)
		if err != nil {
			s.Fatalf("Failed to get addrs of from env %s: %v", env.NetNSName, err)
		}
		if addrs.IPv4Addr == nil || len(addrs.IPv6Addrs) == 0 {
			s.Fatal("The number of addrs is not expected: ", addrs)
		}
		return addrs
	}

	// Check that the base network is reachable via both IPv4 and IPv6. Note that
	// when the service becomes Online, it may not be available in both families
	// and thus the initial ping may fail.
	baseServerAddrs := getEnvAddrs(testEnv.BaseServer)
	for _, user := range []string{"root", "chronos"} {
		for _, ip := range baseServerAddrs.All() {
			if err := routing.ExpectPingSuccessWithTimeout(ctx, ip.String(), user, 10*time.Second); err != nil {
				s.Fatalf("Non-local address %v on the primary network is not reachable as user %s: %v", ip, user, err)
			}
		}
	}

	testNetworkOpts := virtualnet.EnvOptions{
		Priority:   routing.HighPriority,
		NameSuffix: routing.TestSuffix,
		EnableDHCP: true,
		RAServer:   true,
	}
	if err := testEnv.CreateNetworkEnvForTest(ctx, testNetworkOpts); err != nil {
		s.Fatal("Failed to create network for test: ", err)
	}

	if err := testEnv.WaitForServiceOnline(ctx, testEnv.TestService); err != nil {
		s.Fatal("Failed to wait for service in test online: ", err)
	}

	testServerAddrs := getEnvAddrs(testEnv.TestServer)
	baseRouterAddrs := getEnvAddrs(testEnv.BaseRouter)
	for _, user := range []string{"root", "chronos"} {
		// The secondary network should not be used (as default route) as long as the
		// primary network is Online.
		for _, ip := range baseServerAddrs.All() {
			if err := routing.ExpectPingFailure(ctx, ip.String(), user); err != nil {
				s.Errorf("Non-local address %v on the secondary network should not be reachable as user %s: %v", ip, user, err)
			}
		}

		// Verify that default routes work for the primary network.
		for _, ip := range testServerAddrs.All() {
			if err := routing.ExpectPingSuccessWithTimeout(ctx, ip.String(), user, 10*time.Second); err != nil {
				s.Errorf("Non-local address %v on the primary network is not reachable as user %s: %v", ip, user, err)
			}
		}

		// Local addresses on the secondary network should still be reachable.
		// TODO(b/235050937): IPv6 peer on local subnet of the secondary network is
		// not reachable. Only check IPv4 now.
		ip := baseRouterAddrs.IPv4Addr
		if err := routing.ExpectPingSuccess(ctx, ip.String(), user); err != nil {
			s.Errorf("Local address %v on the secondary network is not reachable as user %s: %v", ip, user, err)
		}
	}
}

// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package network

import (
	"context"
	"time"

	"chromiumos/tast/common/shillconst"
	"chromiumos/tast/local/bundles/cros/network/routing"
	"chromiumos/tast/local/bundles/cros/network/virtualnet"
	"chromiumos/tast/local/bundles/cros/network/virtualnet/env"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         RoutingNoIP,
		Desc:         "Verify the shill and routing behavior that there is a new network but no IP is provided on it",
		Contacts:     []string{"jiejiang@google.com", "cros-networking@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		LacrosStatus: testing.LacrosVariantUnneeded,
	})
}

// RoutingNoIP verifies that, if a new network comes up and there is no IP
// configuration on it, its state should become failure after the DHCP timeout,
// and this process should have no routing impact.
func RoutingNoIP(ctx context.Context, s *testing.State) {
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
	getEnvAddrs := func(e *env.Env) *env.IfaceAddrs {
		addrs, err := routing.WaitDualStackIPsInEnv(ctx, e)
		if err != nil {
			s.Fatalf("Failed to get addrs of inside env %s: %v", e.NetNSName, err)
		}
		return addrs
	}

	// Check that the base network is reachable via both IPv4 and IPv6. Note that
	// when the service becomes Online, it may not be available in both families
	// and thus the initial ping may fail.
	baseServerAddrs := getEnvAddrs(testEnv.BaseServer)
	for _, user := range []string{"root", "chronos"} {
		for _, ip := range baseServerAddrs.All() {
			if err := routing.ExpectPingSuccessWithTimeout(ctx, ip.String(), user, 30*time.Second); err != nil {
				s.Fatalf("Non-local address %v on the primary network is not reachable as user %s: %v", ip, user, err)
			}
		}
	}

	// No IP will be provided on this network.
	testNetworkOpts := virtualnet.EnvOptions{
		Priority:   routing.HighPriority,
		NameSuffix: routing.TestSuffix,
		EnableDHCP: false,
		RAServer:   false,
	}
	if err := testEnv.CreateNetworkEnvForTest(ctx, testNetworkOpts); err != nil {
		s.Fatal("Failed to create network for test: ", err)
	}

	// For counting the time accurately, disconnect the service at first.
	// Auto-reconnect will not happen after the disconnection since this is a
	// user-initiate disconnect.
	testing.ContextLog(ctx, "Disconnecting the test service")
	if err := testEnv.TestService.Disconnect(ctx); err != nil {
		s.Fatal("Failed to disconnect the test service: ", err)
	}
	if err := testEnv.TestService.WaitForProperty(ctx, shillconst.ServicePropertyState, shillconst.ServiceStateIdle, 5*time.Second); err != nil {
		s.Fatal("Failed to wait for the test service idle: ", err)
	}

	timeBeforeConnect := time.Now()

	// Create a property watcher before connect before connect to avoid missing
	// signals.
	pw, err := testEnv.TestService.CreateWatcher(ctx)
	if err != nil {
		s.Fatal("Failed to create property watcher for the test service: ", err)
	}
	defer pw.Close(ctx)

	// Connect the service, the service state should become Configuration
	// immediately, and the IP connectivity should not be affected on the primary
	// network.
	testing.ContextLog(ctx, "Connecting the test service")
	if err := testEnv.TestService.Connect(ctx); err != nil {
		s.Fatal("Failed to connect the test service: ", err)
	}
	if err := testEnv.TestService.WaitForProperty(ctx, shillconst.ServicePropertyState, shillconst.ServiceStateConfiguration, 5*time.Second); err != nil {
		s.Fatal("Failed to wait for the test service online: ", err)
	}
	for _, user := range []string{"root", "chronos"} {
		for _, ip := range baseServerAddrs.All() {
			// Routing should be stable here, so use ExpectPingSuccess() instead of
			// ExpectPingSuccessWithTimeout().
			if err := routing.ExpectPingSuccess(ctx, ip.String(), user); err != nil {
				s.Fatalf("After connecting the test service, non-local address %v on the primary network is not reachable as user %s: %v", ip, user, err)
			}
		}
	}

	// DHCP timeout event should turn the service state into failure. Note that an
	// Ethernet service in shill will be reconnected immediately after a failure,
	// so we use the property watch to check the D-Bus signal instead of polling
	// its property value.
	testing.ContextLog(ctx, "Waiting for DHCP timeout")
	dhcpTimeoutCtx, cancel := context.WithTimeout(ctx, time.Second*time.Duration(routing.DHCPTimeout.Seconds()+3))
	defer cancel()
	interestStates := []interface{}{
		shillconst.ServiceStateReady,
		shillconst.ServiceStateOnline,
		shillconst.ServiceStateFailure,
	}
	state, err := pw.ExpectIn(dhcpTimeoutCtx, shillconst.ServicePropertyState, interestStates)
	if err != nil {
		s.Fatal("Failed to wait for service becoming failure: ", err)
	}
	if state != shillconst.ServiceStateFailure {
		s.Fatal("Failed to wait for service becoming failure, current state is ", state)
	}

	// We start counting the time before connecting, so the time diff must exceed
	// the DHCP timeout.
	diff := time.Since(timeBeforeConnect)
	if diff < routing.DHCPTimeout {
		s.Fatal(ctx, "The service state changed to failure in %s which is less than %s", diff, routing.DHCPTimeout)
	}
}

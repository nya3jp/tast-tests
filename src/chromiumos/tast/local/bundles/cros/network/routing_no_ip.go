// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package network

import (
	"context"
	"time"

	"chromiumos/tast/common/shillconst"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/network/routing"
	"chromiumos/tast/local/network/virtualnet"
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

	// Create a property watcher before connect to avoid missing signals.
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

	if errs := testEnv.VerifyBaseNetwork(ctx, routing.VerifyOptions{
		IPv4:      true,
		IPv6:      true,
		IsPrimary: true,
		Timeout:   0,
	}); len(errs) != 0 {
		for _, err := range errs {
			s.Error("Failed to verify base network after creating test network: ", err)
		}
		return
	}

	// DHCP timeout event should turn the service state into failure. Note that an
	// Ethernet service in shill will be reconnected immediately after a failure,
	// so we use the property watch to check the D-Bus signal instead of polling
	// its property value.
	testing.ContextLog(ctx, "Waiting for DHCP timeout for ", routing.DHCPExtraTimeout)
	dhcpTimeoutCtx, cancel := context.WithTimeout(ctx, routing.DHCPExtraTimeout)
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
		s.Fatalf("The service state changed to failure in %s which is less than %s", diff, routing.DHCPTimeout)
	}
}

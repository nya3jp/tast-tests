// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package network

import (
	"context"
	"net"
	"time"

	"chromiumos/tast/common/shillconst"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/network/routing"
	"chromiumos/tast/local/network/virtualnet"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         RoutingDualStackWithStatic,
		Desc:         "Verify the shill behavior and routing semantics when the network is dual-stack with DHCP and SLAAC, configure static IP on it",
		Contacts:     []string{"jiejiang@google.com", "cros-networking@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		LacrosStatus: testing.LacrosVariantUnneeded,
	})
}

// RoutingDualStackWithStatic verifies the scenario described in
// https://b/243336792#comment1.
func RoutingDualStackWithStatic(ctx context.Context, s *testing.State) {
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
		s.Fatal("Failed to wait for test service Online: ", err)
	}

	verifyOpts := routing.VerifyOptions{
		IPv4:      true,
		IPv6:      true,
		IsPrimary: true,
		Timeout:   5 * time.Second,
	}
	if errs := testEnv.VerifyTestNetwork(ctx, verifyOpts); len(errs) > 0 {
		for _, err := range errs {
			s.Error("Failed to verify test network before applying static IP: ", err)
		}
		return
	}

	// Get the IPv4 addr from router. Apply static IP config with another address
	// in the subnet. 254 should not be in DHCP range.
	routerAddrs, err := testEnv.TestRouter.GetVethInAddrs(ctx)
	if err != nil {
		s.Fatal("Failed to get test router addresses: ", err)
	}
	routerIP := routerAddrs.IPv4Addr
	newClientIP := net.IPv4(routerIP[0], routerIP[1], routerIP[2], 254)

	svcStaticIPConfig := map[string]interface{}{
		shillconst.IPConfigPropertyAddress:   newClientIP.String(),
		shillconst.IPConfigPropertyGateway:   routerIP.String(),
		shillconst.IPConfigPropertyPrefixlen: 24,
	}
	testing.ContextLogf(ctx, "Configuring %v on the test interface", svcStaticIPConfig)
	if err := testEnv.TestService.SetProperty(ctx, shillconst.ServicePropertyStaticIPConfig, svcStaticIPConfig); err != nil {
		s.Fatal("Failed to configure StaticIPConfig property on the test service: ", err)
	}
	defer func(ctx context.Context) {
		// Reset StaticIPConfig before removing the test interfaces, to avoid
		// installing this address on the physical interfaces. See
		// b/239753191#comment8 for a racing case.
		if err := testEnv.TestService.SetProperty(ctx, shillconst.ServicePropertyStaticIPConfig, map[string]interface{}{}); err != nil {
			testing.ContextLog(ctx, "Failed to reset StaticIPConfig property on the test service: ", err)
		}
	}(cleanupCtx)

	// Wait for the StaticIPConfig to be applied by checking the address appears
	// in IPConfigs.
	if err := testing.Poll(ctx, func(c context.Context) error {
		ipconfigs, err := testEnv.TestService.GetIPConfigs(ctx)
		if err != nil {
			return errors.Wrap(err, "failed to get IPConfigs from service")
		}
		for _, ipconfig := range ipconfigs {
			props, err := ipconfig.GetIPProperties(ctx)
			if err != nil {
				return errors.Wrap(err, "failed to get IPProperties")
			}
			if props.Address == newClientIP.String() {
				return nil
			}
		}
		return errors.Errorf("no IPConfig for the service have address %s", newClientIP.String())
	}, &testing.PollOptions{Timeout: 10 * time.Second}); err != nil {
		s.Fatal("Failed to wait for the StaticIPConfig to be applied: ", err)
	}

	// We want to verify that nothing will happen after that, so use a sleep here.
	const timeout = 3 * time.Second
	testing.ContextLog(ctx, "Waiting for routing setup stable for ", timeout)
	testing.Sleep(ctx, timeout)

	if errs := testEnv.VerifyTestNetwork(ctx, verifyOpts); len(errs) != 0 {
		for _, err := range errs {
			s.Error("Failed to verify test network after configuring static IP: ", err)
		}
	}
}

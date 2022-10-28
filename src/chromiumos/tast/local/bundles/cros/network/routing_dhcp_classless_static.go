// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package network

import (
	"context"
	"net"
	"time"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/network/routing"
	"chromiumos/tast/local/network/virtualnet"
	"chromiumos/tast/local/network/virtualnet/dnsmasq"
	"chromiumos/tast/local/network/virtualnet/env"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         RoutingDHCPClasslessStatic,
		Desc:         "Verify the shill behavior and routing semantics in a DHCP environment with classless static routes (option 121)",
		Contacts:     []string{"jiejiang@google.com", "cros-networking@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		LacrosStatus: testing.LacrosVariantUnneeded,
	})
}

// RoutingDHCPClasslessStatic verifies if the routing system respects the
// classless static routes sent by DHCP servers. Specifically, the destination
// of a classless static route should be reachable even if it is on the
// secondary network.
//
// In this test, we set up the network as follows:
//
//	veth0 ----- base router ----- base server (primary network, reachable)
//
//	veth1 --+-- test router --+-- test server (secondary network, not reachable)
//	        |   DHCP server   |
//	        |                 +-- server1 (classless static route, reachable)
//	        |
//	        +-- second gateway -- server2 (classless static route, reachable)
//
// In the setup, veth1, second gateway, and DHCP server are on the same subnet,
// which is implemented by bridging the interfaces.
func RoutingDHCPClasslessStatic(ctx context.Context, s *testing.State) {
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
		Priority:   routing.LowPriority,
		NameSuffix: routing.TestSuffix,
		EnableDHCP: false, // start the dnsmasq later to configure classless static routes
		RAServer:   false,
	}
	if err := testEnv.CreateNetworkEnvForTest(ctx, testNetworkOpts); err != nil {
		s.Fatal("Failed to create network for test: ", err)
	}

	dhcpSubnet, err := testEnv.Pool.AllocNextIPv4Subnet()
	if err != nil {
		s.Fatal("Failed to allocate subnet for DHCP: ", err)
	}

	// The interface name of the bridge device in the test router netns.
	const br = "br0"

	// We will create two additional servers in this test. This struct represents
	// such a server.
	type server struct {
		IP      net.IP
		Route   dnsmasq.Route
		Cleanup func(context.Context)
	}

	// A helper function which creates function to cleanup an Env object and log
	// on failure.
	cleanupEnv := func(e *env.Env) func(ctx context.Context) {
		return func(ctx context.Context) {
			if err := e.Cleanup(ctx); err != nil {
				testing.ContextLog(ctx, "Failed to clean up env: ", err)
			}
		}
	}

	createServerBehindSameGateway := func() (*server, error) {
		testing.ContextLog(ctx, "Setting up another server behind the test router")
		success := false

		env := env.New("same-sv")
		if err := env.SetUp(ctx); err != nil {
			return nil, errors.Wrap(err, "failed to set up env")
		}

		if err := env.ConnectToRouterWithPool(ctx, testEnv.TestRouter, testEnv.Pool); err != nil {
			return nil, errors.Wrap(err, "failed to connect to router")
		}

		defer func() {
			if !success {
				cleanupEnv(env)(cleanupCtx)
			}
		}()

		addr, err := env.GetVethInAddrs(ctx)
		if err != nil {
			s.Fatal("Failed to get addrs: ", err)
		}

		dhcpSubnetIP := dhcpSubnet.IP.To4()

		success = true
		return &server{
			IP: addr.IPv4Addr,
			Route: dnsmasq.Route{
				Prefix:  &net.IPNet{IP: addr.IPv4Addr, Mask: net.IPv4Mask(255, 255, 255, 255)},
				Gateway: net.IPv4(dhcpSubnetIP[0], dhcpSubnetIP[1], dhcpSubnetIP[2], 1),
			},
			Cleanup: cleanupEnv(env),
		}, nil
	}

	createServerBehindOtherGateway := func() (*server, error) {
		testing.ContextLog(ctx, "Setting up another gateway and server on the same network of the test router")
		success := false

		gatewayEnv := env.New("other-gw")
		if err := gatewayEnv.SetUp(ctx); err != nil {
			return nil, errors.Wrap(err, "failed to set up the second gateway on the test network")
		}
		defer func() {
			if !success {
				cleanupEnv(gatewayEnv)(cleanupCtx)
			}
		}()

		serverEnv := env.New("other-sv")
		if err := serverEnv.SetUp(ctx); err != nil {
			return nil, errors.Wrap(err, "failed to set up the server behind second gateway on the test network")
		}
		defer func() {
			if !success {
				cleanupEnv(serverEnv)(cleanupCtx)
			}
		}()

		if err := serverEnv.ConnectToRouterWithPool(ctx, gatewayEnv, testEnv.Pool); err != nil {
			return nil, errors.Wrap(err, "TODO")
		}

		// Move the out interface of gatewayEnv into the router netns, and bridge it
		// with the DHCP interface.
		if err := testexec.CommandContext(ctx,
			"ip", "link",
			"set", gatewayEnv.VethOutName,
			"netns", testEnv.TestRouter.NetNSName,
		).Run(testexec.DumpLogOnError); err != nil {
			return nil, err
		}

		// The bridge interface will be removed together with the netns, so no
		// explicit cleanup is needed here.
		for _, cmd := range [][]string{
			{"ip", "link", "set", gatewayEnv.VethOutName, "up"},
			{"brctl", "addbr", br},
			{"ip", "link", "set", br, "up"},
			{"brctl", "addif", br, gatewayEnv.VethOutName},
			{"brctl", "addif", br, testEnv.TestRouter.VethInName},
		} {
			if err := testEnv.TestRouter.RunWithoutChroot(ctx, cmd...); err != nil {
				return nil, err
			}
		}

		dhcpSubnetIP := dhcpSubnet.IP.To4()
		gateway := net.IPv4(dhcpSubnetIP[0], dhcpSubnetIP[1], dhcpSubnetIP[2], 2)
		if err := gatewayEnv.ConfigureInterface(ctx, gatewayEnv.VethInName, gateway, dhcpSubnet); err != nil {
			return nil, errors.Wrap(err, "failed to configure static IP on the second gateway")
		}

		serverAddr, err := serverEnv.GetVethInAddrs(ctx)
		if err != nil {
			return nil, errors.Wrap(err, "failed to get addresses of the server")
		}

		success = true
		return &server{
			IP: serverAddr.IPv4Addr,
			Route: dnsmasq.Route{
				// Use a /24 prefix here.
				Prefix:  &net.IPNet{IP: serverAddr.IPv4Addr, Mask: net.IPv4Mask(255, 255, 255, 0)},
				Gateway: gateway,
			},
			Cleanup: func(ctx context.Context) {
				cleanupEnv(gatewayEnv)(ctx)
				cleanupEnv(serverEnv)(ctx)
			},
		}, nil
	}

	server1, err := createServerBehindSameGateway()
	if err != nil {
		s.Fatal("Failed to set up server behind the same gateway: ", err)
	}
	defer server1.Cleanup(cleanupCtx)

	server2, err := createServerBehindOtherGateway()
	if err != nil {
		s.Fatal("Failed to set up server behind other gateway: ", err)
	}
	defer server2.Cleanup(cleanupCtx)

	dnsmasqServer := dnsmasq.New(
		dnsmasq.WithDHCPServer(dhcpSubnet),
		dnsmasq.WithDHCPClasslessStaticRoutes([]dnsmasq.Route{server1.Route, server2.Route}),
		dnsmasq.WithInterface(br),
	)
	if err := testEnv.TestRouter.StartServer(ctx, "dnsmasq", dnsmasqServer); err != nil {
		s.Fatal("Failed to start dnsmasq in test router: ", err)
	}

	// Reconnect service to speed up DHCP acquisition.
	testing.ContextLog(ctx, "Reconnecting to test service")
	if err := testEnv.TestService.Disconnect(ctx); err != nil {
		s.Fatal("Failed to disconnect test service: ", err)
	}
	if err := testEnv.TestService.Connect(ctx); err != nil {
		s.Fatal("Failed to connect test service: ", err)
	}

	if err := testEnv.WaitForServiceOnline(ctx, testEnv.TestService); err != nil {
		s.Fatal("Failed to wait for the test service online: ", err)
	}

	if errs := testEnv.VerifyBaseNetwork(ctx, routing.VerifyOptions{
		IPv4:      true,
		IPv6:      true,
		IsPrimary: true,
		Timeout:   5 * time.Second,
	}); len(errs) != 0 {
		for _, err := range errs {
			s.Fatal("Failed to verify connectivity to the base network: ", err)
		}
		return
	}

	testServerAddrs, err := testEnv.TestServer.GetVethInAddrs(ctx)
	if err != nil {
		s.Fatal("Failed to get addresses in test server env: ", err)
	}

	for _, u := range []string{"root", "chronos"} {
		if err := routing.ExpectPingFailure(ctx, testServerAddrs.IPv4Addr.String(), u); err != nil {
			s.Errorf("Test server should not be reachable as user %s: %v", u, err)
		}
		if err := routing.ExpectPingSuccessWithTimeout(ctx, server1.IP.String(), u, 3*time.Second); err != nil {
			s.Errorf("Failed to verify server behind same gateway as user %s: %v", u, err)
		}
		if err := routing.ExpectPingSuccessWithTimeout(ctx, server2.IP.String(), u, 3*time.Second); err != nil {
			s.Errorf("Failed to verify server behind other gateway as user %s: %v", u, err)
		}
	}
}

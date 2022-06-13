// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package network

import (
	"context"
	"net"
	"time"

	"github.com/google/go-cmp/cmp"

	"chromiumos/tast/common/shillconst"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/network/routing"
	"chromiumos/tast/local/bundles/cros/network/virtualnet"
	"chromiumos/tast/local/shill"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         RoutingIPv4StaticWithDHCP,
		Desc:         "Verify the shill behavior and routing semantics when the network has DHCP and static config for IPv4 but no IPv6",
		Contacts:     []string{"jiejiang@google.com", "cros-networking@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		LacrosStatus: testing.LacrosVariantUnneeded,
	})
}

func RoutingIPv4StaticWithDHCP(ctx context.Context, s *testing.State) {
	testEnv := routing.NewTestEnv()
	if err := testEnv.SetUp(ctx); err != nil {
		s.Fatal("Failed to set up routing test env: ", err)
	}
	defer func() {
		if err := testEnv.TearDown(ctx); err != nil {
			s.Error("Failed to tear down routing test env: ", err)
		}
	}()

	testNetworkOpts := virtualnet.EnvOptions{
		Priority:   routing.HighPriority,
		NameSuffix: routing.TestSuffix,
		EnableDHCP: true,
		RAServer:   false,
	}
	if err := testEnv.CreateNetworkEnvForTest(ctx, testNetworkOpts); err != nil {
		s.Fatal("Failed to create network for test: ", err)
	}
	if err := testEnv.WaitForServiceOnline(ctx, testEnv.TestService); err != nil {
		s.Fatal("Failed to wait for test service Online: ", err)
	}

	// Since we only have IPv4 in this environment, service Online means IPv4
	// connectivity. Use a small timeout here.
	verifyOpts := routing.VerifyOptions{
		IPv4:      true,
		IPv6:      false,
		IsPrimary: true,
		Timeout:   5 * time.Second,
	}
	if errs := testEnv.VerifyTestNetwork(ctx, verifyOpts); len(errs) > 0 {
		for _, err := range errs {
			s.Error("Failed to verify test network before applying static IP: ", err)
		}
		return
	}

	// A helper function to get IPProperties from a service. The service should
	// have only one IPConfig in this test.
	getIPProps := func(svc *shill.Service) (shill.IPProperties, error) {
		ipconfigs, err := svc.GetIPConfigs(ctx)
		if err != nil {
			return shill.IPProperties{}, errors.Wrap(err, "failed to get IPConfigs from service")
		}
		if len(ipconfigs) != 1 {
			return shill.IPProperties{}, errors.Wrapf(err, "expect only one IPConfig object but got %v", ipconfigs)
		}
		return ipconfigs[0].GetIPProperties(ctx)
	}

	dhcpProps, err := getIPProps(testEnv.TestService)
	if err != nil {
		s.Fatal("Failed to get IP properties after DHCP: ", err)
	}

	// Get the IPv4 addr from router. Apply static IP config with another address
	// in the subnet.
	routerAddrs, err := testEnv.TestRouter.GetVethInAddrs(ctx)
	if err != nil {
		s.Fatal("Failed to get test router addresses: ", err)
	}
	routerIP := routerAddrs.IPv4Addr
	newClientIP := net.IPv4(routerIP[0], routerIP[1], routerIP[2], 254)
	// 254 is not in the pool of DHCP.
	if newClientIP.String() == dhcpProps.Address {
		s.Fatalf("Current IPv4 address %s on the interface is unexpected", dhcpProps.Address)
	}

	svcStaticIPConfig := map[string]interface{}{
		shillconst.IPConfigPropertyAddress:   newClientIP.String(),
		shillconst.IPConfigPropertyGateway:   dhcpProps.Gateway,
		shillconst.IPConfigPropertyPrefixlen: dhcpProps.PrefixLen,
	}
	testing.ContextLogf(ctx, "Configuring %v on the test interface", svcStaticIPConfig)
	if err := testEnv.TestService.SetProperty(ctx, shillconst.ServicePropertyStaticIPConfig, svcStaticIPConfig); err != nil {
		s.Fatal("Failed to configure StaticIPConfig property on the test service: ", err)
	}

	// Wait for the StaticIPConfig to be applied.
	expectedProps := dhcpProps
	expectedProps.Address = newClientIP.String()
	if err := testing.Poll(ctx, func(c context.Context) error {
		props, err := getIPProps(testEnv.TestService)
		if err != nil {
			return err
		}
		if diff := cmp.Diff(props, expectedProps); diff != "" {
			return errors.Errorf("unexpected IPConfig properties, diff: %s", diff)
		}
		return nil
	}, &testing.PollOptions{Timeout: 10 * time.Second}); err != nil {
		s.Fatal("Failed to wait for the StaticIPConfig to be applied: ", err)
	}

	// The test network should still be the primary network. Changing IP
	// parameters may flush the routing table, so use a timeout here for the
	// routing setup.
	if errs := testEnv.VerifyTestNetwork(ctx, routing.VerifyOptions{
		IPv4:      true,
		IPv6:      false,
		IsPrimary: true,
		Timeout:   5 * time.Second,
	}); len(errs) != 0 {
		for _, err := range errs {
			s.Error("Failed to verify test network after configuring static IP: ", err)
		}
	}
}

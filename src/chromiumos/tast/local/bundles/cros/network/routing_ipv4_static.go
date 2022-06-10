// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package network

import (
	"context"
	"net"
	"time"

	"chromiumos/tast/common/shillconst"
	"chromiumos/tast/local/bundles/cros/network/routing"
	"chromiumos/tast/local/bundles/cros/network/virtualnet"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         RoutingIPv4Static,
		Desc:         "Verify the shill behavior and routing semantics when the network does not have DHCP or SLAAC but only static IPv4 config",
		Contacts:     []string{"jiejiang@google.com", "cros-networking@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		LacrosStatus: testing.LacrosVariantUnneeded,
	})
}

func RoutingIPv4Static(ctx context.Context, s *testing.State) {
	testEnv := routing.NewTestEnv()
	if err := testEnv.SetUp(ctx); err != nil {
		s.Fatal("Failed to set up routing test env: ", err)
	}
	defer func() {
		if err := testEnv.TearDown(ctx); err != nil {
			s.Error("Failed to tear down routing test env: ", err)
		}
	}()

	// Start a virtualnet with neither IPv4 nor IPv6. Disconnect it at first, and
	// configure static IP on the router side.
	testNetworkOpts := virtualnet.EnvOptions{
		Priority:   routing.HighPriority,
		NameSuffix: routing.TestSuffix,
		EnableDHCP: false,
		RAServer:   false,
	}
	if err := testEnv.CreateNetworkEnvForTest(ctx, testNetworkOpts); err != nil {
		s.Fatal("Failed to create network for test: ", err)
	}
	testing.ContextLog(ctx, "Disconnecting the test service")
	if err := testEnv.TestService.Disconnect(ctx); err != nil {
		s.Fatal("Failed to disconnect the test service: ", err)
	}
	if err := testEnv.TestService.WaitForProperty(ctx, shillconst.ServicePropertyState, shillconst.ServiceStateIdle, 5*time.Second); err != nil {
		s.Fatal("Failed to wait for the test service idle: ", err)
	}

	ipv4Subnet, err := testEnv.Pool.AllocNextIPv4Subnet()
	if err != nil {
		s.Fatal("Failed to allocate IPv4 subnet for test network: ", err)
	}
	ipv4Addr := ipv4Subnet.IP.To4()
	localIPv4Addr := net.IPv4(ipv4Addr[0], ipv4Addr[1], ipv4Addr[2], 2)
	routerIPv4Addr := net.IPv4(ipv4Addr[0], ipv4Addr[1], ipv4Addr[2], 1)
	if err := testEnv.TestRouter.ConfigureInterface(ctx, testEnv.TestRouter.VethInName, routerIPv4Addr, ipv4Subnet); err != nil {
		s.Fatal("Failed to configure IPv4 inside test router: ", err)
	}

	// Configure static IP config on shill service.
	prefixLen, _ := ipv4Subnet.Mask.Size()
	svcStaticIPConfig := map[string]interface{}{
		shillconst.IPConfigPropertyAddress:   localIPv4Addr.String(),
		shillconst.IPConfigPropertyGateway:   routerIPv4Addr.String(),
		shillconst.IPConfigPropertyPrefixlen: prefixLen,
	}
	testing.ContextLogf(ctx, "Configuring %v on the test interface", svcStaticIPConfig)
	if err := testEnv.TestService.SetProperty(ctx, shillconst.ServicePropertyStaticIPConfig, svcStaticIPConfig); err != nil {
		s.Fatal("Failed to configure StaticIPConfig property on the test service: ", err)
	}

	// Connect the service, and its state should become Online.
	// TODO(b/235330956): The current code verifies that configuring static IP
	// when the service is idle. We also want to verify that configuring static IP
	// when the service is connecting should make the service becoming connected,
	// but currently this is not the case due to b/235330956.
	if err := testEnv.TestService.Connect(ctx); err != nil {
		s.Fatal("Failed to connect the test service: ", err)
	}
	if err := testEnv.TestService.WaitForProperty(ctx, shillconst.ServicePropertyState, shillconst.ServiceStateOnline, 5*time.Second); err != nil {
		s.Fatal("Failed to wait for the test service online: ", err)
	}

	// Verify that the DHCP timeout event does not turn the service down. We
	// cannot trigger this event manually so nothing can be done except for
	// sleeping here. Additional 1 second to make sure the event is triggered.
	testing.ContextLog(ctx, "Waiting for DHCP timeout event for ", routing.DHCPTimeout)
	testing.Sleep(ctx, routing.DHCPTimeout)
	testing.Sleep(ctx, time.Second)
	testing.ContextLog(ctx, "DHCP timeout was triggered")

	// Verify the service state is not changed.
	state, err := testEnv.TestService.GetState(ctx)
	if err != nil {
		s.Fatal("Failed to get service state: ", err)
	}
	if state != shillconst.ServiceStateOnline {
		s.Fatalf("Expect service state %s, but got %s", shillconst.ServiceStateOnline, state)
	}

	// Verify routing setup for test network.
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

	// TODO(b/159725895): When the Ethernet service order changes, the first
	// Ethernet service will be reloaded from the ethernet_any profile, this will
	// also overwrite the StaticIPConfig field. This will not affect routing but
	// will affect the IPConfig objects on the Device, so in this test we rewrite
	// this property again.
	if err := testEnv.TestService.SetProperty(ctx, shillconst.ServicePropertyStaticIPConfig, svcStaticIPConfig); err != nil {
		s.Fatal("Failed to configure StaticIPConfig property on the test service: ", err)
	}

	// Verify IPConfigs.
	ipconfigs, err := testEnv.TestService.GetIPConfigs(ctx)
	if err != nil {
		s.Fatal("Failed to get IPConfigs: ", err)
	}
	if len(ipconfigs) != 1 {
		s.Fatal("Expect 1 IPConfig objects, but got ", len(ipconfigs))
	}
	expectedIPConfig := map[string]interface{}{
		shillconst.IPConfigPropertyAddress:   localIPv4Addr.String(),
		shillconst.IPConfigPropertyGateway:   routerIPv4Addr.String(),
		shillconst.IPConfigPropertyMethod:    "ipv4",
		shillconst.IPConfigPropertyPrefixlen: int32(prefixLen),
	}
	if err := routing.VerifyIPConfig(ctx, ipconfigs[0], expectedIPConfig); err != nil {
		s.Fatal("Failed to verify IPConfig values: ", err)
	}
}

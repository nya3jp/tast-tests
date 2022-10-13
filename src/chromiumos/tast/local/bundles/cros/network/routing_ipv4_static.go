// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package network

import (
	"context"
	"net"
	"time"

	"github.com/google/go-cmp/cmp"

	"chromiumos/tast/common/shillconst"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/network/routing"
	"chromiumos/tast/local/network/virtualnet"
	"chromiumos/tast/local/shill"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         RoutingIPv4Static,
		Desc:         "Verify the shill behavior and routing semantics when the network does not have DHCP or SLAAC but only static IPv4 config",
		Contacts:     []string{"jiejiang@google.com", "cros-networking@google.com"},
		Attr:         []string{"group:mainline"},
		LacrosStatus: testing.LacrosVariantUnneeded,
		Params: []testing.Param{{
			// Apply static IP when the network is idle.
			Val: false,
		}, {
			// Apply static IP when the network is connecting.
			Name:      "apply_when_connecting",
			Val:       true,
			ExtraAttr: []string{"informational"},
		}},
	})
}

func RoutingIPv4Static(ctx context.Context, s *testing.State) {
	// Use a shortened context for test operations to reserve time for cleanup.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	disconnectBeforeApply := s.Param().(bool)

	testEnv := routing.NewTestEnv()
	if err := testEnv.SetUp(ctx); err != nil {
		s.Fatal("Failed to set up routing test env: ", err)
	}
	defer func(ctx context.Context) {
		if err := testEnv.TearDown(ctx); err != nil {
			s.Error("Failed to tear down routing test env: ", err)
		}
	}(cleanupCtx)

	// Start a virtualnet with neither IPv4 nor IPv6, and configure static IP on
	// the router side.
	testNetworkOpts := virtualnet.EnvOptions{
		Priority:   routing.HighPriority,
		NameSuffix: routing.TestSuffix,
		EnableDHCP: false,
		RAServer:   false,
	}
	if err := testEnv.CreateNetworkEnvForTest(ctx, testNetworkOpts); err != nil {
		s.Fatal("Failed to create network for test: ", err)
	}

	// The allocated subnet has a /24 prefix.
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

	if disconnectBeforeApply {
		testing.ContextLog(ctx, "Disconnecting the test service")
		if err := testEnv.TestService.Disconnect(ctx); err != nil {
			s.Fatal("Failed to disconnect the test service: ", err)
		}
		if err := testEnv.TestService.WaitForProperty(ctx, shillconst.ServicePropertyState, shillconst.ServiceStateIdle, 5*time.Second); err != nil {
			s.Fatal("Failed to wait for the test service idle: ", err)
		}
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
	defer func(ctx context.Context) {
		// Reset StaticIPConfig before removing the test interfaces, to avoid
		// installing this address on the physical interfaces. See
		// b/239753191#comment8 for a racing case.
		if err := testEnv.TestService.SetProperty(ctx, shillconst.ServicePropertyStaticIPConfig, map[string]interface{}{}); err != nil {
			testing.ContextLog(ctx, "Failed to reset StaticIPConfig property on the test service: ", err)
		}
	}(cleanupCtx)

	if disconnectBeforeApply {
		testing.ContextLog(ctx, "Connect to the test service")
		if err := testEnv.TestService.Connect(ctx); err != nil {
			s.Fatal("Failed to connect the test service: ", err)
		}
	}

	testing.ContextLog(ctx, "Waiting for test service online")
	if err := testEnv.TestService.WaitForProperty(ctx, shillconst.ServicePropertyState, shillconst.ServiceStateOnline, 5*time.Second); err != nil {
		s.Fatal("Failed to wait for the test service online: ", err)
	}

	// Verify that the DHCP timeout event does not turn the service down. We
	// cannot trigger this event manually so nothing can be done except for
	// sleeping here.
	testing.ContextLog(ctx, "Waiting for DHCP timeout event for ", routing.DHCPExtraTimeout)
	testing.Sleep(ctx, routing.DHCPExtraTimeout)
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
	actualIPProps, err := ipconfigs[0].GetIPProperties(ctx)
	if err != nil {
		s.Fatal("Failed to get IPProperties from IPConfig: ", err)
	}
	expectedIPProps := shill.IPProperties{
		Address:        localIPv4Addr.String(),
		Gateway:        routerIPv4Addr.String(),
		Method:         "ipv4",
		PrefixLen:      int32(prefixLen),
		NameServers:    []string{},
		ISNSOptionData: []uint8{},
	}
	if diff := cmp.Diff(actualIPProps, expectedIPProps); diff != "" {
		s.Fatal("Got unexpected IPProperties with diff: ", diff)
	}
}

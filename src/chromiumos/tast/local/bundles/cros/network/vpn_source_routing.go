// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package network

import (
	"context"
	"time"

	"chromiumos/tast/common/shillconst"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/bundles/cros/network/vpn"
	"chromiumos/tast/local/network/routing"
	"chromiumos/tast/local/network/virtualnet"
	"chromiumos/tast/local/network/virtualnet/env"
	"chromiumos/tast/local/network/virtualnet/subnet"
	"chromiumos/tast/local/shill"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         VPNSourceRouting,
		Desc:         "Verify traffic from different sources is routed to the correct interface",
		Contacts:     []string{"garrick@google.com", "cros-networking@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"ikev2"},
		LacrosStatus: testing.LacrosVariantUnneeded,
	})
}

// VPNSourceRouting verifies that traffic from different user sources is routed
// correctly, either into the VPN, or directly onto the underlying physical
// interface.
func VPNSourceRouting(ctx context.Context, s *testing.State) {
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(cleanupCtx, 10*time.Second)
	defer cancel()

	mgr, err := shill.NewManager(ctx)
	if err != nil {
		s.Fatal("Failed to create manager proxy: ", err)
	}
	if err := mgr.SetProperty(ctx, shillconst.ProfilePropertyCheckPortalList, "wifi,cellular"); err != nil {
		s.Fatal("Failed to disable portal detection on ethernet: ", err)
	}
	defer func() {
		if err := mgr.SetProperty(cleanupCtx, shillconst.ProfilePropertyCheckPortalList, "ethernet,wifi,cellular"); err != nil {
			s.Fatal("Failed to restore portal detection on ethernet: ", err)
		}
	}()
	pool := subnet.NewPool()

	// Setup a router and connect 2 servers.
	svc, rt, svr, err := virtualnet.CreateRouterServerEnv(ctx, mgr, pool, virtualnet.EnvOptions{
		Priority:   10,
		EnableDHCP: true,
	})
	if err != nil {
		s.Fatal("Failed to create router env: ", err)
	}
	defer rt.Cleanup(cleanupCtx)

	vsvr, err := env.New("vserver")
	if err != nil {
		s.Fatal("Failed to create server: ", err)
	}
	if err := vsvr.SetUp(ctx); err != nil {
		s.Fatal("Failed to setup server: ", err)
	}
	s4, err := pool.AllocNextIPv4Subnet()
	if err != nil {
		s.Fatal("Failed to allocate v4 subnet: ", err)
	}
	s6, err := pool.AllocNextIPv6Subnet()
	if err != nil {
		s.Fatal("Failed to allocate v6 subnet: ", err)
	}
	if err := vsvr.ConnectToRouter(ctx, rt, s4, s6); err != nil {
		s.Fatal("Failed to connect server to router: ", err)
	}
	if err := svc.WaitForProperty(ctx, shillconst.ServicePropertyState, shillconst.ServiceStateOnline, 10*time.Second); err != nil {
		s.Fatal("Failed to wait for service: ", err)
	}
	addrs, err := svr.WaitForVethInAddrs(ctx, true, false)
	if err != nil {
		s.Fatal("Failed to get server addrs: ", err)
	}
	saddr := addrs.IPv4Addr.String()

	// Establish a VPN on one of the servers.
	conn, err := vpn.NewConnectionWithEnvs(ctx, vpn.Config{
		Type:     vpn.TypeIKEv2,
		AuthType: vpn.AuthTypePSK,
	}, vsvr, nil)
	if err != nil {
		s.Fatal("Failed to connect vpn: ", err)
	}
	defer conn.Cleanup(cleanupCtx)
	if err := conn.SetUp(ctx); err != nil {
		s.Fatal("Failed to setup vpn: ", err)
	}
	if ok, err := conn.Connect(ctx); !ok || err != nil {
		s.Fatal("Failed to create vpn connection: ", err)
	}
	vaddr := conn.Server.OverlayIPv4

	test := func(user, goodIP, badIP string) {
		if err := routing.ExpectPingSuccessWithTimeout(ctx, goodIP, user, 10*time.Second); err != nil {
			s.Errorf("User %s failed to ping %v: %v", user, goodIP, err)
		}
		if err := routing.ExpectPingFailure(ctx, badIP, user); err != nil {
			s.Errorf("User %s able to ping %v: %v", user, badIP, err)
		}
	}

	// Verify that traffic from certain system user sources will egress on
	// the physical network and not the VPN.
	for _, u := range []string{"root", "dns-proxy"} {
		test(u, saddr, vaddr)
	}
	// Verify that traffic from other user sources should egress on the VPN
	// This list is not comprehensive. See patchpanel/routing_service.h for
	// additional details.
	for _, u := range []string{"chronos", "debugd", "kerberosd", "pluginvm"} {
		test(u, vaddr, saddr)
	}
}

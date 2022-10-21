// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package network

import (
	"context"
	"time"

	"chromiumos/tast/common/shillconst"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/network/virtualnet"
	"chromiumos/tast/local/network/virtualnet/dnsmasq"
	"chromiumos/tast/local/network/virtualnet/subnet"
	"chromiumos/tast/local/shill"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:     DHCPHostname,
		Desc:     "Verify the hostname option sent by the DHCP client",
		Contacts: []string{"jiejiang@google.com", "cros-networking@google.com"},
		Attr:     []string{"group:mainline", "informational"},
		// DHCP hostname property is written into the default profile but not user
		// profile. Use shillReset to guarantee it is clean before and after the
		// test.
		Fixture:      "shillReset",
		LacrosStatus: testing.LacrosVariantUnneeded,
	})
}

// DHCPHostname verifies the hostname option sent by the DHCP client is the
// value set in shill.
func DHCPHostname(ctx context.Context, s *testing.State) {
	// Use a shortened context for test operations to reserve time for cleanup.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	manager, err := shill.NewManager(ctx)
	if err != nil {
		s.Fatal("Failed to create manager proxy: ", err)
	}

	// Prepare the environment.
	pool := subnet.NewPool()
	svc, rt, err := virtualnet.CreateRouterEnv(ctx, manager, pool, virtualnet.EnvOptions{})
	defer func() {
		if err := rt.Cleanup(cleanupCtx); err != nil {
			s.Error("Failed to clean up router: ", err)
		}
	}()
	subnet, err := pool.AllocNextIPv4Subnet()
	if err != nil {
		s.Fatal("Failed to allocate subnet for DHCP: ", err)
	}
	dnsmasqServer := dnsmasq.New(dnsmasq.WithDHCPServer(subnet))
	if err := rt.StartServer(ctx, "dnsmasq", dnsmasqServer); err != nil {
		s.Fatal("Failed to start dnsmasq in test router: ", err)
	}

	setHostname := func(val string) {
		testing.ContextLogf(ctx, "Set hostname to %q and reconnect the service", val)
		if err := manager.SetProperty(ctx, shillconst.ManagerPropertyDHCPHostname, val); err != nil {
			s.Fatal("Failed to set hostname property in shill: ", err)
		}
		if err := svc.Disconnect(ctx); err != nil {
			s.Fatal("Failed to disconnect the service")
		}
		if err := svc.Connect(ctx); err != nil {
			s.Fatal("Failed to reconnect the  service")
		}
		if err := svc.WaitForProperty(ctx, shillconst.ServicePropertyState, shillconst.ServiceStateOnline, 10*time.Second); err != nil {
			s.Fatal("Failed to wait for service in test online: ", err)
		}
	}

	// Helper function to check that if the leases in dnsmasq contain val as the hostname.
	hasHostname := func(val string) bool {
		leases, err := dnsmasqServer.GetLeases(ctx)
		if err != nil {
			s.Fatal("Failed to get leases from dnsmasq: ", err)
		}
		for _, l := range leases {
			if l.Hostname == val {
				return true
			}
		}
		return false
	}

	const hostname = "test-hostname"
	setHostname(hostname)
	if !hasHostname(hostname) {
		s.Fatal("Leases in dnsmasq does not contain the expected hostname")
	}

	setHostname("")
	if hasHostname(hostname) {
		s.Fatal("Leases in dnsmasq still contains the hostname which should be removed")
	}
}

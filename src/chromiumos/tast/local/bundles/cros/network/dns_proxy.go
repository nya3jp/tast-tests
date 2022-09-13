// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package network

import (
	"context"
	"time"

	"chromiumos/tast/common/shillconst"
	"chromiumos/tast/common/testexec"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/bundles/cros/network/dns"
	"chromiumos/tast/local/crostini"
	"chromiumos/tast/local/multivm"
	"chromiumos/tast/local/network"
	"chromiumos/tast/local/network/virtualnet/subnet"
	"chromiumos/tast/local/shill"
	"chromiumos/tast/testing"
)

type dnsProxyTestParams struct {
	mode dns.DoHMode
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         DNSProxy,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Ensure that DNS proxies are working correctly",
		Contacts:     []string{"jasongustaman@google.com", "garrick@google.com", "cros-networking@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome", "vm_host", "arc", "dlc", "no_kernel_upstream"},
		Data:         []string{crostini.GetContainerMetadataArtifact("buster", false), crostini.GetContainerRootfsArtifact("buster", false)},
		Pre:          multivm.ArcCrostiniStartedWithDNSProxy(),
		HardwareDeps: crostini.CrostiniStable,
		Timeout:      7 * time.Minute,
		Params: []testing.Param{{
			Name: "doh_off",
			Val: dnsProxyTestParams{
				mode: dns.DoHOff,
			},
		}, {
			Name: "doh_automatic",
			Val: dnsProxyTestParams{
				mode: dns.DoHAutomatic,
			},
		}, {
			Name: "doh_always_on",
			Val: dnsProxyTestParams{
				mode: dns.DoHAlwaysOn,
			},
		}},
	})
}

// DNSProxy tests DNS functionality with DNS proxy active.
// There are 2 parts to this test:
// 1. Ensuring that DNS queries are successful.
// 2. Ensuring that DNS queries are using proper mode (Off, Automatic, Always On) by blocking the expected ports, expecting the queries to fail.
func DNSProxy(ctx context.Context, s *testing.State) {
	// If the main body of the test times out, we still want to reserve a few
	// seconds to allow for our cleanup code to run.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(cleanupCtx, 3*time.Second)
	defer cancel()

	pre := s.PreValue().(*multivm.PreData)
	cr := pre.Chrome
	tconn := pre.TestAPIConn
	a := multivm.ARCFromPre(pre)
	cont := multivm.CrostiniFromPre(pre)

	// Ensure connectivity is available.
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		return testexec.CommandContext(ctx, "/bin/ping", "-c1", "-w1", "8.8.8.8").Run()
	}, &testing.PollOptions{Timeout: 5 * time.Second}); err != nil {
		s.Fatal("Failed to ping 8.8.8.8: ", err)
	}

	// Ensure connectivity is available inside Crostini's container.
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		return cont.Command(ctx, "ping", "-c1", "-w1", "8.8.8.8").Run()
	}, &testing.PollOptions{Timeout: 5 * time.Second}); err != nil {
		s.Fatal("Failed to ping 8.8.8.8 from Crostini: ", err)
	}

	// Install dig in container.
	if err := dns.InstallDigInContainer(ctx, cont); err != nil {
		s.Fatal("Failed to install dig in container: ", err)
	}

	// Set up virtualnet environment.
	pool := subnet.NewPool()
	cleanup, router, server, _, err := dns.SetupDNSEnv(ctx, pool)
	if err != nil {
		s.Fatal("Failed to setup DNS env: ", err)
	}
	defer cleanup(cleanupCtx)
	defer router.Cleanup(cleanupCtx)
	defer server.Cleanup(cleanupCtx)

	// Toggle plain-text DNS or secureDNS depending on test parameter.
	params := s.Param().(dnsProxyTestParams)
	if err := dns.SetDoHMode(ctx, cr, tconn, params.mode, dns.ExampleDoHProvider); err != nil {
		s.Fatal("Failed to set DNS-over-HTTPS mode: ", err)
	}

	// By default, DNS query should work.
	tc := []dns.ProxyTestCase{
		{Client: dns.System},
		{Client: dns.User},
		{Client: dns.Chrome},
		{Client: dns.Crostini},
		{Client: dns.ARC},
	}
	if errs := dns.TestQueryDNSProxy(ctx, tc, a, cont, dns.NewQueryOptions()); len(errs) != 0 {
		for _, err := range errs {
			s.Error("Failed DNS query check: ", err)
		}
	}

	nss, err := dns.ProxyNamespaces(ctx)
	if err != nil {
		s.Fatal("Failed to get DNS proxy's network namespaces: ", err)
	}

	physIfs, err := network.PhysicalInterfaces(ctx)
	if err != nil {
		s.Fatal("Failed to get physical interfaces: ", err)
	}

	// Block plain-text or secure DNS through iptables.
	var blocks []*dns.Block
	switch params.mode {
	case dns.DoHAutomatic:
		// We need to override the DoH provider <-> nameserver mapping that Chrome gave to shill.
		// This is necessary because we want to test the behavior of the automatic upgrade.
		// Without overriding, devices with an arbitrary nameserver without known DoH provider will only do Do53.
		m, err := shill.NewManager(ctx)
		if err != nil {
			s.Fatal("Failed to obtain shill manager: ", err)
		}
		svc, err := m.FindMatchingService(ctx, map[string]interface{}{
			shillconst.ServicePropertyState: shillconst.ServiceStateOnline,
		})
		if err != nil {
			s.Fatal("Failed to obtain online service: ", err)
		}
		cfgs, err := svc.GetIPConfigs(ctx)
		if err != nil {
			s.Fatal("Failed to get IP configuration: ", err)
		}
		var ns []string
		for _, cfg := range cfgs {
			ip, err := cfg.GetIPProperties(ctx)
			if err != nil {
				s.Fatal("Failed to get IP properties: ", err)
			}
			ns = append(ns, ip.NameServers...)
		}
		s.Log("Found nameservers: ", ns)
		if err := m.SetDNSProxyDOHProviders(ctx, dns.ExampleDoHProvider, ns); err != nil {
			s.Fatal("Failed to set dns-proxy DoH providers: ", err)
		}

		// Confirm blocking plaintext still works (DoH preferred/used).
		blocks = append(blocks, dns.NewPlaintextBlock(nss, physIfs, ""))
		// Verify blocking HTTPS also works (fallback).
		blocks = append(blocks, dns.NewDoHBlock(nss, physIfs))
		// Chrome isn't tested since it manages it's own DoH flow.
		// Allow retry for automatic mode. This is needed because Do53 fallback is only done after a DoH failure.
		// For this case, the failure happens on the proxy's DoH timeout which might be longer than the client's timeout.
		// Allow the client to retry the query. It is expected for the DoH server to be invalidated by then.
		tc = []dns.ProxyTestCase{
			{Client: dns.System, AllowRetry: true},
			{Client: dns.User, AllowRetry: true},
			{Client: dns.Crostini, AllowRetry: true},
			{Client: dns.ARC, AllowRetry: true}}
	case dns.DoHOff:
		// Verify blocking plaintext causes queries fail (no DoH option).
		blocks = append(blocks, dns.NewPlaintextBlock(nss, physIfs, ""))
		tc = []dns.ProxyTestCase{
			{Client: dns.System, ExpectErr: true},
			{Client: dns.User, ExpectErr: true},
			{Client: dns.Chrome, ExpectErr: true},
			{Client: dns.Crostini, ExpectErr: true},
			{Client: dns.ARC}}
	case dns.DoHAlwaysOn:
		// Verify blocking HTTPS causes queries to fail (no plaintext fallback).
		blocks = append(blocks, dns.NewDoHBlock(nss, physIfs))
		tc = []dns.ProxyTestCase{
			{Client: dns.System, ExpectErr: true},
			{Client: dns.User, ExpectErr: true},
			{Client: dns.Crostini, ExpectErr: true},
			{Client: dns.ARC}}
	}

	for _, block := range blocks {
		if errs := block.Run(ctx, func(ctx context.Context) {
			if errs := dns.TestQueryDNSProxy(ctx, tc, a, cont, dns.NewQueryOptions()); len(errs) != 0 {
				s.Error("Failed DNS query check: ", errs)
			}
		}); len(errs) > 0 {
			s.Fatal("Failed to block DNS: ", errs)
		}
	}
}

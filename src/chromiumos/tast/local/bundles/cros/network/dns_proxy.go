// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package network

import (
	"context"
	"time"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/bundles/cros/network/dns"
	"chromiumos/tast/local/crostini"
	"chromiumos/tast/local/multivm"
	"chromiumos/tast/local/network"
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
		SoftwareDeps: []string{"chrome", "vm_host", "arc", "dlc"},
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

	// Toggle plain-text DNS or secureDNS depending on test parameter.
	params := s.Param().(dnsProxyTestParams)
	if err := dns.SetDoHMode(ctx, cr, tconn, params.mode, dns.GoogleDoHProvider); err != nil {
		s.Fatal("Failed to set DNS-over-HTTPS mode: ", err)
	}

	// Ensure connectivity is available inside Crostini's container.
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		return cont.Command(ctx, "ping", "-c1", "-w1", "8.8.8.8").Run()
	}, &testing.PollOptions{Timeout: 5 * time.Second}); err != nil {
		s.Fatal("Failed to ping 8.8.8.8 from Crostini: ", err)
	}

	// Install dig in container after the DoH mode is set up properly.
	if err := dns.InstallDigInContainer(ctx, cont); err != nil {
		s.Fatal("Failed to install dig in container: ", err)
	}

	// By default, DNS query should work.
	var defaultTC = []dns.ProxyTestCase{
		{Client: dns.System},
		{Client: dns.User},
		{Client: dns.Chrome},
		{Client: dns.Crostini},
		{Client: dns.ARC},
	}
	if errs := dns.TestQueryDNSProxy(ctx, defaultTC, a, cont, dns.NewQueryOptions()); len(errs) != 0 {
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
	var (
		block        *dns.Block
		dnsBlockedTC []dns.ProxyTestCase
	)
	switch params.mode {
	case dns.DoHAutomatic:
		return
	case dns.DoHOff:
		block = dns.NewPlaintextBlock(nss, physIfs, "")
		dnsBlockedTC = []dns.ProxyTestCase{
			{Client: dns.System, ExpectErr: true},
			{Client: dns.User, ExpectErr: true},
			{Client: dns.Chrome, ExpectErr: true},
			{Client: dns.Crostini, ExpectErr: true},
			{Client: dns.ARC}}
	case dns.DoHAlwaysOn:
		block = dns.NewDoHBlock(nss, physIfs)
		dnsBlockedTC = []dns.ProxyTestCase{
			{Client: dns.System, ExpectErr: true},
			{Client: dns.User, ExpectErr: true},
			{Client: dns.Crostini, ExpectErr: true},
			{Client: dns.ARC}}
	}

	// DNS queries should fail if corresponding DNS packets (plain-text or secure) are dropped.
	if errs := block.Run(ctx, func(ctx context.Context) {
		if errs := dns.TestQueryDNSProxy(ctx, dnsBlockedTC, a, cont, dns.NewQueryOptions()); len(errs) != 0 {
			s.Error("Failed DNS query check: ", errs)
		}
	}); len(errs) > 0 {
		s.Fatal("Failed to block DNS: ", errs)
	}
}

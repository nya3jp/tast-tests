// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package network

import (
	"context"
	"time"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/local/bundles/cros/network/dns"
	"chromiumos/tast/local/crostini"
	"chromiumos/tast/local/multivm"
	"chromiumos/tast/local/network"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         DNSProxyFallback,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Verify plaintext DNS query is made when DOH query fails, when Secure DNS is in automatic mode",
		Contacts:     []string{"jasongustaman@google.com", "garrick@google.com", "cros-networking@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome", "vm_host", "arc", "dlc"},
		Data:         []string{crostini.GetContainerMetadataArtifact("buster", false), crostini.GetContainerRootfsArtifact("buster", false)},
		Pre:          multivm.ArcCrostiniStartedWithDNSProxy(),
		HardwareDeps: crostini.CrostiniStable,
		Timeout:      5 * time.Minute,
	})
}

// DNSProxyFallback verifies that dns-proxy will fallback to plaintext queries when DOH fails if Secure DNS is in automatic mode.
func DNSProxyFallback(ctx context.Context, s *testing.State) {
	pre := s.PreValue().(*multivm.PreData)
	cr := pre.Chrome
	tconn := pre.TestAPIConn
	arc := multivm.ARCFromPre(pre)
	crst := multivm.CrostiniFromPre(pre)
	if err := dns.SetDoHMode(ctx, cr, tconn, dns.DoHAutomatic, dns.GoogleDoHProvider); err != nil {
		s.Fatal("Failed to set DNS-over-HTTPS mode: ", err)
	}

	// Ensure connectivity is available.
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		return testexec.CommandContext(ctx, "/bin/ping", "-c1", "-w1", "8.8.8.8").Run()
	}, &testing.PollOptions{Timeout: time.Second}); err != nil {
		s.Fatal("No connectivity: ", err)
	}
	// Install dig in container.
	if err := dns.InstallDigInContainer(ctx, crst); err != nil {
		s.Fatal("Failed to install dig in container: ", err)
	}

	// By default, all DNS queries work as-is. We don't include Chrome because it manages it's own DoH.
	tc := []dns.ProxyTestCase{
		{Client: dns.System},
		{Client: dns.User},
		{Client: dns.Crostini},
		{Client: dns.ARC},
	}
	if errs := dns.TestQueryDNSProxy(ctx, tc, arc, crst, dns.NewQueryOptions()); len(errs) > 0 {
		s.Error("Failed initial DNS check: ", errs)
	}

	nss, err := dns.ProxyNamespaces(ctx)
	if err != nil {
		s.Fatal("Failed to get DNS proxy's network namespaces: ", err)
	}
	physIfs, err := network.PhysicalInterfaces(ctx)
	if err != nil {
		s.Fatal("Failed to get physical interfaces: ", err)
	}

	// Next, block plaintext DNS traffic and confirm DNS still works over HTTPS.
	if errs := dns.NewPlaintextBlock(nss, physIfs, "").Run(ctx, func(ctx context.Context) {
		if errs := dns.TestQueryDNSProxy(ctx, tc, arc, crst, dns.NewQueryOptions()); len(errs) > 0 {
			s.Error("Failed nameserver verification: ", errs)
		}
	}); len(errs) > 0 {
		s.Fatal("Failed to block DNS to nameserver: ", errs)
	}

	// Now block HTTPS and confirm DNS still works over port 53.
	if errs := dns.NewDoHBlock(nss, physIfs).Run(ctx, func(ctx context.Context) {
		if errs := dns.TestQueryDNSProxy(ctx, tc, arc, crst, dns.NewQueryOptions()); len(errs) > 0 {
			s.Error("Failed nameserver verification: ", errs)
		}
	}); len(errs) > 0 {
		s.Fatal("Failed to block DNS to nameserver: ", errs)
	}
}

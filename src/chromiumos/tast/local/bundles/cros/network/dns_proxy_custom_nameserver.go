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
		Func:         DNSProxyCustomNameserver,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Verify dns-proxy is elided when a custom nameserver is used",
		Contacts:     []string{"jasongustaman@google.com", "garrick@google.com", "cros-networking@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome", "vm_host", "arc", "dlc"},
		Data:         []string{crostini.GetContainerMetadataArtifact("buster", false), crostini.GetContainerRootfsArtifact("buster", false)},
		Pre:          multivm.ArcCrostiniStartedWithDNSProxy(),
		HardwareDeps: crostini.CrostiniStable,
		Timeout:      5 * time.Minute,
	})
}

// DNSProxyCustomNameserver verifies that when a plaintext DNS query is made to a specific nameserver,
// explicitly overriding the network configuration, that dns-proxy is elided and the query goes directly to that server.
func DNSProxyCustomNameserver(ctx context.Context, s *testing.State) {
	// Ensure plaintext query.
	pre := s.PreValue().(*multivm.PreData)
	cr := pre.Chrome
	tconn := pre.TestAPIConn
	if err := dns.SetDoHMode(ctx, cr, tconn, dns.DoHOff, dns.GoogleDoHProvider); err != nil {
		s.Fatal("Failed to set DNS-over-HTTPS mode: ", err)
	}

	// Ensure connectivity is available.
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		return testexec.CommandContext(ctx, "/bin/ping", "-c1", "-w1", "8.8.8.8").Run()
	}, &testing.PollOptions{Timeout: time.Second}); err != nil {
		s.Fatal("No connectivity: ", err)
	}

	// By default, host DNS queries work as-is.
	// TODO(b/230686377, b/232882301) - Add Crostini and ARC
	tc := []dns.ProxyTestCase{
		{Client: dns.System},
		{Client: dns.User},
		{Client: dns.Chrome},
	}
	if errs := dns.TestQueryDNSProxy(ctx, tc, nil, nil, dns.NewQueryOptions()); len(errs) > 0 {
		s.Error("Failed initial DNS check: ", errs)
	}

	// Confirm that host queries to a different nameserver also work.
	opts := dns.NewQueryOptions()
	opts.Nameserver = "1.1.1.1"
	if errs := dns.TestQueryDNSProxy(ctx, tc, nil, nil, opts); len(errs) > 0 {
		s.Error("Failed nameserver confirmation check: ", errs)
	}

	// Now block plaintext DNS traffic to that server and confirm failure to verify.
	nss, err := dns.ProxyNamespaces(ctx)
	if err != nil {
		s.Fatal("Failed to get DNS proxy's network namespaces: ", err)
	}
	physIfs, err := network.PhysicalInterfaces(ctx)
	if err != nil {
		s.Fatal("Failed to get physical interfaces: ", err)
	}
	if errs := dns.NewPlaintextBlock(nss, physIfs, opts.Nameserver).Run(ctx, func(ctx context.Context) {
		for i := 0; i < len(tc); i++ {
			tc[i].ExpectErr = true
		}
		opts.Domain = dns.RandDomain()
		if errs := dns.TestQueryDNSProxy(ctx, tc, nil, nil, opts); len(errs) > 0 {
			s.Error("Failed nameserver verification: ", errs)
		}
	}); len(errs) > 0 {
		s.Fatal("Failed to block DNS to nameserver: ", errs)
	}
}

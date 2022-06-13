// Copyright 2022 The ChromiumOS Authors.
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

// DNSProxyCustomNameserver Verifies that when a plaintext DNS query is made to a specific nameserver,
// explicitly overriding the network configuration, that dns-proxy is elided and the query goes directly to that server.
func DNSProxyCustomNameserver(ctx context.Context, s *testing.State) {
	ctx, cancel := ctxutil.Shorten(ctx, 30*time.Second)
	defer cancel()

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

	// Verify the system proxy is the current nameserver and name resolution works.
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		return dns.DigMatch(ctx, dns.DigProxyIPRE, true)
	}, &testing.PollOptions{Timeout: 30 * time.Second}); err != nil {
		s.Fatal("Failed to verify system proxy: ", err)
	}

	// Verify that DNS queries targeted at a particular nameserver are not redirected through the proxy.
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		return dns.DigToMatch(ctx, "1.1.1.1", true)
	}, &testing.PollOptions{Timeout: 30 * time.Second}); err != nil {
		s.Fatal("Failed to verify host: ", err)
	}
}

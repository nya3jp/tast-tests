// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package network

import (
	"context"
	"fmt"
	"regexp"
	"time"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/shill"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         DNSProxyCaptivePortal,
		Desc:         "Verify dns-proxy behaves correctly when shill detects a captive portal",
		Contacts:     []string{"jasongustaman@google.com", "garrick@google.com", "cros-networking@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Timeout:      5 * time.Minute,
	})
}

var (
	digProxyIPRE  = regexp.MustCompile(`SERVER: 100.115.92.\d+#53`)
	digGoogleIPRE = regexp.MustCompile(`SERVER: 8.8.\d.\d#53`)
)

func DNSProxyCaptivePortal(ctx context.Context, s *testing.State) {
	// Start Chrome with the dns-proxy feature flags enabled.
	cr, err := chrome.New(ctx, chrome.EnableFeatures("EnableDnsProxy", "DnsProxyEnableDOH"))
	if err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}
	defer cr.Close(ctx)

	// Ensure connectivity is available.
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		return testexec.CommandContext(ctx, "/bin/ping", "-c1", "-w1", "8.8.8.8").Run()
	}, &testing.PollOptions{Timeout: time.Second}); err != nil {
		s.Fatal("Failed to ping 8.8.8.8: ", err)
	}

	// Verify the system proxy is the current nameserver and name resolution works.
	if err := digMatch(ctx, digProxyIPRE); err != nil {
		s.Fatal("Failed to verify system proxy: ", err)
	}

	// Shill's captive portal detector works by probing various endpoints over HTTP and HTTPS.
	// So, first block these ports and then tell shill to rerun its portal detector, which should
	// fail, which should trigger shill to change the connection state from 'online' which in turn
	// should cause dns-proxy to disengage itself and tell shill to use the network's name servers.
	mgr, err := shill.NewManager(ctx)
	if err != nil {
		s.Fatal("Failed to create shill manager: ", err)
	}
	if err := blockPortalDetector(ctx); err != nil {
		unblockPortalDetector(ctx)
		s.Fatal("Failed to add rules to block portal detector: ", err)
	}
	if err := mgr.RecheckPortal(ctx); err != nil {
		unblockPortalDetector(ctx)
		s.Fatal("Failed to invoke RecheckPortal on shill")
	}

	// Verify the system proxy is not the current nameserver and name resolution works.
	// Give shill and dns-proxy sufficient time to respond to the loss of connectivity.
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		return digMatch(ctx, digGoogleIPRE)
	}, &testing.PollOptions{Timeout: 30 * time.Second}); err != nil {
		unblockPortalDetector(ctx)
		s.Fatal("Failed to verify switch over to Google DNS: ", err)
	}

	// Unblock the HTTP/S ports which should cause shill's portal detector to succeed, which will
	// move the connection state back to 'online' and subsequently reconnect dns-proxy.
	if err := unblockPortalDetector(ctx); err != nil {
		s.Fatal("Failed to remove rules to unblock portal detector: ", err)
	}

	// Verify the system proxy is the current nameserver and name resolution works.
	// Give shill and dns-proxy sufficient time to respond to regaining connectivity.
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		return digMatch(ctx, digProxyIPRE)
	}, &testing.PollOptions{Timeout: 30 * time.Second}); err != nil {
		s.Fatal("Failed to verify switchover to system proxy: ", err)
	}
}

// Runs dig to check name resolution works and verifies the expected server was used.
func digMatch(ctx context.Context, re *regexp.Regexp) error {
	out, err := testexec.CommandContext(ctx, "dig", "google.com").Output()
	if err != nil {
		return errors.Wrap(err, "dig failed")
	}
	if !re.MatchString(string(out)) {
		return errors.New("dig used unexpected nameserver")
	}
	return nil
}

func blockPortalDetector(ctx context.Context) error {
	for _, p := range []string{"80", "443"} {
		if err := testexec.CommandContext(ctx, "iptables", "-I", "OUTPUT", "-p", "tcp", "--dport", p, "-j", "DROP", "-w").Run(testexec.DumpLogOnError); err != nil {
			return err
		}
	}
	return nil
}
func unblockPortalDetector(ctx context.Context) error {
	for _, p := range []string{"80", "443"} {
		if err := testexec.CommandContext(ctx, "iptables", "-D", "OUTPUT", "-p", "tcp", "--dport", p, "-j", "DROP", "-w").Run(testexec.DumpLogOnError); err != nil {
			return err
		}
	}
	return nil
}

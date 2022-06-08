// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package network

import (
	"context"
	"strings"
	"time"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/bundles/cros/network/dns"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/network"
	"chromiumos/tast/local/shill"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         DNSProxyCaptivePortalRelog,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Verify dns-proxy behaves correctly when shill detects a captive portal and a re-login is done",
		Contacts:     []string{"jasongustaman@google.com", "garrick@google.com", "cros-networking@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome", "arc"},
		Timeout:      5 * time.Minute,
	})
}

const arcDNSProxyChain = "redirect_arc_dns"

func DNSProxyCaptivePortalRelog(ctx context.Context, s *testing.State) {
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 30*time.Second)
	defer cancel()

	// Start Chrome with the dns-proxy feature flags enabled.
	cr, err := chrome.New(ctx, chrome.ARCEnabled(), chrome.UnRestrictARCCPU(),
		chrome.EnableFeatures("EnableDnsProxy", "DnsProxyEnableDOH"))
	if err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}
	defer cr.Close(cleanupCtx)

	// Start ARC.
	a, err := arc.New(ctx, s.OutDir())
	if err != nil {
		s.Fatal("Failed to start ARC: ", err)
	}
	defer a.Close(cleanupCtx)

	// Ensure connectivity is available.
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		return testexec.CommandContext(ctx, "/bin/ping", "-c1", "-w1", "8.8.8.8").Run()
	}, &testing.PollOptions{Timeout: time.Second}); err != nil {
		s.Fatal("Failed to ping 8.8.8.8: ", err)
	}

	// Verify the system proxy is the current nameserver and name resolution works.
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		return dns.DigMatch(ctx, dns.DigProxyIPRE, true)
	}, &testing.PollOptions{Timeout: 30 * time.Second}); err != nil {
		s.Fatal("Failed to verify system proxy: ", err)
	}

	// Currently, there is no simple way to do plain-text DNS from ARC.
	// Check ARC state by checking its DNS redirection rules instead.
	if exist, err := iptablesRulesExist(ctx, arcDNSProxyChain); err != nil {
		s.Error("Failed to check if iptables rules exist: ", err)
	} else if !exist {
		s.Error("ARC DNS proxy chain does not exist")
	}

	// Shill's captive portal detector works by probing various endpoints over HTTP and HTTPS.
	// So, first block these ports and then tell shill to rerun its portal detector, which should
	// fail, which should trigger shill to change the connection state from 'online' which in turn
	// should cause dns-proxy to disengage itself and tell shill to use the network's name servers.
	mgr, err := shill.NewManager(ctx)
	if err != nil {
		s.Fatal("Failed to create shill manager: ", err)
	}
	unblock := true
	defer func() {
		if unblock {
			network.UnblockShillPortalDetector(cleanupCtx)
		}
	}()
	if err := network.BlockShillPortalDetector(ctx); err != nil {
		s.Fatal("Failed to add rules to block portal detector: ", err)
	}
	if err := mgr.RecheckPortal(ctx); err != nil {
		s.Fatal("Failed to invoke RecheckPortal on shill")
	}

	// Verify the system proxy is not the current nameserver and name resolution works.
	// Give shill and dns-proxy sufficient time to respond to the loss of connectivity.
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		return dns.DigMatch(ctx, dns.DigProxyIPRE, false)
	}, &testing.PollOptions{Timeout: 30 * time.Second}); err != nil {
		s.Fatal("Failed to verify switch over to Google DNS: ", err)
	}

	// Verify that when the state is in captive portal, ARC redirection rules do not exist.
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		if exist, err := iptablesRulesExist(ctx, arcDNSProxyChain); err != nil {
			return err
		} else if exist {
			return errors.New("ARC DNS proxy chain exist in captive portal state")
		}
		return nil
	}, &testing.PollOptions{Timeout: 30 * time.Second}); err != nil {
		s.Error("Failed to get DNS proxy in the correct state: ", err)
	}

	// Re-start Chrome, emulate a logout and login.
	cr, err = chrome.New(ctx, chrome.ARCEnabled(), chrome.UnRestrictARCCPU(),
		chrome.EnableFeatures("EnableDnsProxy", "DnsProxyEnableDOH"))
	if err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}
	defer cr.Close(cleanupCtx)

	// Start ARC.
	a, err = arc.New(ctx, s.OutDir())
	if err != nil {
		s.Fatal("Failed to start ARC: ", err)
	}
	defer a.Close(cleanupCtx)

	// Verify the system proxy is still not the current nameserver and name resolution works.
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		return dns.DigMatch(ctx, dns.DigProxyIPRE, false)
	}, &testing.PollOptions{Timeout: 30 * time.Second}); err != nil {
		s.Fatal("Failed to verify switch over to Google DNS: ", err)
	}

	// Verify that ARC redirection rules still do not exist.
	if exist, err := iptablesRulesExist(ctx, arcDNSProxyChain); err != nil {
		s.Error("Failed to check if iptables rules exist: ", err)
	} else if exist {
		s.Error("ARC DNS proxy chain exist in captive portal state")
	}

	// Unblock the HTTP/S ports which should cause shill's portal detector to succeed, which will
	// move the connection state back to 'online' and subsequently reconnect dns-proxy.
	unblock = false
	if err := network.UnblockShillPortalDetector(ctx); err != nil {
		s.Fatal("Failed to remove rules to unblock portal detector: ", err)
	}

	// Verify the system proxy is the current nameserver and name resolution works.
	// Give shill and dns-proxy sufficient time to respond to regaining connectivity.
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		return dns.DigMatch(ctx, dns.DigProxyIPRE, true)
	}, &testing.PollOptions{Timeout: 30 * time.Second}); err != nil {
		s.Fatal("Failed to verify switchover to system proxy: ", err)
	}

	// When the state is back to online, verify that ARC redirection rules are re-added.
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		if exist, err := iptablesRulesExist(ctx, arcDNSProxyChain); err != nil {
			return err
		} else if !exist {
			return errors.New("ARC DNS proxy chain does not exist")
		}
		return nil
	}, &testing.PollOptions{Timeout: 30 * time.Second}); err != nil {
		s.Error("Failed to get DNS proxy in the correct state: ", err)
	}
}

// iptablesRulesExist check if there is at least one rule in the chain.
func iptablesRulesExist(ctx context.Context, chain string) (bool, error) {
	out, err := testexec.CommandContext(ctx, "iptables", "-t", "nat", "-S", chain).Output()
	if err != nil {
		return false, err
	}
	return strings.Count(string(out), "\n") > 1, nil
}

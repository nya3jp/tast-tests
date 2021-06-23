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
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/bundles/cros/network/dns"
	"chromiumos/tast/local/bundles/cros/network/vpn"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/testing"
)

type dnsProxyOverVPNTestParams struct {
	mode dns.DoHMode
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         DNSProxyOverVPN,
		Desc:         "Ensure that DNS proxies are working correctly",
		Contacts:     []string{"jasongustaman@google.com", "garrick@google.com", "cros-networking@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome", "arc"},
		Fixture:      "shillReset",
		Params: []testing.Param{{
			Name: "doh_off",
			Val: dnsProxyOverVPNTestParams{
				mode: dns.DoHOff,
			},
		}, {
			Name: "doh_automatic",
			Val: dnsProxyOverVPNTestParams{
				mode: dns.DoHAutomatic,
			},
		}, {
			Name: "doh_always_on",
			Val: dnsProxyOverVPNTestParams{
				mode: dns.DoHAlwaysOn,
			},
		}},
	})
}

// VPN interface name prefix for the test.
const vpnIfnamePrefix = "ppp"

// DNSProxyOverVPN tests DNS functionality with DNS proxy active.
// There are 2 parts to this test:
// 1. Ensuring that DNS queries over VPN are successful.
// 2. Ensuring that DNS queries are routed properly through VPN by blocking VPN DNS ports, expecting the queries to fail.
func DNSProxyOverVPN(ctx context.Context, s *testing.State) {
	const (
		// Randomly generated domains to be resolved. Different domains are used to avoid caching.
		domainDefault          = "c30c8b722af2d577.com"
		domainVPNBlocked       = "da39a3ee5e6b4b0d.com"
		domainSecureDNSBlocked = "e8af12bffc0ae7a1.com"

		// DNS-over-HTTPS provider used for the test.
		dohProvider = "https://dns.google/dns-query"
	)

	// If the main body of the test times out, we still want to reserve a few
	// seconds to allow for our cleanup code to run.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(cleanupCtx, 3*time.Second)
	defer cancel()

	cr, err := chrome.New(ctx, chrome.ARCEnabled(), chrome.EnableFeatures("EnableDnsProxy", "DnsProxyEnableDOH"))
	if err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}
	defer cr.Close(cleanupCtx)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to get test API connection: ", err)
	}

	// Start ARC.
	a, err := arc.New(ctx, s.OutDir())
	if err != nil {
		s.Fatal("Failed to start ARC: ", err)
	}
	defer a.Close(cleanupCtx)

	// Toggle plain-text DNS or secureDNS depending on test parameter.
	params := s.Param().(dnsProxyOverVPNTestParams)
	if err := dns.SetDoHMode(ctx, cr, tconn, params.mode, dohProvider); err != nil {
		s.Fatal(ctx, "Failed to set DNS-over-HTTPS mode: ", err)
	}

	// Connect to VPN.
	config := vpn.Config{
		Type:     vpn.TypeL2TPIPsec,
		AuthType: vpn.AuthTypePSK,
	}
	conn, err := vpn.NewConnection(ctx, config)
	if err != nil {
		s.Fatal("Failed to create connection objec: ", err)
	}
	defer func() {
		if err := conn.Cleanup(cleanupCtx); err != nil {
			s.Error("Failed to clean up connection: ", err)
		}
	}()
	if _, err = conn.Start(ctx); err != nil {
		s.Fatal("Failed to connect to VPN server: ", err)
	}

	ns, err := vpnNamespace(ctx)
	if err != nil {
		s.Fatal("Failed to get DNS proxy's network namespaces")
	}

	type testCase struct {
		c         dns.Client
		expectErr bool
	}

	// By default, DNS query should work over VPN.
	var defaultTC = []testCase{
		testCase{c: dns.System, expectErr: false},
		testCase{c: dns.User, expectErr: false},
		testCase{c: dns.Chrome, expectErr: false},
		testCase{c: dns.ARC, expectErr: false},
	}
	for _, tc := range defaultTC {
		err = dns.QueryDNS(ctx, tc.c, a, domainDefault)
		if err != nil && !tc.expectErr {
			s.Errorf("Failed DNS query check for %s: %v", dns.GetClientString(tc.c), err)
		}
		if err == nil && tc.expectErr {
			s.Errorf("Successful DNS query for %s, but expected failure", dns.GetClientString(tc.c))
		}
	}

	// Block DNS queries over VPN through iptables.
	unblock, err := blockDNSOverVPN(ctx, ns)
	if err != nil {
		s.Fatal(ctx, "Failed to block DNS over VPN: ", err)
	}

	// DNS queries that should be routed through VPN should fail if DNS queries on the VPN server are blocked.
	// System traffic bypass VPN, this is to allow things such as updates and crash reports to always work.
	// On the other hand, other traffic (Chrome, ARC, etc.) should always go through VPN.
	vpnBlockedTC := []testCase{
		testCase{c: dns.System, expectErr: false},
		testCase{c: dns.User, expectErr: true},
		testCase{c: dns.Chrome, expectErr: true}}
	for _, tc := range vpnBlockedTC {
		err = dns.QueryDNS(ctx, tc.c, a, domainVPNBlocked)
		if err != nil && !tc.expectErr {
			s.Errorf("Failed DNS query check for %s: %v", dns.GetClientString(tc.c), err)
		}
		if err == nil && tc.expectErr {
			s.Errorf("Successful DNS query for %s, but expected failure", dns.GetClientString(tc.c))
		}
	}
	unblock()

	if params.mode == dns.DoHOff || params.mode == dns.DoHAutomatic {
		return
	}

	// Block DoH queries over VPN to verify that when a VPN is on, DoH is disabled and DNS will work.
	// When a VPN is active, the default proxy and ARC proxy will disable secure DNS in order to have consistent behavior on different VPN types.
	if err := blockSecureDNSOverVPN(ctx, ns); err != nil {
		s.Fatal(ctx, "Failed to block secure DNS over VPN: ", err)
	}
	secureDNSBlockedTC := []testCase{
		testCase{c: dns.System, expectErr: false},
		testCase{c: dns.User, expectErr: false},
		testCase{c: dns.Chrome, expectErr: false}}

	for _, tc := range secureDNSBlockedTC {
		err = dns.QueryDNS(ctx, tc.c, a, domainSecureDNSBlocked)
		if err != nil && !tc.expectErr {
			s.Errorf("Failed DNS query check for %s: %v", dns.GetClientString(tc.c), err)
		}
		if err == nil && tc.expectErr {
			s.Errorf("Successful DNS query for %s, but expected failure", dns.GetClientString(tc.c))
		}
	}
}

// vpnNamespace iterates through available network namespaces and return the namespace with the VPN server.
// VPN namespace is identified by checking if the namespace contains a VPN interface.
func vpnNamespace(ctx context.Context) (string, error) {
	out, err := testexec.CommandContext(ctx, "ip", "netns", "list").Output()
	if err != nil {
		return "", err
	}

	for _, o := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		ns := strings.Fields(o)[0]
		ifnames, err := testexec.CommandContext(ctx, "ip", "netns", "exec", ns, "ls", "/sys/class/net").Output()
		if err != nil {
			return "", nil
		}
		for _, ifname := range strings.Fields(string(ifnames)) {
			if strings.HasPrefix(ifname, vpnIfnamePrefix) {
				return ns, nil
			}
		}
	}
	return "", nil
}

// blockDNSOverVPN blocks DNS outbound packets that go through VPN.
// Blocking is done by dropping outbound TCP packets with port 443 (HTTPS packets) and TCP and UDP packets with port 53 (plaintext DNS packets).
func blockDNSOverVPN(ctx context.Context, ns string) (func(), error) {
	doUnblock := func() {
		for _, cmd := range []string{"iptables", "ip6tables"} {
			testexec.CommandContext(ctx, "ip", "netns", "exec", ns, cmd, "-D", "FORWARD", "-p", "udp", "--dport", "53", "-j", "DROP", "-w").Run()
			testexec.CommandContext(ctx, "ip", "netns", "exec", ns, cmd, "-D", "FORWARD", "-p", "tcp", "--dport", "53", "-j", "DROP", "-w").Run()
			testexec.CommandContext(ctx, "ip", "netns", "exec", ns, cmd, "-D", "FORWARD", "-p", "tcp", "--dport", "443", "-j", "DROP", "-w").Run()
		}
	}

	for _, cmd := range []string{"iptables", "ip6tables"} {
		if err := testexec.CommandContext(ctx, "ip", "netns", "exec", ns, cmd, "-I", "FORWARD", "-p", "udp", "--dport", "53", "-j", "DROP", "-w").Run(); err != nil {
			return doUnblock, err
		}
		if err := testexec.CommandContext(ctx, "ip", "netns", "exec", ns, cmd, "-I", "FORWARD", "-p", "tcp", "--dport", "53", "-j", "DROP", "-w").Run(); err != nil {
			return doUnblock, err
		}
		if err := testexec.CommandContext(ctx, "ip", "netns", "exec", ns, cmd, "-I", "FORWARD", "-p", "tcp", "--dport", "443", "-j", "DROP", "-w").Run(); err != nil {
			return doUnblock, err
		}
	}
	return doUnblock, nil
}

// blockSecureDNSOverVPN blocks secure DNS outbound packets that go through packets that go through VPN.
// Blocking is done by dropping outbound TCP packets with port 443 (HTTPS packets).
func blockSecureDNSOverVPN(ctx context.Context, ns string) error {
	for _, cmd := range []string{"iptables", "ip6tables"} {
		if err := testexec.CommandContext(ctx, "ip", "netns", "exec", ns, cmd, "-I", "FORWARD", "-p", "tcp", "--dport", "443", "-j", "DROP", "-w").Run(); err != nil {
			return err
		}
	}
	return nil
}

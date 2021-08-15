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
	"chromiumos/tast/local/bundles/cros/network/dns"
	"chromiumos/tast/local/bundles/cros/network/vpn"
	"chromiumos/tast/local/crostini"
	"chromiumos/tast/local/multivm"
	"chromiumos/tast/testing"
)

type dnsProxyOverVPNTestParams struct {
	mode dns.DoHMode
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         DNSProxyOverVPN,
		Desc:         "Ensure that DNS proxies are working correctly over VPN",
		Contacts:     []string{"jasongustaman@google.com", "garrick@google.com", "cros-networking@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome", "vm_host", "arc", "dlc"},
		Data:         []string{crostini.GetContainerMetadataArtifact("buster", false), crostini.GetContainerRootfsArtifact("buster", false)},
		Pre:          multivm.ArcCrostiniStartedWithDNSProxy(),
		HardwareDeps: crostini.CrostiniStable,
		Timeout:      7 * time.Minute,
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
// There are 3 parts to this test:
// 1. Ensuring that DNS queries over VPN are successful.
// 2. Ensuring that DNS queries (except from system) are routed properly through VPN by blocking VPN DNS ports, expecting the queries to fail.
// 3. Ensuring that DNS queries (except from system) are not using DNS-over-HTTPS when a VPN is on.
func DNSProxyOverVPN(ctx context.Context, s *testing.State) {
	const (
		// Randomly generated domains to be resolved. Different domains are used to avoid caching.
		domainDefaultDoHOff          = "c30c8b722af2d577.com"
		domainDefaultDoHAutomatic    = "a14bd912acfe9d01.com"
		domainDefaultDoHAlwaysOn     = "ff1deb9cc2d1a03d.com"
		domainVPNBlockedDoHOff       = "d3251abbef91dcc1.com"
		domainVPNBlockedDoHAutomatic = "cac2dcdfa1d4e290.com"
		domainVPNBlockedDoHAlwaysOn  = "eab98dc5180aafda.com"
		domainSecureDNSBlocked       = "e8af12bffc0ae7a1.com"
	)

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

	// Install dig in container.
	if err := dns.InstallDigInContainer(ctx, cont); err != nil {
		s.Fatal("Failed to install dig in container: ", err)
	}

	// Toggle plain-text DNS or secureDNS depending on test parameter.
	params := s.Param().(dnsProxyOverVPNTestParams)
	if err := dns.SetDoHMode(ctx, cr, tconn, params.mode, dns.GoogleDoHProvider); err != nil {
		s.Fatal("Failed to set DNS-over-HTTPS mode: ", err)
	}

	var domainDefault, domainVPNBlocked string
	switch params.mode {
	case dns.DoHOff:
		domainDefault = domainDefaultDoHOff
		domainVPNBlocked = domainVPNBlockedDoHOff
	case dns.DoHAutomatic:
		domainDefault = domainDefaultDoHAutomatic
		domainVPNBlocked = domainVPNBlockedDoHAutomatic
	case dns.DoHAlwaysOn:
		domainDefault = domainDefaultDoHAlwaysOn
		domainVPNBlocked = domainVPNBlockedDoHAlwaysOn
	}

	// Connect to VPN.
	config := vpn.Config{
		Type:     vpn.TypeL2TPIPsec,
		AuthType: vpn.AuthTypePSK,
	}
	conn, err := vpn.NewConnection(ctx, config)
	if err != nil {
		s.Fatal("Failed to create connection object: ", err)
	}
	defer func() {
		if err := conn.Cleanup(cleanupCtx); err != nil {
			s.Error("Failed to clean up connection: ", err)
		}
	}()
	if _, err = conn.Start(ctx); err != nil {
		s.Fatal("Failed to connect to VPN server: ", err)
	}
	if err := conn.Server.SetupInternetAccess(ctx); err != nil {
		s.Fatal("Failed to setup internet connectivity for VPN: ", err)
	}

	// By default, DNS query should work over VPN.
	var defaultTC = []dns.ProxyTestCase{
		dns.ProxyTestCase{Client: dns.System},
		dns.ProxyTestCase{Client: dns.User},
		dns.ProxyTestCase{Client: dns.Chrome},
		dns.ProxyTestCase{Client: dns.ARC},
	}
	if errs := dns.TestQueryDNSProxy(ctx, defaultTC, a, cont, domainDefault); len(errs) != 0 {
		for _, err := range errs {
			s.Error("Failed DNS query check: ", err)
		}
	}

	ns, err := vpnNamespace(ctx)
	if err != nil {
		s.Fatal("Failed to get VPN's network namespaces")
	}

	// Block DNS queries over VPN through iptables. The blocked state will be torn down when the test end alongside the VPN connection.
	if errs := modifyDNSOverVPNBlockRule(ctx, "-I" /*op*/, ns); len(errs) != 0 {
		s.Fatal("Failed to block DNS over VPN: ", errs)
	}

	// DNS queries that should be routed through VPN should fail if DNS queries on the VPN server are blocked.
	// System traffic bypass VPN, this is to allow things such as updates and crash reports to always work.
	// On the other hand, other traffic (Chrome, ARC, etc.) should always go through VPN.
	vpnBlockedTC := []dns.ProxyTestCase{
		dns.ProxyTestCase{Client: dns.System},
		dns.ProxyTestCase{Client: dns.User, ExpectErr: true},
		dns.ProxyTestCase{Client: dns.Chrome, ExpectErr: true},
		dns.ProxyTestCase{Client: dns.Crostini, ExpectErr: true},
		dns.ProxyTestCase{Client: dns.ARC}}
	if errs := dns.TestQueryDNSProxy(ctx, vpnBlockedTC, a, cont, domainVPNBlocked); len(errs) != 0 {
		for _, err := range errs {
			s.Error("Failed DNS query check: ", err)
		}
	}
	if errs := modifyDNSOverVPNBlockRule(ctx, "-D" /*op*/, ns); len(errs) != 0 {
		s.Fatal("Failed to unblock DNS over VPN: ", errs)
	}

	if params.mode == dns.DoHOff || params.mode == dns.DoHAutomatic {
		return
	}

	// Block DoH queries over VPN to verify that when a VPN is on, DoH is disabled and DNS will work.
	// When a VPN is active, the default proxy and ARC proxy will disable secure DNS in order to have consistent behavior on different VPN types.
	// The blocked state will be torn down when the test end alongside the VPN connection.
	if errs := modifyDoHOverVPNBlockRule(ctx, "-I" /*op*/, ns); len(errs) != 0 {
		s.Fatal("Failed to block secure DNS over VPN: ", errs)
	}
	secureDNSBlockedTC := []dns.ProxyTestCase{
		dns.ProxyTestCase{Client: dns.System},
		dns.ProxyTestCase{Client: dns.User},
		dns.ProxyTestCase{Client: dns.Chrome},
		dns.ProxyTestCase{Client: dns.Crostini},
		dns.ProxyTestCase{Client: dns.ARC}}
	if errs := dns.TestQueryDNSProxy(ctx, secureDNSBlockedTC, a, cont, domainSecureDNSBlocked); len(errs) != 0 {
		for _, err := range errs {
			s.Error("Failed DNS query check: ", err)
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

// modifyDNSOverVPNBlockRule blocks DNS outbound packets that go through VPN.
// Blocking is done by dropping outbound TCP packets with port 443 (HTTPS packets) and TCP and UDP packets with port 53 (plaintext DNS packets).
// The caller of this function is required to tear down the updated state. The rules created is torn down alongside VPN connection.
func modifyDNSOverVPNBlockRule(ctx context.Context, op, ns string) []error {
	var e []error
	for _, cmd := range []string{"iptables", "ip6tables"} {
		if err := testexec.CommandContext(ctx, "ip", "netns", "exec", ns, cmd, op, "FORWARD", "-p", "udp", "--dport", "53", "-j", "DROP", "-w").Run(testexec.DumpLogOnError); err != nil {
			e = append(e, err)
		}
		if err := testexec.CommandContext(ctx, "ip", "netns", "exec", ns, cmd, op, "FORWARD", "-p", "tcp", "--dport", "53", "-j", "DROP", "-w").Run(testexec.DumpLogOnError); err != nil {
			e = append(e, err)
		}
		if err := testexec.CommandContext(ctx, "ip", "netns", "exec", ns, cmd, op, "FORWARD", "-p", "tcp", "--dport", "443", "-j", "DROP", "-w").Run(testexec.DumpLogOnError); err != nil {
			e = append(e, err)
		}
	}
	return e
}

// modifyDoHOverVPNBlockRule blocks secure DNS outbound packets that go through packets that go through VPN.
// Blocking is done by dropping outbound TCP packets with port 443 (HTTPS packets).
// The caller of this function is required to tear down the updated state. The rules created is torn down alongside VPN connection.
func modifyDoHOverVPNBlockRule(ctx context.Context, op, ns string) []error {
	var e []error
	for _, cmd := range []string{"iptables", "ip6tables"} {
		if err := testexec.CommandContext(ctx, "ip", "netns", "exec", ns, cmd, op, "FORWARD", "-p", "tcp", "--dport", "443", "-j", "DROP", "-w").Run(testexec.DumpLogOnError); err != nil {
			e = append(e, err)
		}
	}
	return e
}

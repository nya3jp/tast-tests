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
	"chromiumos/tast/local/bundles/cros/network/dns"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/testing"
)

type dnsProxyTestParams struct {
	mode dns.DoHMode
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         DNSProxy,
		Desc:         "Ensure that DNS proxies are working correctly",
		Contacts:     []string{"jasongustaman@google.com", "garrick@google.com", "cros-networking@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
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
	const (
		// Randomly generated domains to be resolved. Different domains are used to avoid caching.
		domainDefault    = "a2ffec2cb85be5e7.com"
		domainDNSBlocked = "da39a3ee5e6b4b0d.com"
	)

	// If the main body of the test times out, we still want to reserve a few
	// seconds to allow for our cleanup code to run.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(cleanupCtx, 3*time.Second)
	defer cancel()

	cr, err := chrome.New(ctx, chrome.EnableFeatures("EnableDnsProxy", "DnsProxyEnableDOH"))
	if err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}
	defer cr.Close(cleanupCtx)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to get test API connection: ", err)
	}

	// Toggle plain-text DNS or secureDNS depending on test parameter.
	params := s.Param().(dnsProxyTestParams)
	if err := dns.SetDoHMode(ctx, cr, tconn, params.mode, dns.GoogleDoHProvider); err != nil {
		s.Fatal("Failed to set DNS-over-HTTPS mode: ", err)
	}

	// By default, DNS query should work.
	var defaultTC = []dns.ProxyTestCase{
		dns.ProxyTestCase{Client: dns.System},
		dns.ProxyTestCase{Client: dns.User},
		dns.ProxyTestCase{Client: dns.Chrome},
	}
	if errs := dns.TestQueryDNSProxy(ctx, defaultTC, domainDefault); len(errs) != 0 {
		for _, err := range errs {
			s.Error("Failed DNS query check: ", err)
		}
	}

	nss, err := dnsProxyNamespaces(ctx)
	if err != nil {
		s.Fatal("Failed to get DNS proxy's network namespaces: ", err)
	}

	physIfs, err := physicalInterfaces(ctx)
	if err != nil {
		s.Fatal("Failed to get phyiscal interfaces: ", err)
	}

	// Block plain-text or secure DNS through iptables.
	var unblock func()
	var dnsBlockedTC []dns.ProxyTestCase
	var errs []error
	switch params.mode {
	case dns.DoHOff:
		errs = modifyPlaintextDNSDropRule(ctx, "-I" /*op*/, nss, physIfs)
		unblock = func() {
			modifyPlaintextDNSDropRule(ctx, "-D" /*op*/, nss, physIfs)
		}
		dnsBlockedTC = []dns.ProxyTestCase{
			dns.ProxyTestCase{Client: dns.System, ExpectErr: true},
			dns.ProxyTestCase{Client: dns.User, ExpectErr: true},
			dns.ProxyTestCase{Client: dns.Chrome, ExpectErr: true}}
	case dns.DoHAutomatic:
		return
	case dns.DoHAlwaysOn:
		errs = modifyDoHDropRule(ctx, "-I" /*op*/, nss, physIfs)
		unblock = func() {
			modifyDoHDropRule(ctx, "-D" /*op*/, nss, physIfs)
		}
		dnsBlockedTC = []dns.ProxyTestCase{
			dns.ProxyTestCase{Client: dns.System, ExpectErr: true},
			dns.ProxyTestCase{Client: dns.User, ExpectErr: true}}
	}
	defer unblock()
	if len(errs) != 0 {
		s.Fatal("Failed to block DNS: ", errs)
	}

	// DNS queries should fail if corresponding DNS packets (plain-text or secure) are dropped.
	if errs := dns.TestQueryDNSProxy(ctx, dnsBlockedTC, domainDNSBlocked); len(errs) != 0 {
		for _, err := range errs {
			s.Error("Failed DNS query check: ", err)
		}
	}
}

// dnsProxyNamespaces iterates through available network namespaces and return the namespaces with DNS proxy.
// DNS proxy namespaces are identified by checking if the namespace contain a listening process named dnsproxyd.
func dnsProxyNamespaces(ctx context.Context) ([]string, error) {
	out, err := testexec.CommandContext(ctx, "ip", "netns", "list").Output()
	if err != nil {
		return nil, err
	}

	var nss []string
	for _, o := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		ns := strings.Fields(o)[0]
		ss, err := testexec.CommandContext(ctx, "ip", "netns", "exec", ns, "ss", "-lptun").Output()
		if err != nil {
			return nil, err
		}
		if strings.Contains(string(ss), "dnsproxyd") {
			nss = append(nss, ns)
		}
	}
	return nss, nil
}

func physicalInterfaces(ctx context.Context) ([]string, error) {
	out, err := testexec.CommandContext(ctx, "/usr/bin/find", "/sys/class/net", "-type", "l", "-not", "-lname", "*virtual*", "-printf", "%f\n").Output()
	if err != nil {
		return nil, errors.Wrap(err, "failed to get physical interfaces")
	}
	return strings.Split(strings.TrimSpace(string(out)), "\n"), nil
}

// modifyPlaintextDNSDropRule blocks plaintext DNS outbound packets that go through the physical interfaces or DNS proxy namespace.
// Blocking is done by dropping outbound UDP and TCP packets with port 53.
func modifyPlaintextDNSDropRule(ctx context.Context, op string, nss, physIfs []string) []error {
	var errs []error
	for _, cmd := range []string{"iptables", "ip6tables"} {
		for _, ns := range nss {
			if err := testexec.CommandContext(ctx, "ip", "netns", "exec", ns, cmd, op, "OUTPUT", "-p", "udp", "--dport", "53", "-j", "DROP", "-w").Run(testexec.DumpLogOnError); err != nil {
				errs = append(errs, errors.Wrapf(err, "failed to modify UDP plaintext DNS block rule on %s", ns))
			}
			if err := testexec.CommandContext(ctx, "ip", "netns", "exec", ns, cmd, op, "OUTPUT", "-p", "tcp", "--dport", "53", "-j", "DROP", "-w").Run(testexec.DumpLogOnError); err != nil {
				errs = append(errs, errors.Wrapf(err, "failed to modify TCP plaintext DNS block rule on %s", ns))
			}
		}
		for _, ifname := range physIfs {
			if err := testexec.CommandContext(ctx, cmd, op, "OUTPUT", "-p", "udp", "--dport", "53", "-o", ifname, "-j", "DROP", "-w").Run(testexec.DumpLogOnError); err != nil {
				errs = append(errs, errors.Wrapf(err, "failed to modify UDP plaintext DNS block rule for %s", ifname))
			}
			if err := testexec.CommandContext(ctx, cmd, op, "OUTPUT", "-p", "tcp", "--dport", "53", "-o", ifname, "-j", "DROP", "-w").Run(testexec.DumpLogOnError); err != nil {
				errs = append(errs, errors.Wrapf(err, "failed to modify TCP plaintext DNS block rule for %s", ifname))
			}
		}
		if err := testexec.CommandContext(ctx, cmd, op, "OUTPUT", "-p", "udp", "--dport", "53", "-m", "owner", "--uid-owner", "chronos", "-j", "DROP", "-w").Run(testexec.DumpLogOnError); err != nil {
			errs = append(errs, errors.Wrap(err, "failed to modify UDP plaintext DNS block rule for Chrome"))
		}
		if err := testexec.CommandContext(ctx, cmd, op, "OUTPUT", "-p", "tcp", "--dport", "53", "-m", "owner", "--uid-owner", "chronos", "-j", "DROP", "-w").Run(testexec.DumpLogOnError); err != nil {
			errs = append(errs, errors.Wrap(err, "failed to modify TCP plaintext DNS block rule for Chrome"))
		}
	}
	return errs
}

// modifyDoHDropRule blocks secure DNS outbound packets that go through the physical interfaces or DNS proxy namespace.
// Blocking is done by dropping outbound TCP packets with port 443 (HTTPS packets).
func modifyDoHDropRule(ctx context.Context, op string, nss, physIfs []string) []error {
	var errs []error
	for _, cmd := range []string{"iptables", "ip6tables"} {
		for _, ns := range nss {
			if err := testexec.CommandContext(ctx, "ip", "netns", "exec", ns, cmd, op, "OUTPUT", "-p", "tcp", "--dport", "443", "-j", "DROP", "-w").Run(testexec.DumpLogOnError); err != nil {
				errs = append(errs, errors.Wrapf(err, "failed to modify secure DNS block rule on %s", ns))
			}
		}
		for _, ifname := range physIfs {
			if err := testexec.CommandContext(ctx, cmd, op, "OUTPUT", "-p", "tcp", "--dport", "443", "-o", ifname, "-j", "DROP", "-w").Run(testexec.DumpLogOnError); err != nil {
				errs = append(errs, errors.Wrapf(err, "failed to modify secure DNS block rule for %s", ifname))
			}
		}
		if err := testexec.CommandContext(ctx, cmd, op, "OUTPUT", "-p", "tcp", "--dport", "443", "-m", "owner", "--uid-owner", "chronos", "-j", "DROP", "-w").Run(testexec.DumpLogOnError); err != nil {
			errs = append(errs, errors.Wrap(err, "failed to modify secure DNS block rule for Chrome"))
		}
	}
	return errs
}

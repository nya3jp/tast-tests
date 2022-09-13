// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package network

import (
	"bytes"
	"context"
	"time"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/network/dns"
	"chromiumos/tast/local/bundles/cros/network/vpn"
	"chromiumos/tast/local/crostini"
	"chromiumos/tast/local/multivm"
	"chromiumos/tast/local/network/virtualnet/certs"
	"chromiumos/tast/local/network/virtualnet/env"
	"chromiumos/tast/local/network/virtualnet/subnet"
	"chromiumos/tast/testing"
)

type dnsProxyOverVPNTestParams struct {
	mode dns.DoHMode
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         DNSProxyOverVPN,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Ensure that DNS proxies are working correctly over VPN",
		Contacts:     []string{"jasongustaman@google.com", "garrick@google.com", "cros-networking@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome", "vm_host", "arc", "dlc", "no_kernel_upstream"},
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

// DNSProxyOverVPN tests DNS functionality with DNS proxy active.
// There are 3 parts to this test:
// 1. Ensuring that DNS queries over VPN are successful.
// 2. Ensuring that DNS queries (except from system) are routed properly through VPN by blocking VPN DNS ports, expecting the queries to fail.
// 3. Ensuring that DNS queries (except from system) are not using DNS-over-HTTPS when a VPN is on.
func DNSProxyOverVPN(ctx context.Context, s *testing.State) {
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

	// Install dig in container after the DoH mode is set up properly.
	if err := dns.InstallDigInContainer(ctx, cont); err != nil {
		s.Fatal("Failed to install dig in container: ", err)
	}

	// Set up virtualnet environment.
	pool := subnet.NewPool()
	env, err := dns.SetupDNSEnv(ctx, pool)
	if err != nil {
		s.Fatal("Failed to setup DNS env: ", err)
	}
	defer env.Cleanup(cleanupCtx)
	defer env.Router.Cleanup(cleanupCtx)
	defer env.Server.Cleanup(cleanupCtx)

	// Create and connect to a VPN server.
	vpnServer, err := connectToVPN(ctx, pool, env.Router, env.Certs)
	if err != nil {
		s.Fatal("Failed to create VPN server: ", err)
	}
	defer vpnServer.Cleanup(cleanupCtx)

	// Wait for the updated network configuration (VPN) to be propagated to the proxy.
	if err := waitUntilNATIptablesConfigured(ctx); err != nil {
		s.Fatal("iptables NAT output is not fully configured: ", err)
	}

	// By default, DNS query should work over VPN.
	var defaultTC = []dns.ProxyTestCase{
		{Client: dns.System},
		{Client: dns.User},
		{Client: dns.Chrome},
		{Client: dns.ARC},
	}
	if errs := dns.TestQueryDNSProxy(ctx, defaultTC, a, cont, dns.NewQueryOptions()); len(errs) != 0 {
		for _, err := range errs {
			s.Error("Failed DNS query check: ", err)
		}
	}

	// Toggle plain-text DNS or secureDNS depending on test parameter.
	params := s.Param().(dnsProxyOverVPNTestParams)
	if err := dns.SetDoHMode(ctx, cr, tconn, params.mode, dns.ExampleDoHProvider); err != nil {
		s.Fatal("Failed to set DNS-over-HTTPS mode: ", err)
	}

	// DNS queries that should be routed through VPN should fail if DNS queries on the VPN server are blocked.
	// System traffic bypass VPN, this is to allow things such as updates and crash reports to always work.
	// On the other hand, other traffic (Chrome, ARC, etc.) should always go through VPN.
	vpnBlockedTC := []dns.ProxyTestCase{
		{Client: dns.System},
		{Client: dns.User, ExpectErr: true},
		{Client: dns.Chrome, ExpectErr: true},
		{Client: dns.Crostini, ExpectErr: true},
		{Client: dns.ARC}}
	// Block DNS queries over VPN through iptables.
	if errs := dns.NewVPNBlock(vpnServer.NetNSName).Run(ctx, func(ctx context.Context) {
		if errs := dns.TestQueryDNSProxy(ctx, vpnBlockedTC, a, cont, dns.NewQueryOptions()); len(errs) != 0 {
			s.Error("Failed DNS query check: ", errs)
		}
	}); len(errs) > 0 {
		s.Fatal("Failed to block DNS over VPN: ", errs)
	}

	if params.mode == dns.DoHOff || params.mode == dns.DoHAutomatic {
		return
	}

	secureDNSBlockedTC := []dns.ProxyTestCase{
		{Client: dns.System},
		{Client: dns.User},
		{Client: dns.Chrome},
		{Client: dns.Crostini},
		{Client: dns.ARC}}
	// Block DoH queries over VPN to verify that when a VPN is on, DoH is disabled and DNS will work.
	// When a VPN is active, the default proxy and ARC proxy will disable secure DNS in order to have consistent behavior on different VPN types.
	if errs := dns.NewDoHVPNBlock(vpnServer.NetNSName).Run(ctx, func(ctx context.Context) {
		if errs := dns.TestQueryDNSProxy(ctx, secureDNSBlockedTC, a, cont, dns.NewQueryOptions()); len(errs) != 0 {
			s.Error("Failed DNS query check: ", errs)
		}
	}); len(errs) > 0 {
		s.Fatal("Failed to block secure DNS over VPN: ", errs)
	}
}

// waitUntilNATIptablesConfigured waits until the NAT rule output of iptables is fully configured.
// Whenever a network setting related to DNS is changed, DNS proxy updates iptables by deleting the old rule and creating a new rule.
// This function confirms that the expected changes have been fully propagated by periodically comparing the rules until no differences are detected between successive iterations.
func waitUntilNATIptablesConfigured(ctx context.Context) error {
	var lastRules, lastRules6 []byte
	return testing.Poll(ctx, func(ctx context.Context) error {
		rules, err := testexec.CommandContext(ctx, "iptables", "-t", "nat", "-S", "-w").Output(testexec.DumpLogOnError)
		if err != nil {
			return errors.Wrap(err, "failed to execute iptables")
		}
		rules6, err := testexec.CommandContext(ctx, "ip6tables", "-t", "nat", "-S", "-w").Output(testexec.DumpLogOnError)
		if err != nil {
			return errors.Wrap(err, "failed to execute ip6tables")
		}
		if bytes.Compare(lastRules, rules) != 0 || bytes.Compare(lastRules6, rules6) != 0 {
			lastRules = rules
			lastRules6 = rules6
			return errors.New("iptables NAT rules are still being configured")
		}
		return nil
	}, &testing.PollOptions{Interval: 2 * time.Second, Timeout: 15 * time.Second})
}

// connectToVPN creates a VPN server and connects to it.
func connectToVPN(ctx context.Context, pool *subnet.Pool, router *env.Env, httpsCerts *certs.Certs) (serverEnv *env.Env, err error) {
	serverIPv4Subnet, err := pool.AllocNextIPv4Subnet()
	if err != nil {
		return nil, errors.Wrap(err, "failed to allocate v4 subnet")
	}
	serverIPv6Subnet, err := pool.AllocNextIPv6Subnet()
	if err != nil {
		return nil, errors.Wrap(err, "failed to allocate v6 subnet")
	}
	server, err := dns.CreateDNSServerEnv(ctx, "vpnserver", serverIPv4Subnet, serverIPv6Subnet, router, httpsCerts)
	if err != nil {
		return nil, errors.Wrap(err, "failed to set up server env")
	}

	success := false
	defer func() {
		if success {
			return
		}
		if err := server.Cleanup(ctx); err != nil {
			testing.ContextLog(ctx, "Failed to cleanup server env: ", err)
		}
	}()

	// Connect to VPN.
	config := vpn.Config{
		Type:     vpn.TypeL2TPIPsec,
		AuthType: vpn.AuthTypePSK,
	}
	conn, err := vpn.NewConnectionWithEnvs(ctx, config, server, nil)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create connection object")
	}
	defer func() {
		if success {
			return
		}
		if err := conn.Cleanup(ctx); err != nil {
			testing.ContextLog(ctx, "Failed to clean up connection: ", err)
		}
	}()

	if err := conn.SetUp(ctx); err != nil {
		return nil, errors.Wrap(err, "failed to setup VPN server")
	}
	if _, err := conn.Connect(ctx); err != nil {
		return nil, errors.Wrap(err, "failed to connect to VPN server")
	}

	success = true
	return server, nil
}

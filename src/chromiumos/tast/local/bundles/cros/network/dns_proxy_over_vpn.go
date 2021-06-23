// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package network

import (
	"context"
	"time"

	"chromiumos/tast/ctxutil"
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
		SoftwareDeps: []string{"chrome"},
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

	type testCase struct {
		c         dns.Client
		expectErr bool
	}

	// By default, DNS query should work over VPN.
	var defaultTC = []testCase{
		testCase{c: dns.System, expectErr: false},
		testCase{c: dns.User, expectErr: false},
		testCase{c: dns.Chrome, expectErr: false},
	}
	for _, tc := range defaultTC {
		err = dns.QueryDNS(ctx, tc.c, domainDefault)
		if err != nil && !tc.expectErr {
			s.Errorf("Failed DNS query check for %s: %v", dns.GetClientString(tc.c), err)
		}
		if err == nil && tc.expectErr {
			s.Errorf("Successful DNS query for %s, but expected failure", dns.GetClientString(tc.c))
		}
	}
}

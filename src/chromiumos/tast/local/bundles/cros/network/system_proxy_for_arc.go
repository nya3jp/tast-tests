// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package network

import (
	"context"
	"fmt"
	"regexp"

	"chromiumos/tast/common/policy"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/bundles/cros/network/proxy"
	"chromiumos/tast/local/network"
	"chromiumos/tast/local/policyutil"
	"chromiumos/tast/local/policyutil/fixtures"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: SystemProxyForArc,
		Desc: "Test that ARC++ apps can successfully connect to the remote host through the system-proxy daemon",
		Contacts: []string{
			"acostinas@google.com", // Test author
			"chromeos-commercial-networking@google.com",
			"hugobenichi@google.com",
		},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:mainline", "informational"},
		Fixture:      "chromeEnrolledLoggedIn",
		Params: []testing.Param{{
			ExtraSoftwareDeps: []string{"android_p"},
		}},
	})
}

func SystemProxyForArc(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(*fixtures.FixtData).Chrome
	fdms := s.FixtValue().(*fixtures.FixtData).FakeDMS

	const username = "testuser"
	const password = "testpwd"

	// Perform cleanup.
	if err := policyutil.ResetChrome(ctx, fdms, cr); err != nil {
		s.Fatal("Failed to clean up: ", err)
	}

	// Start an HTTP proxy instance on the DUT which requires username and password authentication.
	ps := proxy.NewServer()

	cred := &proxy.AuthCredentials{Username: username, Password: password}
	err := ps.Start(ctx, 3128, cred, []string{})
	if err != nil {
		s.Fatal("Failed to start a local proxy on the DUT: ", err)
	}

	defer ps.Stop(ctx)

	// Configure the proxy on the DUT via policy to point to the local proxy instance started via the `ProxyService`.
	proxyPolicy := &policy.ProxySettings{
		Val: &policy.ProxySettingsValue{
			ProxyMode:       "fixed_servers",
			ProxyServer:     fmt.Sprintf("http://%s", ps.HostAndPort),
			ProxyBypassList: "www.googleapis.com",
		}}
	// Start system-proxy and configure it with the credentials of the local proxy instance.
	systemProxySettingsPolicy := &policy.SystemProxySettings{
		Val: &policy.SystemProxySettingsValue{
			SystemProxyEnabled:           true,
			SystemServicesUsername:       username,
			SystemServicesPassword:       password,
			PolicyCredentialsAuthSchemes: []string{},
		}}

	arcEnabledPolicy := &policy.ArcEnabled{Val: true}

	// Update policies.
	if err := policyutil.ServeAndRefresh(ctx, fdms, cr, []policy.Policy{proxyPolicy, systemProxySettingsPolicy, arcEnabledPolicy}); err != nil {
		s.Fatal("Failed to update policies: ", err)
	}

	a, err := arc.New(ctx, s.OutDir())
	if err != nil {
		s.Fatal("Failed to start ARC: ", err)
	}
	defer a.Close(ctx)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create test API connection: ", err)
	}

	actualUsedProxy, err := network.RunArcConnectivityApp(ctx, a, tconn, "https://www.google.com/" /*useSystemProxy=*/, true, username, password)
	if err != nil {
		s.Fatal("Failed to test app connectivity: ", err)
	}

	// System-proxy has an address in the 100.115.92.0/24 subnet (assigned by patchpanel) and listens on port 3128.
	expectedProxy := regexp.MustCompile("100.115.92.[0-9]+:3128")
	if !expectedProxy.Match([]byte(actualUsedProxy)) {
		s.Fatalf("The ARC++ app is not using the system-proxy daemon: %s", actualUsedProxy)
	}
}

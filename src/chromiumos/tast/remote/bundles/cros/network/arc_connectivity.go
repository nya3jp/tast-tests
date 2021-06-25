// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package network

import (
	"context"
	"time"

	"chromiumos/tast/remote/bundles/cros/network/allowlist"
	"chromiumos/tast/rpc"
	"chromiumos/tast/services/cros/network"
	"chromiumos/tast/testing"
)

func init() {
	// TODO(acostinas, b/191845062) Re-enable the test when OTA credentials are available in tast tests.
	testing.AddTest(&testing.Test{
		Func: ArcConnectivity,
		Desc: "Test that the PlayStore works behind a firewall configured according to our support page",
		Contacts: []string{
			"acostinas@google.com", // Test author
			"chromeos-commercial-networking@google.com",
		},
		Attr:         []string{},
		Data:         []string{"allowlist_ssl_inspection.json"},
		SoftwareDeps: []string{"reboot", "chrome", "chrome_internal"},
		ServiceDeps:  []string{"tast.cros.network.AllowlistService", "tast.cros.network.ProxyService"},
		VarDeps: []string{
			"allowlist.username",
			"allowlist.password",
		},
		Timeout: 12 * time.Minute,
		Params: []testing.Param{{
			ExtraSoftwareDeps: []string{"android_p"},
		}, {
			Name:              "vm",
			ExtraSoftwareDeps: []string{"android_vm"},
		}},
	})
}

// ArcConnectivity calls the AllowlistService to setup a firewall and verifies PlayStore connectivity.
func ArcConnectivity(ctx context.Context, s *testing.State) {
	defer func(ctx context.Context) {
		// Since this test is changing the iptable rules to create a firewall on the DUT, we need to reboot to make sure the
		// DUT gets back to its initial state, which doesn't restrict connectivity to http/s default ports.
		if err := s.DUT().Reboot(ctx); err != nil {
			s.Fatal("Failed to reboot DUT: ", err)
		}
	}(ctx)

	cl, err := rpc.Dial(ctx, s.DUT(), s.RPCHint(), "cros")
	if err != nil {
		s.Fatal("Failed to connect to the RPC service on the DUT: ", err)
	}
	defer cl.Close(ctx)

	allowlist, err := allowlist.ReadHostnames(ctx, s.DataPath("allowlist_ssl_inspection.json"), true, false)
	if err != nil {
		s.Fatal("Failed to read hostnames: ", err)
	}

	const port uint32 = 3129

	// Start an HTTP proxy instance on the DUT which only allows connections to the allowlisted hostnames.
	proxyClient := network.NewProxyServiceClient(cl.Conn)
	response, err := proxyClient.StartServer(ctx,
		&network.StartServerRequest{
			Port:      port,
			Allowlist: allowlist,
		})
	if err != nil {
		s.Fatal("Failed to start a local proxy on the DUT: ", err)
	}

	al := network.NewAllowlistServiceClient(cl.Conn)
	if _, err := al.SetupFirewall(ctx, &network.SetupFirewallRequest{AllowedPort: port}); err != nil {
		s.Fatal("Failed to setup a firewall on the DUT: ", err)
	}

	user := s.RequiredVar("allowlist.username")
	password := s.RequiredVar("allowlist.password")
	if _, err := al.GaiaLogin(ctx, &network.GaiaLoginRequest{
		Username: user, Password: password, ProxyHostAndPort: response.HostAndPort}); err != nil {
		s.Fatal("Failed to login through the proxy: ", err)
	}

	// The user account allowlist.username/allowlist.password belongs to the OU allowlist-tast-test on the production DMServer.
	// The OU is configured to force install the Gmail app via policy.
	if _, err := al.CheckArcAppInstalled(ctx, &network.CheckArcAppInstalledRequest{AppName: "com.google.android.gm"}); err != nil {
		s.Fatal("Failed to install ARC app: ", err)
	}

}

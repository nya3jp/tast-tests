// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package network

import (
	"context"
	"fmt"

	"chromiumos/tast/local/bundles/cros/network/vpn"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/shill"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         VPNMojoConf,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Test that VPN can correctly be configured through Chrome mojo API",
		Contacts: []string{
			"taoyl@google.com",
			"cros-networking@google.com",
		},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:mainline", "informational"},
		Fixture:      "shillReset",
	})
}

// configureNetworkResult holds the return value of
// chromeos.networkConfig.mojom.CrosNetworkConfig.configureNetwork().
type configureNetworkResult struct {
	GUID         string `json:"guid"`
	ErrorMessage string `json:"errorMessage"`
}

// JavaScript wrapper to call CrosNetworkConfig.configureNetwork().
const jsTemplate = `
async function() {
	const networkConfig = chromeos.networkConfig.mojom.CrosNetworkConfig.getRemote();
	const properties = %s;
	return await networkConfig.configureNetwork(properties, false);
}
`

func VPNMojoConf(ctx context.Context, s *testing.State) {
	cr, err := chrome.New(ctx)
	if err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}
	defer cr.Close(ctx)

	// Load network configuration page to get access to networkConfig mojom component.
	conn, err := cr.NewConn(ctx, "chrome://network")
	if err != nil {
		s.Fatal("Failed to load network configuration page: ", err)
	}
	defer conn.Close()

	m, err := shill.NewManager(ctx)
	if err != nil {
		s.Fatal("Failed to create shill manager proxy: ", err)
	}

	for _, tc := range []struct {
		subtest    string
		properties string
	}{
		{
			subtest: "WireGuard",
			properties: `{
				name: 'temp-wg1',
				typeConfig: {
					vpn: {
						host: 'wireguard',
						type: {value: chromeos.networkConfig.mojom.VpnType.kWireGuard},
						wireguard: {
							peers:[{
								publicKey: 'xLwB3ayvpYqvRrkyiEfK1YtipzpZKAdLJBP9ikHJbhg=',
								presharedKey: '',
								endpoint: '2.2.2.2:30000',
								allowedIps: '0.0.0.0/0',
							}],
							privateKey: 'wHyvoCjtnN/jFnOl1M9kDTWkWgdV6Nh1fawQm9NvOm0=',
						},
					},
				},
			}`,
		},
		{
			subtest: "WireGuard-NoPrivateKey",
			properties: `{
				name: 'temp-wg2',
				typeConfig: {
					vpn: {
						host: 'wireguard',
						type: {value: chromeos.networkConfig.mojom.VpnType.kWireGuard},
						wireguard: {
							peers:[{
								publicKey: 'xLwB3ayvpYqvRrkyiEfK1YtipzpZKAdLJBP9ikHJbhg=',
								presharedKey: '',
								endpoint: '2.2.2.2:30000',
								allowedIps: '0.0.0.0/0',
							}],
							privateKey: '',
						},
					},
				},
			}`,
		},
		{
			subtest: "L2TPIPsec-PSK",
			properties: `{
				name: 'temp',
				typeConfig: {
					vpn: {
						host:'host',
						type: {value: chromeos.networkConfig.mojom.VpnType.kL2TPIPsec},
						ipsec: {
							authenticationType: 'PSK',
							ikeVersion: 1,
							psk: 'psk',
						},
						l2tp: {
							password: 'password',
							username: 'username',
						},
					}
				}
			}`,
		},
		// TODO(b/216386693): Add L2TPIPsec-cert and OpenVPN subtests
	} {
		s.Run(ctx, tc.subtest, func(ctx context.Context, s *testing.State) {
			jsWrap := fmt.Sprintf(jsTemplate, tc.properties)
			var result configureNetworkResult
			if err := conn.Call(ctx, &result, jsWrap); err != nil {
				s.Fatal("Failed to call configureNetwork(): ", err)
			}
			if result.ErrorMessage != "" {
				s.Fatal("configureNetwork() returns error: ", result.ErrorMessage)
			} else {
				testing.ContextLog(ctx, "configureNetwork() returns guid ", result.GUID)
			}

			if err := vpn.VerifyVPNProfile(ctx, m, result.GUID, false); err != nil {
				s.Errorf("Failed to verify %s VPN service of guid %s: %v", tc.subtest, result.GUID, err)
			}
		})
	}
}

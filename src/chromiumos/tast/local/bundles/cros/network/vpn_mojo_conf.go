// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package network

import (
	"context"
	"fmt"
	"reflect"

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
		subtest        string
		mojoProperties string
		// |providerProperties| contains all the expected properties in the Provider
		// property, while |serviceProperties| contains all the expected properties
		// except for Provider. Most VPN properties are held in the Provider
		// properties but some are not (e.g., EAP).
		providerProperties map[string]interface{}
		serviceProperties  map[string]string
	}{
		{
			subtest: "WireGuard",
			mojoProperties: `{
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
			providerProperties: map[string]interface{}{
				"Host": "wireguard",
				"Type": "wireguard",
				// PSK and private key are not exposed so they cannot be verified.
				"WireGuard.Peers": []map[string]string{
					{
						"PublicKey":           "xLwB3ayvpYqvRrkyiEfK1YtipzpZKAdLJBP9ikHJbhg=",
						"Endpoint":            "2.2.2.2:30000",
						"AllowedIPs":          "0.0.0.0/0",
						"PersistentKeepalive": "",
					},
				},
			},
		},
		{
			subtest: "WireGuard-NoPrivateKey",
			mojoProperties: `{
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
			// This subtest has the same peer struct as the previous one, so skip the
			// property verification.
		},
		{
			subtest: "L2TPIPsec-PSK",
			mojoProperties: `{
				name: 'temp-l2tpipsec-psk',
				typeConfig: {
					vpn: {
						host:'host',
						type: {value: chromeos.networkConfig.mojom.VpnType.kL2TPIPsec},
						ipSec: {
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
			providerProperties: map[string]interface{}{
				"Host":           "host",
				"Type":           "l2tpipsec",
				"L2TPIPsec.User": "username",
			},
		},
		{
			subtest: "IKEv2-EAP",
			mojoProperties: `{
				name: 'temp-ikev2-eap',
				typeConfig: {
					vpn: {
						host:'host',
						type: {value: chromeos.networkConfig.mojom.VpnType.kIKEv2},
						ipSec: {
							authenticationType: 'EAP',
							ikeVersion: 2,
							serverCaPems: ['1234'],
							eap: {
								domainSuffixMatch: [],
								identity: 'eap-identity',
								outer: 'MSCHAPv2',
								password: 'eap-password',
								subjectAltNameMatch: [],
							}
						},
					}
				}
			}`,
			providerProperties: map[string]interface{}{
				"Host":                     "host",
				"Type":                     "ikev2",
				"IKEv2.AuthenticationType": "EAP",
				"IKEv2.CACertPEM":          []string{"1234"},
			},
			serviceProperties: map[string]string{
				"EAP.EAP":      "MSCHAPV2",
				"EAP.Identity": "eap-identity",
			},
		},
		{
			subtest: "IKEv2-PSK",
			mojoProperties: `{
				name: 'temp-ikev2-psk',
				typeConfig: {
					vpn: {
						host:'host',
						type: {value: chromeos.networkConfig.mojom.VpnType.kIKEv2},
						ipSec: {
							authenticationType: 'PSK',
							ikeVersion: 2,
							psk: 'psk',
							localIdentity: 'local-id',
							remoteIdentity: 'remote-id',
						},
					}
				}
			}`,
			providerProperties: map[string]interface{}{
				"Host":                     "host",
				"Type":                     "ikev2",
				"IKEv2.AuthenticationType": "PSK",
				"IKEv2.LocalIdentity":      "local-id",
				"IKEv2.RemoteIdentity":     "remote-id",
			},
		},
		{
			subtest: "OpenVPN",
			mojoProperties: `{
				name: 'temp-openvpn',
				typeConfig: {
					vpn: {
						host: 'host',
						type: {value: chromeos.networkConfig.mojom.VpnType.kOpenVPN},
						openVpn: {
							clientCertType: 'PKCS11Id',
							clientCertPkcs11Id: '1234',
							extraHosts: ['host1', 'host2'],
							password: 'password',
							serverCaPems: ['pem1', 'pem2'],
							username: 'username',
							userAuthenticationType: 'Password',
						}
					}
				}
			}`,
			providerProperties: map[string]interface{}{
				"Host":               "host",
				"Type":               "openvpn",
				"OpenVPN.CACertPEM":  []string{"pem1", "pem2"},
				"OpenVPN.Pkcs11.ID":  "1234",
				"OpenVPN.ExtraHosts": []string{"host1", "host2"},
				"OpenVPN.User":       "username",
			},
		},
		// Note that for other cert-based VPN services (e.g., L2TP/IPsec-cert), the
		// NetworkCertMigrator class in Chrome will check if the configured cert
		// exists on the device, and thus they cannot be verified here without
		// really importing a cert.
	} {
		s.Run(ctx, tc.subtest, func(ctx context.Context, s *testing.State) {
			jsWrap := fmt.Sprintf(jsTemplate, tc.mojoProperties)
			var result configureNetworkResult
			if err := conn.Call(ctx, &result, jsWrap); err != nil {
				s.Fatal("Failed to call configureNetwork(): ", err)
			}
			if result.ErrorMessage != "" {
				s.Fatal("configureNetwork() returns error: ", result.ErrorMessage)
			} else {
				testing.ContextLog(ctx, "configureNetwork() returns guid ", result.GUID)
			}

			// Verifies that service is created in shill.
			svc, err := vpn.FindVPNService(ctx, m, result.GUID)
			if err != nil {
				s.Fatalf("Failed to verify %s VPN service: %v", tc.subtest, err)
			}

			// Verifies service properties.
			props, err := svc.GetProperties(ctx)
			if err != nil {
				s.Fatal("Failed to get service properties: ", err)
			}

			for k, v := range tc.serviceProperties {
				got, err := props.GetString(k)
				if err != nil {
					s.Errorf("Unexpected failure for getting %s: %v", k, err)
				}
				if got != v {
					s.Errorf("Value mismatched for %s: expect %v, got %v", k, v, got)
				}
			}

			provider, err := props.Get("Provider")
			if err != nil {
				s.Fatal("Failed to get Provider property: ", err)
			}
			for k, v := range tc.providerProperties {
				got := provider.(map[string]interface{})[k]
				if reflect.DeepEqual(v, got) == false {
					s.Errorf("Value mismatched for %s: expect %v, got %v", k, v, got)
				}
			}
		})
	}
}

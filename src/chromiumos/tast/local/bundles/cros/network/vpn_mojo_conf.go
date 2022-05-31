// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package network

import (
	"context"
	"reflect"

	"chromiumos/tast/local/bundles/cros/network/vpn"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/network/netconfig"
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

func VPNMojoConf(ctx context.Context, s *testing.State) {
	cr, err := chrome.New(ctx)
	if err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}
	defer cr.Close(ctx)

	networkConfig, err := netconfig.CreateLoggedInCrosNetworkConfig(ctx, cr)
	if err != nil {
		s.Fatal("Failed to create CrosNetworkConfig: ", err)
	}
	defer networkConfig.Close(ctx)

	m, err := shill.NewManager(ctx)
	if err != nil {
		s.Fatal("Failed to create shill manager proxy: ", err)
	}

	// Constants used in the following tests. Cannot make them const since we need
	// to get address of some of them. Keys are generated randomly, only for test
	// purposes.
	emptyStr := ""
	privKey := "wHyvoCjtnN/jFnOl1M9kDTWkWgdV6Nh1fawQm9NvOm0="
	const pubKey = "xLwB3ayvpYqvRrkyiEfK1YtipzpZKAdLJBP9ikHJbhg="

	for _, tc := range []struct {
		subtest        string
		mojoProperties netconfig.ConfigProperties
		// |providerProperties| contains all the expected properties in the Provider
		// property, while |serviceProperties| contains all the expected properties
		// except for Provider. Most VPN properties are held in the Provider
		// properties but some are not (e.g., EAP).
		providerProperties map[string]interface{}
		serviceProperties  map[string]string
	}{
		{
			subtest: "WireGuard",
			mojoProperties: netconfig.ConfigProperties{
				Name: "temp-wg1",
				TypeConfig: netconfig.NetworkTypeConfigProperties{
					VPN: &netconfig.VPNConfigProperties{
						Host: "wireguard",
						Type: netconfig.VPNTypeConfig{Value: netconfig.VPNTypeWireGuard},
						WireGuard: &netconfig.WireGuardConfigProperties{
							Peers: []netconfig.WireGuardPeerProperties{
								netconfig.WireGuardPeerProperties{
									PublicKey:    pubKey,
									PresharedKey: &emptyStr,
									Endpoint:     "2.2.2.2:30000",
									AllowedIPs:   "0.0.0.0/0",
								}},
							PrivateKey: &privKey,
						},
					},
				},
			},
			providerProperties: map[string]interface{}{
				"Host": "wireguard",
				"Type": "wireguard",
				// PSK and private key are not exposed so they cannot be verified.
				"WireGuard.Peers": []map[string]string{
					{
						"PublicKey":           pubKey,
						"Endpoint":            "2.2.2.2:30000",
						"AllowedIPs":          "0.0.0.0/0",
						"PersistentKeepalive": "",
					},
				},
			},
		},
		{
			subtest: "WireGuard-NoPrivateKey",
			mojoProperties: netconfig.ConfigProperties{
				Name: "temp-wg2",
				TypeConfig: netconfig.NetworkTypeConfigProperties{
					VPN: &netconfig.VPNConfigProperties{
						Host: "wireguard",
						Type: netconfig.VPNTypeConfig{Value: netconfig.VPNTypeWireGuard},
						WireGuard: &netconfig.WireGuardConfigProperties{
							Peers: []netconfig.WireGuardPeerProperties{
								netconfig.WireGuardPeerProperties{
									PublicKey:    pubKey,
									PresharedKey: &emptyStr,
									Endpoint:     "2.2.2.2:30000",
									AllowedIPs:   "0.0.0.0/0",
								}},
							PrivateKey: &emptyStr,
						},
					},
				},
			},
			// This subtest has the same peer struct as the previous one, so skip the
			// property verification.
		},
		{
			subtest: "L2TPIPsec-PSK",
			mojoProperties: netconfig.ConfigProperties{
				Name: "temp-l2tpipsec-psk",
				TypeConfig: netconfig.NetworkTypeConfigProperties{
					VPN: &netconfig.VPNConfigProperties{
						Host: "host",
						Type: netconfig.VPNTypeConfig{Value: netconfig.VPNTypeL2TPIPsec},
						IPsec: &netconfig.IPsecConfigProperties{
							AuthType:   "PSK",
							IKEVersion: 1,
							PSK:        "psk",
						},
						L2TP: &netconfig.L2TPConfigProperties{
							Password: "password",
							Username: "username",
						},
					},
				},
			},
			providerProperties: map[string]interface{}{
				"Host":           "host",
				"Type":           "l2tpipsec",
				"L2TPIPsec.User": "username",
			},
		},
		{
			subtest: "IKEv2-EAP",
			mojoProperties: netconfig.ConfigProperties{
				Name: "temp-ikev2-eap",
				TypeConfig: netconfig.NetworkTypeConfigProperties{
					VPN: &netconfig.VPNConfigProperties{
						Host: "host",
						Type: netconfig.VPNTypeConfig{Value: netconfig.VPNTypeIKEv2},
						IPsec: &netconfig.IPsecConfigProperties{
							AuthType:     "EAP",
							IKEVersion:   2,
							ServerCAPEMs: []string{"1234"},
							EAP: &netconfig.EAPConfigProperties{
								DomainSuffixMatch:   []string{},
								Identity:            "eap-identity",
								Outer:               "MSCHAPv2",
								Password:            "eap-password",
								SubjectAltNameMatch: []netconfig.SubjectAltName{},
							},
						},
					},
				},
			},
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
			mojoProperties: netconfig.ConfigProperties{
				Name: "temp-ikev2-psk",
				TypeConfig: netconfig.NetworkTypeConfigProperties{
					VPN: &netconfig.VPNConfigProperties{
						Host: "host",
						Type: netconfig.VPNTypeConfig{Value: netconfig.VPNTypeIKEv2},
						IPsec: &netconfig.IPsecConfigProperties{
							AuthType:   "PSK",
							IKEVersion: 2,
							PSK:        "psk",
							LocalID:    "local-id",
							RemoteID:   "remote-id",
						},
					},
				},
			},
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
			mojoProperties: netconfig.ConfigProperties{
				Name: "temp-openvpn",
				TypeConfig: netconfig.NetworkTypeConfigProperties{
					VPN: &netconfig.VPNConfigProperties{
						Host: "host",
						Type: netconfig.VPNTypeConfig{Value: netconfig.VPNTypeOpenVPN},
						OpenVPN: &netconfig.OpenVPNConfigProperties{
							ClientCertType:         "PKCS11Id",
							ClientCertPkcs11Id:     "1234",
							ExtraHosts:             []string{"host1", "host2"},
							Password:               "password",
							ServerCAPEMs:           []string{"pem1", "pem2"},
							Username:               "username",
							UserAuthenticationType: "Password",
						},
					},
				},
			},
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
			guid, err := networkConfig.ConfigureNetwork(ctx, tc.mojoProperties, false)
			if err != nil {
				s.Fatal("Failed to call ConfigureNetwork(): ", err)
			}
			testing.ContextLog(ctx, "ConfigureNetwork() returns guid ", guid)

			// Verifies that service is created in shill.
			svc, err := vpn.FindVPNService(ctx, m, guid)
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

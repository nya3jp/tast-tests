// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package network

import (
	"context"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/bundles/cros/network/vpn"
	"chromiumos/tast/local/network/dumputil"
	"chromiumos/tast/local/network/routing"
	"chromiumos/tast/testing"
)

type vpnTestParams struct {
	config     vpn.Config
	shouldFail bool
}

func init() {
	testing.AddTest(&testing.Test{
		Func:     VPNConnect,
		Desc:     "Ensure that we can connect to a VPN",
		Contacts: []string{"jiejiang@google.com", "cros-networking@google.com"},
		Attr:     []string{"group:mainline", "informational"},
		// Note that this test does not involve Chrome by intention, but for VPN
		// services with certificates, Chrome may change the cert properties of them
		// proactively, and thus we need Chrome is logged-in as the same user with
		// our fake TPM. Also see b/192425378#comment5.
		Fixture: "vpnShillResetWithChromeLoggedIn",
		Params: []testing.Param{{
			Name: "ikev2_psk",
			Val: vpnTestParams{
				config: vpn.Config{
					Type:     vpn.TypeIKEv2,
					AuthType: vpn.AuthTypePSK,
				},
			},
			ExtraSoftwareDeps: []string{"ikev2"},
		}, {
			Name: "ikev2_cert",
			Val: vpnTestParams{
				config: vpn.Config{
					Type:     vpn.TypeIKEv2,
					AuthType: vpn.AuthTypeCert,
				},
			},
			ExtraSoftwareDeps: []string{"ikev2"},
		}, {
			Name: "ikev2_eap_mschapv2",
			Val: vpnTestParams{
				config: vpn.Config{
					Type:     vpn.TypeIKEv2,
					AuthType: vpn.AuthTypeEAP,
				},
			},
			ExtraSoftwareDeps: []string{"ikev2"},
		}, {
			Name: "l2tp_ipsec_psk",
			Val: vpnTestParams{
				config: vpn.Config{
					Type:     vpn.TypeL2TPIPsec,
					AuthType: vpn.AuthTypePSK,
				},
			},
		}, {
			Name: "l2tp_ipsec_psk_evil",
			Val: vpnTestParams{
				config: vpn.Config{
					Type:                  vpn.TypeL2TPIPsec,
					AuthType:              vpn.AuthTypePSK,
					UnderlayIPIsOverlayIP: true,
				},
			},
		}, {
			Name: "l2tp_ipsec_psk_xauth",
			Val: vpnTestParams{
				config: vpn.Config{
					Type:          vpn.TypeL2TPIPsec,
					AuthType:      vpn.AuthTypePSK,
					IPsecUseXauth: true,
				},
			},
		}, {
			Name: "l2tp_ipsec_psk_xauth_missing_user",
			Val: vpnTestParams{
				config: vpn.Config{
					Type:                  vpn.TypeL2TPIPsec,
					AuthType:              vpn.AuthTypePSK,
					IPsecUseXauth:         true,
					IPsecXauthMissingUser: true,
				},
				shouldFail: true,
			},
		}, {
			Name: "l2tp_ipsec_psk_xauth_wrong_user",
			Val: vpnTestParams{
				config: vpn.Config{
					Type:                vpn.TypeL2TPIPsec,
					AuthType:            vpn.AuthTypePSK,
					IPsecUseXauth:       true,
					IPsecXauthWrongUser: true,
				},
				shouldFail: true,
			},
		}, {
			Name: "l2tp_ipsec_cert",
			Val: vpnTestParams{
				config: vpn.Config{
					Type:     vpn.TypeL2TPIPsec,
					AuthType: vpn.AuthTypeCert,
				},
			},
		}, {
			Name: "openvpn",
			Val: vpnTestParams{
				config: vpn.Config{
					Type:           vpn.TypeOpenVPN,
					AuthType:       vpn.AuthTypeCert,
					OpenVPNTLSAuth: true,
				},
			},
		}, {
			Name: "openvpn_user_pass",
			Val: vpnTestParams{
				config: vpn.Config{
					Type:                   vpn.TypeOpenVPN,
					AuthType:               vpn.AuthTypeCert,
					OpenVPNUseUserPassword: true,
				},
			},
		}, {
			Name: "openvpn_cert_verify",
			Val: vpnTestParams{
				config: vpn.Config{
					Type:              vpn.TypeOpenVPN,
					AuthType:          vpn.AuthTypeCert,
					OpenVPNCertVerify: true,
				},
			},
		}, {
			Name: "openvpn_cert_verify_wrong_hash",
			Val: vpnTestParams{
				config: vpn.Config{
					Type:                       vpn.TypeOpenVPN,
					AuthType:                   vpn.AuthTypeCert,
					OpenVPNCertVerify:          true,
					OpenVPNCertVerifyWrongHash: true,
				},
				shouldFail: true,
			},
		}, {
			Name: "openvpn_cert_verify_wrong_subject",
			Val: vpnTestParams{
				config: vpn.Config{
					Type:                          vpn.TypeOpenVPN,
					AuthType:                      vpn.AuthTypeCert,
					OpenVPNCertVerify:             true,
					OpenVPNCertVeirfyWrongSubject: true,
				},
				shouldFail: true,
			},
		}, {
			Name: "openvpn_cert_verify_wrong_cn",
			Val: vpnTestParams{
				config: vpn.Config{
					Type:                     vpn.TypeOpenVPN,
					AuthType:                 vpn.AuthTypeCert,
					OpenVPNCertVerify:        true,
					OpenVPNCertVerifyWrongCN: true,
				},
				shouldFail: true,
			},
		}, {
			Name: "openvpn_cert_verify_cn_only",
			Val: vpnTestParams{
				config: vpn.Config{
					Type:                    vpn.TypeOpenVPN,
					AuthType:                vpn.AuthTypeCert,
					OpenVPNCertVerify:       true,
					OpenVPNCertVerifyCNOnly: true,
				},
			},
		}, {
			Name: "wireguard",
			Val: vpnTestParams{
				config: vpn.Config{
					Type: vpn.TypeWireGuard,
				},
			},
			ExtraSoftwareDeps: []string{"wireguard"},
		}, {
			Name: "wireguard_psk",
			Val: vpnTestParams{
				config: vpn.Config{
					Type:     vpn.TypeWireGuard,
					AuthType: vpn.AuthTypePSK,
				},
			},
			ExtraSoftwareDeps: []string{"wireguard"},
		}, {
			Name: "wireguard_two_peers",
			Val: vpnTestParams{
				config: vpn.Config{
					Type:       vpn.TypeWireGuard,
					WGTwoPeers: true,
				},
			},
			ExtraSoftwareDeps: []string{"wireguard"},
		}, {
			Name: "wireguard_generate_key",
			Val: vpnTestParams{
				config: vpn.Config{
					Type:         vpn.TypeWireGuard,
					WGAutoGenKey: true,
				},
			},
			ExtraSoftwareDeps: []string{"wireguard"},
		}, {
			Name: "wireguard_ipv4",
			Val: vpnTestParams{
				config: vpn.Config{
					Type:   vpn.TypeWireGuard,
					WGIPv4: true,
				},
			},
			ExtraSoftwareDeps: []string{"wireguard"},
		}, {
			Name: "wireguard_ipv6",
			Val: vpnTestParams{
				config: vpn.Config{
					Type:   vpn.TypeWireGuard,
					WGIPv6: true,
				},
			},
			ExtraSoftwareDeps: []string{"wireguard"},
		}, {
			Name: "wireguard_ipv4_ipv6",
			Val: vpnTestParams{
				config: vpn.Config{
					Type:          vpn.TypeWireGuard,
					WGIPv4AndIPv6: true,
				},
			},
			ExtraSoftwareDeps: []string{"wireguard"},
		},
		},
	})
}

func VPNConnect(ctx context.Context, s *testing.State) {
	// If the main body of the test times out, we still want to reserve a few
	// seconds to allow for our cleanup code to run.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(cleanupCtx, 10*time.Second)
	defer cancel()

	config := s.Param().(vpnTestParams).config
	config.CertVals = s.FixtValue().(vpn.FixtureEnv).CertVals
	conn, err := vpn.NewConnection(ctx, config)
	if err != nil {
		s.Fatal("Failed to create connection object: ", err)
	}

	defer func() {
		if err := conn.Cleanup(cleanupCtx); err != nil {
			s.Error("Failed to clean up connection: ", err)
		}
	}()

	if err := conn.SetUp(ctx); err != nil {
		s.Fatal("Failed to setup VPN server: ", err)
	}
	connected, err := conn.Connect(ctx)
	shouldFail := s.Param().(vpnTestParams).shouldFail
	if err := dumputil.DumpNetworkInfo(ctx, "network_dump_after_vpn_connect.txt"); err != nil {
		testing.ContextLog(ctx, "Failed to dump network info after VPN connect")
	}
	if err != nil {
		s.Fatal("Failed to connect to VPN server: ", err)
	} else if !connected && !shouldFail {
		s.Fatal("Failed to connect to VPN server: the service state changed to failure")
	} else if connected && shouldFail {
		s.Fatal("Connect to VPN server should fail")
	} else if !connected && shouldFail {
		return
	}

	if err := routing.ExpectPingSuccessWithTimeout(ctx, conn.Server.OverlayIP, "chronos", 10*time.Second); err != nil {
		s.Fatalf("Failed to ping %s: %v", conn.Server.OverlayIP, err)
	}

	if conn.SecondServer != nil {
		if err := routing.ExpectPingSuccessWithTimeout(ctx, conn.SecondServer.OverlayIP, "chronos", 10*time.Second); err != nil {
			s.Fatalf("Failed to ping %s: %v", conn.SecondServer.OverlayIP, err)
		}
	}

	// IPv6 should be blackholed.
	if err := routing.ExpectPingFailure(ctx, "2001:db8::1", "chronos"); err != nil {
		s.Fatal("IPv6 ping should fail: ", err)
	}
}

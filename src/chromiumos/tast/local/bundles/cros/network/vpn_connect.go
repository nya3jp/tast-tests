// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package network

import (
	"context"
	"time"

	"chromiumos/tast/common/network/ping"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/network/vpn"
	localping "chromiumos/tast/local/network/ping"
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
		Fixture:  "shillReset",
		Params: []testing.Param{{
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
					Type:     vpn.TypeOpenVPN,
					AuthType: vpn.AuthTypeCert,
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
		}},
	})
}

func VPNConnect(ctx context.Context, s *testing.State) {
	// If the main body of the test times out, we still want to reserve a few
	// seconds to allow for our cleanup code to run.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(cleanupCtx, 3*time.Second)
	defer cancel()

	conn, err := vpn.NewConnection(ctx, s.Param().(vpnTestParams).config)
	if err != nil {
		s.Fatal("Failed to create connection object: ", err)
	}

	defer func() {
		if err := conn.Cleanup(cleanupCtx); err != nil {
			s.Error("Failed to clean up connection: ", err)
		}
	}()

	connected, err := conn.Start(ctx)
	shouldFail := s.Param().(vpnTestParams).shouldFail
	if err != nil {
		s.Fatal("Failed to connect to VPN server: ", err)
	} else if !connected && !shouldFail {
		s.Fatal("Failed to connect to VPN server: the service state changed to failure")
	} else if connected && shouldFail {
		s.Fatal("Connect to VPN server should fail")
	} else if !connected && shouldFail {
		return
	}

	pr := localping.NewLocalRunner()
	if err := expectPingSuccess(ctx, pr, conn.Server.OverlayIP); err != nil {
		s.Fatalf("Failed to ping %s: %v", conn.Server.OverlayIP, err)
	}

	if conn.SecondServer != nil {
		if err := expectPingSuccess(ctx, pr, conn.SecondServer.OverlayIP); err != nil {
			s.Fatalf("Failed to ping %s: %v", conn.SecondServer.OverlayIP, err)
		}
	}

	// IPv6 should be blackholed.
	if res, err := pr.Ping(ctx, "2001:db8::1", ping.Count(1), ping.User("chronos")); err == nil && res.Received != 0 {
		s.Fatal("IPv6 ping should fail: ", err)
	}
}

func expectPingSuccess(ctx context.Context, pr *ping.Runner, addr string) error {
	testing.ContextLog(ctx, "Start to ping ", addr)
	res, err := pr.Ping(ctx, addr, ping.Count(3), ping.User("chronos"))
	if err != nil {
		return err
	}
	if res.Received == 0 {
		return errors.New("no response received")
	}
	return nil
}

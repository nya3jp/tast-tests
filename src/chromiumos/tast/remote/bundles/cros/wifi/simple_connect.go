// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wifi

import (
	"context"

	"chromiumos/tast/errors"
	"chromiumos/tast/remote/wificell"
	"chromiumos/tast/remote/wificell/hostapd"
	"chromiumos/tast/remote/wificell/hostapd/secconf"
	"chromiumos/tast/testing"
)

type simpleConnectTestcase struct {
	apOptions         []hostapd.Option
	genSecurityConfig secconf.Generator
}

type simpleConnectParam struct {
	testcases []simpleConnectTestcase
}

func init() {
	testing.AddTest(&testing.Test{
		Func:        SimpleConnect,
		Desc:        "Verifies that DUT can connect to the host via AP in different WiFi configuration",
		Contacts:    []string{"yenlinlai@google.com", "chromeos-kernel-wifi@google.com"},
		Attr:        []string{"group:wificell", "wifi_func"},
		ServiceDeps: []string{"tast.cros.network.Wifi"},
		Vars:        []string{"router"},
		Params: []testing.Param{
			{
				// Verifies that DUT can connect to an open 802.11a network on channels 48, 64.
				Name: "80211a",
				Val: simpleConnectParam{
					testcases: []simpleConnectTestcase{
						{apOptions: []hostapd.Option{hostapd.Mode(hostapd.Mode80211a), hostapd.Channel(48)}},
						{apOptions: []hostapd.Option{hostapd.Mode(hostapd.Mode80211a), hostapd.Channel(64)}},
					},
				},
			}, {
				// Verifies that DUT can connect to an open 802.11b network on channels 1, 6, 11.
				Name: "80211b",
				Val: simpleConnectParam{
					testcases: []simpleConnectTestcase{
						{apOptions: []hostapd.Option{hostapd.Mode(hostapd.Mode80211b), hostapd.Channel(1)}},
						{apOptions: []hostapd.Option{hostapd.Mode(hostapd.Mode80211b), hostapd.Channel(6)}},
						{apOptions: []hostapd.Option{hostapd.Mode(hostapd.Mode80211b), hostapd.Channel(11)}},
					},
				},
			}, {
				// Verifies that DUT can connect to an open 802.11g network on channels 1, 6, 11.
				Name: "80211g",
				Val: simpleConnectParam{
					testcases: []simpleConnectTestcase{
						{apOptions: []hostapd.Option{hostapd.Mode(hostapd.Mode80211g), hostapd.Channel(1)}},
						{apOptions: []hostapd.Option{hostapd.Mode(hostapd.Mode80211g), hostapd.Channel(6)}},
						{apOptions: []hostapd.Option{hostapd.Mode(hostapd.Mode80211g), hostapd.Channel(11)}},
					},
				},
			}, {
				// Verifies that DUT can connect to an open 802.11n network on channels 1, 6, 11 with a channel width of 20MHz.
				Name: "80211n24ht20",
				Val: simpleConnectParam{
					testcases: []simpleConnectTestcase{
						{apOptions: []hostapd.Option{hostapd.Mode(hostapd.Mode80211nPure), hostapd.Channel(1), hostapd.HTCaps(hostapd.HTCapHT20)}},
						{apOptions: []hostapd.Option{hostapd.Mode(hostapd.Mode80211nPure), hostapd.Channel(6), hostapd.HTCaps(hostapd.HTCapHT20)}},
						{apOptions: []hostapd.Option{hostapd.Mode(hostapd.Mode80211nPure), hostapd.Channel(11), hostapd.HTCaps(hostapd.HTCapHT20)}},
					},
				},
			}, {
				// Verifies that DUT can connect to an open 802.11n network on channel 6 with a channel width of 40MHz.
				Name: "80211n24ht40",
				Val: simpleConnectParam{
					testcases: []simpleConnectTestcase{
						{apOptions: []hostapd.Option{hostapd.Mode(hostapd.Mode80211nPure), hostapd.Channel(6), hostapd.HTCaps(hostapd.HTCapHT40)}},
					},
				},
			}, {
				// Verifies that DUT can connect to an open 802.11n network on channel 48 with a channel width of 20MHz.
				Name: "80211n5ht20",
				Val: simpleConnectParam{
					testcases: []simpleConnectTestcase{
						{apOptions: []hostapd.Option{hostapd.Mode(hostapd.Mode80211nPure), hostapd.Channel(48), hostapd.HTCaps(hostapd.HTCapHT20)}},
					},
				},
			}, {
				// Verifies that DUT can connect to an open 802.11n network on channel 48
				// (40MHz channel with the second 20MHz chunk of the 40MHz channel on the channel below the center channel).
				Name: "80211n5ht40",
				Val: simpleConnectParam{
					testcases: []simpleConnectTestcase{
						{apOptions: []hostapd.Option{hostapd.Mode(hostapd.Mode80211nPure), hostapd.Channel(48), hostapd.HTCaps(hostapd.HTCapHT40Minus)}},
					},
				},
			}, {
				// Verifies that DUT can connect to an open 802.11ac network on channel 60 with a channel width of 20MHz.
				Name: "80211ac5vht20",
				Val: simpleConnectParam{
					testcases: []simpleConnectTestcase{
						{apOptions: []hostapd.Option{
							hostapd.Mode(hostapd.Mode80211acPure), hostapd.Channel(60),
							hostapd.VHTChWidth(hostapd.VHTChWidth20Or40),
						}},
					},
				},
			}, {
				// Verifies that DUT can connect to an open 802.11ac network on channel 120 with a channel width of 40MHz.
				Name: "80211ac5vht40",
				Val: simpleConnectParam{
					testcases: []simpleConnectTestcase{
						{apOptions: []hostapd.Option{
							hostapd.Mode(hostapd.Mode80211acPure), hostapd.Channel(120), hostapd.HTCaps(hostapd.HTCapHT40),
							hostapd.VHTChWidth(hostapd.VHTChWidth20Or40),
						}},
					},
				},
			}, {
				// Verifies that DUT can connect to an open 802.11ac network on 5GHz channel 36 with center channel of 42 and channel width of 80MHz.
				Name: "80211ac5vht80mixed",
				Val: simpleConnectParam{
					testcases: []simpleConnectTestcase{
						{apOptions: []hostapd.Option{
							hostapd.Mode(hostapd.Mode80211acMixed), hostapd.Channel(36), hostapd.HTCaps(hostapd.HTCapHT40Plus),
							hostapd.VHTCaps(hostapd.VHTCapSGI80), hostapd.VHTCenterChannel(42), hostapd.VHTChWidth(hostapd.VHTChWidth80),
						}},
					},
				},
			}, {
				// Verifies that DUT can connect to an open 802.11ac network on channel 157 with center channel of 155 and channel width of 80MHz.
				// The router is forced to use 80 MHz wide rates only.
				Name: "80211ac5vht80pure",
				Val: simpleConnectParam{
					testcases: []simpleConnectTestcase{
						{apOptions: []hostapd.Option{
							hostapd.Mode(hostapd.Mode80211acPure), hostapd.Channel(157), hostapd.HTCaps(hostapd.HTCapHT40Plus),
							hostapd.VHTCaps(hostapd.VHTCapSGI80), hostapd.VHTCenterChannel(155), hostapd.VHTChWidth(hostapd.VHTChWidth80),
						}},
					},
				},
			}, {
				// Verifies that DUT can connect to an protected network supporting for pure WPA with TKIP.
				Name: "wpatkip",
				Val: simpleConnectParam{
					testcases: []simpleConnectTestcase{
						{
							apOptions: []hostapd.Option{hostapd.Mode(hostapd.Mode80211g), hostapd.Channel(1)},
							genSecurityConfig: secconf.WpaGenerator{
								secconf.WpaPsk("chromeos"), secconf.WpaMode(secconf.WpaPure),
								secconf.WpaCiphers(secconf.CipherTKIP),
							},
						},
					},
				},
			}, {
				// Verifies that DUT can connect to an protected network supporting for pure WPA with AES based CCMP.
				Name: "wpaccmp",
				Val: simpleConnectParam{
					testcases: []simpleConnectTestcase{
						{
							apOptions: []hostapd.Option{hostapd.Mode(hostapd.Mode80211g), hostapd.Channel(1)},
							genSecurityConfig: secconf.WpaGenerator{
								secconf.WpaPsk("chromeos"), secconf.WpaMode(secconf.WpaPure),
								secconf.WpaCiphers(secconf.CipherCCMP),
							},
						},
					},
				},
			}, {
				// Verifies that DUT can connect to an protected network supporting for pure WPA with both AES based CCMP and TKIP.
				Name: "wpamuti",
				Val: simpleConnectParam{
					testcases: []simpleConnectTestcase{
						{
							apOptions: []hostapd.Option{hostapd.Mode(hostapd.Mode80211g), hostapd.Channel(1)},
							genSecurityConfig: secconf.WpaGenerator{
								secconf.WpaPsk("chromeos"), secconf.WpaMode(secconf.WpaPure),
								secconf.WpaCiphers(secconf.CipherTKIP, secconf.CipherCCMP),
							},
						},
					},
				},
			}, {
				// Verifies that DUT can connect to an protected network supporting for WPA2 (aka RSN) with TKIP. Some AP still uses TKIP in WPA2.
				Name: "wpa2tkip",
				Val: simpleConnectParam{
					testcases: []simpleConnectTestcase{
						{
							apOptions: []hostapd.Option{hostapd.Mode(hostapd.Mode80211g), hostapd.Channel(1)},
							genSecurityConfig: secconf.WpaGenerator{
								secconf.WpaPsk("chromeos"), secconf.WpaMode(secconf.Wpa2Pure),
								secconf.Wpa2Ciphers(secconf.CipherTKIP),
							},
						},
					},
				},
			}, {
				// Verifies that DUT can connect to an protected network supporting for WPA2 (aka RSN) and encrypted under AES.
				Name: "wpa2",
				Val: simpleConnectParam{
					testcases: []simpleConnectTestcase{
						{
							apOptions: []hostapd.Option{hostapd.Mode(hostapd.Mode80211g), hostapd.Channel(1)},
							genSecurityConfig: secconf.WpaGenerator{
								secconf.WpaPsk("chromeos"), secconf.WpaMode(secconf.Wpa2Pure),
								secconf.Wpa2Ciphers(secconf.CipherCCMP),
							},
						},
					},
				},
			}, {
				// Verifies that DUT can connect to an protected network supporting for both WPA and WPA2 with TKIP/AES supported for WPA and AES supported for WPA2.
				Name: "wpamixed",
				Val: simpleConnectParam{
					testcases: []simpleConnectTestcase{
						{
							apOptions: []hostapd.Option{hostapd.Mode(hostapd.Mode80211g), hostapd.Channel(1)},
							genSecurityConfig: secconf.WpaGenerator{
								secconf.WpaPsk("chromeos"), secconf.WpaMode(secconf.WpaMixed),
								secconf.WpaCiphers(secconf.CipherTKIP, secconf.CipherCCMP),
								secconf.Wpa2Ciphers(secconf.CipherCCMP),
							},
						},
					},
				},
			}, {
				// Verifies that DUT can connect to an protected 802.11ac network supporting for WPA.
				Name: "wpa5vht80",
				Val: simpleConnectParam{
					testcases: []simpleConnectTestcase{
						{
							apOptions: []hostapd.Option{
								hostapd.Mode(hostapd.Mode80211acPure), hostapd.Channel(36), hostapd.HTCaps(hostapd.HTCapHT40Plus),
								hostapd.VHTCenterChannel(42), hostapd.VHTChWidth(hostapd.VHTChWidth80),
							},
							genSecurityConfig: secconf.WpaGenerator{
								secconf.WpaPsk("chromeos"), secconf.WpaMode(secconf.WpaPure),
								secconf.WpaCiphers(secconf.CipherTKIP, secconf.CipherCCMP),
							},
						},
					},
				},
			}, {
				// Verifies that DUT can connect to an protected network whose WPA passphrase can be pure unicode, mixed unicode and ASCII, and all the punctuations.
				Name: "wpaoddpassphrase",
				Val: simpleConnectParam{
					testcases: []simpleConnectTestcase{
						{
							apOptions: []hostapd.Option{hostapd.Mode(hostapd.Mode80211g), hostapd.Channel(1)},
							genSecurityConfig: secconf.WpaGenerator{
								secconf.WpaPsk("\xe4\xb8\x80\xe4\xba\x8c\xe4\xb8\x89"), secconf.WpaMode(secconf.WpaPure),
								secconf.WpaCiphers(secconf.CipherTKIP),
							},
						},
						{
							apOptions: []hostapd.Option{hostapd.Mode(hostapd.Mode80211g), hostapd.Channel(1)},
							genSecurityConfig: secconf.WpaGenerator{
								secconf.WpaPsk("\xe4\xb8\x80\xe4\xba\x8c\xe4\xb8\x89"), secconf.WpaMode(secconf.Wpa2Pure),
								secconf.Wpa2Ciphers(secconf.CipherCCMP),
							},
						},
						{
							apOptions: []hostapd.Option{hostapd.Mode(hostapd.Mode80211g), hostapd.Channel(1)},
							genSecurityConfig: secconf.WpaGenerator{
								secconf.WpaPsk("abcdef\xc2\xa2"), secconf.WpaMode(secconf.WpaPure),
								secconf.WpaCiphers(secconf.CipherTKIP),
							},
						},
						{
							apOptions: []hostapd.Option{hostapd.Mode(hostapd.Mode80211g), hostapd.Channel(1)},
							genSecurityConfig: secconf.WpaGenerator{
								secconf.WpaPsk("abcdef\xc2\xa2"), secconf.WpaMode(secconf.Wpa2Pure),
								secconf.Wpa2Ciphers(secconf.CipherCCMP),
							},
						},
						{
							apOptions: []hostapd.Option{hostapd.Mode(hostapd.Mode80211g), hostapd.Channel(1)},
							genSecurityConfig: secconf.WpaGenerator{
								secconf.WpaPsk(" !\"#$%&'()>*+,-./:;<=>?@[\\]^_{|}~"), secconf.WpaMode(secconf.WpaPure),
								secconf.WpaCiphers(secconf.CipherTKIP),
							},
						},
						{
							apOptions: []hostapd.Option{hostapd.Mode(hostapd.Mode80211g), hostapd.Channel(1)},
							genSecurityConfig: secconf.WpaGenerator{
								secconf.WpaPsk(" !\"#$%&'()>*+,-./:;<=>?@[\\]^_{|}~"), secconf.WpaMode(secconf.Wpa2Pure),
								secconf.Wpa2Ciphers(secconf.CipherCCMP),
							},
						},
					},
				},
			}, {
				// Verifies that DUT can connect to an hidden network on 2.4GHz and 5GHz channels.
				Name: "hidden",
				Val: simpleConnectParam{
					testcases: []simpleConnectTestcase{
						{apOptions: []hostapd.Option{hostapd.Mode(hostapd.Mode80211g), hostapd.Channel(6), hostapd.Hidden(true)}},
						{apOptions: []hostapd.Option{hostapd.Mode(hostapd.Mode80211nPure), hostapd.Channel(36), hostapd.Hidden(true)}},
						{apOptions: []hostapd.Option{hostapd.Mode(hostapd.Mode80211nPure), hostapd.Channel(48), hostapd.Hidden(true)}},
					},
				},
			}, {
				// Verifies that DUT can connect to an hidden network supporting for WPA with TKIP, WPA with TKIP/AES, WPA2 with AES, and mixed WPA with TKIP/AES and WPA2 with AES.
				Name: "hiddenwpatkip",
				Val: simpleConnectParam{
					testcases: []simpleConnectTestcase{
						{
							apOptions: []hostapd.Option{hostapd.Mode(hostapd.Mode80211g), hostapd.Channel(1), hostapd.Hidden(true)},
							genSecurityConfig: secconf.WpaGenerator{
								secconf.WpaPsk("chromeos"), secconf.WpaMode(secconf.WpaPure),
								secconf.WpaCiphers(secconf.CipherTKIP),
							},
						},
						{
							apOptions: []hostapd.Option{hostapd.Mode(hostapd.Mode80211g), hostapd.Channel(1), hostapd.Hidden(true)},
							genSecurityConfig: secconf.WpaGenerator{
								secconf.WpaPsk("chromeos"), secconf.WpaMode(secconf.WpaPure),
								secconf.WpaCiphers(secconf.CipherTKIP, secconf.CipherCCMP),
							},
						},
						{
							apOptions: []hostapd.Option{hostapd.Mode(hostapd.Mode80211g), hostapd.Channel(1), hostapd.Hidden(true)},
							genSecurityConfig: secconf.WpaGenerator{
								secconf.WpaPsk("chromeos"), secconf.WpaMode(secconf.Wpa2Pure),
								secconf.Wpa2Ciphers(secconf.CipherCCMP),
							},
						},
						{
							apOptions: []hostapd.Option{hostapd.Mode(hostapd.Mode80211g), hostapd.Channel(1), hostapd.Hidden(true)},
							genSecurityConfig: secconf.WpaGenerator{
								secconf.WpaPsk("chromeos"), secconf.WpaMode(secconf.WpaMixed),
								secconf.WpaCiphers(secconf.CipherTKIP, secconf.CipherCCMP),
								secconf.Wpa2Ciphers(secconf.CipherCCMP),
							},
						},
					},
				},
			},
		},
	})
}

func SimpleConnect(ctx context.Context, s *testing.State) {
	router, _ := s.Var("router")
	tf, err := wificell.NewTestFixture(ctx, s.DUT(), s.RPCHint(), router)
	if err != nil {
		s.Fatal("Failed to set up test fixture: ", err)
	}
	defer func() {
		if err := tf.Close(ctx); err != nil {
			s.Logf("Failed to tear down test fixture, err=%q", err.Error())
		}
	}()

	testOnce := func(ctx context.Context, options []hostapd.Option, gener secconf.Generator) error {
		ap, err := tf.ConfigureAP(ctx, options, gener)
		if err != nil {
			return errors.Wrap(err, "failed to configure ap")
		}
		defer func() {
			if err := tf.DeconfigAP(ctx, ap); err != nil {
				s.Logf("Failed to deconfig ap, err=%q", err.Error())
			}
		}()
		s.Log("AP setup done")

		if err := tf.ConnectWifi(ctx, ap); err != nil {
			return errors.Wrap(err, "failed to connect to WiFi")
		}
		defer func() {
			if err := tf.DisconnectWifi(ctx); err != nil {
				s.Logf("Failed to disconnect wifi, err=%q", err.Error())
			}
		}()
		s.Log("Connected")

		if err := tf.AssertNoDisconnect(ctx, tf.PingFromDUT); err != nil {
			return errors.Wrap(err, "failed to ping from DUT")
		}
		// TODO(crbug.com/1034875): Assert no deauth detected from the server side.
		// TODO(crbug.com/1034875): Maybe some more check on the WiFi capabilities to
		// verify we really have the settings as expected. (ref: crrev.com/c/1995105)
		s.Log("Deconfiguring")
		return nil
	}

	param := s.Param().(simpleConnectParam)
	for i, tc := range param.testcases {
		s.Logf("Testcase #%d", i)
		if err := testOnce(ctx, tc.apOptions, tc.genSecurityConfig); err != nil {
			s.Fatalf("testcase #%d failed with err=%s", i, err.Error())
		}
	}
	s.Log("Tearing down")
}

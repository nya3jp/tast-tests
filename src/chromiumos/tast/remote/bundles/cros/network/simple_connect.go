// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package network

import (
	"context"

	"chromiumos/tast/remote/wifi"
	"chromiumos/tast/remote/wifi/hostap"
	"chromiumos/tast/remote/wifi/hostap/secconf"
	"chromiumos/tast/testing"
)

type simpleConnectParm struct {
	apOptions []hostap.Option
}

func init() {
	testing.AddTest(&testing.Test{
		Func:        SimpleConnect,
		Desc:        "PoC of Wifi SimpleConnect test using gRPC",
		Contacts:    []string{"yenlinlai@google.com"}, // TODO(crbug.com/1034878): add wifi group here.
		Attr:        []string{"informational"},
		ServiceDeps: []string{"tast.cros.network.Wifi"},
		Vars:        []string{"router"},
		Params: []testing.Param{
			{
				Name: "poc",
				Val: simpleConnectParm{
					apOptions: []hostap.Option{
						hostap.Mode(hostap.Mode80211nPure),
						hostap.Channel(6),
						hostap.HTCaps(hostap.HTCapHT40Plus),
					},
				},
			}, {
				Name: "80211a",
				Val: simpleConnectParm{
					apOptions: []hostap.Option{
						hostap.Mode(hostap.Mode80211a),
						hostap.Channel(48),
					},
				},
			}, {
				Name: "80211b",
				Val: simpleConnectParm{
					apOptions: []hostap.Option{
						hostap.Mode(hostap.Mode80211b),
						hostap.Channel(1),
					},
				},
			}, {
				Name: "80211g",
				Val: simpleConnectParm{
					apOptions: []hostap.Option{
						hostap.Mode(hostap.Mode80211g),
						hostap.Channel(1),
					},
				},
			}, {
				// Open 802.11n network on 2.4 GHz channels (20MHz channels only).
				Name: "80211n24ht20",
				Val: simpleConnectParm{
					apOptions: []hostap.Option{
						hostap.Mode(hostap.Mode80211nPure),
						hostap.Channel(1),
						hostap.HTCaps(hostap.HTCapGreenfield),
					},
				},
			}, {
				// Open 802.11n network on 2.4 GHz channel (40MHz-channel).
				Name: "80211n24ht40",
				Val: simpleConnectParm{
					apOptions: []hostap.Option{
						hostap.Mode(hostap.Mode80211nPure),
						hostap.Channel(6),
						hostap.HTCaps(hostap.HTCapHT40, hostap.HTCapGreenfield),
					},
				},
			}, {
				// Open 802.11n network on 5 GHz channel (20MHz channel only).
				Name: "80211n5ht20",
				Val: simpleConnectParm{
					apOptions: []hostap.Option{
						hostap.Mode(hostap.Mode80211nPure),
						hostap.Channel(48),
						hostap.HTCaps(hostap.HTCapGreenfield),
					},
				},
			}, {
				// Open 802.11n network on 5 GHz channel (40MHz-channel with the second 20MHz
				// chunk of the 40MHz channel on the channel below the center channel).
				Name: "80211n5ht40",
				Val: simpleConnectParm{
					apOptions: []hostap.Option{
						hostap.Mode(hostap.Mode80211nPure),
						hostap.Channel(48),
						hostap.HTCaps(hostap.HTCapHT40, hostap.HTCapGreenfield),
					},
				},
			}, {
				// Open 802.11ac network on channel 60 with a channel width of 20MHz.
				Name: "80211ac5vht20",
				Val: simpleConnectParm{
					apOptions: []hostap.Option{
						hostap.Mode(hostap.Mode80211acPure),
						hostap.Channel(60),
						// VHTChWidth40 is the correct configuration option for VHT20 and VHT40
						hostap.VHTChWidth(hostap.VHTChWidth40),
					},
				},
			}, {
				// Open 802.11ac network on channel 120 with a channel width of 40MHz.
				Name: "80211ac5vht40",
				Val: simpleConnectParm{
					apOptions: []hostap.Option{
						hostap.Mode(hostap.Mode80211acPure),
						hostap.Channel(120),
						hostap.HTCaps(hostap.HTCapHT40),
						hostap.VHTChWidth(hostap.VHTChWidth40),
					},
				},
			}, {
				// Open 802.11ac network on channel 36 with center channel of 42 and channel width of 80MHz.
				Name: "80211ac5vht80mixed",
				Val: simpleConnectParm{
					apOptions: []hostap.Option{
						hostap.Mode(hostap.Mode80211acMixed),
						hostap.Channel(36),
						hostap.HTCaps(hostap.HTCapHT40Plus),
						hostap.VHTCaps(hostap.VHTCapSGI80),
						hostap.VHTCenterChannel(42),
						hostap.VHTChWidth(hostap.VHTChWidth80),
					},
				},
			}, {
				// Open 802.11ac network on channel 157 with center channel of 155 and channel width of 80MHz.
				// The router is forced to use 80 MHz wide rates only.
				Name: "80211ac5vht80pure",
				Val: simpleConnectParm{
					apOptions: []hostap.Option{
						hostap.Mode(hostap.Mode80211acPure),
						hostap.Channel(157),
						hostap.HTCaps(hostap.HTCapHT40Plus),
						hostap.VHTCaps(hostap.VHTCapSGI80),
						hostap.VHTCenterChannel(155),
						hostap.VHTChWidth(hostap.VHTChWidth80),
					},
				},
			}, {
				// Network supporting for pure WPA with TKIP.
				Name: "wpatkip",
				Val: simpleConnectParm{
					apOptions: []hostap.Option{
						hostap.Mode(hostap.Mode80211g),
						hostap.Channel(1),
						hostap.SecurityConfig(&secconf.WpaConfig{
							Psk:     "chromeos",
							WpaMode: secconf.WpaPure,
							WpaCiphers: []secconf.Cipher{
								secconf.CipherTKIP,
							},
						}),
					},
				},
			}, {
				// Network supporting for pure WPA with AES based CCMP.
				Name: "wpaccmp",
				Val: simpleConnectParm{
					apOptions: []hostap.Option{
						hostap.Mode(hostap.Mode80211g),
						hostap.Channel(1),
						hostap.SecurityConfig(&secconf.WpaConfig{
							Psk:     "chromeos",
							WpaMode: secconf.WpaPure,
							WpaCiphers: []secconf.Cipher{
								secconf.CipherCCMP,
							},
						}),
					},
				},
			}, {
				// Network supporting for pure WPA with both AES based CCMP and TKIP.
				Name: "wpatkipccmp",
				Val: simpleConnectParm{
					apOptions: []hostap.Option{
						hostap.Mode(hostap.Mode80211g),
						hostap.Channel(1),
						hostap.SecurityConfig(&secconf.WpaConfig{
							Psk:     "chromeos",
							WpaMode: secconf.WpaPure,
							WpaCiphers: []secconf.Cipher{
								secconf.CipherTKIP,
								secconf.CipherCCMP,
							},
						}),
					},
				},
			}, {
				// Network supporting for WPA2 (aka RSN) with TKIP. Some AP still uses TKIP in WPA2.
				Name: "wpa2tkip",
				Val: simpleConnectParm{
					apOptions: []hostap.Option{
						hostap.Mode(hostap.Mode80211g),
						hostap.Channel(1),
						hostap.SecurityConfig(&secconf.WpaConfig{
							Psk:     "chromeos",
							WpaMode: secconf.Wpa2Pure,
							Wpa2Ciphers: []secconf.Cipher{
								secconf.CipherTKIP,
							},
						}),
					},
				},
			}, {
				// Network supporting for WPA2 (aka RSN) and encrypted under AES.
				Name: "wpa2",
				Val: simpleConnectParm{
					apOptions: []hostap.Option{
						hostap.Mode(hostap.Mode80211g),
						hostap.Channel(1),
						hostap.SecurityConfig(&secconf.WpaConfig{
							Psk:     "chromeos",
							WpaMode: secconf.Wpa2Pure,
							Wpa2Ciphers: []secconf.Cipher{
								secconf.CipherCCMP,
							},
						}),
					},
				},
			}, {
				// Network supporting for both WPA and WPA2 with TKIP/AES supported for WPA and AES supported for WPA2.
				Name: "wpamixed",
				Val: simpleConnectParm{
					apOptions: []hostap.Option{
						hostap.Mode(hostap.Mode80211g),
						hostap.Channel(1),
						hostap.SecurityConfig(&secconf.WpaConfig{
							Psk:     "chromeos",
							WpaMode: secconf.WpaMixed,
							WpaCiphers: []secconf.Cipher{
								secconf.CipherTKIP,
								secconf.CipherCCMP,
							},
							Wpa2Ciphers: []secconf.Cipher{
								secconf.CipherCCMP,
							},
						}),
					},
				},
			}, {
				// 802.11ac network supporting for WPA.
				Name: "wpa5vht80",
				Val: simpleConnectParm{
					apOptions: []hostap.Option{
						hostap.Mode(hostap.Mode80211acPure),
						hostap.Channel(36),
						hostap.HTCaps(hostap.HTCapHT40Plus),
						hostap.VHTCenterChannel(42),
						hostap.VHTChWidth(hostap.VHTChWidth80),
						hostap.SecurityConfig(&secconf.WpaConfig{
							Psk:     "chromeos",
							WpaMode: secconf.WpaPure,
							WpaCiphers: []secconf.Cipher{
								secconf.CipherTKIP,
								secconf.CipherCCMP,
							},
						}),
					},
				},
			}, {
				// WPA with unicode passphrase.
				Name: "wpaunicode",
				Val: simpleConnectParm{
					apOptions: []hostap.Option{
						hostap.Mode(hostap.Mode80211g),
						hostap.Channel(1),
						hostap.SecurityConfig(&secconf.WpaConfig{
							Psk:     "\xe4\xb8\x80\xe4\xba\x8c\xe4\xb8\x89",
							WpaMode: secconf.WpaPure,
							WpaCiphers: []secconf.Cipher{
								secconf.CipherTKIP,
							},
						}),
					},
				},
			}, {
				// WPA2 with unicode passphrase.
				Name: "wpa2unicode",
				Val: simpleConnectParm{
					apOptions: []hostap.Option{
						hostap.Mode(hostap.Mode80211g),
						hostap.Channel(1),
						hostap.SecurityConfig(&secconf.WpaConfig{
							Psk:     "\xe4\xb8\x80\xe4\xba\x8c\xe4\xb8\x89",
							WpaMode: secconf.Wpa2Pure,
							Wpa2Ciphers: []secconf.Cipher{
								secconf.CipherCCMP,
							},
						}),
					},
				},
			}, {
				// WPA with mixed unicode passphrase and ASCII.
				Name: "wpaunicodeascii",
				Val: simpleConnectParm{
					apOptions: []hostap.Option{
						hostap.Mode(hostap.Mode80211g),
						hostap.Channel(1),
						hostap.SecurityConfig(&secconf.WpaConfig{
							Psk:     "abcdef\xc2\xa2",
							WpaMode: secconf.WpaPure,
							WpaCiphers: []secconf.Cipher{
								secconf.CipherTKIP,
							},
						}),
					},
				},
			}, {
				// WPA2 with mixed unicode passphrase and ASCII.
				Name: "wpa2unicodeascii",
				Val: simpleConnectParm{
					apOptions: []hostap.Option{
						hostap.Mode(hostap.Mode80211g),
						hostap.Channel(1),
						hostap.SecurityConfig(&secconf.WpaConfig{
							Psk:     "abcdef\xc2\xa2",
							WpaMode: secconf.Wpa2Pure,
							Wpa2Ciphers: []secconf.Cipher{
								secconf.CipherCCMP,
							},
						}),
					},
				},
			}, {
				// WPA with punctuations.
				Name: "wpapunctuation",
				Val: simpleConnectParm{
					apOptions: []hostap.Option{
						hostap.Mode(hostap.Mode80211g),
						hostap.Channel(1),
						hostap.SecurityConfig(&secconf.WpaConfig{
							Psk:     " !\"#$%&'()>*+,-./:;<=>?@[\\]^_{|}~",
							WpaMode: secconf.WpaPure,
							WpaCiphers: []secconf.Cipher{
								secconf.CipherTKIP,
							},
						}),
					},
				},
			}, {
				// WPA2 with punctuations.
				Name: "wpa2punctuation",
				Val: simpleConnectParm{
					apOptions: []hostap.Option{
						hostap.Mode(hostap.Mode80211g),
						hostap.Channel(1),
						hostap.SecurityConfig(&secconf.WpaConfig{
							Psk:     " !\"#$%&'()>*+,-./:;<=>?@[\\]^_{|}~",
							WpaMode: secconf.Wpa2Pure,
							Wpa2Ciphers: []secconf.Cipher{
								secconf.CipherCCMP,
							},
						}),
					},
				},
			}, {
				// Open 802.11g hidden network on 2.4 Ghz channel.
				Name: "hidden80211g",
				Val: simpleConnectParm{
					apOptions: []hostap.Option{
						hostap.Mode(hostap.Mode80211g),
						hostap.Channel(6),
						hostap.Hidden(true),
					},
				},
			}, {
				// Open 802.11n hidden network on 5 Ghz channels. TODO: add more channel.
				Name: "hidden80211n",
				Val: simpleConnectParm{
					apOptions: []hostap.Option{
						hostap.Mode(hostap.Mode80211nPure),
						hostap.Channel(36),
						hostap.Hidden(true),
					},
				},
			}, {
				// Hidden network supporting for pure WPA with TKIP.
				Name: "hiddenwpatkip",
				Val: simpleConnectParm{
					apOptions: []hostap.Option{
						hostap.Mode(hostap.Mode80211g),
						hostap.Channel(1),
						hostap.Hidden(true),
						hostap.SecurityConfig(&secconf.WpaConfig{
							Psk:     "chromeos",
							WpaMode: secconf.WpaPure,
							WpaCiphers: []secconf.Cipher{
								secconf.CipherTKIP,
							},
						}),
					},
				},
			}, {
				// Hidden network supporting for pure WPA with both AES based CCMP and TKIP.
				Name: "hiddenwpatkipccmp",
				Val: simpleConnectParm{
					apOptions: []hostap.Option{
						hostap.Mode(hostap.Mode80211g),
						hostap.Channel(1),
						hostap.Hidden(true),
						hostap.SecurityConfig(&secconf.WpaConfig{
							Psk:     "chromeos",
							WpaMode: secconf.WpaPure,
							WpaCiphers: []secconf.Cipher{
								secconf.CipherTKIP,
								secconf.CipherCCMP,
							},
						}),
					},
				},
			}, {
				// Hidden network supporting for WPA2 (aka RSN) and encrypted under AES.
				Name: "hiddenwpa2",
				Val: simpleConnectParm{
					apOptions: []hostap.Option{
						hostap.Mode(hostap.Mode80211g),
						hostap.Channel(1),
						hostap.Hidden(true),
						hostap.SecurityConfig(&secconf.WpaConfig{
							Psk:     "chromeos",
							WpaMode: secconf.Wpa2Pure,
							Wpa2Ciphers: []secconf.Cipher{
								secconf.CipherCCMP,
							},
						}),
					},
				},
			}, {
				// Hidden network supporting for both WPA and WPA2 with TKIP/AES supported for WPA and AES supported for WPA2.
				Name: "hiddenwpamixed",
				Val: simpleConnectParm{
					apOptions: []hostap.Option{
						hostap.Mode(hostap.Mode80211g),
						hostap.Channel(1),
						hostap.Hidden(true),
						hostap.SecurityConfig(&secconf.WpaConfig{
							Psk:     "chromeos",
							WpaMode: secconf.WpaMixed,
							WpaCiphers: []secconf.Cipher{
								secconf.CipherTKIP,
								secconf.CipherCCMP,
							},
							Wpa2Ciphers: []secconf.Cipher{
								secconf.CipherCCMP,
							},
						}),
					},
				},
			},
		},
	})
}

func SimpleConnect(ctx context.Context, s *testing.State) {
	router, _ := s.Var("router")
	tf, err := wifi.NewTestFixture(ctx, s.DUT(), s.RPCHint(), router)
	if err != nil {
		s.Fatal("Failed to set up test fixture: ", err)
	}
	defer func() {
		if err := tf.Close(ctx); err != nil {
			s.Logf("Failed to tear down test fixture, err=%q", err.Error())
		}
	}()
	parm := s.Param().(simpleConnectParm)
	ap, err := tf.ConfigureAP(ctx, parm.apOptions...)
	if err != nil {
		s.Fatal("Failed to configure ap: ", err)
	}
	defer func() {
		if err := tf.DeconfigAP(ctx, ap); err != nil {
			s.Logf("Failed to deconfig ap, err=%q", err.Error())
		}
	}()
	s.Log("AP setup done")

	if err := tf.ConnectWifi(ctx, ap); err != nil {
		s.Fatal("Failed to connect to wifi: ", err)
	}
	defer func() {
		if err := tf.DisconnectWifi(ctx); err != nil {
			s.Logf("Failed to disconnect wifi, err=%q", err.Error())
		}
	}()
	s.Log("Connected")

	if err := tf.PingFromDUT(ctx); err != nil {
		s.Fatal("Failed to ping from DUT: ", err)
	}
	s.Log("Tearing down")
}

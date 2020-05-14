// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wifi

import (
	"context"
	"fmt"

	"chromiumos/tast/common/network/eapcerts"
	"chromiumos/tast/common/wifi/security"
	"chromiumos/tast/common/wifi/security/dynamicwep"
	"chromiumos/tast/common/wifi/security/wep"
	"chromiumos/tast/common/wifi/security/wpa"
	"chromiumos/tast/remote/wificell"
	ap "chromiumos/tast/remote/wificell/hostapd"
	"chromiumos/tast/services/cros/network"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

type simpleConnectTestcase struct {
	apOpts []ap.Option
	// If unassigned, use default security config: open network.
	secConfFac security.ConfigFactory
}

func init() {
	testing.AddTest(&testing.Test{
		Func:        SimpleConnect,
		Desc:        "Verifies that DUT can connect to the host via AP in different WiFi configuration",
		Contacts:    []string{"yenlinlai@google.com", "chromeos-kernel-wifi@google.com"},
		Attr:        []string{"group:wificell", "wificell_func", "wificell_unstable"},
		ServiceDeps: []string{"tast.cros.network.Wifi"},
		Vars:        []string{"router", "pcap"},
		Params: []testing.Param{
			{
				// Verifies that DUT can connect to an open 802.11a network on channels 48, 64.
				Name: "80211a",
				Val: []simpleConnectTestcase{
					{apOpts: []ap.Option{ap.Mode(ap.Mode80211a), ap.Channel(48)}},
					{apOpts: []ap.Option{ap.Mode(ap.Mode80211a), ap.Channel(64)}},
				},
			}, {
				// Verifies that DUT can connect to an open 802.11b network on channels 1, 6, 11.
				Name: "80211b",
				Val: []simpleConnectTestcase{
					{apOpts: []ap.Option{ap.Mode(ap.Mode80211b), ap.Channel(1)}},
					{apOpts: []ap.Option{ap.Mode(ap.Mode80211b), ap.Channel(6)}},
					{apOpts: []ap.Option{ap.Mode(ap.Mode80211b), ap.Channel(11)}},
				},
			}, {
				// Verifies that DUT can connect to an open 802.11g network on channels 1, 6, 11.
				Name: "80211g",
				Val: []simpleConnectTestcase{
					{apOpts: []ap.Option{ap.Mode(ap.Mode80211g), ap.Channel(1)}},
					{apOpts: []ap.Option{ap.Mode(ap.Mode80211g), ap.Channel(6)}},
					{apOpts: []ap.Option{ap.Mode(ap.Mode80211g), ap.Channel(11)}},
				},
			}, {
				// Verifies that DUT can connect to an open 802.11n network on 2.4GHz channels 1, 6, 11 with a channel width of 20MHz.
				Name: "80211n24ht20",
				Val: []simpleConnectTestcase{
					{apOpts: []ap.Option{ap.Mode(ap.Mode80211nPure), ap.Channel(1), ap.HTCaps(ap.HTCapHT20)}},
					{apOpts: []ap.Option{ap.Mode(ap.Mode80211nPure), ap.Channel(6), ap.HTCaps(ap.HTCapHT20)}},
					{apOpts: []ap.Option{ap.Mode(ap.Mode80211nPure), ap.Channel(11), ap.HTCaps(ap.HTCapHT20)}},
				},
			}, {
				// Verifies that DUT can connect to an open 802.11n network on 2.4GHz channel 6 with a channel width of 40MHz.
				Name: "80211n24ht40",
				Val: []simpleConnectTestcase{
					{apOpts: []ap.Option{ap.Mode(ap.Mode80211nPure), ap.Channel(6), ap.HTCaps(ap.HTCapHT40)}},
				},
			}, {
				// Verifies that DUT can connect to an open 802.11n network on 5GHz channel 48 with a channel width of 20MHz.
				Name: "80211n5ht20",
				Val: []simpleConnectTestcase{
					{apOpts: []ap.Option{ap.Mode(ap.Mode80211nPure), ap.Channel(48), ap.HTCaps(ap.HTCapHT20)}},
				},
			}, {
				// Verifies that DUT can connect to an open 802.11n network on 5GHz channel 48
				// (40MHz channel with the second 20MHz chunk of the 40MHz channel on the channel below the center channel).
				Name: "80211n5ht40",
				Val: []simpleConnectTestcase{
					{apOpts: []ap.Option{ap.Mode(ap.Mode80211nPure), ap.Channel(48), ap.HTCaps(ap.HTCapHT40Minus)}},
				},
			}, {
				// Verifies that DUT can connect to an open 802.11ac network on channel 60 with a channel width of 20MHz.
				Name: "80211acvht20",
				Val: []simpleConnectTestcase{
					{apOpts: []ap.Option{
						ap.Mode(ap.Mode80211acPure), ap.Channel(60), ap.HTCaps(ap.HTCapHT20),
						ap.VHTChWidth(ap.VHTChWidth20Or40),
					}},
				},
				ExtraHardwareDeps: hwdep.D(hwdep.Wifi80211ac()),
			}, {
				// Verifies that DUT can connect to an open 802.11ac network on channel 120 with a channel width of 40MHz.
				Name: "80211acvht40",
				Val: []simpleConnectTestcase{
					{apOpts: []ap.Option{
						ap.Mode(ap.Mode80211acPure), ap.Channel(120), ap.HTCaps(ap.HTCapHT40),
						ap.VHTChWidth(ap.VHTChWidth20Or40),
					}},
				},
				ExtraHardwareDeps: hwdep.D(hwdep.Wifi80211ac()),
			}, {
				// Verifies that DUT can connect to an open 802.11ac network on 5GHz channel 36 with center channel of 42 and channel width of 80MHz.
				Name: "80211acvht80mixed",
				Val: []simpleConnectTestcase{
					{apOpts: []ap.Option{
						ap.Mode(ap.Mode80211acMixed), ap.Channel(36), ap.HTCaps(ap.HTCapHT40Plus),
						ap.VHTCaps(ap.VHTCapSGI80), ap.VHTCenterChannel(42), ap.VHTChWidth(ap.VHTChWidth80),
					}},
				},
			}, {
				// Verifies that DUT can connect to an open 802.11ac network on channel 157 with center channel of 155 and channel width of 80MHz.
				// The router is forced to use 80 MHz wide rates only.
				Name: "80211acvht80pure",
				Val: []simpleConnectTestcase{
					{apOpts: []ap.Option{
						ap.Mode(ap.Mode80211acPure), ap.Channel(157), ap.HTCaps(ap.HTCapHT40Plus),
						ap.VHTCaps(ap.VHTCapSGI80), ap.VHTCenterChannel(155), ap.VHTChWidth(ap.VHTChWidth80),
					}},
				},
				ExtraHardwareDeps: hwdep.D(hwdep.Wifi80211ac()),
			}, {
				// Verifies that DUT can connect to an hidden network on 2.4GHz and 5GHz channels.
				Name: "hidden",
				Val: []simpleConnectTestcase{
					{apOpts: []ap.Option{ap.Mode(ap.Mode80211g), ap.Channel(6), ap.Hidden()}},
					{apOpts: []ap.Option{ap.Mode(ap.Mode80211nPure), ap.Channel(36), ap.HTCaps(ap.HTCapHT20), ap.Hidden()}},
					{apOpts: []ap.Option{ap.Mode(ap.Mode80211nPure), ap.Channel(48), ap.HTCaps(ap.HTCapHT20), ap.Hidden()}},
				},
			}, {
				// Verifies that DUT can connect to an WEP network with both open and shared system authentication and 40-bit pre-shared keys.
				Name: "wep40",
				Val: []simpleConnectTestcase{
					{
						apOpts:     []ap.Option{ap.Mode(ap.Mode80211g), ap.Channel(1)},
						secConfFac: wep.NewConfigFactory(wep40Keys(), wep.DefaultKey(0), wep.AuthAlgs(wep.AuthAlgoOpen)),
					},
					{
						apOpts:     []ap.Option{ap.Mode(ap.Mode80211g), ap.Channel(1)},
						secConfFac: wep.NewConfigFactory(wep40Keys(), wep.DefaultKey(1), wep.AuthAlgs(wep.AuthAlgoOpen)),
					},
					{
						apOpts:     []ap.Option{ap.Mode(ap.Mode80211g), ap.Channel(1)},
						secConfFac: wep.NewConfigFactory(wep40Keys(), wep.DefaultKey(2), wep.AuthAlgs(wep.AuthAlgoOpen)),
					},
					{
						apOpts:     []ap.Option{ap.Mode(ap.Mode80211g), ap.Channel(1)},
						secConfFac: wep.NewConfigFactory(wep40Keys(), wep.DefaultKey(3), wep.AuthAlgs(wep.AuthAlgoOpen)),
					},
					{
						apOpts:     []ap.Option{ap.Mode(ap.Mode80211g), ap.Channel(1)},
						secConfFac: wep.NewConfigFactory(wep40Keys(), wep.DefaultKey(0), wep.AuthAlgs(wep.AuthAlgoShared)),
					},
					{
						apOpts:     []ap.Option{ap.Mode(ap.Mode80211g), ap.Channel(1)},
						secConfFac: wep.NewConfigFactory(wep40Keys(), wep.DefaultKey(1), wep.AuthAlgs(wep.AuthAlgoShared)),
					},
					{
						apOpts:     []ap.Option{ap.Mode(ap.Mode80211g), ap.Channel(1)},
						secConfFac: wep.NewConfigFactory(wep40Keys(), wep.DefaultKey(2), wep.AuthAlgs(wep.AuthAlgoShared)),
					},
					{
						apOpts:     []ap.Option{ap.Mode(ap.Mode80211g), ap.Channel(1)},
						secConfFac: wep.NewConfigFactory(wep40Keys(), wep.DefaultKey(3), wep.AuthAlgs(wep.AuthAlgoShared)),
					},
				},
			}, {
				// Verifies that DUT can connect to an WEP network with both open and shared system authentication and 104-bit pre-shared keys.
				Name: "wep104",
				Val: []simpleConnectTestcase{
					{
						apOpts:     []ap.Option{ap.Mode(ap.Mode80211g), ap.Channel(1)},
						secConfFac: wep.NewConfigFactory(wep104Keys(), wep.DefaultKey(0), wep.AuthAlgs(wep.AuthAlgoOpen)),
					},
					{
						apOpts:     []ap.Option{ap.Mode(ap.Mode80211g), ap.Channel(1)},
						secConfFac: wep.NewConfigFactory(wep104Keys(), wep.DefaultKey(1), wep.AuthAlgs(wep.AuthAlgoOpen)),
					},
					{
						apOpts:     []ap.Option{ap.Mode(ap.Mode80211g), ap.Channel(1)},
						secConfFac: wep.NewConfigFactory(wep104Keys(), wep.DefaultKey(2), wep.AuthAlgs(wep.AuthAlgoOpen)),
					},
					{
						apOpts:     []ap.Option{ap.Mode(ap.Mode80211g), ap.Channel(1)},
						secConfFac: wep.NewConfigFactory(wep104Keys(), wep.DefaultKey(3), wep.AuthAlgs(wep.AuthAlgoOpen)),
					},
					{
						apOpts:     []ap.Option{ap.Mode(ap.Mode80211g), ap.Channel(1)},
						secConfFac: wep.NewConfigFactory(wep104Keys(), wep.DefaultKey(0), wep.AuthAlgs(wep.AuthAlgoShared)),
					},
					{
						apOpts:     []ap.Option{ap.Mode(ap.Mode80211g), ap.Channel(1)},
						secConfFac: wep.NewConfigFactory(wep104Keys(), wep.DefaultKey(1), wep.AuthAlgs(wep.AuthAlgoShared)),
					},
					{
						apOpts:     []ap.Option{ap.Mode(ap.Mode80211g), ap.Channel(1)},
						secConfFac: wep.NewConfigFactory(wep104Keys(), wep.DefaultKey(2), wep.AuthAlgs(wep.AuthAlgoShared)),
					},
					{
						apOpts:     []ap.Option{ap.Mode(ap.Mode80211g), ap.Channel(1)},
						secConfFac: wep.NewConfigFactory(wep104Keys(), wep.DefaultKey(3), wep.AuthAlgs(wep.AuthAlgoShared)),
					},
				},
			}, {
				// Verifies that DUT can connect to an hidden WEP network with open/shared system authentication and 40/104-bit pre-shared keys.
				Name: "wephidden",
				Val: []simpleConnectTestcase{
					{
						apOpts:     []ap.Option{ap.Mode(ap.Mode80211g), ap.Channel(1), ap.Hidden()},
						secConfFac: wep.NewConfigFactory(wep40KeysHidden(), wep.AuthAlgs(wep.AuthAlgoOpen)),
					},
					{
						apOpts:     []ap.Option{ap.Mode(ap.Mode80211g), ap.Channel(1), ap.Hidden()},
						secConfFac: wep.NewConfigFactory(wep40KeysHidden(), wep.AuthAlgs(wep.AuthAlgoShared)),
					},
					{
						apOpts:     []ap.Option{ap.Mode(ap.Mode80211g), ap.Channel(1), ap.Hidden()},
						secConfFac: wep.NewConfigFactory(wep104KeysHidden(), wep.AuthAlgs(wep.AuthAlgoOpen)),
					},
					{
						apOpts:     []ap.Option{ap.Mode(ap.Mode80211g), ap.Channel(1), ap.Hidden()},
						secConfFac: wep.NewConfigFactory(wep104KeysHidden(), wep.AuthAlgs(wep.AuthAlgoShared)),
					},
				},
			}, {
				// Verifies that DUT can connect to an protected network supporting for pure WPA with TKIP.
				Name: "wpatkip",
				Val: []simpleConnectTestcase{
					{
						apOpts: []ap.Option{ap.Mode(ap.Mode80211g), ap.Channel(1)},
						secConfFac: wpa.NewConfigFactory(
							"chromeos", wpa.Mode(wpa.ModePureWPA),
							wpa.Ciphers(wpa.CipherTKIP),
						),
					},
				},
			}, {
				// Verifies that DUT can connect to an protected network supporting for pure WPA with AES based CCMP.
				Name: "wpaccmp",
				Val: []simpleConnectTestcase{
					{
						apOpts: []ap.Option{ap.Mode(ap.Mode80211g), ap.Channel(1)},
						secConfFac: wpa.NewConfigFactory(
							"chromeos", wpa.Mode(wpa.ModePureWPA),
							wpa.Ciphers(wpa.CipherCCMP),
						),
					},
				},
			}, {
				// Verifies that DUT can connect to an protected network supporting for pure WPA with both AES based CCMP and TKIP.
				Name: "wpamuti",
				Val: []simpleConnectTestcase{
					{
						apOpts: []ap.Option{ap.Mode(ap.Mode80211g), ap.Channel(1)},
						secConfFac: wpa.NewConfigFactory(
							"chromeos", wpa.Mode(wpa.ModePureWPA),
							wpa.Ciphers(wpa.CipherTKIP, wpa.CipherCCMP),
						),
					},
				},
			}, {
				// Verifies that DUT can connect to an protected network supporting for WPA2 (aka RSN) with TKIP. Some AP still uses TKIP in WPA2.
				Name: "wpa2tkip",
				Val: []simpleConnectTestcase{
					{
						apOpts: []ap.Option{ap.Mode(ap.Mode80211g), ap.Channel(1)},
						secConfFac: wpa.NewConfigFactory(
							"chromeos", wpa.Mode(wpa.ModePureWPA2),
							wpa.Ciphers2(wpa.CipherTKIP),
						),
					},
				},
			}, {
				// Verifies that DUT can connect to an protected network supporting for WPA2 (aka RSN) and encrypted under AES.
				Name: "wpa2",
				Val: []simpleConnectTestcase{
					{
						apOpts: []ap.Option{ap.Mode(ap.Mode80211g), ap.Channel(1)},
						secConfFac: wpa.NewConfigFactory(
							"chromeos", wpa.Mode(wpa.ModePureWPA2),
							wpa.Ciphers2(wpa.CipherCCMP),
						),
					},
				},
			}, {
				// Verifies that DUT can connect to an protected network supporting for both WPA and WPA2 with TKIP/AES supported for WPA and AES supported for WPA2.
				Name: "wpamixed",
				Val: []simpleConnectTestcase{
					{
						apOpts: []ap.Option{ap.Mode(ap.Mode80211g), ap.Channel(1)},
						secConfFac: wpa.NewConfigFactory(
							"chromeos", wpa.Mode(wpa.ModeMixed),
							wpa.Ciphers(wpa.CipherTKIP, wpa.CipherCCMP),
							wpa.Ciphers2(wpa.CipherCCMP),
						),
					},
				},
			}, {
				// Verifies that DUT can connect to an protected 802.11ac network supporting for WPA.
				Name: "wpavht80",
				Val: []simpleConnectTestcase{
					{
						apOpts: []ap.Option{
							ap.Mode(ap.Mode80211acPure), ap.Channel(36), ap.HTCaps(ap.HTCapHT40Plus),
							ap.VHTCenterChannel(42), ap.VHTChWidth(ap.VHTChWidth80),
						},
						secConfFac: wpa.NewConfigFactory(
							"chromeos", wpa.Mode(wpa.ModePureWPA),
							wpa.Ciphers(wpa.CipherTKIP, wpa.CipherCCMP),
						),
					},
				},
				ExtraHardwareDeps: hwdep.D(hwdep.Wifi80211ac()),
			}, {
				// Verifies that DUT can connect to an protected network whose WPA passphrase can be pure unicode, mixed unicode and ASCII, and all the punctuations.
				Name: "wpaoddpassphrase",
				Val: []simpleConnectTestcase{
					{
						apOpts: []ap.Option{ap.Mode(ap.Mode80211g), ap.Channel(1)},
						secConfFac: wpa.NewConfigFactory(
							"\xe4\xb8\x80\xe4\xba\x8c\xe4\xb8\x89", wpa.Mode(wpa.ModePureWPA),
							wpa.Ciphers(wpa.CipherTKIP),
						),
					},
					{
						apOpts: []ap.Option{ap.Mode(ap.Mode80211g), ap.Channel(1)},
						secConfFac: wpa.NewConfigFactory(
							"\xe4\xb8\x80\xe4\xba\x8c\xe4\xb8\x89", wpa.Mode(wpa.ModePureWPA2),
							wpa.Ciphers2(wpa.CipherCCMP),
						),
					},
					{
						apOpts: []ap.Option{ap.Mode(ap.Mode80211g), ap.Channel(1)},
						secConfFac: wpa.NewConfigFactory(
							"abcdef\xc2\xa2", wpa.Mode(wpa.ModePureWPA),
							wpa.Ciphers(wpa.CipherTKIP),
						),
					},
					{
						apOpts: []ap.Option{ap.Mode(ap.Mode80211g), ap.Channel(1)},
						secConfFac: wpa.NewConfigFactory(
							"abcdef\xc2\xa2", wpa.Mode(wpa.ModePureWPA2),
							wpa.Ciphers2(wpa.CipherCCMP),
						),
					},
					{
						apOpts: []ap.Option{ap.Mode(ap.Mode80211g), ap.Channel(1)},
						secConfFac: wpa.NewConfigFactory(
							" !\"#$%&'()>*+,-./:;<=>?@[\\]^_{|}~", wpa.Mode(wpa.ModePureWPA),
							wpa.Ciphers(wpa.CipherTKIP),
						),
					},
					{
						apOpts: []ap.Option{ap.Mode(ap.Mode80211g), ap.Channel(1)},
						secConfFac: wpa.NewConfigFactory(
							" !\"#$%&'()>*+,-./:;<=>?@[\\]^_{|}~", wpa.Mode(wpa.ModePureWPA2),
							wpa.Ciphers2(wpa.CipherCCMP),
						),
					},
				},
			}, {
				// Verifies that DUT can connect to an hidden network supporting for WPA with TKIP, WPA with TKIP/AES, WPA2 with AES, and mixed WPA with TKIP/AES and WPA2 with AES.
				Name: "wpahidden",
				Val: []simpleConnectTestcase{
					{
						apOpts: []ap.Option{ap.Mode(ap.Mode80211g), ap.Channel(1), ap.Hidden()},
						secConfFac: wpa.NewConfigFactory(
							"chromeos", wpa.Mode(wpa.ModePureWPA),
							wpa.Ciphers(wpa.CipherTKIP),
						),
					},
					{
						apOpts: []ap.Option{ap.Mode(ap.Mode80211g), ap.Channel(1), ap.Hidden()},
						secConfFac: wpa.NewConfigFactory(
							"chromeos", wpa.Mode(wpa.ModePureWPA),
							wpa.Ciphers(wpa.CipherTKIP, wpa.CipherCCMP),
						),
					},
					{
						apOpts: []ap.Option{ap.Mode(ap.Mode80211g), ap.Channel(1), ap.Hidden()},
						secConfFac: wpa.NewConfigFactory(
							"chromeos", wpa.Mode(wpa.ModePureWPA2),
							wpa.Ciphers2(wpa.CipherCCMP),
						),
					},
					{
						apOpts: []ap.Option{ap.Mode(ap.Mode80211g), ap.Channel(1), ap.Hidden()},
						secConfFac: wpa.NewConfigFactory(
							"chromeos", wpa.Mode(wpa.ModeMixed),
							wpa.Ciphers(wpa.CipherTKIP, wpa.CipherCCMP),
							wpa.Ciphers2(wpa.CipherCCMP),
						),
					},
				},
			}, {
				// Verifies that DUT can connect to a protected network supporting for dynamic WEP encryption.
				Name: "8021xwep",
				Val: []simpleConnectTestcase{
					{
						apOpts: []ap.Option{ap.Mode(ap.Mode80211g), ap.Channel(1)},
						secConfFac: dynamicwep.NewConfigFactory(
							eapcerts.CACert1, eapcerts.ServerCert1, eapcerts.ServerPrivateKey1,
							dynamicwep.ClientCACert(eapcerts.CACert1),
							dynamicwep.ClientCert(eapcerts.ClientCert1),
							dynamicwep.ClientKey(eapcerts.ClientPrivateKey1),
							dynamicwep.RekeyPeriod(20),
						),
					},
				},
			},
		},
	})
}

func SimpleConnect(fullCtx context.Context, s *testing.State) {
	ops := []wificell.TFOption{
		wificell.TFCapture(true),
	}
	if router, _ := s.Var("router"); router != "" {
		ops = append(ops, wificell.TFRouter(router))
	}
	if pcap, _ := s.Var("pcap"); pcap != "" {
		ops = append(ops, wificell.TFPcap(pcap))
	}
	// As we are not in precondition, we have fullCtx as both method context and
	// daemon context.
	tf, err := wificell.NewTestFixture(fullCtx, fullCtx, s.DUT(), s.RPCHint(), ops...)
	if err != nil {
		s.Fatal("Failed to set up test fixture: ", err)
	}
	defer func() {
		if err := tf.Close(fullCtx); err != nil {
			s.Log("Failed to tear down test fixture, err: ", err)
		}
	}()

	ctx, cancel := tf.ReserveForClose(fullCtx)
	defer cancel()

	testOnce := func(fullCtx context.Context, s *testing.State, options []ap.Option, fac security.ConfigFactory) {
		ap, err := tf.ConfigureAP(fullCtx, options, fac)
		if err != nil {
			s.Fatal("Failed to configure ap, err: ", err)
		}
		defer func() {
			if err := tf.DeconfigAP(fullCtx, ap); err != nil {
				s.Error("Failed to deconfig ap, err: ", err)
			}
		}()
		ctx, cancel := tf.ReserveForDeconfigAP(fullCtx, ap)
		defer cancel()
		s.Log("AP setup done")

		if err := tf.ConnectWifi(ctx, ap); err != nil {
			s.Fatal("Failed to connect to WiFi, err: ", err)
		}
		defer func() {
			if err := tf.DisconnectWifi(fullCtx); err != nil {
				s.Error("Failed to disconnect WiFi, err: ", err)
			}
			if _, err := tf.WifiClient().DeleteEntriesForSSID(fullCtx, &network.SSID{Ssid: ap.Config().Ssid}); err != nil {
				s.Errorf("Failed to remove entries for ssid=%s, err: %v", ap.Config().Ssid, err)
			}
		}()
		s.Log("Connected")

		ping := func(ctx context.Context) error {
			return tf.PingFromDUT(ctx)
		}

		if err := tf.AssertNoDisconnect(ctx, ping); err != nil {
			s.Fatal("Failed to ping from DUT, err: ", err)
		}

		s.Log("Checking the status of the SSID in the DUT")
		serInfo, err := tf.QueryService(ctx)
		if err != nil {
			s.Fatal("Failed to get the WiFi service information from DUT, err: ", err)
		}

		if serInfo.Hidden != ap.Config().Hidden {
			s.Fatalf("Unexpected hidden SSID status: got %t, want %t ", serInfo.Hidden, ap.Config().Hidden)
		}

		// TODO(crbug.com/1034875): Assert no deauth detected from the server side.
		// TODO(crbug.com/1034875): Maybe some more check on the WiFi capabilities to
		// verify we really have the settings as expected. (ref: crrev.com/c/1995105)
		s.Log("Deconfiguring")
	}

	testcases := s.Param().([]simpleConnectTestcase)
	for i, tc := range testcases {
		subtest := func(ctx context.Context, s *testing.State) {
			testOnce(ctx, s, tc.apOpts, tc.secConfFac)
		}
		if !s.Run(ctx, fmt.Sprintf("Testcase #%d", i), subtest) {
			// Stop if any sub-test failed.
			return
		}
	}
	s.Log("Tearing down")
}

// WEP keys for WEP tests.

func wep40Keys() []string {
	return []string{"abcde", "fedcba9876", "ab\xe4\xb8\x89", "\xe4\xb8\x89\xc2\xa2"}
}
func wep104Keys() []string {
	return []string{
		"0123456789abcdef0123456789", "mlk:ihgfedcba",
		"d\xe4\xb8\x80\xe4\xba\x8c\xe4\xb8\x89\xe5\x9b\x9b",
		"\xe4\xb8\x80\xe4\xba\x8c\xe4\xb8\x89\xc2\xa2\xc2\xa3",
	}
}
func wep40KeysHidden() []string {
	return []string{"0123456789", "89abcdef01", "9876543210", "fedcba9876"}
}
func wep104KeysHidden() []string {
	return []string{
		"0123456789abcdef0123456789", "89abcdef0123456789abcdef01",
		"fedcba9876543210fedcba9876", "109fedcba987654321fedcba98",
	}
}

// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wifi

import (
	"context"
	"fmt"
	"strings"

	"chromiumos/tast/common/crypto/certificate"
	"chromiumos/tast/common/network/ping"
	"chromiumos/tast/common/perf"
	"chromiumos/tast/common/wifi/security"
	"chromiumos/tast/common/wifi/security/dynamicwep"
	"chromiumos/tast/common/wifi/security/tunneled1x"
	"chromiumos/tast/common/wifi/security/wep"
	"chromiumos/tast/common/wifi/security/wpa"
	"chromiumos/tast/common/wifi/security/wpaeap"
	"chromiumos/tast/remote/wificell"
	ap "chromiumos/tast/remote/wificell/hostapd"
	"chromiumos/tast/services/cros/network"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

type simpleConnectTestcase struct {
	apOpts []ap.Option
	// If unassigned, use default security config: open network.
	secConfFac      security.ConfigFactory
	pingOps         []ping.Option
	expectedFailure bool
}

// EAP certs/keys for EAP tests.
var (
	eapCert1       = certificate.TestCert1()
	eapCert2       = certificate.TestCert2()
	eapCert3       = certificate.TestCert3()
	eapCert3AltSub = certificate.TestCert3AltSubjectMatch()
)

func init() {
	testing.AddTest(&testing.Test{
		Func:        SimpleConnect,
		Desc:        "Verifies that DUT can connect to the host via AP in different WiFi configuration",
		Contacts:    []string{"yenlinlai@google.com", "chromeos-kernel-wifi@google.com"},
		Attr:        []string{"group:wificell", "wificell_func"},
		ServiceDeps: []string{wificell.TFServiceName},
		Pre:         wificell.TestFixturePreWithCapture(),
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
				// This test verifies that DUT can connect to an open 802.11n network on 5 GHz channel with short guard intervals enabled (both 20/40 Mhz).
				Name:      "80211nsgi",
				ExtraAttr: []string{"wificell_cq"},
				Val: []simpleConnectTestcase{
					{apOpts: []ap.Option{ap.Mode(ap.Mode80211nPure), ap.Channel(48), ap.HTCaps(ap.HTCapHT20, ap.HTCapSGI20)}},
					{apOpts: []ap.Option{ap.Mode(ap.Mode80211nPure), ap.Channel(48), ap.HTCaps(ap.HTCapHT40Minus, ap.HTCapSGI40)}},
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
				// Verifies that DUT can connect to an open 802.11ac network on channel 48 with a channel width of 40MHz.
				Name: "80211acvht40",
				Val: []simpleConnectTestcase{
					{apOpts: []ap.Option{
						ap.Mode(ap.Mode80211acPure), ap.Channel(48), ap.HTCaps(ap.HTCapHT40),
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
				// Verifies that DUT can connect to a hidden network on 2.4GHz and 5GHz channels.
				Name: "hidden",
				Val: []simpleConnectTestcase{
					{apOpts: []ap.Option{ap.Mode(ap.Mode80211g), ap.Channel(6), ap.Hidden()}},
					{apOpts: []ap.Option{ap.Mode(ap.Mode80211nPure), ap.Channel(36), ap.HTCaps(ap.HTCapHT20), ap.Hidden()}},
					{apOpts: []ap.Option{ap.Mode(ap.Mode80211nPure), ap.Channel(48), ap.HTCaps(ap.HTCapHT20), ap.Hidden()}},
				},
			}, {
				// Verifies that DUT can connect to a WEP network with both open and shared system authentication and 40-bit pre-shared keys.
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
				// Verifies that DUT can connect to a WEP network with both open and shared system authentication and 104-bit pre-shared keys.
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
				// Verifies that DUT can connect to a hidden WEP network with open/shared system authentication and 40/104-bit pre-shared keys.
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
				// Verifies that DUT can connect to a protected network supporting for pure WPA with TKIP.
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
				// Verifies that DUT can connect to a protected network supporting for pure WPA with AES based CCMP.
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
				// Verifies that DUT can connect to a protected network supporting for pure WPA with both AES based CCMP and TKIP.
				Name:      "wpamulti",
				ExtraAttr: []string{"wificell_cq"},
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
				// Verifies that DUT can connect to a protected network supporting for WPA2 (aka RSN) with TKIP. Some AP still uses TKIP in WPA2.
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
				// Verifies that we can connect to an AP broadcasting a WPA2 network using AES based CCMP.
				// In addition, the client must also support 802.11w protected management frames.
				Name:      "wpa2pmf",
				ExtraAttr: []string{"wificell_cq"},
				Val: []simpleConnectTestcase{
					{
						apOpts: []ap.Option{ap.Mode(ap.Mode80211g), ap.Channel(1), ap.PMF(ap.PMFRequired)},
						secConfFac: wpa.NewConfigFactory(
							"chromeos", wpa.Mode(wpa.ModePureWPA2),
							wpa.Ciphers2(wpa.CipherCCMP),
						),
					},
				},
			}, {
				// Verifies that we can connect to an AP broadcasting a WPA2 network using AES based CCMP.
				// In addition, the client may also negotiate use of 802.11w protected management frames.
				Name: "wpa2pmfoptional",
				Val: []simpleConnectTestcase{
					{
						apOpts: []ap.Option{ap.Mode(ap.Mode80211g), ap.Channel(1), ap.PMF(ap.PMFOptional)},
						secConfFac: wpa.NewConfigFactory(
							"chromeos", wpa.Mode(wpa.ModePureWPA2),
							wpa.Ciphers2(wpa.CipherCCMP),
						),
					},
				},
			}, {
				// Verifies that DUT can connect to a protected network supporting for WPA2 (aka RSN) and encrypted under AES.
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
				// Verifies that DUT can connect to a protected network supporting for both WPA and WPA2 with TKIP/AES supported for WPA and AES supported for WPA2.
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
				// Verifies that DUT can connect to an AP in WPA2/WPA3 mixed mode. WiFi alliance suggests PMF in this mode.
				Name: "wpa3mixed",
				Val: []simpleConnectTestcase{
					{
						apOpts: []ap.Option{
							ap.Mode(ap.Mode80211acMixed), ap.Channel(36), ap.HTCaps(ap.HTCapHT40Plus),
							ap.VHTCenterChannel(42), ap.VHTChWidth(ap.VHTChWidth80),
							ap.PMF(ap.PMFOptional),
						},
						secConfFac: wpa.NewConfigFactory(
							"chromeos", wpa.Mode(wpa.ModeMixedWPA3),
							wpa.Ciphers2(wpa.CipherCCMP),
						),
					},
				},
			}, {
				// Verifies that DUT can connect to a protected 802.11ac network supporting for WPA.
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
				// Verifies that DUT can connect to a protected network whose WPA passphrase can be pure unicode, mixed unicode and ASCII, and all the punctuations.
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
				// Verifies that DUT can connect to a hidden network supporting for WPA with TKIP, WPA with TKIP/AES, WPA2 with AES, and mixed WPA with TKIP/AES and WPA2 with AES.
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
				// Verifies that DUT can connect to a WPA network using a raw PMK value instead of an ASCII passphrase.
				Name: "raw_pmk",
				Val: []simpleConnectTestcase{
					{
						apOpts: []ap.Option{ap.Mode(ap.Mode80211g), ap.Channel(1)},
						secConfFac: wpa.NewConfigFactory(
							strings.Repeat("0123456789abcdef", 4), // length = 64.
							wpa.Mode(wpa.ModePureWPA),
							wpa.Ciphers(wpa.CipherTKIP),
						),
					},
				},
			}, {
				// Verifies that DUT can connect to an open network on a DFS channel.
				// DFS (dynamic frequency selection) channels are channels that may be unavailable if radar interference is detected.
				// See: https://en.wikipedia.org/wiki/Dynamic_frequency_selection, https://en.wikipedia.org/wiki/List_of_WLAN_channels
				Name:      "dfs",
				ExtraAttr: []string{"wificell_cq"},
				Val: []simpleConnectTestcase{
					{apOpts: []ap.Option{ap.Mode(ap.Mode80211nMixed), ap.Channel(136), ap.HTCaps(ap.HTCapHT40)}},
				},
			}, {
				// Verifies that DUT can connect to an open network on the DFS channel 120 (5600MHz).
				// TODO(b/154440798): Investigate why this fails on veyron_mickey and consider merge this with
				// "dfs" case after resolution.
				Name:      "dfs_ch120",
				ExtraAttr: []string{"wificell_unstable"},
				Val: []simpleConnectTestcase{
					{apOpts: []ap.Option{ap.Mode(ap.Mode80211nMixed), ap.Channel(120), ap.HTCaps(ap.HTCapHT40)}},
				},
			}, {
				// Verifies that DUT can connect to a networks with the longest and shortest SSID.
				Name:      "ssid_limits",
				ExtraAttr: []string{"wificell_cq"},
				Val: []simpleConnectTestcase{
					{apOpts: wificell.CommonAPOptions(ap.SSID("a"))},
					{apOpts: wificell.CommonAPOptions(ap.SSID(strings.Repeat("MaxLengthSSID", 4)[:32]))},
				},
			}, {
				// This test case verifies that the DUT accepts ascii and non-ascii type characters as the SSID.
				Name: "non_ascii_ssid",
				// TODO(b/158150763): Skip Marvell WiFi as there's a known issue to make the test always fail.
				ExtraHardwareDeps: hwdep.D(hwdep.WifiNotMarvell()),
				Val: []simpleConnectTestcase{
					// TODO(crbug.com/1082582): shill don't allow leading 0x00 now, so let's append it in the
					// end to keep the coverage.
					{apOpts: wificell.CommonAPOptions(ap.SSID(byteSequenceStr(1, 31) + "\x00"))},
					{apOpts: wificell.CommonAPOptions(ap.SSID(byteSequenceStr(32, 63)))},
					{apOpts: wificell.CommonAPOptions(ap.SSID(byteSequenceStr(64, 95)))},
					{apOpts: wificell.CommonAPOptions(ap.SSID(byteSequenceStr(96, 127)))},
					{apOpts: wificell.CommonAPOptions(ap.SSID(byteSequenceStr(128, 159)))},
					{apOpts: wificell.CommonAPOptions(ap.SSID(byteSequenceStr(160, 191)))},
					{apOpts: wificell.CommonAPOptions(ap.SSID(byteSequenceStr(192, 223)))},
					{apOpts: wificell.CommonAPOptions(ap.SSID(byteSequenceStr(224, 255)))},
					// Valid Unicode characters.
					{apOpts: wificell.CommonAPOptions(ap.SSID("\xe4\xb8\xad\xe5\x9b\xbd"))},
					// Single extended ASCII character (a-grave).
					{apOpts: wificell.CommonAPOptions(ap.SSID("\xe0"))},
					// Mix of ASCII and Unicode characters as SSID.
					{apOpts: wificell.CommonAPOptions(ap.SSID("Chrome\xe7\xac\x94\xe8\xae\xb0\xe6\x9c\xac"))},
				},
			}, {
				// Verifies that DUT can connect to a protected network supporting for dynamic WEP encryption.
				Name: "8021xwep",
				Val: []simpleConnectTestcase{
					{
						apOpts: []ap.Option{ap.Mode(ap.Mode80211g), ap.Channel(1)},
						secConfFac: dynamicwep.NewConfigFactory(
							eapCert1.CACert, eapCert1.ServerCred,
							dynamicwep.ClientCACert(eapCert1.CACert),
							dynamicwep.ClientCred(eapCert1.ClientCred),
							dynamicwep.RekeyPeriod(10),
						),
						pingOps: []ping.Option{ping.Count(15), ping.Interval(1)},
					},
				},
			}, {
				// Verifies that DUT can connect to a protected network supporting for WPA-EAP encryption.
				Name:      "8021xwpa",
				ExtraAttr: []string{"wificell_unstable"},
				Val: []simpleConnectTestcase{
					{
						apOpts: []ap.Option{ap.Mode(ap.Mode80211g), ap.Channel(1)},
						secConfFac: wpaeap.NewConfigFactory(
							eapCert1.CACert, eapCert1.ServerCred,
							wpaeap.ClientCACert(eapCert1.CACert),
							wpaeap.ClientCred(eapCert1.ClientCred),
						),
					},
					{
						// Failure due to lack of CACert on client.
						apOpts: []ap.Option{ap.Mode(ap.Mode80211g), ap.Channel(1)},
						secConfFac: wpaeap.NewConfigFactory(
							eapCert1.CACert, eapCert1.ServerCred,
							wpaeap.ClientCred(eapCert1.ClientCred),
						),
						expectedFailure: true,
					},
					{
						// Failure due to unmatched CACert.
						apOpts: []ap.Option{ap.Mode(ap.Mode80211g), ap.Channel(1)},
						secConfFac: wpaeap.NewConfigFactory(
							eapCert1.CACert, eapCert1.ServerCred,
							wpaeap.ClientCACert(eapCert2.CACert),
							wpaeap.ClientCred(eapCert1.ClientCred),
						),
						expectedFailure: true,
					},
					{
						// Should succeed if we specify that we have no CACert.
						apOpts: []ap.Option{ap.Mode(ap.Mode80211g), ap.Channel(1)},
						secConfFac: wpaeap.NewConfigFactory(
							eapCert1.CACert, eapCert1.ServerCred,
							wpaeap.ClientCred(eapCert1.ClientCred),
							wpaeap.NotUseSystemCAs(),
						),
					},
					{
						// Failure due to wrong certificate chain on client.
						apOpts: []ap.Option{ap.Mode(ap.Mode80211g), ap.Channel(1)},
						secConfFac: wpaeap.NewConfigFactory(
							eapCert1.CACert, eapCert1.ServerCred,
							wpaeap.ClientCACert(eapCert1.CACert),
							wpaeap.ClientCred(eapCert2.ClientCred),
						),
						expectedFailure: true,
					},
					{
						// Failure due to expired cert on server.
						apOpts: []ap.Option{ap.Mode(ap.Mode80211g), ap.Channel(1)},
						secConfFac: wpaeap.NewConfigFactory(
							eapCert1.CACert, eapCert1.ExpiredServerCred,
							wpaeap.ClientCACert(eapCert1.CACert),
							wpaeap.ClientCred(eapCert1.ClientCred),
						),
						expectedFailure: true,
					},
				},
			}, {
				// Verifies that DUT CANNOT connect to a PEAP network with wrong settings.
				// We do these tests for only one inner authentication protocol because we
				// presume that supplicant reuses this code between inner authentication types.
				Name:      "8021xpeap_fail",
				ExtraAttr: []string{"wificell_unstable"},
				Val: []simpleConnectTestcase{
					{
						// Failure due to bad password.
						apOpts: []ap.Option{ap.Mode(ap.Mode80211g), ap.Channel(1)},
						secConfFac: tunneled1x.NewConfigFactory(
							eapCert1.CACert, eapCert1.ServerCred, eapCert1.CACert, "testuser", "password",
							tunneled1x.OuterProtocol(tunneled1x.Layer1TypePEAP),
							tunneled1x.InnerProtocol(tunneled1x.Layer2TypeMSCHAPV2),
							tunneled1x.ClientPassword("wrongpassword"),
						),
						expectedFailure: true,
					},
					{
						// Failure due to wrong client CA.
						apOpts: []ap.Option{ap.Mode(ap.Mode80211g), ap.Channel(1)},
						secConfFac: tunneled1x.NewConfigFactory(
							eapCert1.CACert, eapCert1.ServerCred, eapCert2.CACert, "testuser", "password",
							tunneled1x.OuterProtocol(tunneled1x.Layer1TypePEAP),
							tunneled1x.InnerProtocol(tunneled1x.Layer2TypeMSCHAPV2),
						),
						expectedFailure: true,
					},
					{
						// Failure due to expired server cred.
						apOpts: []ap.Option{ap.Mode(ap.Mode80211g), ap.Channel(1)},
						secConfFac: tunneled1x.NewConfigFactory(
							eapCert1.CACert, eapCert1.ExpiredServerCred, eapCert1.CACert, "testuser", "password",
							tunneled1x.OuterProtocol(tunneled1x.Layer1TypePEAP),
							tunneled1x.InnerProtocol(tunneled1x.Layer2TypeMSCHAPV2),
						),
						expectedFailure: true,
					},
					{
						// Failure due to that a subject alternative name (SAN) is set but does not match any of the server certificate SANs.
						apOpts: []ap.Option{ap.Mode(ap.Mode80211g), ap.Channel(1)},
						secConfFac: tunneled1x.NewConfigFactory(
							eapCert3.CACert, eapCert3.ServerCred, eapCert3.CACert, "testuser", "password",
							tunneled1x.OuterProtocol(tunneled1x.Layer1TypePEAP),
							tunneled1x.InnerProtocol(tunneled1x.Layer2TypeMSCHAPV2),
							tunneled1x.AltSubjectMatch([]string{`{"Type":"DNS","Value":"wrong_dns.com"}`}),
						),
						expectedFailure: true,
					},
				},
			}, {
				// Verifies that DUT can connect to a protected network supporting for PEAP authentication with tunneled MSCHAPV2.
				Name: "8021xpeap_mschapv2",
				Val: []simpleConnectTestcase{
					{
						apOpts: []ap.Option{ap.Mode(ap.Mode80211g), ap.Channel(1)},
						secConfFac: tunneled1x.NewConfigFactory(
							eapCert1.CACert, eapCert1.ServerCred, eapCert1.CACert, "testuser", "password",
							tunneled1x.OuterProtocol(tunneled1x.Layer1TypePEAP),
							tunneled1x.InnerProtocol(tunneled1x.Layer2TypeMSCHAPV2),
						),
					},
					{
						apOpts: []ap.Option{ap.Mode(ap.Mode80211g), ap.Channel(1)},
						secConfFac: tunneled1x.NewConfigFactory(
							eapCert3.CACert, eapCert3.ServerCred, eapCert3.CACert, "testuser", "password",
							tunneled1x.OuterProtocol(tunneled1x.Layer1TypePEAP),
							tunneled1x.InnerProtocol(tunneled1x.Layer2TypeMSCHAPV2),
							tunneled1x.AltSubjectMatch([]string{eapCert3AltSub[0]}),
						),
					},
					{
						apOpts: []ap.Option{ap.Mode(ap.Mode80211g), ap.Channel(1)},
						secConfFac: tunneled1x.NewConfigFactory(
							eapCert3.CACert, eapCert3.ServerCred, eapCert3.CACert, "testuser", "password",
							tunneled1x.OuterProtocol(tunneled1x.Layer1TypePEAP),
							tunneled1x.InnerProtocol(tunneled1x.Layer2TypeMSCHAPV2),
							tunneled1x.AltSubjectMatch([]string{eapCert3AltSub[1]}),
						),
					},
					{
						apOpts: []ap.Option{ap.Mode(ap.Mode80211g), ap.Channel(1)},
						secConfFac: tunneled1x.NewConfigFactory(
							eapCert3.CACert, eapCert3.ServerCred, eapCert3.CACert, "testuser", "password",
							tunneled1x.OuterProtocol(tunneled1x.Layer1TypePEAP),
							tunneled1x.InnerProtocol(tunneled1x.Layer2TypeMSCHAPV2),
							tunneled1x.AltSubjectMatch([]string{eapCert3AltSub[2]}),
						),
					},
					{
						// Should success since having multiple entries in 'altsubject_match' is treated as OR, not AND.
						// For more information about how wpa_supplicant uses altsubject_match field:
						// https://w1.fi/cgit/hostap/plain/wpa_supplicant/wpa_supplicant.conf
						apOpts: []ap.Option{ap.Mode(ap.Mode80211g), ap.Channel(1)},
						secConfFac: tunneled1x.NewConfigFactory(
							eapCert3.CACert, eapCert3.ServerCred, eapCert3.CACert, "testuser", "password",
							tunneled1x.OuterProtocol(tunneled1x.Layer1TypePEAP),
							tunneled1x.InnerProtocol(tunneled1x.Layer2TypeMSCHAPV2),
							tunneled1x.AltSubjectMatch([]string{`{"Type":"DNS","Value":"wrong_dns.com"}`, eapCert3AltSub[0]}),
						),
					},
				},
			}, {
				// Verifies that DUT can connect to a protected network supporting for PEAP authentication with tunneled MD5.
				Name: "8021xpeap_md5",
				Val: []simpleConnectTestcase{
					{
						apOpts: []ap.Option{ap.Mode(ap.Mode80211g), ap.Channel(1)},
						secConfFac: tunneled1x.NewConfigFactory(
							eapCert1.CACert, eapCert1.ServerCred, eapCert1.CACert, "testuser", "password",
							tunneled1x.OuterProtocol(tunneled1x.Layer1TypePEAP),
							tunneled1x.InnerProtocol(tunneled1x.Layer2TypeMD5),
						),
					},
					{
						apOpts: []ap.Option{ap.Mode(ap.Mode80211g), ap.Channel(1)},
						secConfFac: tunneled1x.NewConfigFactory(
							eapCert3.CACert, eapCert3.ServerCred, eapCert3.CACert, "testuser", "password",
							tunneled1x.OuterProtocol(tunneled1x.Layer1TypePEAP),
							tunneled1x.InnerProtocol(tunneled1x.Layer2TypeMD5),
							tunneled1x.AltSubjectMatch([]string{eapCert3AltSub[0]}),
						),
					},
					{
						apOpts: []ap.Option{ap.Mode(ap.Mode80211g), ap.Channel(1)},
						secConfFac: tunneled1x.NewConfigFactory(
							eapCert3.CACert, eapCert3.ServerCred, eapCert3.CACert, "testuser", "password",
							tunneled1x.OuterProtocol(tunneled1x.Layer1TypePEAP),
							tunneled1x.InnerProtocol(tunneled1x.Layer2TypeMD5),
							tunneled1x.AltSubjectMatch([]string{eapCert3AltSub[1]}),
						),
					},
					{
						apOpts: []ap.Option{ap.Mode(ap.Mode80211g), ap.Channel(1)},
						secConfFac: tunneled1x.NewConfigFactory(
							eapCert3.CACert, eapCert3.ServerCred, eapCert3.CACert, "testuser", "password",
							tunneled1x.OuterProtocol(tunneled1x.Layer1TypePEAP),
							tunneled1x.InnerProtocol(tunneled1x.Layer2TypeMD5),
							tunneled1x.AltSubjectMatch([]string{eapCert3AltSub[2]}),
						),
					},
					{
						// Should success since having multiple entries in 'altsubject_match' is treated as OR, not AND.
						// For more information about how wpa_supplicant uses altsubject_match field:
						// https://w1.fi/cgit/hostap/plain/wpa_supplicant/wpa_supplicant.conf
						apOpts: []ap.Option{ap.Mode(ap.Mode80211g), ap.Channel(1)},
						secConfFac: tunneled1x.NewConfigFactory(
							eapCert3.CACert, eapCert3.ServerCred, eapCert3.CACert, "testuser", "password",
							tunneled1x.OuterProtocol(tunneled1x.Layer1TypePEAP),
							tunneled1x.InnerProtocol(tunneled1x.Layer2TypeMD5),
							tunneled1x.AltSubjectMatch([]string{`{"Type":"DNS","Value":"wrong_dns.com"}`, eapCert3AltSub[0]}),
						),
					},
				},
			}, {
				// Verifies that DUT can connect to a protected network supporting for PEAP authentication with tunneled GTC.
				Name:      "8021xpeap_gtc",
				ExtraAttr: []string{"wificell_unstable"},
				Val: []simpleConnectTestcase{
					{
						apOpts: []ap.Option{ap.Mode(ap.Mode80211g), ap.Channel(1)},
						secConfFac: tunneled1x.NewConfigFactory(
							eapCert1.CACert, eapCert1.ServerCred, eapCert1.CACert, "testuser", "password",
							tunneled1x.OuterProtocol(tunneled1x.Layer1TypePEAP),
							tunneled1x.InnerProtocol(tunneled1x.Layer2TypeGTC),
						),
					},
					{
						apOpts: []ap.Option{ap.Mode(ap.Mode80211g), ap.Channel(1)},
						secConfFac: tunneled1x.NewConfigFactory(
							eapCert3.CACert, eapCert3.ServerCred, eapCert3.CACert, "testuser", "password",
							tunneled1x.OuterProtocol(tunneled1x.Layer1TypePEAP),
							tunneled1x.InnerProtocol(tunneled1x.Layer2TypeGTC),
							tunneled1x.AltSubjectMatch([]string{eapCert3AltSub[0]}),
						),
					},
					{
						apOpts: []ap.Option{ap.Mode(ap.Mode80211g), ap.Channel(1)},
						secConfFac: tunneled1x.NewConfigFactory(
							eapCert3.CACert, eapCert3.ServerCred, eapCert3.CACert, "testuser", "password",
							tunneled1x.OuterProtocol(tunneled1x.Layer1TypePEAP),
							tunneled1x.InnerProtocol(tunneled1x.Layer2TypeGTC),
							tunneled1x.AltSubjectMatch([]string{eapCert3AltSub[1]}),
						),
					},
					{
						apOpts: []ap.Option{ap.Mode(ap.Mode80211g), ap.Channel(1)},
						secConfFac: tunneled1x.NewConfigFactory(
							eapCert3.CACert, eapCert3.ServerCred, eapCert3.CACert, "testuser", "password",
							tunneled1x.OuterProtocol(tunneled1x.Layer1TypePEAP),
							tunneled1x.InnerProtocol(tunneled1x.Layer2TypeGTC),
							tunneled1x.AltSubjectMatch([]string{eapCert3AltSub[2]}),
						),
					},
					{
						// Should success since having multiple entries in 'altsubject_match' is treated as OR, not AND.
						// For more information about how wpa_supplicant uses altsubject_match field:
						// https://w1.fi/cgit/hostap/plain/wpa_supplicant/wpa_supplicant.conf
						apOpts: []ap.Option{ap.Mode(ap.Mode80211g), ap.Channel(1)},
						secConfFac: tunneled1x.NewConfigFactory(
							eapCert3.CACert, eapCert3.ServerCred, eapCert3.CACert, "testuser", "password",
							tunneled1x.OuterProtocol(tunneled1x.Layer1TypePEAP),
							tunneled1x.InnerProtocol(tunneled1x.Layer2TypeGTC),
							tunneled1x.AltSubjectMatch([]string{`{"Type":"DNS","Value":"wrong_dns.com"}`, eapCert3AltSub[0]}),
						),
					},
				},
			}, {
				// Verifies that DUT CANNOT connect to a TTLS network with wrong settings.
				// We do these tests for only one inner authentication protocol because we
				// presume that supplicant reuses this code between inner authentication types.
				Name:      "8021xttls_fail",
				ExtraAttr: []string{"wificell_unstable"},
				Val: []simpleConnectTestcase{
					{
						// Failure due to bad password.
						apOpts: []ap.Option{ap.Mode(ap.Mode80211g), ap.Channel(1)},
						secConfFac: tunneled1x.NewConfigFactory(
							eapCert1.CACert, eapCert1.ServerCred, eapCert1.CACert, "testuser", "password",
							tunneled1x.OuterProtocol(tunneled1x.Layer1TypeTTLS),
							tunneled1x.InnerProtocol(tunneled1x.Layer2TypeMD5),
							tunneled1x.ClientPassword("wrongpassword"),
						),
						expectedFailure: true,
					},
					{
						// Failure due to wrong client CA.
						apOpts: []ap.Option{ap.Mode(ap.Mode80211g), ap.Channel(1)},
						secConfFac: tunneled1x.NewConfigFactory(
							eapCert1.CACert, eapCert1.ServerCred, eapCert2.CACert, "testuser", "password",
							tunneled1x.OuterProtocol(tunneled1x.Layer1TypeTTLS),
							tunneled1x.InnerProtocol(tunneled1x.Layer2TypeMD5),
						),
						expectedFailure: true,
					},
					{
						// Failure due to expired server cred.
						apOpts: []ap.Option{ap.Mode(ap.Mode80211g), ap.Channel(1)},
						secConfFac: tunneled1x.NewConfigFactory(
							eapCert1.CACert, eapCert1.ExpiredServerCred, eapCert1.CACert, "testuser", "password",
							tunneled1x.OuterProtocol(tunneled1x.Layer1TypeTTLS),
							tunneled1x.InnerProtocol(tunneled1x.Layer2TypeMD5),
						),
						expectedFailure: true,
					},
					{
						// Failure due to that a subject alternative name (SAN) is set but does not match any of the server certificate SANs.
						apOpts: []ap.Option{ap.Mode(ap.Mode80211g), ap.Channel(1)},
						secConfFac: tunneled1x.NewConfigFactory(
							eapCert3.CACert, eapCert3.ServerCred, eapCert3.CACert, "testuser", "password",
							tunneled1x.OuterProtocol(tunneled1x.Layer1TypeTTLS),
							tunneled1x.InnerProtocol(tunneled1x.Layer2TypeMD5),
							tunneled1x.AltSubjectMatch([]string{`{"Type":"DNS","Value":"wrong_dns.com"}`}),
						),
						expectedFailure: true,
					},
				},
			}, {
				// Verifies that DUT can connect to a protected network supporting for TTLS authentication with tunneled MSCHAPV2.
				Name: "8021xttls_mschapv2",
				Val: []simpleConnectTestcase{
					{
						apOpts: []ap.Option{ap.Mode(ap.Mode80211g), ap.Channel(1)},
						secConfFac: tunneled1x.NewConfigFactory(
							eapCert1.CACert, eapCert1.ServerCred, eapCert1.CACert, "testuser", "password",
							tunneled1x.OuterProtocol(tunneled1x.Layer1TypeTTLS),
							tunneled1x.InnerProtocol(tunneled1x.Layer2TypeMSCHAPV2),
						),
					},
					{
						apOpts: []ap.Option{ap.Mode(ap.Mode80211g), ap.Channel(1)},
						secConfFac: tunneled1x.NewConfigFactory(
							eapCert3.CACert, eapCert3.ServerCred, eapCert3.CACert, "testuser", "password",
							tunneled1x.OuterProtocol(tunneled1x.Layer1TypeTTLS),
							tunneled1x.InnerProtocol(tunneled1x.Layer2TypeMSCHAPV2),
							tunneled1x.AltSubjectMatch([]string{eapCert3AltSub[0]}),
						),
					},
					{
						apOpts: []ap.Option{ap.Mode(ap.Mode80211g), ap.Channel(1)},
						secConfFac: tunneled1x.NewConfigFactory(
							eapCert3.CACert, eapCert3.ServerCred, eapCert3.CACert, "testuser", "password",
							tunneled1x.OuterProtocol(tunneled1x.Layer1TypeTTLS),
							tunneled1x.InnerProtocol(tunneled1x.Layer2TypeMSCHAPV2),
							tunneled1x.AltSubjectMatch([]string{eapCert3AltSub[1]}),
						),
					},
					{
						apOpts: []ap.Option{ap.Mode(ap.Mode80211g), ap.Channel(1)},
						secConfFac: tunneled1x.NewConfigFactory(
							eapCert3.CACert, eapCert3.ServerCred, eapCert3.CACert, "testuser", "password",
							tunneled1x.OuterProtocol(tunneled1x.Layer1TypeTTLS),
							tunneled1x.InnerProtocol(tunneled1x.Layer2TypeMSCHAPV2),
							tunneled1x.AltSubjectMatch([]string{eapCert3AltSub[2]}),
						),
					},
					{
						// Should success since having multiple entries in 'altsubject_match' is treated as OR, not AND.
						// For more information about how wpa_supplicant uses altsubject_match field:
						// https://w1.fi/cgit/hostap/plain/wpa_supplicant/wpa_supplicant.conf
						apOpts: []ap.Option{ap.Mode(ap.Mode80211g), ap.Channel(1)},
						secConfFac: tunneled1x.NewConfigFactory(
							eapCert3.CACert, eapCert3.ServerCred, eapCert3.CACert, "testuser", "password",
							tunneled1x.OuterProtocol(tunneled1x.Layer1TypeTTLS),
							tunneled1x.InnerProtocol(tunneled1x.Layer2TypeMSCHAPV2),
							tunneled1x.AltSubjectMatch([]string{`{"Type":"DNS","Value":"wrong_dns.com"}`, eapCert3AltSub[0]}),
						),
					},
				},
			}, {
				// Verifies that DUT can connect to a protected network supporting for TTLS authentication with tunneled MD5.
				Name: "8021xttls_md5",
				Val: []simpleConnectTestcase{
					{
						apOpts: []ap.Option{ap.Mode(ap.Mode80211g), ap.Channel(1)},
						secConfFac: tunneled1x.NewConfigFactory(
							eapCert1.CACert, eapCert1.ServerCred, eapCert1.CACert, "testuser", "password",
							tunneled1x.OuterProtocol(tunneled1x.Layer1TypeTTLS),
							tunneled1x.InnerProtocol(tunneled1x.Layer2TypeMD5),
						),
					},
					{
						apOpts: []ap.Option{ap.Mode(ap.Mode80211g), ap.Channel(1)},
						secConfFac: tunneled1x.NewConfigFactory(
							eapCert3.CACert, eapCert3.ServerCred, eapCert3.CACert, "testuser", "password",
							tunneled1x.OuterProtocol(tunneled1x.Layer1TypeTTLS),
							tunneled1x.InnerProtocol(tunneled1x.Layer2TypeMD5),
							tunneled1x.AltSubjectMatch([]string{eapCert3AltSub[0]}),
						),
					},
					{
						apOpts: []ap.Option{ap.Mode(ap.Mode80211g), ap.Channel(1)},
						secConfFac: tunneled1x.NewConfigFactory(
							eapCert3.CACert, eapCert3.ServerCred, eapCert3.CACert, "testuser", "password",
							tunneled1x.OuterProtocol(tunneled1x.Layer1TypeTTLS),
							tunneled1x.InnerProtocol(tunneled1x.Layer2TypeMD5),
							tunneled1x.AltSubjectMatch([]string{eapCert3AltSub[1]}),
						),
					},
					{
						apOpts: []ap.Option{ap.Mode(ap.Mode80211g), ap.Channel(1)},
						secConfFac: tunneled1x.NewConfigFactory(
							eapCert3.CACert, eapCert3.ServerCred, eapCert3.CACert, "testuser", "password",
							tunneled1x.OuterProtocol(tunneled1x.Layer1TypeTTLS),
							tunneled1x.InnerProtocol(tunneled1x.Layer2TypeMD5),
							tunneled1x.AltSubjectMatch([]string{eapCert3AltSub[2]}),
						),
					},
					{
						// Should success since having multiple entries in 'altsubject_match' is treated as OR, not AND.
						// For more information about how wpa_supplicant uses altsubject_match field:
						// https://w1.fi/cgit/hostap/plain/wpa_supplicant/wpa_supplicant.conf
						apOpts: []ap.Option{ap.Mode(ap.Mode80211g), ap.Channel(1)},
						secConfFac: tunneled1x.NewConfigFactory(
							eapCert3.CACert, eapCert3.ServerCred, eapCert3.CACert, "testuser", "password",
							tunneled1x.OuterProtocol(tunneled1x.Layer1TypeTTLS),
							tunneled1x.InnerProtocol(tunneled1x.Layer2TypeMD5),
							tunneled1x.AltSubjectMatch([]string{`{"Type":"DNS","Value":"wrong_dns.com"}`, eapCert3AltSub[0]}),
						),
					},
				},
			}, {
				// Verifies that DUT can connect to a protected network supporting for TTLS authentication with tunneled GTC.
				Name:      "8021xttls_gtc",
				ExtraAttr: []string{"wificell_unstable"},
				Val: []simpleConnectTestcase{
					{
						apOpts: []ap.Option{ap.Mode(ap.Mode80211g), ap.Channel(1)},
						secConfFac: tunneled1x.NewConfigFactory(
							eapCert1.CACert, eapCert1.ServerCred, eapCert1.CACert, "testuser", "password",
							tunneled1x.OuterProtocol(tunneled1x.Layer1TypeTTLS),
							tunneled1x.InnerProtocol(tunneled1x.Layer2TypeGTC),
						),
					},
					{
						apOpts: []ap.Option{ap.Mode(ap.Mode80211g), ap.Channel(1)},
						secConfFac: tunneled1x.NewConfigFactory(
							eapCert3.CACert, eapCert3.ServerCred, eapCert3.CACert, "testuser", "password",
							tunneled1x.OuterProtocol(tunneled1x.Layer1TypeTTLS),
							tunneled1x.InnerProtocol(tunneled1x.Layer2TypeGTC),
							tunneled1x.AltSubjectMatch([]string{eapCert3AltSub[0]}),
						),
					},
					{
						apOpts: []ap.Option{ap.Mode(ap.Mode80211g), ap.Channel(1)},
						secConfFac: tunneled1x.NewConfigFactory(
							eapCert3.CACert, eapCert3.ServerCred, eapCert3.CACert, "testuser", "password",
							tunneled1x.OuterProtocol(tunneled1x.Layer1TypeTTLS),
							tunneled1x.InnerProtocol(tunneled1x.Layer2TypeGTC),
							tunneled1x.AltSubjectMatch([]string{eapCert3AltSub[1]}),
						),
					},
					{
						apOpts: []ap.Option{ap.Mode(ap.Mode80211g), ap.Channel(1)},
						secConfFac: tunneled1x.NewConfigFactory(
							eapCert3.CACert, eapCert3.ServerCred, eapCert3.CACert, "testuser", "password",
							tunneled1x.OuterProtocol(tunneled1x.Layer1TypeTTLS),
							tunneled1x.InnerProtocol(tunneled1x.Layer2TypeGTC),
							tunneled1x.AltSubjectMatch([]string{eapCert3AltSub[2]}),
						),
					},
					{
						// Should success since having multiple entries in 'altsubject_match' is treated as OR, not AND.
						// For more information about how wpa_supplicant uses altsubject_match field:
						// https://w1.fi/cgit/hostap/plain/wpa_supplicant/wpa_supplicant.conf
						apOpts: []ap.Option{ap.Mode(ap.Mode80211g), ap.Channel(1)},
						secConfFac: tunneled1x.NewConfigFactory(
							eapCert3.CACert, eapCert3.ServerCred, eapCert3.CACert, "testuser", "password",
							tunneled1x.OuterProtocol(tunneled1x.Layer1TypeTTLS),
							tunneled1x.InnerProtocol(tunneled1x.Layer2TypeGTC),
							tunneled1x.AltSubjectMatch([]string{`{"Type":"DNS","Value":"wrong_dns.com"}`, eapCert3AltSub[0]}),
						),
					},
				},
			}, {
				// Verifies that DUT can connect to a protected network supporting for TTLS authentication with tunneled TTLSMSCHAPV2.
				Name: "8021xttls_ttlsmschapv2",
				Val: []simpleConnectTestcase{
					{
						apOpts: []ap.Option{ap.Mode(ap.Mode80211g), ap.Channel(1)},
						secConfFac: tunneled1x.NewConfigFactory(
							eapCert1.CACert, eapCert1.ServerCred, eapCert1.CACert, "testuser", "password",
							tunneled1x.OuterProtocol(tunneled1x.Layer1TypeTTLS),
							tunneled1x.InnerProtocol(tunneled1x.Layer2TypeTTLSMSCHAPV2),
						),
					},
					{
						apOpts: []ap.Option{ap.Mode(ap.Mode80211g), ap.Channel(1)},
						secConfFac: tunneled1x.NewConfigFactory(
							eapCert3.CACert, eapCert3.ServerCred, eapCert3.CACert, "testuser", "password",
							tunneled1x.OuterProtocol(tunneled1x.Layer1TypeTTLS),
							tunneled1x.InnerProtocol(tunneled1x.Layer2TypeTTLSMSCHAPV2),
							tunneled1x.AltSubjectMatch([]string{eapCert3AltSub[0]}),
						),
					},
					{
						apOpts: []ap.Option{ap.Mode(ap.Mode80211g), ap.Channel(1)},
						secConfFac: tunneled1x.NewConfigFactory(
							eapCert3.CACert, eapCert3.ServerCred, eapCert3.CACert, "testuser", "password",
							tunneled1x.OuterProtocol(tunneled1x.Layer1TypeTTLS),
							tunneled1x.InnerProtocol(tunneled1x.Layer2TypeTTLSMSCHAPV2),
							tunneled1x.AltSubjectMatch([]string{eapCert3AltSub[1]}),
						),
					},
					{
						apOpts: []ap.Option{ap.Mode(ap.Mode80211g), ap.Channel(1)},
						secConfFac: tunneled1x.NewConfigFactory(
							eapCert3.CACert, eapCert3.ServerCred, eapCert3.CACert, "testuser", "password",
							tunneled1x.OuterProtocol(tunneled1x.Layer1TypeTTLS),
							tunneled1x.InnerProtocol(tunneled1x.Layer2TypeTTLSMSCHAPV2),
							tunneled1x.AltSubjectMatch([]string{eapCert3AltSub[2]}),
						),
					},
					{
						// Should success since having multiple entries in 'altsubject_match' is treated as OR, not AND.
						// For more information about how wpa_supplicant uses altsubject_match field:
						// https://w1.fi/cgit/hostap/plain/wpa_supplicant/wpa_supplicant.conf
						apOpts: []ap.Option{ap.Mode(ap.Mode80211g), ap.Channel(1)},
						secConfFac: tunneled1x.NewConfigFactory(
							eapCert3.CACert, eapCert3.ServerCred, eapCert3.CACert, "testuser", "password",
							tunneled1x.OuterProtocol(tunneled1x.Layer1TypeTTLS),
							tunneled1x.InnerProtocol(tunneled1x.Layer2TypeTTLSMSCHAPV2),
							tunneled1x.AltSubjectMatch([]string{`{"Type":"DNS","Value":"wrong_dns.com"}`, eapCert3AltSub[0]}),
						),
					},
				},
			}, {
				// Verifies that DUT can connect to a protected network supporting for TTLS authentication with tunneled TTLSMSCHAP.
				Name: "8021xttls_ttlsmschap",
				Val: []simpleConnectTestcase{
					{
						apOpts: []ap.Option{ap.Mode(ap.Mode80211g), ap.Channel(1)},
						secConfFac: tunneled1x.NewConfigFactory(
							eapCert1.CACert, eapCert1.ServerCred, eapCert1.CACert, "testuser", "password",
							tunneled1x.OuterProtocol(tunneled1x.Layer1TypeTTLS),
							tunneled1x.InnerProtocol(tunneled1x.Layer2TypeTTLSMSCHAP),
						),
					},
					{
						apOpts: []ap.Option{ap.Mode(ap.Mode80211g), ap.Channel(1)},
						secConfFac: tunneled1x.NewConfigFactory(
							eapCert3.CACert, eapCert3.ServerCred, eapCert3.CACert, "testuser", "password",
							tunneled1x.OuterProtocol(tunneled1x.Layer1TypeTTLS),
							tunneled1x.InnerProtocol(tunneled1x.Layer2TypeTTLSMSCHAP),
							tunneled1x.AltSubjectMatch([]string{eapCert3AltSub[0]}),
						),
					},
					{
						apOpts: []ap.Option{ap.Mode(ap.Mode80211g), ap.Channel(1)},
						secConfFac: tunneled1x.NewConfigFactory(
							eapCert3.CACert, eapCert3.ServerCred, eapCert3.CACert, "testuser", "password",
							tunneled1x.OuterProtocol(tunneled1x.Layer1TypeTTLS),
							tunneled1x.InnerProtocol(tunneled1x.Layer2TypeTTLSMSCHAP),
							tunneled1x.AltSubjectMatch([]string{eapCert3AltSub[1]}),
						),
					},
					{
						apOpts: []ap.Option{ap.Mode(ap.Mode80211g), ap.Channel(1)},
						secConfFac: tunneled1x.NewConfigFactory(
							eapCert3.CACert, eapCert3.ServerCred, eapCert3.CACert, "testuser", "password",
							tunneled1x.OuterProtocol(tunneled1x.Layer1TypeTTLS),
							tunneled1x.InnerProtocol(tunneled1x.Layer2TypeTTLSMSCHAP),
							tunneled1x.AltSubjectMatch([]string{eapCert3AltSub[2]}),
						),
					},
					{
						// Should success since having multiple entries in 'altsubject_match' is treated as OR, not AND.
						// For more information about how wpa_supplicant uses altsubject_match field:
						// https://w1.fi/cgit/hostap/plain/wpa_supplicant/wpa_supplicant.conf
						apOpts: []ap.Option{ap.Mode(ap.Mode80211g), ap.Channel(1)},
						secConfFac: tunneled1x.NewConfigFactory(
							eapCert3.CACert, eapCert3.ServerCred, eapCert3.CACert, "testuser", "password",
							tunneled1x.OuterProtocol(tunneled1x.Layer1TypeTTLS),
							tunneled1x.InnerProtocol(tunneled1x.Layer2TypeTTLSMSCHAP),
							tunneled1x.AltSubjectMatch([]string{`{"Type":"DNS","Value":"wrong_dns.com"}`, eapCert3AltSub[0]}),
						),
					},
				},
			}, {
				// Verifies that DUT can connect to a protected network supporting for TTLS authentication with tunneled TTLSPAP.
				Name: "8021xttls_ttlspap",
				Val: []simpleConnectTestcase{
					{
						apOpts: []ap.Option{ap.Mode(ap.Mode80211g), ap.Channel(1)},
						secConfFac: tunneled1x.NewConfigFactory(
							eapCert1.CACert, eapCert1.ServerCred, eapCert1.CACert, "testuser", "password",
							tunneled1x.OuterProtocol(tunneled1x.Layer1TypeTTLS),
							tunneled1x.InnerProtocol(tunneled1x.Layer2TypeTTLSPAP),
						),
					},
					{
						apOpts: []ap.Option{ap.Mode(ap.Mode80211g), ap.Channel(1)},
						secConfFac: tunneled1x.NewConfigFactory(
							eapCert3.CACert, eapCert3.ServerCred, eapCert3.CACert, "testuser", "password",
							tunneled1x.OuterProtocol(tunneled1x.Layer1TypeTTLS),
							tunneled1x.InnerProtocol(tunneled1x.Layer2TypeTTLSPAP),
							tunneled1x.AltSubjectMatch([]string{eapCert3AltSub[0]}),
						),
					},
					{
						apOpts: []ap.Option{ap.Mode(ap.Mode80211g), ap.Channel(1)},
						secConfFac: tunneled1x.NewConfigFactory(
							eapCert3.CACert, eapCert3.ServerCred, eapCert3.CACert, "testuser", "password",
							tunneled1x.OuterProtocol(tunneled1x.Layer1TypeTTLS),
							tunneled1x.InnerProtocol(tunneled1x.Layer2TypeTTLSPAP),
							tunneled1x.AltSubjectMatch([]string{eapCert3AltSub[1]}),
						),
					},
					{
						apOpts: []ap.Option{ap.Mode(ap.Mode80211g), ap.Channel(1)},
						secConfFac: tunneled1x.NewConfigFactory(
							eapCert3.CACert, eapCert3.ServerCred, eapCert3.CACert, "testuser", "password",
							tunneled1x.OuterProtocol(tunneled1x.Layer1TypeTTLS),
							tunneled1x.InnerProtocol(tunneled1x.Layer2TypeTTLSPAP),
							tunneled1x.AltSubjectMatch([]string{eapCert3AltSub[2]}),
						),
					},
					{
						// Should success since having multiple entries in 'altsubject_match' is treated as OR, not AND.
						// For more information about how wpa_supplicant uses altsubject_match field:
						// https://w1.fi/cgit/hostap/plain/wpa_supplicant/wpa_supplicant.conf
						apOpts: []ap.Option{ap.Mode(ap.Mode80211g), ap.Channel(1)},
						secConfFac: tunneled1x.NewConfigFactory(
							eapCert3.CACert, eapCert3.ServerCred, eapCert3.CACert, "testuser", "password",
							tunneled1x.OuterProtocol(tunneled1x.Layer1TypeTTLS),
							tunneled1x.InnerProtocol(tunneled1x.Layer2TypeTTLSPAP),
							tunneled1x.AltSubjectMatch([]string{`{"Type":"DNS","Value":"wrong_dns.com"}`, eapCert3AltSub[0]}),
						),
					},
				},
			},
		},
	})
}

func SimpleConnect(ctx context.Context, s *testing.State) {
	tf := s.PreValue().(*wificell.TestFixture)
	defer func(ctx context.Context) {
		if err := tf.CollectLogs(ctx); err != nil {
			s.Log("Error collecting logs, err: ", err)
		}
	}(ctx)
	ctx, cancel := tf.ReserveForCollectLogs(ctx)
	defer cancel()

	pv := perf.NewValues()
	defer func() {
		if err := pv.Save(s.OutDir()); err != nil {
			s.Log("Failed to save perf data, err: ", err)
		}
	}()

	testOnce := func(ctx context.Context, s *testing.State, options []ap.Option, fac security.ConfigFactory, pingOps []ping.Option, expectedFailure bool) {
		ap, err := tf.ConfigureAP(ctx, options, fac)
		if err != nil {
			s.Fatal("Failed to configure ap, err: ", err)
		}
		defer func(ctx context.Context) {
			if err := tf.DeconfigAP(ctx, ap); err != nil {
				s.Error("Failed to deconfig ap, err: ", err)
			}
		}(ctx)
		ctx, cancel := tf.ReserveForDeconfigAP(ctx, ap)
		defer cancel()
		s.Log("AP setup done")

		// Some tests may fail as expected at following ConnectWifiAP(). In that case entries should still be deleted properly.
		defer func(ctx context.Context) {
			req := &network.DeleteEntriesForSSIDRequest{Ssid: []byte(ap.Config().SSID)}
			if _, err := tf.WifiClient().DeleteEntriesForSSID(ctx, req); err != nil {
				s.Errorf("Failed to remove entries for ssid=%s, err: %v", ap.Config().SSID, err)
			}
		}(ctx)

		resp, err := tf.ConnectWifiAP(ctx, ap)
		if err != nil {
			if expectedFailure {
				s.Log("Failed to connect to WiFi as expected")
				// If we expect to fail, then this test is already done.
				return
			}
			s.Fatal("Failed to connect to WiFi, err: ", err)
		}
		defer func(ctx context.Context) {
			if err := tf.CleanDisconnectWifi(ctx); err != nil {
				s.Error("Failed to disconnect WiFi, err: ", err)
			}
		}(ctx)
		ctx, cancel = tf.ReserveForDisconnect(ctx)
		defer cancel()
		if expectedFailure {
			s.Fatal("Expected to fail to connect to WiFi, but it was successful")
		}
		s.Log("Connected")

		desc := ap.Config().PerfDesc()

		pv.Set(perf.Metric{
			Name:      desc,
			Variant:   "Discovery",
			Unit:      "seconds",
			Direction: perf.SmallerIsBetter,
		}, float64(resp.DiscoveryTime)/1e9)
		pv.Set(perf.Metric{
			Name:      desc,
			Variant:   "Association",
			Unit:      "seconds",
			Direction: perf.SmallerIsBetter,
		}, float64(resp.AssociationTime)/1e9)
		pv.Set(perf.Metric{
			Name:      desc,
			Variant:   "Configuration",
			Unit:      "seconds",
			Direction: perf.SmallerIsBetter,
		}, float64(resp.ConfigurationTime)/1e9)
		ping := func(ctx context.Context) error {
			return tf.PingFromDUT(ctx, ap.ServerIP().String(), pingOps...)
		}

		if err := tf.AssertNoDisconnect(ctx, ping); err != nil {
			s.Fatal("Failed to ping from DUT, err: ", err)
		}

		s.Log("Checking the status of the SSID in the DUT")
		serInfo, err := tf.QueryService(ctx)
		if err != nil {
			s.Fatal("Failed to get the WiFi service information from DUT, err: ", err)
		}

		if serInfo.Wifi.HiddenSsid != ap.Config().Hidden {
			s.Fatalf("Unexpected hidden SSID status: got %t, want %t ", serInfo.Wifi.HiddenSsid, ap.Config().Hidden)
		}

		// TODO(crbug.com/1034875): Assert no deauth detected from the server side.
		// TODO(crbug.com/1034875): Maybe some more check on the WiFi capabilities to
		// verify we really have the settings as expected. (ref: crrev.com/c/1995105)
		s.Log("Deconfiguring")
	}

	testcases := s.Param().([]simpleConnectTestcase)
	for i, tc := range testcases {
		subtest := func(ctx context.Context, s *testing.State) {
			testOnce(ctx, s, tc.apOpts, tc.secConfFac, tc.pingOps, tc.expectedFailure)
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

// byteSequenceStr generates a string from the slice of bytes in [start, end].
// Both start and end are included in the result string.
// If start > end, empty string will be returned.
func byteSequenceStr(start, end byte) string {
	var ret []byte
	if start > end {
		return ""
	}
	for i := start; i < end; i++ {
		ret = append(ret, i)
	}
	ret = append(ret, end)
	return string(ret)
}

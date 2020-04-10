// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wifi

import (
	"context"
	"fmt"
	"strings"

	"chromiumos/tast/common/perf"
	"chromiumos/tast/common/wifi/security"
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
		Attr:        []string{"group:wificell", "wificell_func"},
		ServiceDeps: []string{wificell.TFServiceName},
		Vars:        []string{"router", "pcap"},
		Pre:         wificell.TestFixturePre(),
		Params: []testing.Param{
			{
				// Verifies that DUT can connect to an open 802.11a network on channels 48, 64.
				Name:      "80211a",
				ExtraAttr: []string{"wificell_unstable"},
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
				Name:      "80211n24ht20",
				ExtraAttr: []string{"wificell_unstable"},
				Val: []simpleConnectTestcase{
					{apOpts: []ap.Option{ap.Mode(ap.Mode80211nPure), ap.Channel(1), ap.HTCaps(ap.HTCapHT20)}},
					{apOpts: []ap.Option{ap.Mode(ap.Mode80211nPure), ap.Channel(6), ap.HTCaps(ap.HTCapHT20)}},
					{apOpts: []ap.Option{ap.Mode(ap.Mode80211nPure), ap.Channel(11), ap.HTCaps(ap.HTCapHT20)}},
				},
			}, {
				// Verifies that DUT can connect to an open 802.11n network on 2.4GHz channel 6 with a channel width of 40MHz.
				Name:      "80211n24ht40",
				ExtraAttr: []string{"wificell_unstable"},
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
				Name: "80211nsgi",
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
				// Verifies that DUT can connect to a hidden network on 2.4GHz and 5GHz channels.
				Name: "hidden",
				Val: []simpleConnectTestcase{
					{apOpts: []ap.Option{ap.Mode(ap.Mode80211g), ap.Channel(6), ap.Hidden()}},
					{apOpts: []ap.Option{ap.Mode(ap.Mode80211nPure), ap.Channel(36), ap.HTCaps(ap.HTCapHT20), ap.Hidden()}},
					{apOpts: []ap.Option{ap.Mode(ap.Mode80211nPure), ap.Channel(48), ap.HTCaps(ap.HTCapHT20), ap.Hidden()}},
				},
			}, {
				// Verifies that DUT can connect to a WEP network with both open and shared system authentication and 40-bit pre-shared keys.
				Name:      "wep40",
				ExtraAttr: []string{"wificell_unstable"},
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
				Name:      "wep104",
				ExtraAttr: []string{"wificell_unstable"},
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
				Name:      "wephidden",
				ExtraAttr: []string{"wificell_unstable"},
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
				Name:      "wpatkip",
				ExtraAttr: []string{"wificell_unstable"},
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
				Name:      "wpaccmp",
				ExtraAttr: []string{"wificell_unstable"},
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
				ExtraAttr: []string{"wificell_unstable"},
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
				Name:      "wpa2tkip",
				ExtraAttr: []string{"wificell_unstable"},
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
				ExtraAttr: []string{"wificell_unstable"},
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
				Name:      "wpa2pmfoptional",
				ExtraAttr: []string{"wificell_unstable"},
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
				Name:      "wpa2",
				ExtraAttr: []string{"wificell_unstable"},
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
				Name:      "wpamixed",
				ExtraAttr: []string{"wificell_unstable"},
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
				Name:      "wpaoddpassphrase",
				ExtraAttr: []string{"wificell_unstable"},
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
				Name:      "wpahidden",
				ExtraAttr: []string{"wificell_unstable"},
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
				Name:      "raw_pmk",
				ExtraAttr: []string{"wificell_unstable"},
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
				Name: "dfs",
				Val: []simpleConnectTestcase{
					{apOpts: []ap.Option{ap.Mode(ap.Mode80211nMixed), ap.Channel(136), ap.HTCaps(ap.HTCapHT40)}},
				},
			}, {
				// Verifies that DUT can connect to a networks with the longest and shortest SSID.
				Name: "ssid_limits",
				Val: []simpleConnectTestcase{
					{apOpts: wificell.CommonAPOptions(ap.SSID("a"))},
					{apOpts: wificell.CommonAPOptions(ap.SSID(strings.Repeat("MaxLengthSSID", 4)[:32]))},
				},
			}, {
				// This test case verifies that the DUT accepts ascii and non-ascii type characters as the SSID.
				Name:      "non_ascii_ssid",
				ExtraAttr: []string{"wificell_unstable"},
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
			},
		},
	})
}

func SimpleConnect(fullCtx context.Context, s *testing.State) {
	tf := s.PreValue().(*wificell.TestFixture)
	defer func() {
		if err := tf.CollectLogs(fullCtx); err != nil {
			s.Log("Error collecting logs, err: ", err)
		}
	}()

	pv := perf.NewValues()
	defer func() {
		if err := pv.Save(s.OutDir()); err != nil {
			s.Log("Failed to save perf data, err: ", err)
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

		resp, err := tf.ConnectWifiAP(ctx, ap)
		if err != nil {
			s.Fatal("Failed to connect to WiFi, err: ", err)
		}
		defer func() {
			if err := tf.DisconnectWifi(fullCtx); err != nil {
				s.Error("Failed to disconnect WiFi, err: ", err)
			}
			req := &network.DeleteEntriesForSSIDRequest{Ssid: []byte(ap.Config().SSID)}
			if _, err := tf.WifiClient().DeleteEntriesForSSID(fullCtx, req); err != nil {
				s.Errorf("Failed to remove entries for ssid=%s, err: %v", ap.Config().SSID, err)
			}
		}()
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
			return tf.PingFromDUT(ctx, ap.ServerIP().String())
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

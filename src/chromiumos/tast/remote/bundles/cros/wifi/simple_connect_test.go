// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wifi

import (
	"fmt"
	"strings"
	"testing"

	"chromiumos/tast/common/genparams"
)

type params struct {
	Name              string
	Doc               []string
	ExtraAttr         []string
	Val               string
	ExtraHardwareDeps string
}

func open80211abg() []params {
	mkOps := func(mode string, channels ...int) string {
		var b strings.Builder
		for _, ch := range channels {
			fmt.Fprintf(&b, "{apOpts: []ap.Option{ap.Mode(ap.Mode80211%s), ap.Channel(%d)}},\n", mode, ch)
		}
		return b.String()
	}
	return []params{{
		Name:      "80211a",
		Doc:       []string{"Verifies that DUT can connect to an open 802.11a network on channels 48, 64."},
		ExtraAttr: []string{"wificell_unstable"},
		Val:       mkOps("a", 48, 64),
	}, {
		Name: "80211b",
		Doc:  []string{"Verifies that DUT can connect to an open 802.11b network on channels 1, 6, 11."},
		Val:  mkOps("b", 1, 6, 11),
	}, {
		Name: "80211g",
		Doc:  []string{"Verifies that DUT can connect to an open 802.11g network on channels 1, 6, 11."},
		Val:  mkOps("g", 1, 6, 11),
	}}
}

func open80211n() []params {
	mkOps := func(htCaps string, channels ...int) string {
		var b strings.Builder
		for _, ch := range channels {
			fmt.Fprintf(&b, "{apOpts: []ap.Option{ap.Mode(ap.Mode80211nPure), ap.Channel(%d), ap.HTCaps(%s)}},\n", ch, htCaps)
		}
		return b.String()
	}
	return []params{{
		Name:      "80211n24ht20",
		Doc:       []string{"Verifies that DUT can connect to an open 802.11n network on 2.4GHz channels 1, 6, 11 with a channel width of 20MHz."},
		ExtraAttr: []string{"wificell_unstable"},
		Val:       mkOps("ap.HTCapHT20", 1, 6, 11),
	}, {
		Name:      "80211n24ht40",
		Doc:       []string{"Verifies that DUT can connect to an open 802.11n network on 2.4GHz channel 6 with a channel width of 40MHz."},
		ExtraAttr: []string{"wificell_unstable"},
		Val:       mkOps("ap.HTCapHT40", 6),
	}, {
		Name: "80211n5ht20",
		Doc:  []string{"Verifies that DUT can connect to an open 802.11n network on 5GHz channel 48 with a channel width of 20MHz."},
		Val:  mkOps("ap.HTCapHT20", 48),
	}, {
		Name: "80211n5ht40",
		Doc: []string{
			"Verifies that DUT can connect to an open 802.11n network on 5GHz channel 48",
			"(40MHz channel with the second 20MHz chunk of the 40MHz channel on the channel below the center channel).",
		},
		Val: mkOps("ap.HTCapHT40Minus", 48),
	}}
}

func open80211nsgi() []params {
	return []params{{
		Name: "80211nsgi",
		Doc:  []string{"This test verifies that DUT can connect to an open 802.11n network on 5 GHz channel with short guard intervals enabled (both 20/40 Mhz)."},
		Val: `{apOpts: []ap.Option{ap.Mode(ap.Mode80211nPure), ap.Channel(48), ap.HTCaps(ap.HTCapHT20, ap.HTCapSGI20)}},
		      {apOpts: []ap.Option{ap.Mode(ap.Mode80211nPure), ap.Channel(48), ap.HTCaps(ap.HTCapHT40Minus, ap.HTCapSGI40)}},`,
	}}
}

func open80211ac() []params {
	mkOps := func(mode string, channel int, chWid, vht string) string {
		return fmt.Sprintf("{apOpts: []ap.Option{\nap.Mode(ap.Mode80211ac%s), ap.Channel(%d), ap.HTCaps(ap.HTCapHT%s),\n%s,\n}},", mode, channel, chWid, vht)
	}
	vhtWid20Or40 := "ap.VHTChWidth(ap.VHTChWidth20Or40)"
	mkVHT80 := func(centerCh int) string {
		return fmt.Sprintf("ap.VHTCaps(ap.VHTCapSGI80), ap.VHTCenterChannel(%d), ap.VHTChWidth(ap.VHTChWidth80)", centerCh)
	}

	return []params{{
		Name:              "80211acvht20",
		Doc:               []string{"Verifies that DUT can connect to an open 802.11ac network on channel 60 with a channel width of 20MHz."},
		Val:               mkOps("Pure", 60, "20", vhtWid20Or40),
		ExtraHardwareDeps: "hwdep.D(hwdep.Wifi80211ac())",
	}, {
		Name:              "80211acvht40",
		Doc:               []string{"Verifies that DUT can connect to an open 802.11ac network on channel 48 with a channel width of 40MHz."},
		Val:               mkOps("Pure", 48, "40", vhtWid20Or40),
		ExtraHardwareDeps: "hwdep.D(hwdep.Wifi80211ac())",
	}, {
		Name: "80211acvht80mixed",
		Doc:  []string{"Verifies that DUT can connect to an open 802.11ac network on 5GHz channel 36 with center channel of 42 and channel width of 80MHz."},
		Val:  mkOps("Mixed", 36, "40Plus", mkVHT80(42)),
	}, {
		Name: "80211acvht80pure",
		Doc: []string{
			"Verifies that DUT can connect to an open 802.11ac network on channel 157 with center channel of 155 and channel width of 80MHz.",
			"The router is forced to use 80 MHz wide rates only.",
		},
		Val:               mkOps("Pure", 157, "40Plus", mkVHT80(155)),
		ExtraHardwareDeps: "hwdep.D(hwdep.Wifi80211ac())",
	}}
}

func TestSimpleConnect(t *testing.T) {
	var ps []params

	ps = append(ps, open80211abg()...)
	ps = append(ps, open80211n()...)
	ps = append(ps, open80211nsgi()...)
	ps = append(ps, open80211ac()...)

	genparams.Ensure(t, "simple_connect.go", genparams.Template(t, `{{ range . }}{
	{{ range .Doc }}
	// {{ . }}
	{{ end }}
	Name: {{ .Name | fmt }},
	{{ if .ExtraAttr }}
	ExtraAttr: {{ .ExtraAttr | fmt }},
	{{ end }}
	Val:  []simpleConnectTestcase{
		{{ .Val }}
	},
	{{ if .ExtraHardwareDeps }}
	ExtraHardwareDeps: {{ .ExtraHardwareDeps }},
	{{ end }}
},{{ end }}{
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
	// Verifies that DUT can connect to an AP in WPA2/WPA3 mixed mode. WiFi alliance suggests PMF in this mode.
	Name:      "wpa3mixed",
	ExtraAttr: []string{"wificell_unstable"},
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
	Name: "ssid_limits",
	Val: []simpleConnectTestcase{
		{apOpts: wificell.CommonAPOptions(ap.SSID("a"))},
		{apOpts: wificell.CommonAPOptions(ap.SSID(strings.Repeat("MaxLengthSSID", 4)[:32]))},
	},
}, {
	// This test case verifies that the DUT accepts ascii and non-ascii type characters as the SSID.
	Name:      "non_ascii_ssid",
	ExtraAttr: []string{"wificell_unstable"},
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
	Name:      "8021xwep",
	ExtraAttr: []string{"wificell_unstable"},
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
				tunneled1x.AltSubjectMatch([]string{`+"`"+`{"Type":"DNS","Value":"wrong_dns.com"}`+"`"+`}),
			),
			expectedFailure: true,
		},
	},
}, {
	// Verifies that DUT can connect to a protected network supporting for PEAP authentication with tunneled MSCHAPV2.
	Name:      "8021xpeap_mschapv2",
	ExtraAttr: []string{"wificell_unstable"},
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
				tunneled1x.AltSubjectMatch([]string{`+"`"+`{"Type":"DNS","Value":"wrong_dns.com"}`+"`"+`, eapCert3AltSub[0]}),
			),
		},
	},
}, {
	// Verifies that DUT can connect to a protected network supporting for PEAP authentication with tunneled MD5.
	Name:      "8021xpeap_md5",
	ExtraAttr: []string{"wificell_unstable"},
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
				tunneled1x.AltSubjectMatch([]string{`+"`"+`{"Type":"DNS","Value":"wrong_dns.com"}`+"`"+`, eapCert3AltSub[0]}),
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
				tunneled1x.AltSubjectMatch([]string{`+"`"+`{"Type":"DNS","Value":"wrong_dns.com"}`+"`"+`, eapCert3AltSub[0]}),
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
				tunneled1x.AltSubjectMatch([]string{`+"`"+`{"Type":"DNS","Value":"wrong_dns.com"}`+"`"+`}),
			),
			expectedFailure: true,
		},
	},
}, {
	// Verifies that DUT can connect to a protected network supporting for TTLS authentication with tunneled MSCHAPV2.
	Name:      "8021xttls_mschapv2",
	ExtraAttr: []string{"wificell_unstable"},
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
				tunneled1x.AltSubjectMatch([]string{`+"`"+`{"Type":"DNS","Value":"wrong_dns.com"}`+"`"+`, eapCert3AltSub[0]}),
			),
		},
	},
}, {
	// Verifies that DUT can connect to a protected network supporting for TTLS authentication with tunneled MD5.
	Name:      "8021xttls_md5",
	ExtraAttr: []string{"wificell_unstable"},
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
				tunneled1x.AltSubjectMatch([]string{`+"`"+`{"Type":"DNS","Value":"wrong_dns.com"}`+"`"+`, eapCert3AltSub[0]}),
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
				tunneled1x.AltSubjectMatch([]string{`+"`"+`{"Type":"DNS","Value":"wrong_dns.com"}`+"`"+`, eapCert3AltSub[0]}),
			),
		},
	},
}, {
	// Verifies that DUT can connect to a protected network supporting for TTLS authentication with tunneled TTLSMSCHAPV2.
	Name:      "8021xttls_ttlsmschapv2",
	ExtraAttr: []string{"wificell_unstable"},
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
				tunneled1x.AltSubjectMatch([]string{`+"`"+`{"Type":"DNS","Value":"wrong_dns.com"}`+"`"+`, eapCert3AltSub[0]}),
			),
		},
	},
}, {
	// Verifies that DUT can connect to a protected network supporting for TTLS authentication with tunneled TTLSMSCHAP.
	Name:      "8021xttls_ttlsmschap",
	ExtraAttr: []string{"wificell_unstable"},
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
				tunneled1x.AltSubjectMatch([]string{`+"`"+`{"Type":"DNS","Value":"wrong_dns.com"}`+"`"+`, eapCert3AltSub[0]}),
			),
		},
	},
}, {
	// Verifies that DUT can connect to a protected network supporting for TTLS authentication with tunneled TTLSPAP.
	Name:      "8021xttls_ttlspap",
	ExtraAttr: []string{"wificell_unstable"},
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
				tunneled1x.AltSubjectMatch([]string{`+"`"+`{"Type":"DNS","Value":"wrong_dns.com"}`+"`"+`, eapCert3AltSub[0]}),
			),
		},
	},
},
`, ps))
}

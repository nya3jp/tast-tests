// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// This test file generates the Params in simple_connect.go.
// After modified, to overwrite the old Params, run:
// TAST_GENERATE_UPDATE=1 ~/trunk/src/platform/tast/tools/go.sh test -count=1 chromiumos/tast/remote/bundles/cros/wifi
// To check only, run:
// ~/trunk/src/platform/fast_build.sh -t chromiumos/tast/remote/bundles/cros/wifi

package wifi

import (
	"fmt"
	"strings"
	"testing"

	"chromiumos/tast/common/genparams"
)

type params struct {
	Name                 string
	Doc                  []string
	ExtraAttr            []string
	ExtraHardwareDeps    string
	ExtraHardwareDepsDoc []string
	Val                  string
}

func param80211abg() []params {
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

func param80211n() []params {
	mkOps := func(htCaps string, channels ...int) string {
		var b strings.Builder
		for _, ch := range channels {
			fmt.Fprintf(&b, "{apOpts: []ap.Option{ap.Mode(ap.Mode80211nPure), ap.Channel(%d), ap.HTCaps(ap.HTCap%s)}},\n", ch, htCaps)
		}
		return b.String()
	}
	return []params{{
		Name:      "80211n24ht20",
		Doc:       []string{"Verifies that DUT can connect to an open 802.11n network on 2.4GHz channels 1, 6, 11 with a channel width of 20MHz."},
		ExtraAttr: []string{"wificell_unstable"},
		Val:       mkOps("HT20", 1, 6, 11),
	}, {
		Name:      "80211n24ht40",
		Doc:       []string{"Verifies that DUT can connect to an open 802.11n network on 2.4GHz channel 6 with a channel width of 40MHz."},
		ExtraAttr: []string{"wificell_unstable"},
		Val:       mkOps("HT40", 6),
	}, {
		Name: "80211n5ht20",
		Doc:  []string{"Verifies that DUT can connect to an open 802.11n network on 5GHz channel 48 with a channel width of 20MHz."},
		Val:  mkOps("HT20", 48),
	}, {
		Name: "80211n5ht40",
		Doc: []string{
			"Verifies that DUT can connect to an open 802.11n network on 5GHz channel 48",
			"(40MHz channel with the second 20MHz chunk of the 40MHz channel on the channel below the center channel)."},
		Val: mkOps("HT40Minus", 48),
	}}
}

func param80211nsgi() params {
	return params{
		Name: "80211nsgi",
		Doc:  []string{"This test verifies that DUT can connect to an open 802.11n network on 5 GHz channel with short guard intervals enabled (both 20/40 Mhz)."},
		Val: `{apOpts: []ap.Option{ap.Mode(ap.Mode80211nPure), ap.Channel(48), ap.HTCaps(ap.HTCapHT20, ap.HTCapSGI20)}},
		      {apOpts: []ap.Option{ap.Mode(ap.Mode80211nPure), ap.Channel(48), ap.HTCaps(ap.HTCapHT40Minus, ap.HTCapSGI40)}},`,
	}
}

func param80211ac() []params {
	return []params{{
		Name: "80211acvht20",
		Doc:  []string{"Verifies that DUT can connect to an open 802.11ac network on channel 60 with a channel width of 20MHz."},
		Val: `{apOpts: []ap.Option{
			ap.Mode(ap.Mode80211acPure), ap.Channel(60), ap.HTCaps(ap.HTCapHT20),
			ap.VHTChWidth(ap.VHTChWidth20Or40),
		}},`,
		ExtraHardwareDeps: "hwdep.D(hwdep.Wifi80211ac())",
	}, {
		Name: "80211acvht40",
		Doc:  []string{"Verifies that DUT can connect to an open 802.11ac network on channel 48 with a channel width of 40MHz."},
		Val: `{apOpts: []ap.Option{
			ap.Mode(ap.Mode80211acPure), ap.Channel(48), ap.HTCaps(ap.HTCapHT40),
			ap.VHTChWidth(ap.VHTChWidth20Or40),
		}},`,
		ExtraHardwareDeps: "hwdep.D(hwdep.Wifi80211ac())",
	}, {
		Name: "80211acvht80mixed",
		Doc:  []string{"Verifies that DUT can connect to an open 802.11ac network on 5GHz channel 36 with center channel of 42 and channel width of 80MHz."},
		Val: `{apOpts: []ap.Option{
			ap.Mode(ap.Mode80211acMixed), ap.Channel(36), ap.HTCaps(ap.HTCapHT40Plus),
			ap.VHTCaps(ap.VHTCapSGI80), ap.VHTCenterChannel(42), ap.VHTChWidth(ap.VHTChWidth80),
		}},`,
	}, {
		Name: "80211acvht80pure",
		Doc: []string{
			"Verifies that DUT can connect to an open 802.11ac network on channel 157 with center channel of 155 and channel width of 80MHz.",
			"The router is forced to use 80 MHz wide rates only."},
		Val: `{apOpts: []ap.Option{
			ap.Mode(ap.Mode80211acPure), ap.Channel(157), ap.HTCaps(ap.HTCapHT40Plus),
			ap.VHTCaps(ap.VHTCapSGI80), ap.VHTCenterChannel(155), ap.VHTChWidth(ap.VHTChWidth80),
		}},`,
		ExtraHardwareDeps: "hwdep.D(hwdep.Wifi80211ac())",
	}}
}

func paramHidden() params {
	return params{
		Name: "hidden",
		Doc:  []string{"Verifies that DUT can connect to a hidden network on 2.4GHz and 5GHz channels."},
		Val: `{apOpts: []ap.Option{ap.Mode(ap.Mode80211g), ap.Channel(6), ap.Hidden()}},
		      {apOpts: []ap.Option{ap.Mode(ap.Mode80211nPure), ap.Channel(36), ap.HTCaps(ap.HTCapHT20), ap.Hidden()}},
		      {apOpts: []ap.Option{ap.Mode(ap.Mode80211nPure), ap.Channel(48), ap.HTCaps(ap.HTCapHT20), ap.Hidden()}},`,
	}
}

func paramWEP() []params {
	mkOps := func(keyLen int) string {
		var b strings.Builder
		for _, algo := range []string{"Open", "Shared"} {
			for key := 0; key < 4; key++ {
				fmt.Fprintf(&b, `{
					apOpts:     []ap.Option{ap.Mode(ap.Mode80211g), ap.Channel(1)},
					secConfFac: wep.NewConfigFactory(wep%dKeys(), wep.DefaultKey(%d), wep.AuthAlgs(wep.AuthAlgo%s)),
				},
				`, keyLen, key, algo)
			}
		}
		return b.String()
	}
	return []params{{
		Name:      "wep40",
		Doc:       []string{"Verifies that DUT can connect to a WEP network with both open and shared system authentication and 40-bit pre-shared keys."},
		ExtraAttr: []string{"wificell_unstable"},
		Val:       mkOps(40),
	}, {
		Doc:       []string{"Verifies that DUT can connect to a WEP network with both open and shared system authentication and 104-bit pre-shared keys."},
		Name:      "wep104",
		ExtraAttr: []string{"wificell_unstable"},
		Val:       mkOps(104),
	}}
}

func paramWEPHidden() params {
	var b strings.Builder
	for _, keyLen := range []int{40, 104} {
		for _, algo := range []string{"Open", "Shared"} {
			fmt.Fprintf(&b, `{
				apOpts:     []ap.Option{ap.Mode(ap.Mode80211g), ap.Channel(1), ap.Hidden()},
				secConfFac: wep.NewConfigFactory(wep%dKeysHidden(), wep.AuthAlgs(wep.AuthAlgo%s)),
			},
			`, keyLen, algo)
		}
	}
	return params{
		Name:      "wephidden",
		Doc:       []string{"Verifies that DUT can connect to a hidden WEP network with open/shared system authentication and 40/104-bit pre-shared keys."},
		ExtraAttr: []string{"wificell_unstable"},
		Val:       b.String(),
	}
}

func paramWPA() []params {
	mkOps := func(pmf, mode, cipher string) string {
		return fmt.Sprintf(`{
			apOpts: []ap.Option{ap.Mode(ap.Mode80211g), ap.Channel(1), %s},
			secConfFac: wpa.NewConfigFactory(
				"chromeos", wpa.Mode(wpa.Mode%s),
				%s
			),
		},`, pmf, mode, cipher)
	}
	return []params{{
		Name:      "wpatkip",
		Doc:       []string{"Verifies that DUT can connect to a protected network supporting for pure WPA with TKIP."},
		ExtraAttr: []string{"wificell_unstable"},
		Val:       mkOps("", "PureWPA", "wpa.Ciphers(wpa.CipherTKIP),"),
	}, {
		Name:      "wpaccmp",
		Doc:       []string{"Verifies that DUT can connect to a protected network supporting for pure WPA with AES based CCMP."},
		ExtraAttr: []string{"wificell_unstable"},
		Val:       mkOps("", "PureWPA", "wpa.Ciphers(wpa.CipherCCMP),"),
	}, {
		Name:      "wpamulti",
		Doc:       []string{"Verifies that DUT can connect to a protected network supporting for pure WPA with both AES based CCMP and TKIP."},
		ExtraAttr: []string{"wificell_unstable"},
		Val:       mkOps("", "PureWPA", "wpa.Ciphers(wpa.CipherTKIP, wpa.CipherCCMP),"),
	}, {
		Name:      "wpa2tkip",
		Doc:       []string{"Verifies that DUT can connect to a protected network supporting for WPA2 (aka RSN) with TKIP. Some AP still uses TKIP in WPA2."},
		ExtraAttr: []string{"wificell_unstable"},
		Val:       mkOps("", "PureWPA2", "wpa.Ciphers2(wpa.CipherTKIP),"),
	}, {
		Name: "wpa2pmf",
		Doc: []string{
			"Verifies that we can connect to an AP broadcasting a WPA2 network using AES based CCMP.",
			"In addition, the client must also support 802.11w protected management frames."},
		ExtraAttr: []string{"wificell_unstable"},
		Val:       mkOps("ap.PMF(ap.PMFRequired)", "PureWPA2", "wpa.Ciphers2(wpa.CipherCCMP),"),
	}, {
		Name: "wpa2pmfoptional",
		Doc: []string{
			"Verifies that we can connect to an AP broadcasting a WPA2 network using AES based CCMP.",
			"In addition, the client may also negotiate use of 802.11w protected management frames."},
		ExtraAttr: []string{"wificell_unstable"},
		Val:       mkOps("ap.PMF(ap.PMFOptional)", "PureWPA2", "wpa.Ciphers2(wpa.CipherCCMP),"),
	}, {
		Name:      "wpa2",
		Doc:       []string{"Verifies that DUT can connect to a protected network supporting for WPA2 (aka RSN) and encrypted under AES."},
		ExtraAttr: []string{"wificell_unstable"},
		Val:       mkOps("", "PureWPA2", "wpa.Ciphers2(wpa.CipherCCMP),"),
	}, {
		Name:      "wpamixed",
		Doc:       []string{"Verifies that DUT can connect to a protected network supporting for both WPA and WPA2 with TKIP/AES supported for WPA and AES supported for WPA2."},
		ExtraAttr: []string{"wificell_unstable"},
		Val:       mkOps("", "Mixed", "wpa.Ciphers(wpa.CipherTKIP, wpa.CipherCCMP),\nwpa.Ciphers2(wpa.CipherCCMP),"),
	}}
}

func paramWPA3() params {
	return params{
		Name:      "wpa3mixed",
		Doc:       []string{"Verifies that DUT can connect to an AP in WPA2/WPA3 mixed mode. WiFi alliance suggests PMF in this mode."},
		ExtraAttr: []string{"wificell_unstable"},
		Val: `{
			apOpts: []ap.Option{
				ap.Mode(ap.Mode80211acMixed), ap.Channel(36), ap.HTCaps(ap.HTCapHT40Plus),
				ap.VHTCenterChannel(42), ap.VHTChWidth(ap.VHTChWidth80),
				ap.PMF(ap.PMFOptional),
			},
			secConfFac: wpa.NewConfigFactory(
				"chromeos", wpa.Mode(wpa.ModeMixedWPA3),
				wpa.Ciphers2(wpa.CipherCCMP),
			),
		},`,
	}
}

func paramWPAVHT80() params {
	return params{
		Name: "wpavht80",
		Doc:  []string{"Verifies that DUT can connect to a protected 802.11ac network supporting for WPA."},
		Val: `{
			apOpts: []ap.Option{
				ap.Mode(ap.Mode80211acPure), ap.Channel(36), ap.HTCaps(ap.HTCapHT40Plus),
				ap.VHTCenterChannel(42), ap.VHTChWidth(ap.VHTChWidth80),
			},
			secConfFac: wpa.NewConfigFactory(
				"chromeos", wpa.Mode(wpa.ModePureWPA),
				wpa.Ciphers(wpa.CipherTKIP, wpa.CipherCCMP),
			),
		},`,
		ExtraHardwareDeps: "hwdep.D(hwdep.Wifi80211ac())",
	}
}

func paramWPAOddPassphrase() params {
	var b strings.Builder
	for _, p := range []string{`"\xe4\xb8\x80\xe4\xba\x8c\xe4\xb8\x89"`, `"abcdef\xc2\xa2"`, `" !\"#$%&'()>*+,-./:;<=>?@[\\]^_{|}~"`} {
		temp := `{
			apOpts: []ap.Option{ap.Mode(ap.Mode80211g), ap.Channel(1)},
			secConfFac: wpa.NewConfigFactory(
				%s, wpa.Mode(wpa.ModePure%s),
				%s,
			),
		},
		`
		fmt.Fprintf(&b, temp, p, "WPA", "wpa.Ciphers(wpa.CipherTKIP)")
		fmt.Fprintf(&b, temp, p, "WPA2", "wpa.Ciphers2(wpa.CipherCCMP)")
	}
	return params{
		Name:      "wpaoddpassphrase",
		Doc:       []string{"Verifies that DUT can connect to a protected network whose WPA passphrase can be pure unicode, mixed unicode and ASCII, and all the punctuations."},
		ExtraAttr: []string{"wificell_unstable"},
		Val:       b.String(),
	}
}

func paramWPAHidden() params {
	var b strings.Builder
	for _, c := range []struct {
		mode   string
		cipher string
	}{
		{mode: "PureWPA", cipher: "wpa.Ciphers(wpa.CipherTKIP)"},
		{mode: "PureWPA", cipher: "wpa.Ciphers(wpa.CipherTKIP, wpa.CipherCCMP)"},
		{mode: "PureWPA2", cipher: "wpa.Ciphers2(wpa.CipherCCMP)"},
		{mode: "Mixed", cipher: "wpa.Ciphers(wpa.CipherTKIP, wpa.CipherCCMP),\nwpa.Ciphers2(wpa.CipherCCMP)"},
	} {
		fmt.Fprintf(&b, `{
			apOpts: []ap.Option{ap.Mode(ap.Mode80211g), ap.Channel(1), ap.Hidden()},
			secConfFac: wpa.NewConfigFactory(
				"chromeos", wpa.Mode(wpa.Mode%s),
				%s,
			),
		},
		`, c.mode, c.cipher)
	}
	return params{
		Name:      "wpahidden",
		Doc:       []string{"Verifies that DUT can connect to a hidden network supporting for WPA with TKIP, WPA with TKIP/AES, WPA2 with AES, and mixed WPA with TKIP/AES and WPA2 with AES."},
		ExtraAttr: []string{"wificell_unstable"},
		Val:       b.String(),
	}
}

func paramRawPMK() params {
	return params{
		Name:      "raw_pmk",
		Doc:       []string{"Verifies that DUT can connect to a WPA network using a raw PMK value instead of an ASCII passphrase."},
		ExtraAttr: []string{"wificell_unstable"},
		Val: `{
			apOpts: []ap.Option{ap.Mode(ap.Mode80211g), ap.Channel(1)},
			secConfFac: wpa.NewConfigFactory(
				strings.Repeat("0123456789abcdef", 4), // length = 64.
				wpa.Mode(wpa.ModePureWPA),
				wpa.Ciphers(wpa.CipherTKIP),
			),
		},`,
	}
}

func paramDFS() []params {
	return []params{{
		Name: "dfs",
		Doc: []string{
			"Verifies that DUT can connect to an open network on a DFS channel.",
			"DFS (dynamic frequency selection) channels are channels that may be unavailable if radar interference is detected.",
			"See: https://en.wikipedia.org/wiki/Dynamic_frequency_selection, https://en.wikipedia.org/wiki/List_of_WLAN_channels",
		},
		Val: "{apOpts: []ap.Option{ap.Mode(ap.Mode80211nMixed), ap.Channel(136), ap.HTCaps(ap.HTCapHT40)}},",
	}, {
		Name: "dfs_ch120",
		Doc: []string{
			"Verifies that DUT can connect to an open network on the DFS channel 120 (5600MHz).",
			"TODO(b/154440798): Investigate why this fails on veyron_mickey and consider merge this with",
			"\"dfs\" case after resolution.",
		},
		ExtraAttr: []string{"wificell_unstable"},
		Val:       "{apOpts: []ap.Option{ap.Mode(ap.Mode80211nMixed), ap.Channel(120), ap.HTCaps(ap.HTCapHT40)}},",
	}}
}

func paramSSIDLimits() params {
	return params{
		Name: "ssid_limits",
		Doc:  []string{"Verifies that DUT can connect to a networks with the longest and shortest SSID."},
		Val: `{apOpts: wificell.CommonAPOptions(ap.SSID("a"))},
		      {apOpts: wificell.CommonAPOptions(ap.SSID(strings.Repeat("MaxLengthSSID", 4)[:32]))},`,
	}
}

func paramNonASCIISSID() params {
	return params{
		Name:                 "non_ascii_ssid",
		Doc:                  []string{"This test case verifies that the DUT accepts ascii and non-ascii type characters as the SSID."},
		ExtraAttr:            []string{"wificell_unstable"},
		ExtraHardwareDeps:    "hwdep.D(hwdep.WifiNotMarvell())",
		ExtraHardwareDepsDoc: []string{"TODO(b/158150763): Skip Marvell WiFi as there's a known issue to make the test always fail."},
		Val: `// TODO(crbug.com/1082582): shill don't allow leading 0x00 now, so let's append it in the
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
		      {apOpts: wificell.CommonAPOptions(ap.SSID("Chrome\xe7\xac\x94\xe8\xae\xb0\xe6\x9c\xac"))},`,
	}
}

func param8021xWEP() params {
	return params{
		Name:      "8021xwep",
		Doc:       []string{"Verifies that DUT can connect to a protected network supporting for dynamic WEP encryption."},
		ExtraAttr: []string{"wificell_unstable"},
		Val: `{
			apOpts: []ap.Option{ap.Mode(ap.Mode80211g), ap.Channel(1)},
			secConfFac: dynamicwep.NewConfigFactory(
				eapCert1.CACert, eapCert1.ServerCred,
				dynamicwep.ClientCACert(eapCert1.CACert),
				dynamicwep.ClientCred(eapCert1.ClientCred),
				dynamicwep.RekeyPeriod(10),
			),
			pingOps: []ping.Option{ping.Count(15), ping.Interval(1)},
		},`,
	}
}

func param8021xWPA() params {
	return params{
		Name:      "8021xwpa",
		Doc:       []string{"Verifies that DUT can connect to a protected network supporting for WPA-EAP encryption."},
		ExtraAttr: []string{"wificell_unstable"},
		Val: `{
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
		},`,
	}
}

func paramTunneled1x() []params {
	mkFailOps := func(outer, inner string) string {
		return fmt.Sprintf(`{
			// Failure due to bad password.
			apOpts: []ap.Option{ap.Mode(ap.Mode80211g), ap.Channel(1)},
			secConfFac: tunneled1x.NewConfigFactory(
				eapCert1.CACert, eapCert1.ServerCred, eapCert1.CACert, "testuser", "password",
				tunneled1x.OuterProtocol(tunneled1x.Layer1Type%[1]s),
				tunneled1x.InnerProtocol(tunneled1x.Layer2Type%[2]s),
				tunneled1x.ClientPassword("wrongpassword"),
			),
			expectedFailure: true,
		},
		{
			// Failure due to wrong client CA.
			apOpts: []ap.Option{ap.Mode(ap.Mode80211g), ap.Channel(1)},
			secConfFac: tunneled1x.NewConfigFactory(
				eapCert1.CACert, eapCert1.ServerCred, eapCert2.CACert, "testuser", "password",
				tunneled1x.OuterProtocol(tunneled1x.Layer1Type%[1]s),
				tunneled1x.InnerProtocol(tunneled1x.Layer2Type%[2]s),
			),
			expectedFailure: true,
		},
		{
			// Failure due to expired server cred.
			apOpts: []ap.Option{ap.Mode(ap.Mode80211g), ap.Channel(1)},
			secConfFac: tunneled1x.NewConfigFactory(
				eapCert1.CACert, eapCert1.ExpiredServerCred, eapCert1.CACert, "testuser", "password",
				tunneled1x.OuterProtocol(tunneled1x.Layer1Type%[1]s),
				tunneled1x.InnerProtocol(tunneled1x.Layer2Type%[2]s),
			),
			expectedFailure: true,
		},
		{
			// Failure due to that a subject alternative name (SAN) is set but does not match any of the server certificate SANs.
			apOpts: []ap.Option{ap.Mode(ap.Mode80211g), ap.Channel(1)},
			secConfFac: tunneled1x.NewConfigFactory(
				eapCert3.CACert, eapCert3.ServerCred, eapCert3.CACert, "testuser", "password",
				tunneled1x.OuterProtocol(tunneled1x.Layer1Type%[1]s),
				tunneled1x.InnerProtocol(tunneled1x.Layer2Type%[2]s),
				tunneled1x.AltSubjectMatch([]string{`+"`"+`{"Type":"DNS","Value":"wrong_dns.com"}`+"`"+`}),
			),
			expectedFailure: true,
		},
		`, outer, inner)
	}
	mkOps := func(outer, inner string) string {
		var b strings.Builder
		fmt.Fprintf(&b, `{
			apOpts: []ap.Option{ap.Mode(ap.Mode80211g), ap.Channel(1)},
			secConfFac: tunneled1x.NewConfigFactory(
				eapCert1.CACert, eapCert1.ServerCred, eapCert1.CACert, "testuser", "password",
				tunneled1x.OuterProtocol(tunneled1x.Layer1Type%s),
				tunneled1x.InnerProtocol(tunneled1x.Layer2Type%s),
			),
		},
		`, outer, inner)
		for i := 0; i < 3; i++ {
			fmt.Fprintf(&b, `{
				apOpts: []ap.Option{ap.Mode(ap.Mode80211g), ap.Channel(1)},
				secConfFac: tunneled1x.NewConfigFactory(
					eapCert3.CACert, eapCert3.ServerCred, eapCert3.CACert, "testuser", "password",
					tunneled1x.OuterProtocol(tunneled1x.Layer1Type%s),
					tunneled1x.InnerProtocol(tunneled1x.Layer2Type%s),
					tunneled1x.AltSubjectMatch([]string{eapCert3AltSub[%d]}),
				),
			},
			`, outer, inner, i)
		}
		fmt.Fprintf(&b, `{
			// Should success since having multiple entries in 'altsubject_match' is treated as OR, not AND.
			// For more information about how wpa_supplicant uses altsubject_match field:
			// https://w1.fi/cgit/hostap/plain/wpa_supplicant/wpa_supplicant.conf
			apOpts: []ap.Option{ap.Mode(ap.Mode80211g), ap.Channel(1)},
			secConfFac: tunneled1x.NewConfigFactory(
				eapCert3.CACert, eapCert3.ServerCred, eapCert3.CACert, "testuser", "password",
				tunneled1x.OuterProtocol(tunneled1x.Layer1Type%s),
				tunneled1x.InnerProtocol(tunneled1x.Layer2Type%s),
				tunneled1x.AltSubjectMatch([]string{`+"`"+`{"Type":"DNS","Value":"wrong_dns.com"}`+"`"+`, eapCert3AltSub[0]}),
			),
		},
		`, outer, inner)
		return b.String()
	}
	return []params{{
		Name: "8021xpeap_fail",
		Doc: []string{
			"Verifies that DUT CANNOT connect to a PEAP network with wrong settings.",
			"We do these tests for only one inner authentication protocol because we",
			"presume that supplicant reuses this code between inner authentication types."},
		ExtraAttr: []string{"wificell_unstable"},
		Val:       mkFailOps("PEAP", "MSCHAPV2"),
	}, {
		Name:      "8021xpeap_mschapv2",
		Doc:       []string{"Verifies that DUT can connect to a protected network supporting for PEAP authentication with tunneled MSCHAPV2."},
		ExtraAttr: []string{"wificell_unstable"},
		Val:       mkOps("PEAP", "MSCHAPV2"),
	}, {
		Name:      "8021xpeap_md5",
		Doc:       []string{"Verifies that DUT can connect to a protected network supporting for PEAP authentication with tunneled MD5."},
		ExtraAttr: []string{"wificell_unstable"},
		Val:       mkOps("PEAP", "MD5"),
	}, {
		Name:      "8021xpeap_gtc",
		Doc:       []string{"Verifies that DUT can connect to a protected network supporting for PEAP authentication with tunneled GTC."},
		ExtraAttr: []string{"wificell_unstable"},
		Val:       mkOps("PEAP", "GTC"),
	}, {
		Name: "8021xttls_fail",
		Doc: []string{
			"Verifies that DUT CANNOT connect to a TTLS network with wrong settings.",
			"We do these tests for only one inner authentication protocol because we",
			"presume that supplicant reuses this code between inner authentication types."},
		ExtraAttr: []string{"wificell_unstable"},
		Val:       mkFailOps("TTLS", "MD5"),
	}, {
		Name:      "8021xttls_mschapv2",
		Doc:       []string{"Verifies that DUT can connect to a protected network supporting for TTLS authentication with tunneled MSCHAPV2."},
		ExtraAttr: []string{"wificell_unstable"},
		Val:       mkOps("TTLS", "MSCHAPV2"),
	}, {
		Name:      "8021xttls_md5",
		Doc:       []string{"Verifies that DUT can connect to a protected network supporting for TTLS authentication with tunneled MD5."},
		ExtraAttr: []string{"wificell_unstable"},
		Val:       mkOps("TTLS", "MD5"),
	}, {
		Name:      "8021xttls_gtc",
		Doc:       []string{"Verifies that DUT can connect to a protected network supporting for TTLS authentication with tunneled GTC."},
		ExtraAttr: []string{"wificell_unstable"},
		Val:       mkOps("TTLS", "GTC"),
	}, {
		Name:      "8021xttls_ttlsmschapv2",
		Doc:       []string{"Verifies that DUT can connect to a protected network supporting for TTLS authentication with tunneled TTLSMSCHAPV2."},
		ExtraAttr: []string{"wificell_unstable"},
		Val:       mkOps("TTLS", "TTLSMSCHAPV2"),
	}, {
		Name:      "8021xttls_ttlsmschap",
		Doc:       []string{"Verifies that DUT can connect to a protected network supporting for TTLS authentication with tunneled TTLSMSCHAP."},
		ExtraAttr: []string{"wificell_unstable"},
		Val:       mkOps("TTLS", "TTLSMSCHAP"),
	}, {
		Name:      "8021xttls_ttlspap",
		Doc:       []string{"Verifies that DUT can connect to a protected network supporting for TTLS authentication with tunneled TTLSPAP."},
		ExtraAttr: []string{"wificell_unstable"},
		Val:       mkOps("TTLS", "TTLSPAP"),
	}}
}

func TestSimpleConnect(t *testing.T) {
	var ps []params
	ps = append(ps, param80211abg()...)
	ps = append(ps, param80211n()...)
	ps = append(ps, param80211nsgi())
	ps = append(ps, param80211ac()...)
	ps = append(ps, paramHidden())
	ps = append(ps, paramWEP()...)
	ps = append(ps, paramWEPHidden())
	ps = append(ps, paramWPA()...)
	ps = append(ps, paramWPA3())
	ps = append(ps, paramWPAVHT80())
	ps = append(ps, paramWPAOddPassphrase())
	ps = append(ps, paramWPAHidden())
	ps = append(ps, paramRawPMK())
	ps = append(ps, paramDFS()...)
	ps = append(ps, paramSSIDLimits())
	ps = append(ps, paramNonASCIISSID())
	ps = append(ps, param8021xWEP())
	ps = append(ps, param8021xWPA())
	ps = append(ps, paramTunneled1x()...)

	genparams.Ensure(t, "simple_connect.go", genparams.Template(t, `{{ range . }}{
	{{ range .Doc }}
	// {{ . }}
	{{ end }}
	Name: {{ .Name | fmt }},
	{{ if .ExtraAttr }}
	ExtraAttr: {{ .ExtraAttr | fmt }},
	{{ end }}
	{{ if .ExtraHardwareDeps }}
	{{ if .ExtraHardwareDepsDoc }}{{ range .ExtraHardwareDepsDoc }}
	// {{ . }}
	{{ end }}{{ end }}
	ExtraHardwareDeps: {{ .ExtraHardwareDeps }},
	{{ end }}
	Val:  []simpleConnectTestcase{
		{{ .Val }}
	},
},{{ end }}`, ps))
}

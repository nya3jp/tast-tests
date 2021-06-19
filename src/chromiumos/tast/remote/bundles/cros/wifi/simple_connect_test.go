// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// This test file generates the Params in simple_connect.go.
// Refer to go/tast-generate-params
// After modified, to overwrite the old Params, run:
// TAST_GENERATE_UPDATE=1 ~/trunk/src/platform/tast/tools/go.sh test -count=1 chromiumos/tast/remote/bundles/cros/wifi
// To check only, run:
// ~/trunk/src/platform/fast_build.sh -t chromiumos/tast/remote/bundles/cros/wifi

package wifi

import (
	"fmt"
	"strconv"
	"strings"
	"testing"

	"chromiumos/tast/common/genparams"
)

type simpleConnectParamsVal struct {
	Doc []string

	// APOpts is used if it is not empty; Otherwise CommonAPOptions is used instead.
	APOpts          string // apOpts: []ap.Option{ %s },
	CommonAPOptions string // apOpts: wifiutil.CommonAPOptions( %s ),

	SecConfFac      string
	PingOps         string
	ExpectedFailure bool
}

type simpleConnectParams struct {
	Name                 string
	Doc                  []string
	ExtraAttr            []string
	ExtraSoftwareDeps    []string
	ExtraSoftwareDepsDoc []string
	Val                  []simpleConnectParamsVal
	ExtraHardwareDeps    string
	ExtraHardwareDepsDoc []string
}

func simpleConnectDocPref(text string) []string {
	return []string{"Verifies that DUT can connect to " + text}
}

const simpleConnectCommonSecApOpts = "ap.Mode(ap.Mode80211g), ap.Channel(1)"

func simpleConnect80211abg() []simpleConnectParams {
	mkOps := func(mode string, channels ...int) []simpleConnectParamsVal {
		p := make([]simpleConnectParamsVal, len(channels))
		for i, ch := range channels {
			p[i].APOpts = fmt.Sprintf("ap.Mode(ap.Mode80211%s), ap.Channel(%d)", mode, ch)
		}
		return p
	}
	return []simpleConnectParams{{
		Name: "80211a",
		Doc:  simpleConnectDocPref("an open 802.11a network on channels 48, 64."),
		Val:  mkOps("a", 48, 64),
	}, {
		Name: "80211b",
		Doc:  simpleConnectDocPref("an open 802.11b network on channels 1, 6, 11."),
		Val:  mkOps("b", 1, 6, 11),
	}, {
		Name: "80211g",
		Doc:  simpleConnectDocPref("an open 802.11g network on channels 1, 6, 11."),
		Val:  mkOps("g", 1, 6, 11),
	}}
}

func simpleConnect80211n() []simpleConnectParams {
	mkOps := func(htCaps string, channels ...int) []simpleConnectParamsVal {
		p := make([]simpleConnectParamsVal, len(channels))
		for i, ch := range channels {
			p[i].APOpts = fmt.Sprintf("ap.Mode(ap.Mode80211nPure), ap.Channel(%d), ap.HTCaps(ap.HTCap%s)", ch, htCaps)
		}
		return p
	}
	return []simpleConnectParams{{
		Name: "80211n24ht20",
		Doc:  simpleConnectDocPref("an open 802.11n network on 2.4GHz channels 1, 6, 11 with a channel width of 20MHz."),
		Val:  mkOps("HT20", 1, 6, 11),
	}, {
		Name: "80211n24ht40",
		Doc:  simpleConnectDocPref("an open 802.11n network on 2.4GHz channel 6 with a channel width of 40MHz."),
		Val:  mkOps("HT40", 6),
	}, {
		Name: "80211n5ht20",
		Doc:  simpleConnectDocPref("an open 802.11n network on 5GHz channel 48 with a channel width of 20MHz."),
		Val:  mkOps("HT20", 48),
	}, {
		Name: "80211n5ht40",
		Doc: append(simpleConnectDocPref("an open 802.11n network on 5GHz channel 48"),
			"(40MHz channel with the second 20MHz chunk of the 40MHz channel on the channel below the center channel)."),
		Val: mkOps("HT40Minus", 48),
	}}
}

func simpleConnect80211nsgi() simpleConnectParams {
	return simpleConnectParams{
		Name:      "80211nsgi",
		Doc:       simpleConnectDocPref("an open 802.11n network on 5 GHz channel with short guard intervals enabled (both 20/40 Mhz)."),
		ExtraAttr: []string{"wificell_cq"},
		Val: []simpleConnectParamsVal{
			{APOpts: "ap.Mode(ap.Mode80211nPure), ap.Channel(48), ap.HTCaps(ap.HTCapHT20, ap.HTCapSGI20)"},
			{APOpts: "ap.Mode(ap.Mode80211nPure), ap.Channel(48), ap.HTCaps(ap.HTCapHT40Minus, ap.HTCapSGI40)"},
		},
	}
}

func simpleConnect80211ac() []simpleConnectParams {
	return []simpleConnectParams{{
		Name: "80211acvht20",
		Doc:  simpleConnectDocPref("an open 802.11ac network on channel 60 with a channel width of 20MHz."),
		Val: []simpleConnectParamsVal{{APOpts: `
			ap.Mode(ap.Mode80211acPure), ap.Channel(60), ap.HTCaps(ap.HTCapHT20),
			ap.VHTChWidth(ap.VHTChWidth20Or40),
		`}},
		ExtraHardwareDeps: "hwdep.D(hwdep.Wifi80211ac())",
	}, {
		Name: "80211acvht40",
		Doc:  simpleConnectDocPref("an open 802.11ac network on channel 48 with a channel width of 40MHz."),
		Val: []simpleConnectParamsVal{{APOpts: `
			ap.Mode(ap.Mode80211acPure), ap.Channel(48), ap.HTCaps(ap.HTCapHT40),
			ap.VHTChWidth(ap.VHTChWidth20Or40),
		`}},
		ExtraHardwareDeps: "hwdep.D(hwdep.Wifi80211ac())",
	}, {
		Name: "80211acvht80mixed",
		Doc:  simpleConnectDocPref("an open 802.11ac network on 5GHz channel 36 with center channel of 42 and channel width of 80MHz."),
		Val: []simpleConnectParamsVal{{APOpts: `
			ap.Mode(ap.Mode80211acMixed), ap.Channel(36), ap.HTCaps(ap.HTCapHT40Plus),
			ap.VHTCaps(ap.VHTCapSGI80), ap.VHTCenterChannel(42), ap.VHTChWidth(ap.VHTChWidth80),
		`}},
	}, {
		Name: "80211acvht80pure",
		Doc: append(simpleConnectDocPref("an open 802.11ac network on channel 157 with center channel of 155 and channel width of 80MHz."),
			"The router is forced to use 80 MHz wide rates only."),
		Val: []simpleConnectParamsVal{{APOpts: `
			ap.Mode(ap.Mode80211acPure), ap.Channel(157), ap.HTCaps(ap.HTCapHT40Plus),
			ap.VHTCaps(ap.VHTCapSGI80), ap.VHTCenterChannel(155), ap.VHTChWidth(ap.VHTChWidth80),
		`}},
		ExtraHardwareDeps: "hwdep.D(hwdep.Wifi80211ac())",
	}}
}

func simpleConnectHidden() simpleConnectParams {
	return simpleConnectParams{
		Name: "hidden",
		Doc:  simpleConnectDocPref("a hidden network on 2.4GHz and 5GHz channels."),
		Val: []simpleConnectParamsVal{
			{APOpts: "ap.Mode(ap.Mode80211g), ap.Channel(6), ap.Hidden()"},
			{APOpts: "ap.Mode(ap.Mode80211nPure), ap.Channel(36), ap.HTCaps(ap.HTCapHT20), ap.Hidden()"},
			{APOpts: "ap.Mode(ap.Mode80211nPure), ap.Channel(48), ap.HTCaps(ap.HTCapHT20), ap.Hidden()"},
		},
	}
}

func simpleConnectWEP() []simpleConnectParams {
	mkP := func(keyLen int) simpleConnectParams {
		ret := simpleConnectParams{
			Name: "wep" + strconv.Itoa(keyLen),
			Doc:  simpleConnectDocPref(fmt.Sprintf("a WEP network with both open and shared system authentication and %d-bit pre-shared keys.", keyLen)),
		}
		for _, algo := range []string{"Open", "Shared"} {
			for key := 0; key < 4; key++ {
				ret.Val = append(ret.Val, simpleConnectParamsVal{
					APOpts:     simpleConnectCommonSecApOpts,
					SecConfFac: fmt.Sprintf("wep.NewConfigFactory(wep%dKeys(), wep.DefaultKey(%d), wep.AuthAlgs(wep.AuthAlgo%s))", keyLen, key, algo),
				})
			}
		}
		return ret
	}
	return []simpleConnectParams{mkP(40), mkP(104)}
}

func simpleConnectWEPHidden() simpleConnectParams {
	var p []simpleConnectParamsVal
	for _, keyLen := range []int{40, 104} {
		for _, algo := range []string{"Open", "Shared"} {
			p = append(p, simpleConnectParamsVal{
				APOpts:     simpleConnectCommonSecApOpts + ", ap.Hidden()",
				SecConfFac: fmt.Sprintf("wep.NewConfigFactory(wep%dKeysHidden(), wep.AuthAlgs(wep.AuthAlgo%s))", keyLen, algo),
			})
		}
	}
	return simpleConnectParams{
		Name: "wephidden",
		Doc:  simpleConnectDocPref("a hidden WEP network with open/shared system authentication and 40/104-bit pre-shared keys."),
		Val:  p,
	}
}

func simpleConnectWPA() []simpleConnectParams {
	const (
		tkip = 1 << iota
		ccmp
	)
	mkCipher := func(c int) string {
		var ret []string
		if c&tkip > 0 {
			ret = append(ret, "wpa.CipherTKIP")
		}
		if c&ccmp > 0 {
			ret = append(ret, "wpa.CipherCCMP")
		}
		return strings.Join(ret, ", ")
	}
	mkOps := func(pmf, mode string, cipher1, cipher2 int) []simpleConnectParamsVal {
		var cipher []string
		if cipher1 > 0 {
			cipher = append(cipher, "wpa.Ciphers("+mkCipher(cipher1)+")")
		}
		if cipher2 > 0 {
			cipher = append(cipher, "wpa.Ciphers2("+mkCipher(cipher2)+")")
		}
		return []simpleConnectParamsVal{{
			APOpts: simpleConnectCommonSecApOpts + ", " + pmf,
			SecConfFac: fmt.Sprintf(`wpa.NewConfigFactory(
				"chromeos", wpa.Mode(wpa.Mode%s),
				%s,
			)`, mode, strings.Join(cipher, ", ")),
		}}
	}
	return []simpleConnectParams{{
		Name: "wpatkip",
		Doc:  simpleConnectDocPref("a protected network supporting for pure WPA with TKIP."),
		Val:  mkOps("", "PureWPA", tkip, 0),
	}, {
		Name: "wpaccmp",
		Doc:  simpleConnectDocPref("a protected network supporting for pure WPA with AES based CCMP."),
		Val:  mkOps("", "PureWPA", ccmp, 0),
	}, {
		Name:      "wpamulti",
		Doc:       simpleConnectDocPref("a protected network supporting for pure WPA with both AES based CCMP and TKIP."),
		ExtraAttr: []string{"wificell_cq"},
		Val:       mkOps("", "PureWPA", tkip|ccmp, 0),
	}, {
		Name: "wpa2tkip",
		Doc:  simpleConnectDocPref("a protected network supporting for WPA2 (aka RSN) with TKIP. Some AP still uses TKIP in WPA2."),
		Val:  mkOps("", "PureWPA2", 0, tkip),
	}, {
		Name: "wpa2pmf",
		Doc: append(simpleConnectDocPref("an AP broadcasting a WPA2 network using AES based CCMP."),
			"In addition, the client must also support 802.11w protected management frames."),
		ExtraAttr: []string{"wificell_cq"},
		Val:       mkOps("ap.PMF(ap.PMFRequired)", "PureWPA2", 0, ccmp),
	}, {
		Name: "wpa2pmfoptional",
		Doc: append(simpleConnectDocPref("an AP broadcasting a WPA2 network using AES based CCMP."),
			"In addition, the client may also negotiate use of 802.11w protected management frames."),
		Val: mkOps("ap.PMF(ap.PMFOptional)", "PureWPA2", 0, ccmp),
	}, {
		Name: "wpa2",
		Doc:  simpleConnectDocPref("a protected network supporting for WPA2 (aka RSN) and encrypted under AES."),
		Val:  mkOps("", "PureWPA2", 0, ccmp),
	}, {
		Name: "wpamixed",
		Doc:  simpleConnectDocPref("a protected network supporting for both WPA and WPA2 with TKIP/AES supported for WPA and AES supported for WPA2."),
		Val:  mkOps("", "Mixed", tkip|ccmp, ccmp),
	}}
}

func simpleConnectWPA3() []simpleConnectParams {
	mkOps := func(pmf, mode string) []simpleConnectParamsVal {
		return []simpleConnectParamsVal{{
			APOpts: fmt.Sprintf(`
				ap.Mode(ap.Mode80211acMixed), ap.Channel(36), ap.HTCaps(ap.HTCapHT40Plus),
				ap.VHTCenterChannel(42), ap.VHTChWidth(ap.VHTChWidth80),
				ap.PMF(ap.PMF%s),
			`, pmf),
			SecConfFac: fmt.Sprintf(`wpa.NewConfigFactory(
				"chromeos", wpa.Mode(wpa.Mode%sWPA3),
				wpa.Ciphers2(wpa.CipherCCMP),
			)`, mode),
		}}
	}
	return []simpleConnectParams{{
		Name: "wpa3mixed",
		Doc:  simpleConnectDocPref("an AP in WPA2/WPA3 mixed mode. WiFi alliance suggests PMF in this mode."),
		Val:  mkOps("Optional", "Mixed"),
	}, {
		Name:              "wpa3",
		ExtraSoftwareDeps: []string{"wpa3_sae"},
		ExtraSoftwareDepsDoc: []string{
			"Not all WiFi chips support SAE. We enable the feature as a Software dependency for now, but eventually",
			"this will require a hardware dependency (crbug.com/1070299).",
		},
		Doc: simpleConnectDocPref(`an AP in WPA3-SAE ("pure") mode. WiFi alliance requires PMF in this mode.`),
		Val: mkOps("Required", "Pure"),
	}}
}

func simpleConnectWPAVHT80() simpleConnectParams {
	return simpleConnectParams{
		Name: "wpavht80",
		Doc:  simpleConnectDocPref("a protected 802.11ac network supporting for WPA."),
		Val: []simpleConnectParamsVal{{
			APOpts: `
				ap.Mode(ap.Mode80211acPure), ap.Channel(36), ap.HTCaps(ap.HTCapHT40Plus),
				ap.VHTCenterChannel(42), ap.VHTChWidth(ap.VHTChWidth80),
			`,
			SecConfFac: `wpa.NewConfigFactory(
				"chromeos", wpa.Mode(wpa.ModePureWPA),
				wpa.Ciphers(wpa.CipherTKIP, wpa.CipherCCMP),
			)`,
		}},
		ExtraHardwareDeps: "hwdep.D(hwdep.Wifi80211ac())",
	}
}

func simpleConnectWPAOddPassphrase() simpleConnectParams {
	var p []simpleConnectParamsVal
	for _, pw := range []string{`"\xe4\xb8\x80\xe4\xba\x8c\xe4\xb8\x89"`, `"abcdef\xc2\xa2"`, `" !\"#$%&'()>*+,-./:;<=>?@[\\]^_{|}~"`} {
		temp := `wpa.NewConfigFactory(
			%s, wpa.Mode(wpa.ModePure%s),
			%s,
		)`
		p = append(p, simpleConnectParamsVal{
			APOpts:     simpleConnectCommonSecApOpts,
			SecConfFac: fmt.Sprintf(temp, pw, "WPA", "wpa.Ciphers(wpa.CipherTKIP)"),
		}, simpleConnectParamsVal{
			APOpts:     simpleConnectCommonSecApOpts,
			SecConfFac: fmt.Sprintf(temp, pw, "WPA2", "wpa.Ciphers2(wpa.CipherCCMP)"),
		})
	}
	return simpleConnectParams{
		Name: "wpaoddpassphrase",
		Doc:  simpleConnectDocPref("a protected network whose WPA passphrase can be pure unicode, mixed unicode and ASCII, and all the punctuations."),
		Val:  p,
	}
}

func simpleConnectWPAHidden() simpleConnectParams {
	var p []simpleConnectParamsVal
	for _, c := range []struct{ mode, cipher string }{
		{"PureWPA", "wpa.Ciphers(wpa.CipherTKIP)"},
		{"PureWPA", "wpa.Ciphers(wpa.CipherTKIP, wpa.CipherCCMP)"},
		{"PureWPA2", "wpa.Ciphers2(wpa.CipherCCMP)"},
		{"Mixed", "wpa.Ciphers(wpa.CipherTKIP, wpa.CipherCCMP), wpa.Ciphers2(wpa.CipherCCMP)"},
	} {
		p = append(p, simpleConnectParamsVal{
			APOpts: simpleConnectCommonSecApOpts + ", ap.Hidden()",
			SecConfFac: fmt.Sprintf(`wpa.NewConfigFactory(
				"chromeos", wpa.Mode(wpa.Mode%s),
				%s,
			)`, c.mode, c.cipher),
		})
	}
	return simpleConnectParams{
		Name: "wpahidden",
		Doc:  simpleConnectDocPref("a hidden network supporting for WPA with TKIP, WPA with TKIP/AES, WPA2 with AES, and mixed WPA with TKIP/AES and WPA2 with AES."),
		Val:  p,
	}
}

func simpleConnectRawPMK() simpleConnectParams {
	return simpleConnectParams{
		Name: "raw_pmk",
		Doc:  simpleConnectDocPref("a WPA network using a raw PMK value instead of an ASCII passphrase."),
		Val: []simpleConnectParamsVal{{
			APOpts: simpleConnectCommonSecApOpts,
			SecConfFac: `wpa.NewConfigFactory(
				strings.Repeat("0123456789abcdef", 4), // length = 64.
				wpa.Mode(wpa.ModePureWPA),
				wpa.Ciphers(wpa.CipherTKIP),
			)`,
		}},
	}
}

func simpleConnectDFS() []simpleConnectParams {
	return []simpleConnectParams{{
		Name: "dfs",
		Doc: append(simpleConnectDocPref("an open network on a DFS channel."),
			"DFS (dynamic frequency selection) channels are channels that may be unavailable if radar interference is detected.",
			"See: https://en.wikipedia.org/wiki/Dynamic_frequency_selection, https://en.wikipedia.org/wiki/List_of_WLAN_channels"),
		ExtraAttr: []string{"wificell_cq"},
		Val: []simpleConnectParamsVal{
			{APOpts: "ap.Mode(ap.Mode80211nMixed), ap.Channel(120), ap.HTCaps(ap.HTCapHT40)"},
			{APOpts: "ap.Mode(ap.Mode80211nMixed), ap.Channel(136), ap.HTCaps(ap.HTCapHT40)"},
		},
	}}
}

func simpleConnectSSIDLimits() simpleConnectParams {
	return simpleConnectParams{
		Name:      "ssid_limits",
		Doc:       simpleConnectDocPref("a networks with the longest and shortest SSID."),
		ExtraAttr: []string{"wificell_cq"},
		Val: []simpleConnectParamsVal{
			{CommonAPOptions: `ap.SSID("a")`},
			{CommonAPOptions: `ap.SSID(strings.Repeat("MaxLengthSSID", 4)[:32])`},
		},
	}
}

func simpleConnectNonASCIISSID() simpleConnectParams {
	return simpleConnectParams{
		Name:                 "non_ascii_ssid",
		Doc:                  []string{"This test case verifies that the DUT accepts ascii and non-ascii type characters as the SSID."},
		ExtraHardwareDepsDoc: []string{"TODO(b/158150763): Skip Marvell WiFi as there's a known issue to make the test always fail."},
		ExtraHardwareDeps:    "hwdep.D(hwdep.WifiNotMarvell())",
		Val: []simpleConnectParamsVal{
			{
				Doc: []string{
					"TODO(crbug.com/1082582): shill don't allow leading 0x00 now, so let's append it in the",
					"end to keep the coverage.",
				},
				CommonAPOptions: `ap.SSID(byteSequenceStr(1, 31) + "\x00")`,
			},
			{CommonAPOptions: "ap.SSID(byteSequenceStr(32, 63))"},
			{CommonAPOptions: "ap.SSID(byteSequenceStr(64, 95))"},
			{CommonAPOptions: "ap.SSID(byteSequenceStr(96, 127))"},
			{CommonAPOptions: "ap.SSID(byteSequenceStr(128, 159))"},
			{CommonAPOptions: "ap.SSID(byteSequenceStr(160, 191))"},
			{CommonAPOptions: "ap.SSID(byteSequenceStr(192, 223))"},
			{CommonAPOptions: "ap.SSID(byteSequenceStr(224, 255))"},
			{
				Doc:             []string{"Valid Unicode characters."},
				CommonAPOptions: `ap.SSID("\xe4\xb8\xad\xe5\x9b\xbd")`,
			},
			{
				Doc:             []string{"Single extended ASCII character (a-grave)."},
				CommonAPOptions: `ap.SSID("\xe0")`,
			},
			{
				Doc:             []string{"Mix of ASCII and Unicode characters as SSID."},
				CommonAPOptions: `ap.SSID("Chrome\xe7\xac\x94\xe8\xae\xb0\xe6\x9c\xac")`,
			},
		},
	}
}

func simpleConnect8021xWEP() simpleConnectParams {
	return simpleConnectParams{
		Name:                 "8021xwep",
		Doc:                  simpleConnectDocPref("a protected network supporting for dynamic WEP encryption."),
		ExtraHardwareDepsDoc: []string{"Skip on Marvell because of 8021xwep test failure post security fixes b/187853331, no plans to fix."},
		ExtraHardwareDeps:    "hwdep.D(hwdep.WifiNotMarvell())",
		Val: []simpleConnectParamsVal{{
			APOpts: simpleConnectCommonSecApOpts,
			SecConfFac: `dynamicwep.NewConfigFactory(
				eapCert1.CACred.Cert, eapCert1.ServerCred,
				dynamicwep.ClientCACert(eapCert1.CACred.Cert),
				dynamicwep.ClientCred(eapCert1.ClientCred),
				dynamicwep.RekeyPeriod(10),
			)`,
			PingOps: "ping.Count(15), ping.Interval(1)",
		}},
	}
}

func simpleConnect8021xWPA() simpleConnectParams {
	return simpleConnectParams{
		Name:                 "8021xwpa",
		Doc:                  simpleConnectDocPref("a protected network supporting for WPA-EAP encryption."),
		ExtraHardwareDepsDoc: []string{"TODO(b/189986748): Remove the skiplist once those flaky boards have reached AUE."},
		ExtraHardwareDeps:    `hwdep.D(hwdep.SkipOnPlatform("banjo", "candy", "gnawty", "kip", "ninja", "sumo", "swanky", "winky"))`,
		Val: []simpleConnectParamsVal{{
			APOpts: simpleConnectCommonSecApOpts,
			SecConfFac: `wpaeap.NewConfigFactory(
				eapCert1.CACred.Cert, eapCert1.ServerCred,
				wpaeap.ClientCACert(eapCert1.CACred.Cert),
				wpaeap.ClientCred(eapCert1.ClientCred),
			)`,
		}, {
			Doc:    []string{"Failure due to lack of CACert on client."},
			APOpts: simpleConnectCommonSecApOpts,
			SecConfFac: `wpaeap.NewConfigFactory(
				eapCert1.CACred.Cert, eapCert1.ServerCred,
				wpaeap.ClientCred(eapCert1.ClientCred),
			)`,
			ExpectedFailure: true,
		}, {
			Doc:    []string{"Failure due to unmatched CACert."},
			APOpts: simpleConnectCommonSecApOpts,
			SecConfFac: `wpaeap.NewConfigFactory(
				eapCert1.CACred.Cert, eapCert1.ServerCred,
				wpaeap.ClientCACert(eapCert2.CACred.Cert),
				wpaeap.ClientCred(eapCert1.ClientCred),
			)`,
			ExpectedFailure: true,
		}, {
			Doc:    []string{"Should succeed if we specify that we have no CACert."},
			APOpts: simpleConnectCommonSecApOpts,
			SecConfFac: `wpaeap.NewConfigFactory(
				eapCert1.CACred.Cert, eapCert1.ServerCred,
				wpaeap.ClientCred(eapCert1.ClientCred),
				wpaeap.NotUseSystemCAs(),
			)`,
		}, {
			Doc:    []string{"Failure due to wrong certificate chain on client."},
			APOpts: simpleConnectCommonSecApOpts,
			SecConfFac: `wpaeap.NewConfigFactory(
				eapCert1.CACred.Cert, eapCert1.ServerCred,
				wpaeap.ClientCACert(eapCert1.CACred.Cert),
				wpaeap.ClientCred(eapCert2.ClientCred),
			)`,
			ExpectedFailure: true,
		}, {
			Doc:    []string{"Failure due to expired cert on server."},
			APOpts: simpleConnectCommonSecApOpts,
			SecConfFac: `wpaeap.NewConfigFactory(
				eapCert1.CACred.Cert, eapCert1.ExpiredServerCred,
				wpaeap.ClientCACert(eapCert1.CACred.Cert),
				wpaeap.ClientCred(eapCert1.ClientCred),
			)`,
			ExpectedFailure: true,
		}},
	}
}

func simpleConnect8021xWPA3() []simpleConnectParams {
	mkOps := func(pmf, mode string) []simpleConnectParamsVal {
		return []simpleConnectParamsVal{{
			APOpts: fmt.Sprintf("%s, ap.PMF(ap.PMF%s)", simpleConnectCommonSecApOpts, pmf),
			SecConfFac: fmt.Sprintf(`wpaeap.NewConfigFactory(
				eapCert1.CACred.Cert, eapCert1.ServerCred,
				wpaeap.ClientCACert(eapCert1.CACred.Cert),
				wpaeap.ClientCred(eapCert1.ClientCred),
				wpaeap.Mode(wpa.Mode%sWPA3),
			)`, mode),
		}}
	}
	return []simpleConnectParams{
		{
			Name: "8021xwpa3mixed",
			Doc:  simpleConnectDocPref("an WPA3-Enterprise-transition AP"),
			Val:  mkOps("Optional", "Mixed"),
		},
		{
			Name:      "8021xwpa3",
			Doc:       simpleConnectDocPref("an WPA3-Enterprise-only AP"),
			Val:       mkOps("Required", "Pure"),
			ExtraAttr: []string{"wificell_unstable"},
		},
	}

}

func simpleConnectTunneled1x() []simpleConnectParams {
	mkP := func(outer, inner string, extraAttr []string) simpleConnectParams {
		ret := simpleConnectParams{
			Name:      "8021x" + strings.ToLower(outer) + "_" + strings.ToLower(inner),
			Doc:       []string{fmt.Sprintf("Verifies that DUT can connect to a protected network supporting for %s authentication with tunneled %s.", outer, inner)},
			ExtraAttr: extraAttr,
		}
		ret.Val = append(ret.Val, simpleConnectParamsVal{
			APOpts: simpleConnectCommonSecApOpts,
			SecConfFac: fmt.Sprintf(`tunneled1x.NewConfigFactory(
				eapCert1.CACred.Cert, eapCert1.ServerCred, eapCert1.CACred.Cert, "testuser", "password",
				tunneled1x.Mode(wpa.ModePureWPA2),
				tunneled1x.OuterProtocol(tunneled1x.Layer1Type%s),
				tunneled1x.InnerProtocol(tunneled1x.Layer2Type%s),
			)`, outer, inner),
		})
		for i := 0; i < 3; i++ {
			ret.Val = append(ret.Val, simpleConnectParamsVal{
				APOpts: simpleConnectCommonSecApOpts,
				SecConfFac: fmt.Sprintf(`tunneled1x.NewConfigFactory(
					eapCert3.CACred.Cert, eapCert3.ServerCred, eapCert3.CACred.Cert, "testuser", "password",
					tunneled1x.OuterProtocol(tunneled1x.Layer1Type%s),
					tunneled1x.InnerProtocol(tunneled1x.Layer2Type%s),
					tunneled1x.AltSubjectMatch([]string{eapCert3AltSub[%d]}),
				)`, outer, inner, i),
			})
		}
		ret.Val = append(ret.Val, simpleConnectParamsVal{
			Doc: []string{
				"Should success since having multiple entries in 'altsubject_match' is treated as OR, not AND.",
				"For more information about how wpa_supplicant uses altsubject_match field:",
				"https://w1.fi/cgit/hostap/plain/wpa_supplicant/wpa_supplicant.conf",
			},
			APOpts: simpleConnectCommonSecApOpts,
			SecConfFac: fmt.Sprintf(`tunneled1x.NewConfigFactory(
				eapCert3.CACred.Cert, eapCert3.ServerCred, eapCert3.CACred.Cert, "testuser", "password",
				tunneled1x.OuterProtocol(tunneled1x.Layer1Type%s),
				tunneled1x.InnerProtocol(tunneled1x.Layer2Type%s),
				tunneled1x.AltSubjectMatch([]string{`+"`"+`{"Type":"DNS","Value":"wrong_dns.com"}`+"`"+`, eapCert3AltSub[0]}),
			)`, outer, inner),
		})
		return ret
	}
	mkPFail := func(outer, inner string, extraAttr []string) simpleConnectParams {
		ret := simpleConnectParams{
			Name: "8021x" + strings.ToLower(outer) + "_fail",
			Doc: []string{
				fmt.Sprintf("Verifies that DUT CANNOT connect to a %s network with wrong settings.", outer),
				"We do these tests for only one inner authentication protocol because we",
				"presume that supplicant reuses this code between inner authentication types."},
			ExtraAttr: extraAttr,
		}
		for _, v := range []struct{ doc, sec string }{
			{"Failure due to bad password.",
				`tunneled1x.NewConfigFactory(
				eapCert1.CACred.Cert, eapCert1.ServerCred, eapCert1.CACred.Cert, "testuser", "password",
				tunneled1x.OuterProtocol(tunneled1x.Layer1Type%s),
				tunneled1x.InnerProtocol(tunneled1x.Layer2Type%s),
				tunneled1x.ClientPassword("wrongpassword"),
			)`},
			{"Failure due to wrong client CA.",
				`tunneled1x.NewConfigFactory(
				eapCert1.CACred.Cert, eapCert1.ServerCred, eapCert2.CACred.Cert, "testuser", "password",
				tunneled1x.OuterProtocol(tunneled1x.Layer1Type%s),
				tunneled1x.InnerProtocol(tunneled1x.Layer2Type%s),
			)`},
			{"Failure due to expired server cred.",
				`tunneled1x.NewConfigFactory(
				eapCert1.CACred.Cert, eapCert1.ExpiredServerCred, eapCert1.CACred.Cert, "testuser", "password",
				tunneled1x.OuterProtocol(tunneled1x.Layer1Type%s),
				tunneled1x.InnerProtocol(tunneled1x.Layer2Type%s),
			)`},
			{"Failure due to that a subject alternative name (SAN) is set but does not match any of the server certificate SANs.",
				`tunneled1x.NewConfigFactory(
				eapCert3.CACred.Cert, eapCert3.ServerCred, eapCert3.CACred.Cert, "testuser", "password",
				tunneled1x.OuterProtocol(tunneled1x.Layer1Type%s),
				tunneled1x.InnerProtocol(tunneled1x.Layer2Type%s),
				tunneled1x.AltSubjectMatch([]string{` + "`" + `{"Type":"DNS","Value":"wrong_dns.com"}` + "`" + `}),
			)`},
		} {
			ret.Val = append(ret.Val, simpleConnectParamsVal{
				Doc:             []string{v.doc},
				APOpts:          simpleConnectCommonSecApOpts,
				SecConfFac:      fmt.Sprintf(v.sec, outer, inner),
				ExpectedFailure: true,
			})
		}
		return ret
	}
	return []simpleConnectParams{
		mkPFail("PEAP", "MSCHAPV2", nil),
		mkP("PEAP", "MSCHAPV2", nil),
		mkP("PEAP", "MD5", nil),
		mkP("PEAP", "GTC", nil),
		mkPFail("TTLS", "MD5", nil),
		mkP("TTLS", "MSCHAPV2", nil),
		mkP("TTLS", "MD5", nil),
		mkP("TTLS", "GTC", nil),
		mkP("TTLS", "TTLSMSCHAPV2", nil),
		mkP("TTLS", "TTLSMSCHAP", nil),
		mkP("TTLS", "TTLSPAP", nil),
	}
}

func TestSimpleConnect(t *testing.T) {
	var ps []simpleConnectParams
	ps = append(ps, simpleConnect80211abg()...)
	ps = append(ps, simpleConnect80211n()...)
	ps = append(ps, simpleConnect80211nsgi())
	ps = append(ps, simpleConnect80211ac()...)
	ps = append(ps, simpleConnectHidden())
	ps = append(ps, simpleConnectWEP()...)
	ps = append(ps, simpleConnectWEPHidden())
	ps = append(ps, simpleConnectWPA()...)
	ps = append(ps, simpleConnectWPA3()...)
	ps = append(ps, simpleConnectWPAVHT80())
	ps = append(ps, simpleConnectWPAOddPassphrase())
	ps = append(ps, simpleConnectWPAHidden())
	ps = append(ps, simpleConnectRawPMK())
	ps = append(ps, simpleConnectDFS()...)
	ps = append(ps, simpleConnectSSIDLimits())
	ps = append(ps, simpleConnectNonASCIISSID())
	ps = append(ps, simpleConnect8021xWEP())
	ps = append(ps, simpleConnect8021xWPA())
	ps = append(ps, simpleConnect8021xWPA3()...)
	ps = append(ps, simpleConnectTunneled1x()...)

	genparams.Ensure(t, "simple_connect.go", genparams.Template(t, `{{ range . }}{
	{{ range .Doc }}
	// {{ . }}
	{{ end }}
	Name: {{ .Name | fmt }},
	{{ if .ExtraAttr }}
	ExtraAttr: {{ .ExtraAttr | fmt }},
	{{ end }}
	{{ if .ExtraSoftwareDeps }}
	{{ range .ExtraSoftwareDepsDoc }}
	// {{ . }}
	{{ end }}
	ExtraSoftwareDeps: {{ .ExtraSoftwareDeps | fmt }},
	{{ end }}
	Val:  []simpleConnectTestcase{ {{ range .Val }} {
		{{ range .Doc }}
		// {{ . }}
		{{ end }}
		{{ if .APOpts }}
		apOpts: []ap.Option{ {{ .APOpts }} },
		{{ else if .CommonAPOptions }}
		apOpts: wifiutil.CommonAPOptions({{ .CommonAPOptions }}),
		{{ end }}
		{{ if .SecConfFac }}
		secConfFac: {{ .SecConfFac }},
		{{ end }}
		{{ if .PingOps }}
		pingOps: []ping.Option{ {{ .PingOps }} },
		{{ end }}
		{{ if .ExpectedFailure }}
		expectedFailure: true,
		{{ end }}
	}, {{ end }} },
	{{ if .ExtraHardwareDeps }}
	{{ range .ExtraHardwareDepsDoc }}
	// {{ . }}
	{{ end }}
	ExtraHardwareDeps: {{ .ExtraHardwareDeps }},
	{{ end }}
},{{ end }}`, ps))
}

// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wifi

import (
	"context"

	"chromiumos/tast/common/shillconst"
	"chromiumos/tast/common/wifi/security"
	"chromiumos/tast/common/wifi/security/wep"
	"chromiumos/tast/common/wifi/security/wpa"
	"chromiumos/tast/remote/wificell"
	ap "chromiumos/tast/remote/wificell/hostapd"
	"chromiumos/tast/testing"
)

type fgsecTestcase struct {
	ap1Opts          []ap.Option
	sec1Conf         security.ConfigFactory
	ap2Opts          []ap.Option
	sec2Conf         security.ConfigFactory
	expectedSecurity string
}

func init() {
	testing.AddTest(&testing.Test{
		Func: FgsecConnect,
		Desc: "Verifies connectivity with more detailed security settings than just broad PSK class",
		Contacts: []string{
			"amo@semihalf.com",                // Test author
			"chromeos-wifi-champs@google.com", // WiFi oncall rotation; or http://b/new?component=893827
		},
		Attr:        []string{"group:wificell", "wificell_func", "wificell_unstable"},
		ServiceDeps: []string{wificell.TFServiceName},
		Fixture:     "wificellFixt",
		Params: []testing.Param{
			// These couple first are just some single AP "regression" tests
			{
				Name: "none",
				Val: fgsecTestcase{
					ap1Opts:          []ap.Option{ap.Mode(ap.Mode80211g), ap.Channel(1)},
					expectedSecurity: shillconst.SecurityNone,
				},
			},
			{
				Name: "wep",
				Val: fgsecTestcase{
					ap1Opts:          []ap.Option{ap.Mode(ap.Mode80211g), ap.Channel(1)},
					sec1Conf:         wep.NewConfigFactory([]string{"abcde"}, wep.DefaultKey(0), wep.AuthAlgs(wep.AuthAlgoOpen)),
					expectedSecurity: shillconst.SecurityWEP,
				},
			},
			{
				Name: "wpa",
				Val: fgsecTestcase{
					ap1Opts: []ap.Option{ap.Mode(ap.Mode80211g), ap.Channel(1)},
					sec1Conf: wpa.NewConfigFactory("chromeos",
						wpa.Mode(wpa.ModePureWPA), wpa.Ciphers(wpa.CipherTKIP)),
					expectedSecurity: shillconst.SecurityWPA,
				},
			},
			{
				Name: "wpa2",
				Val: fgsecTestcase{
					ap1Opts: []ap.Option{ap.Mode(ap.Mode80211g), ap.Channel(1)},
					sec1Conf: wpa.NewConfigFactory("chromeos",
						wpa.Mode(wpa.ModePureWPA2), wpa.Ciphers2(wpa.CipherCCMP)),
					expectedSecurity: shillconst.SecurityWPA2,
				},
			},
			{
				// WpaWpa2 mixed mode resulting from single AP configured in a mixed mode.
				Name: "wpa2mixed",
				Val: fgsecTestcase{
					ap1Opts: []ap.Option{ap.Mode(ap.Mode80211g), ap.Channel(1)},
					sec1Conf: wpa.NewConfigFactory("chromeos",
						wpa.Mode(wpa.ModeMixed), wpa.Ciphers(wpa.CipherTKIP, wpa.CipherCCMP), wpa.Ciphers2(wpa.CipherCCMP)),
					expectedSecurity: shillconst.SecurityWPAWPA2,
				},
			},
			{
				// WpaWpa2 mixed mode resulting from seeing two APs configured with
				// Wpa and Wpa2 and combined into a single service with
				// mixed/transitional security.
				Name: "wpawpa2",
				Val: fgsecTestcase{
					ap1Opts: []ap.Option{ap.Mode(ap.Mode80211nPure), ap.Channel(1), ap.HTCaps(ap.HTCapHT20)},
					sec1Conf: wpa.NewConfigFactory("chromeos",
						wpa.Mode(wpa.ModePureWPA), wpa.Ciphers(wpa.CipherTKIP)),
					ap2Opts: []ap.Option{ap.Mode(ap.Mode80211nPure), ap.Channel(48), ap.HTCaps(ap.HTCapHT20)},
					sec2Conf: wpa.NewConfigFactory("chromeos",
						wpa.Mode(wpa.ModePureWPA2), wpa.Ciphers2(wpa.CipherCCMP)),
					expectedSecurity: shillconst.SecurityWPAWPA2,
				},
			},
			{
				Name: "wpa3",
				Val: fgsecTestcase{
					ap1Opts: []ap.Option{ap.Mode(ap.Mode80211g), ap.Channel(1)},
					sec1Conf: wpa.NewConfigFactory("chromeos",
						wpa.Mode(wpa.ModePureWPA3), wpa.Ciphers2(wpa.CipherCCMP)),
					expectedSecurity: shillconst.SecurityWPA3,
				},
				ExtraSoftwareDeps: []string{"wpa3_sae"},
			},
			{
				// Wpa2Wpa3 mixed mode resulting from single AP configured in a mixed mode.
				Name: "wpa3mixed",
				Val: fgsecTestcase{
					ap1Opts: []ap.Option{ap.Mode(ap.Mode80211g), ap.Channel(1)},
					sec1Conf: wpa.NewConfigFactory("chromeos",
						wpa.Mode(wpa.ModeMixedWPA3), wpa.Ciphers(wpa.CipherCCMP), wpa.Ciphers2(wpa.CipherCCMP)),
					expectedSecurity: shillconst.SecurityWPA2WPA3,
				},
			},
			{
				// Wpa2Wpa3 mixed mode resulting from seeing two APs configured with
				// Wpa2 and Wpa3 and combined into a single service with
				// mixed/transitional security.
				Name: "wpa2wpa3",
				Val: fgsecTestcase{
					ap1Opts: []ap.Option{ap.Mode(ap.Mode80211nPure), ap.Channel(1), ap.HTCaps(ap.HTCapHT20)},
					sec1Conf: wpa.NewConfigFactory("chromeos",
						wpa.Mode(wpa.ModePureWPA2), wpa.Ciphers(wpa.CipherCCMP)),
					ap2Opts: []ap.Option{ap.Mode(ap.Mode80211nPure), ap.Channel(48), ap.HTCaps(ap.HTCapHT20)},
					sec2Conf: wpa.NewConfigFactory("chromeos",
						wpa.Mode(wpa.ModePureWPA3), wpa.Ciphers2(wpa.CipherCCMP)),
					expectedSecurity: shillconst.SecurityWPA2WPA3,
				},
				ExtraSoftwareDeps: []string{"wpa3_sae"},
			},
			{
				// In initial phase of FGSec deployment we keep legacy behaviour so when
				// meeting wpa1+wpa2+wpa3 we transition to wpa-all security.
				Name: "wpaall",
				Val: fgsecTestcase{
					ap1Opts: []ap.Option{ap.Mode(ap.Mode80211nPure), ap.Channel(1), ap.HTCaps(ap.HTCapHT20)},
					sec1Conf: wpa.NewConfigFactory("chromeos",
						wpa.Mode(wpa.ModeMixed), wpa.Ciphers(wpa.CipherTKIP, wpa.CipherCCMP), wpa.Ciphers2(wpa.CipherCCMP)),
					ap2Opts: []ap.Option{ap.Mode(ap.Mode80211nPure), ap.Channel(48), ap.HTCaps(ap.HTCapHT20)},
					sec2Conf: wpa.NewConfigFactory("chromeos",
						wpa.Mode(wpa.ModePureWPA3), wpa.Ciphers2(wpa.CipherCCMP)),
					expectedSecurity: shillconst.SecurityWPAAll,
				},
				ExtraSoftwareDeps: []string{"wpa3_sae"},
			},
		},
	})
}

// FgsecConnect tests connectivity to a various network configurations
// (parametrized above) and verifies that DUT is able to connect and
// that the resulting Security of the service matches the expectations.
// Step-by-step procedure:
// 1. Configure AP1 with security specified by the test case.
// 2. Optionally (if test case specifies ap2Opts/sec2Conf) configure AP2 - both APs form a single network.
// 3. For the cases with 2 APs make sure that both network endpoints have been noticed.
// 4. Connect to the network (AP1).
// 5. Query service Security and check if it agrees with expectation.
// 6. Disconnect and deconfigure AP(s)
func FgsecConnect(ctx context.Context, s *testing.State) {
	tf := s.FixtValue().(*wificell.TestFixture)
	ssid := ap.RandomSSID("TAST_FGSEC_")
	tc := s.Param().(fgsecTestcase)

	ap1, err := tf.ConfigureAP(ctx, append(tc.ap1Opts, ap.SSID(ssid)), tc.sec1Conf)
	if err != nil {
		s.Fatal("Failed to configure the AP1: ", err)
	}
	defer func(ctx context.Context) {
		if err := tf.DeconfigAP(ctx, ap1); err != nil {
			s.Error("Failed to deconfig the AP1: ", err)
		}
	}(ctx)
	ctx, cancel := tf.ReserveForDeconfigAP(ctx, ap1)
	defer cancel()

	if tc.ap2Opts != nil && tc.sec2Conf != nil {
		ap2, err := tf.ConfigureAP(ctx, append(tc.ap2Opts, ap.SSID(ssid)), tc.sec2Conf)
		if err != nil {
			s.Fatal("Failed to configure the AP2: ", err)
		}
		defer func(ctx context.Context) {
			if err := tf.DeconfigAP(ctx, ap2); err != nil {
				s.Error("Failed to deconfig the AP2: ", err)
			}
		}(ctx)
		ctx, cancel = tf.ReserveForDeconfigAP(ctx, ap2)
		defer cancel()

		// Make sure that DUT sees AP2 before attempting to connect to AP1.
		clientIface, err := tf.ClientInterface(ctx)
		if err != nil {
			s.Fatal("Failed to get client interface: ", err)
		}
		if err := tf.WifiClient().DiscoverBSSID(ctx, ap2.Config().BSSID, clientIface, []byte(ssid)); err != nil {
			s.Fatal("Failed to discover AP2: ", err)
		}
	}

	_, err = tf.ConnectWifiAPFromDUT(ctx, wificell.DefaultDUT, ap1)
	if err != nil {
		s.Fatal("Failed to connect to WiFi: ", err)
	}
	defer func(ctx context.Context) {
		if err := tf.DisconnectDUTFromWifi(ctx, wificell.DefaultDUT); err != nil {
			s.Error("Failed to disconnect WiFi: ", err)
		}
	}(ctx)
	ctx, cancel = tf.ReserveForDisconnect(ctx)
	defer cancel()

	srvcResp, err := tf.WifiClient().QueryService(ctx)
	if err != nil {
		s.Fatal("Failed to get service properties: ", err)
	}
	s.Log("Connected with Security: ", srvcResp.Wifi.Security)
	if srvcResp.Wifi.Security != tc.expectedSecurity {
		s.Fatalf("Wrong service security: got %s, want %s",
			srvcResp.Wifi.Security, tc.expectedSecurity)
	}
}

// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wifi

import (
	"context"

	"chromiumos/tast/common/shillconst"
	"chromiumos/tast/common/wifi/security"
	"chromiumos/tast/common/wifi/security/wpa"
	"chromiumos/tast/remote/wificell"
	ap "chromiumos/tast/remote/wificell/hostapd"
	"chromiumos/tast/testing"
)

type secConf struct {
	name             string
	config           security.ConfigFactory
	expectedSecurity string
}

func init() {
	testing.AddTest(&testing.Test{
		Func: FgsecWpaUpgrade,
		Desc: "Verifies connection to AP that has upgraded security",
		Contacts: []string{
			"amo@semihalf.com",                // Test author
			"chromeos-wifi-champs@google.com", // WiFi oncall rotation; or http://b/new?component=893827
		},
		Attr:         []string{"group:wificell", "wificell_func", "wificell_unstable"},
		ServiceDeps:  []string{wificell.TFServiceName},
		SoftwareDeps: []string{"wpa3_sae"},
		Fixture:      "wificellFixt",
	})
}

func FgsecWpaUpgrade(ctx context.Context, s *testing.State) {
	tf := s.FixtValue().(*wificell.TestFixture)
	ssid := ap.RandomSSID("TAST_FGSEC_")
	apOpts := []ap.Option{ap.SSID(ssid), ap.Mode(ap.Mode80211g), ap.Channel(1)}

	connectAP := func(ctx context.Context, s *testing.State, sec *secConf) {
		ap, err := tf.ConfigureAP(ctx, apOpts, sec.config)
		if err != nil {
			s.Fatal("Failed to configure the AP1: ", err)
		}
		cleanUpCtx := ctx
		ctx, cancel := tf.ReserveForDeconfigAP(ctx, ap)
		defer cancel()
		defer func(ctx context.Context) {
			if err := tf.DeconfigAP(ctx, ap); err != nil {
				s.Error("Failed to deconfig the AP: ", err)
			}
		}(cleanUpCtx)
		_, err = tf.ConnectWifiAP(ctx, ap)
		if err != nil {
			s.Fatal("Failed to connect to WiFi: ", err)
		}
		srvcResp, err := tf.WifiClient().QueryService(ctx)
		if err != nil {
			s.Fatal("Failed to get service properties: ", err)
		}
		s.Log("Connected with Security: ", srvcResp.Wifi.Security)
		if srvcResp.Wifi.Security != sec.expectedSecurity {
			s.Fatalf("Wrong service security: got %s, want %s",
				srvcResp.Wifi.Security, sec.expectedSecurity)
		}
		if err := tf.DisconnectWifi(ctx); err != nil {
			s.Fatal("Failed to disconnect WiFi: ", err)
		}
	}

	securityConfigs := []secConf{
		{
			config: wpa.NewConfigFactory("chromeos-wpa",
				wpa.Mode(wpa.ModePureWPA), wpa.Ciphers(wpa.CipherTKIP)),
			expectedSecurity: shillconst.SecurityWPA,
		},
		{
			config: wpa.NewConfigFactory("chromeos-wpa2",
				wpa.Mode(wpa.ModePureWPA2), wpa.Ciphers2(wpa.CipherCCMP)),
			expectedSecurity: shillconst.SecurityWPAWPA2,
		},
		{
			config: wpa.NewConfigFactory("chromeos-wpa3",
				wpa.Mode(wpa.ModePureWPA3), wpa.Ciphers2(wpa.CipherCCMP)),
			expectedSecurity: shillconst.SecurityWPAAll,
		},
	}

	// Get the name of the DUT WiFi interface to flush BSS from WPA
	// supplicant after each connection to make sure it uses
	// currently visible BSSes for reconnection.
	clientIface, err := tf.ClientInterface(ctx)
	if err != nil {
		s.Fatal("Unable to get DUT interface name: ", err)
	}
	for _, c := range securityConfigs {
		connectAP(ctx, s, &c)
		s.Log("Flushing BSS cache")
		if err := tf.WifiClient().FlushBSS(ctx, clientIface, 0); err != nil {
			s.Fatal("Failed to flush BSS list: ", err)
		}
	}
	s.Log("Completed successfully, cleaning up")
}

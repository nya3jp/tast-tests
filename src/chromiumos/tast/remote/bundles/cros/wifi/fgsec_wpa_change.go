// Copyright 2022 The ChromiumOS Authors
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
	config           security.ConfigFactory
	expectedSecurity string
}

func init() {
	testing.AddTest(&testing.Test{
		Func: FgsecWpaChange,
		Desc: "Verifies connection to AP that has changed security",
		Contacts: []string{
			"amo@semihalf.com",                // Test author
			"chromeos-wifi-champs@google.com", // WiFi oncall rotation; or http://b/new?component=893827
		},
		Attr:         []string{"group:wificell", "wificell_func", "wificell_unstable"},
		ServiceDeps:  []string{wificell.TFServiceName},
		SoftwareDeps: []string{"wpa3_sae"},
		Fixture:      "wificellFixt",
		Params: []testing.Param{
			{
				Name: "wpa_1_2_3",
				Val: []secConf{
					{
						config:           wpa.NewConfigFactory("chromeos", wpa.Mode(wpa.ModePureWPA), wpa.Ciphers(wpa.CipherTKIP)),
						expectedSecurity: shillconst.SecurityWPA,
					},
					{
						config:           wpa.NewConfigFactory("chromeos", wpa.Mode(wpa.ModePureWPA2), wpa.Ciphers2(wpa.CipherCCMP)),
						expectedSecurity: shillconst.SecurityWPAWPA2,
					},
					{
						config:           wpa.NewConfigFactory("chromeos", wpa.Mode(wpa.ModePureWPA3), wpa.Ciphers2(wpa.CipherCCMP)),
						expectedSecurity: shillconst.SecurityWPAAll,
					},
				},
			},
			{
				Name: "wpa_2_1_3",
				Val: []secConf{
					{
						config:           wpa.NewConfigFactory("chromeos", wpa.Mode(wpa.ModePureWPA2), wpa.Ciphers2(wpa.CipherCCMP)),
						expectedSecurity: shillconst.SecurityWPA2,
					},
					{
						config:           wpa.NewConfigFactory("chromeos", wpa.Mode(wpa.ModePureWPA), wpa.Ciphers(wpa.CipherTKIP)),
						expectedSecurity: shillconst.SecurityWPAWPA2,
					},
					{
						config:           wpa.NewConfigFactory("chromeos", wpa.Mode(wpa.ModePureWPA3), wpa.Ciphers2(wpa.CipherCCMP)),
						expectedSecurity: shillconst.SecurityWPAAll,
					},
				},
			},
			{
				Name: "wpa_1_3",
				Val: []secConf{
					{
						config:           wpa.NewConfigFactory("chromeos", wpa.Mode(wpa.ModePureWPA), wpa.Ciphers(wpa.CipherTKIP)),
						expectedSecurity: shillconst.SecurityWPA,
					},
					{
						config:           wpa.NewConfigFactory("chromeos", wpa.Mode(wpa.ModePureWPA3), wpa.Ciphers2(wpa.CipherCCMP)),
						expectedSecurity: shillconst.SecurityWPAAll,
					},
				},
			},
			{
				Name: "wpa_1_23",
				Val: []secConf{
					{
						config:           wpa.NewConfigFactory("chromeos", wpa.Mode(wpa.ModePureWPA), wpa.Ciphers(wpa.CipherTKIP)),
						expectedSecurity: shillconst.SecurityWPA,
					},
					{
						config:           wpa.NewConfigFactory("chromeos", wpa.Mode(wpa.ModeMixedWPA3), wpa.Ciphers2(wpa.CipherCCMP)),
						expectedSecurity: shillconst.SecurityWPAAll,
					},
				},
			},
			{
				Name: "wpa_2_23_1",
				Val: []secConf{
					{
						config:           wpa.NewConfigFactory("chromeos", wpa.Mode(wpa.ModePureWPA2), wpa.Ciphers2(wpa.CipherCCMP)),
						expectedSecurity: shillconst.SecurityWPA2,
					},
					{
						config:           wpa.NewConfigFactory("chromeos", wpa.Mode(wpa.ModeMixedWPA3), wpa.Ciphers2(wpa.CipherCCMP)),
						expectedSecurity: shillconst.SecurityWPA2WPA3,
					},
					{
						config:           wpa.NewConfigFactory("chromeos", wpa.Mode(wpa.ModePureWPA), wpa.Ciphers(wpa.CipherTKIP)),
						expectedSecurity: shillconst.SecurityWPAAll,
					},
				},
			},
			{
				Name: "wpa_3_12",
				Val: []secConf{
					{
						config:           wpa.NewConfigFactory("chromeos", wpa.Mode(wpa.ModePureWPA3), wpa.Ciphers2(wpa.CipherCCMP)),
						expectedSecurity: shillconst.SecurityWPA3,
					},
					{
						config:           wpa.NewConfigFactory("chromeos", wpa.Mode(wpa.ModeMixed), wpa.Ciphers(wpa.CipherTKIP), wpa.Ciphers2(wpa.CipherCCMP)),
						expectedSecurity: shillconst.SecurityWPAAll,
					},
				},
			},
			{
				Name: "wpa_3_2_1",
				Val: []secConf{
					{
						config:           wpa.NewConfigFactory("chromeos", wpa.Mode(wpa.ModePureWPA3), wpa.Ciphers2(wpa.CipherCCMP)),
						expectedSecurity: shillconst.SecurityWPA3,
					},
					{
						config:           wpa.NewConfigFactory("chromeos", wpa.Mode(wpa.ModePureWPA2), wpa.Ciphers2(wpa.CipherCCMP)),
						expectedSecurity: shillconst.SecurityWPA2WPA3,
					},
					{
						config:           wpa.NewConfigFactory("chromeos", wpa.Mode(wpa.ModePureWPA), wpa.Ciphers(wpa.CipherTKIP)),
						expectedSecurity: shillconst.SecurityWPAAll,
					},
				},
			},
			{
				Name: "wpa_12_3",
				Val: []secConf{
					{
						config:           wpa.NewConfigFactory("chromeos", wpa.Mode(wpa.ModeMixed), wpa.Ciphers(wpa.CipherTKIP), wpa.Ciphers2(wpa.CipherCCMP)),
						expectedSecurity: shillconst.SecurityWPAWPA2,
					},
					{
						config:           wpa.NewConfigFactory("chromeos", wpa.Mode(wpa.ModePureWPA3), wpa.Ciphers2(wpa.CipherCCMP)),
						expectedSecurity: shillconst.SecurityWPAAll,
					},
				},
			},
			{
				Name: "wpa_23_1",
				Val: []secConf{
					{
						config:           wpa.NewConfigFactory("chromeos", wpa.Mode(wpa.ModeMixedWPA3), wpa.Ciphers2(wpa.CipherCCMP)),
						expectedSecurity: shillconst.SecurityWPA2WPA3,
					},
					{
						config:           wpa.NewConfigFactory("chromeos", wpa.Mode(wpa.ModePureWPA), wpa.Ciphers(wpa.CipherTKIP)),
						expectedSecurity: shillconst.SecurityWPAAll,
					},
				},
			},
		},
	})
}

// FgsecWpaChange tests connectivity to an AP with changed WPA settings
// security mode.  Each subtest is a loop over list of security configuration specified above.  For
// each element:
// 1. Configure AP according to security configuration (keeping SSID so all the time it is regarded
//    as the same network by the shill)
// 2. Test ability to connect.
// 3. Check that:
//    - the service path has not changed,
//    - service has correct Security property.
// 4. Deconfigure AP.
func FgsecWpaChange(ctx context.Context, s *testing.State) {
	tf := s.FixtValue().(*wificell.TestFixture)
	ssid := ap.RandomSSID("TAST_FGSEC_")
	apOpts := []ap.Option{ap.SSID(ssid), ap.Mode(ap.Mode80211g), ap.Channel(1)}
	servicePath := ""

	connectAP := func(ctx context.Context, s *testing.State, sec *secConf) {
		ap, err := tf.ConfigureAP(ctx, apOpts, sec.config)
		if err != nil {
			s.Fatal("Failed to configure the AP1: ", err)
		}
		defer func(ctx context.Context) {
			if err := tf.DeconfigAP(ctx, ap); err != nil {
				s.Error("Failed to deconfig the AP: ", err)
			}
		}(ctx)
		ctx, cancel := tf.ReserveForDeconfigAP(ctx, ap)
		defer cancel()

		connResp, err := tf.ConnectWifiAPFromDUT(ctx, wificell.DefaultDUT, ap)
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

		if servicePath == "" {
			servicePath = connResp.ServicePath
		} else if servicePath != connResp.ServicePath {
			s.Fatalf("Service path has changed: got %s, want %s", connResp.ServicePath, servicePath)
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
	}

	securityConfigs := s.Param().([]secConf)

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
}

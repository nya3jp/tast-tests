// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wifi

import (
	"context"
	"encoding/hex"
	"strings"

	"chromiumos/tast/common/crypto/certificate"
	"chromiumos/tast/common/shillconst"
	"chromiumos/tast/common/wifi/security"
	"chromiumos/tast/common/wifi/security/wpa"
	"chromiumos/tast/common/wifi/security/wpaeap"
	"chromiumos/tast/remote/wificell"
	hap "chromiumos/tast/remote/wificell/hostapd"
	"chromiumos/tast/testing"
)

type apConfig struct {
	router           int
	apOpts           []hap.Option
	secConf          security.ConfigFactory
	expectedSecurity string
}

// EAP cert for EAP tests.
var cert1 = certificate.TestCert1()

func init() {
	testing.AddTest(&testing.Test{
		Func: FgsecMultiConnect,
		Desc: "Verifies connectivity with more detailed security settings than just broad PSK class",
		Contacts: []string{
			"amo@semihalf.com",                // Test author
			"chromeos-wifi-champs@google.com", // WiFi oncall rotation; or http://b/new?component=893827
		},
		Attr:        []string{"group:wificell", "wificell_func", "wificell_unstable"},
		ServiceDeps: []string{wificell.TFServiceName},
		Fixture:     "wificellFixtRouters",
		Params: []testing.Param{
			{
				// WpaWpa2 mixed mode for network with two APs configured with Wpa and Wpa2.
				Name: "wpawpa2_1_2",
				Val: []apConfig{
					{
						apOpts:           []hap.Option{hap.Mode(hap.Mode80211nPure), hap.Channel(1), hap.HTCaps(hap.HTCapHT20)},
						secConf:          wpa.NewConfigFactory("chromeos", wpa.Mode(wpa.ModePureWPA), wpa.Ciphers(wpa.CipherTKIP)),
						expectedSecurity: shillconst.SecurityWPA,
					}, {
						apOpts:           []hap.Option{hap.Mode(hap.Mode80211nPure), hap.Channel(48), hap.HTCaps(hap.HTCapHT20)},
						secConf:          wpa.NewConfigFactory("chromeos", wpa.Mode(wpa.ModePureWPA2), wpa.Ciphers2(wpa.CipherCCMP)),
						expectedSecurity: shillconst.SecurityWPAWPA2,
					},
				},
			},
			{
				// Wpa2Wpa3 mixed mode for network with two APs configured with Wpa2 and Wpa3.
				Name: "wpa2wpa3_2_3",
				Val: []apConfig{
					{
						apOpts:           []hap.Option{hap.Mode(hap.Mode80211nPure), hap.Channel(1), hap.HTCaps(hap.HTCapHT20)},
						secConf:          wpa.NewConfigFactory("chromeos", wpa.Mode(wpa.ModePureWPA2), wpa.Ciphers2(wpa.CipherCCMP)),
						expectedSecurity: shillconst.SecurityWPA2,
					}, {
						apOpts:           []hap.Option{hap.Mode(hap.Mode80211nPure), hap.Channel(48), hap.HTCaps(hap.HTCapHT20)},
						secConf:          wpa.NewConfigFactory("chromeos", wpa.Mode(wpa.ModePureWPA3), wpa.Ciphers2(wpa.CipherCCMP)),
						expectedSecurity: shillconst.SecurityWPA2WPA3,
					},
				},
				ExtraSoftwareDeps: []string{"wpa3_sae"},
			},
			{
				// Wpa2Wpa3 mixed mode for network with two APs configured with Wpa2Wpa3 and Wpa3.
				Name: "wpa2wpa3_23_2",
				Val: []apConfig{
					{
						apOpts:           []hap.Option{hap.Mode(hap.Mode80211nPure), hap.Channel(1), hap.HTCaps(hap.HTCapHT20)},
						secConf:          wpa.NewConfigFactory("chromeos", wpa.Mode(wpa.ModeMixedWPA3), wpa.Ciphers2(wpa.CipherCCMP)),
						expectedSecurity: shillconst.SecurityWPA3,
					}, {
						apOpts:           []hap.Option{hap.Mode(hap.Mode80211nPure), hap.Channel(48), hap.HTCaps(hap.HTCapHT20)},
						secConf:          wpa.NewConfigFactory("chromeos", wpa.Mode(wpa.ModePureWPA2), wpa.Ciphers2(wpa.CipherCCMP)),
						expectedSecurity: shillconst.SecurityWPA2WPA3,
					},
				},
				ExtraSoftwareDeps: []string{"wpa3_sae"},
			},
			{
				// Wpa2Wpa3 mixed mode for network with two APs configured with Wpa2 and Wpa3.
				Name: "wpa2wpa3_23_3",
				Val: []apConfig{
					{
						apOpts:           []hap.Option{hap.Mode(hap.Mode80211nPure), hap.Channel(1), hap.HTCaps(hap.HTCapHT20)},
						secConf:          wpa.NewConfigFactory("chromeos", wpa.Mode(wpa.ModeMixedWPA3), wpa.Ciphers2(wpa.CipherCCMP)),
						expectedSecurity: shillconst.SecurityWPA2WPA3,
					}, {
						apOpts:           []hap.Option{hap.Mode(hap.Mode80211nPure), hap.Channel(48), hap.HTCaps(hap.HTCapHT20)},
						secConf:          wpa.NewConfigFactory("chromeos", wpa.Mode(wpa.ModePureWPA3), wpa.Ciphers2(wpa.CipherCCMP)),
						expectedSecurity: shillconst.SecurityWPA2WPA3,
					},
				},
				ExtraSoftwareDeps: []string{"wpa3_sae"},
			},
			{
				// WpaAll mode for network with two APs configured with WpaWpa2 and Wpa3.
				Name: "wpaall_12_3",
				Val: []apConfig{
					{
						apOpts:           []hap.Option{hap.Mode(hap.Mode80211nPure), hap.Channel(1), hap.HTCaps(hap.HTCapHT20)},
						secConf:          wpa.NewConfigFactory("chromeos", wpa.Mode(wpa.ModeMixed), wpa.Ciphers(wpa.CipherTKIP, wpa.CipherCCMP), wpa.Ciphers2(wpa.CipherCCMP)),
						expectedSecurity: shillconst.SecurityWPAWPA2,
					}, {
						apOpts:           []hap.Option{hap.Mode(hap.Mode80211nPure), hap.Channel(48), hap.HTCaps(hap.HTCapHT20)},
						secConf:          wpa.NewConfigFactory("chromeos", wpa.Mode(wpa.ModePureWPA3), wpa.Ciphers2(wpa.CipherCCMP)),
						expectedSecurity: shillconst.SecurityWPAAll,
					},
				},
				ExtraSoftwareDeps: []string{"wpa3_sae"},
			},
			{
				// WpaAll mode for network with three APs configured with Wpa, Wpa2 and Wpa3.
				Name: "wpaall_1_2_3",
				Val: []apConfig{
					{
						router:           0,
						apOpts:           []hap.Option{hap.Mode(hap.Mode80211nPure), hap.Channel(1), hap.HTCaps(hap.HTCapHT20)},
						secConf:          wpa.NewConfigFactory("chromeos", wpa.Mode(wpa.ModePureWPA), wpa.Ciphers(wpa.CipherTKIP)),
						expectedSecurity: shillconst.SecurityWPA,
					}, {
						router:           0,
						apOpts:           []hap.Option{hap.Mode(hap.Mode80211nPure), hap.Channel(48), hap.HTCaps(hap.HTCapHT20)},
						secConf:          wpa.NewConfigFactory("chromeos", wpa.Mode(wpa.ModePureWPA2), wpa.Ciphers2(wpa.CipherCCMP)),
						expectedSecurity: shillconst.SecurityWPAWPA2,
					},
					{
						router:           1,
						apOpts:           []hap.Option{hap.Mode(hap.Mode80211nPure), hap.Channel(1), hap.HTCaps(hap.HTCapHT20)},
						secConf:          wpa.NewConfigFactory("chromeos", wpa.Mode(wpa.ModePureWPA3), wpa.Ciphers2(wpa.CipherCCMP)),
						expectedSecurity: shillconst.SecurityWPAAll,
					},
				},
				ExtraSoftwareDeps: []string{"wpa3_sae"},
			},
			// And the same below but this time in "Enterprise" mode.
			{
				Name: "8021xwpawpa2_1_2",
				Val: []apConfig{
					{
						apOpts: []hap.Option{hap.Mode(hap.Mode80211nPure), hap.Channel(1), hap.HTCaps(hap.HTCapHT20)},
						secConf: wpaeap.NewConfigFactory(
							cert1.CACred.Cert, cert1.ServerCred,
							wpaeap.ClientCACert(cert1.CACred.Cert),
							wpaeap.ClientCred(cert1.ClientCred),
							wpaeap.Mode(wpa.ModePureWPA)),
						expectedSecurity: shillconst.SecurityWPAEnterprise,
					}, {
						apOpts: []hap.Option{hap.Mode(hap.Mode80211nPure), hap.Channel(48), hap.HTCaps(hap.HTCapHT20)},
						secConf: wpaeap.NewConfigFactory(
							cert1.CACred.Cert, cert1.ServerCred,
							wpaeap.ClientCACert(cert1.CACred.Cert),
							wpaeap.ClientCred(cert1.ClientCred),
							wpaeap.Mode(wpa.ModePureWPA2)),
						expectedSecurity: shillconst.SecurityWPAWPA2Enterprise,
					},
				},
			},
			{
				Name: "8021xwpa2wpa3_2_3",
				Val: []apConfig{
					{
						apOpts: []hap.Option{hap.Mode(hap.Mode80211nPure), hap.Channel(1), hap.HTCaps(hap.HTCapHT20)},
						secConf: wpaeap.NewConfigFactory(
							cert1.CACred.Cert, cert1.ServerCred,
							wpaeap.ClientCACert(cert1.CACred.Cert),
							wpaeap.ClientCred(cert1.ClientCred),
							wpaeap.Mode(wpa.ModePureWPA2)),
						expectedSecurity: shillconst.SecurityWPA2Enterprise,
					}, {
						apOpts: []hap.Option{hap.Mode(hap.Mode80211nPure), hap.Channel(48), hap.HTCaps(hap.HTCapHT20)},
						secConf: wpaeap.NewConfigFactory(
							cert1.CACred.Cert, cert1.ServerCred,
							wpaeap.ClientCACert(cert1.CACred.Cert),
							wpaeap.ClientCred(cert1.ClientCred),
							wpaeap.Mode(wpa.ModePureWPA3)),
						expectedSecurity: shillconst.SecurityWPA2WPA3Enterprise,
					},
				},
				ExtraSoftwareDeps: []string{"wpa3_sae"},
			},
			{
				Name: "8021xwpa2wpa3_23_2",
				Val: []apConfig{
					{
						apOpts: []hap.Option{hap.Mode(hap.Mode80211nPure), hap.Channel(1), hap.HTCaps(hap.HTCapHT20)},
						secConf: wpaeap.NewConfigFactory(
							cert1.CACred.Cert, cert1.ServerCred,
							wpaeap.ClientCACert(cert1.CACred.Cert),
							wpaeap.ClientCred(cert1.ClientCred),
							wpaeap.Mode(wpa.ModeMixedWPA3)),
						expectedSecurity: shillconst.SecurityWPA3Enterprise,
					}, {
						apOpts: []hap.Option{hap.Mode(hap.Mode80211nPure), hap.Channel(48), hap.HTCaps(hap.HTCapHT20)},
						secConf: wpaeap.NewConfigFactory(
							cert1.CACred.Cert, cert1.ServerCred,
							wpaeap.ClientCACert(cert1.CACred.Cert),
							wpaeap.ClientCred(cert1.ClientCred),
							wpaeap.Mode(wpa.ModePureWPA2)),
						expectedSecurity: shillconst.SecurityWPA2WPA3Enterprise,
					},
				},
				ExtraSoftwareDeps: []string{"wpa3_sae"},
			},
			{
				Name: "8021xwpa2wpa3_23_3",
				Val: []apConfig{
					{
						apOpts: []hap.Option{hap.Mode(hap.Mode80211nPure), hap.Channel(1), hap.HTCaps(hap.HTCapHT20)},
						secConf: wpaeap.NewConfigFactory(
							cert1.CACred.Cert, cert1.ServerCred,
							wpaeap.ClientCACert(cert1.CACred.Cert),
							wpaeap.ClientCred(cert1.ClientCred),
							wpaeap.Mode(wpa.ModeMixedWPA3)),
						expectedSecurity: shillconst.SecurityWPA2WPA3Enterprise,
					}, {
						apOpts: []hap.Option{hap.Mode(hap.Mode80211nPure), hap.Channel(48), hap.HTCaps(hap.HTCapHT20)},
						secConf: wpaeap.NewConfigFactory(
							cert1.CACred.Cert, cert1.ServerCred,
							wpaeap.ClientCACert(cert1.CACred.Cert),
							wpaeap.ClientCred(cert1.ClientCred),
							wpaeap.Mode(wpa.ModePureWPA3)),
						expectedSecurity: shillconst.SecurityWPA2WPA3Enterprise,
					},
				},
				ExtraSoftwareDeps: []string{"wpa3_sae"},
			},
			{
				Name: "8021xwpaall_12_3",
				Val: []apConfig{
					{
						apOpts: []hap.Option{hap.Mode(hap.Mode80211nPure), hap.Channel(1), hap.HTCaps(hap.HTCapHT20)},
						secConf: wpaeap.NewConfigFactory(
							cert1.CACred.Cert, cert1.ServerCred,
							wpaeap.ClientCACert(cert1.CACred.Cert),
							wpaeap.ClientCred(cert1.ClientCred),
							wpaeap.Mode(wpa.ModeMixed)),
						expectedSecurity: shillconst.SecurityWPAWPA2Enterprise,
					}, {
						apOpts: []hap.Option{hap.Mode(hap.Mode80211nPure), hap.Channel(48), hap.HTCaps(hap.HTCapHT20)},
						secConf: wpaeap.NewConfigFactory(
							cert1.CACred.Cert, cert1.ServerCred,
							wpaeap.ClientCACert(cert1.CACred.Cert),
							wpaeap.ClientCred(cert1.ClientCred),
							wpaeap.Mode(wpa.ModePureWPA3)),
						expectedSecurity: shillconst.SecurityWPAAllEnterprise,
					},
				},
				ExtraSoftwareDeps: []string{"wpa3_sae"},
			},
			{
				Name: "8021xwpaall_1_2_3",
				Val: []apConfig{
					{
						router: 0,
						apOpts: []hap.Option{hap.Mode(hap.Mode80211nPure), hap.Channel(1), hap.HTCaps(hap.HTCapHT20)},
						secConf: wpaeap.NewConfigFactory(
							cert1.CACred.Cert, cert1.ServerCred,
							wpaeap.ClientCACert(cert1.CACred.Cert),
							wpaeap.ClientCred(cert1.ClientCred),
							wpaeap.Mode(wpa.ModePureWPA)),
						expectedSecurity: shillconst.SecurityWPAEnterprise,
					}, {
						router: 0,
						apOpts: []hap.Option{hap.Mode(hap.Mode80211nPure), hap.Channel(48), hap.HTCaps(hap.HTCapHT20)},
						secConf: wpaeap.NewConfigFactory(
							cert1.CACred.Cert, cert1.ServerCred,
							wpaeap.ClientCACert(cert1.CACred.Cert),
							wpaeap.ClientCred(cert1.ClientCred),
							wpaeap.Mode(wpa.ModePureWPA2)),
						expectedSecurity: shillconst.SecurityWPAWPA2Enterprise,
					},
					{
						router: 1,
						apOpts: []hap.Option{hap.Mode(hap.Mode80211nPure), hap.Channel(1), hap.HTCaps(hap.HTCapHT20)},
						secConf: wpaeap.NewConfigFactory(
							cert1.CACred.Cert, cert1.ServerCred,
							wpaeap.ClientCACert(cert1.CACred.Cert),
							wpaeap.ClientCred(cert1.ClientCred),
							wpaeap.Mode(wpa.ModePureWPA3)),
						expectedSecurity: shillconst.SecurityWPAAllEnterprise,
					},
				},
				ExtraSoftwareDeps: []string{"wpa3_sae"},
			},
		},
	})
}

func getServicePath(ctx context.Context, tf *wificell.TestFixture, ssid string, sec *security.ConfigFactory) (string, error) {
	conf, err := (*sec).Gen()
	if err != nil {
		return "", err
	}
	props := map[string]interface{}{
		shillconst.ServicePropertyType:          shillconst.TypeWifi,
		shillconst.ServicePropertyWiFiHexSSID:   strings.ToUpper(hex.EncodeToString([]byte(ssid))),
		shillconst.ServicePropertySecurityClass: conf.Class(),
	}
	return tf.WifiClient().GetServicePath(ctx, props)
}

// FgsecMultiConnect tests connectivity to a various network configurations (parametrized above) and
// verifies that DUT is able to connect and that the resulting Security of the service matches the
// expectations.
// Step-by-step procedure:
// 1. Configure APs with security specified by the test case (all APs form a single network).
// 2. Make sure that all network endpoints have been noticed.
// 3. Connect to the network.
// 4. Query service Security and check if it agrees with expectation.
// 5. Disconnect and deconfigure AP(s)
func FgsecMultiConnect(ctx context.Context, s *testing.State) {
	tf := s.FixtValue().(*wificell.TestFixture)
	ssid := hap.RandomSSID("TAST_FGSEC_")
	apConfigs := s.Param().([]apConfig)

	var cancel context.CancelFunc
	var ap *wificell.APIface
	var srvcPath string
	var security string

	clientIface, err := tf.ClientInterface(ctx)
	if err != nil {
		s.Fatal("Failed to get client interface: ", err)
	}

	for _, cfg := range apConfigs {
		ap, err = tf.ConfigureAPOnRouterID(ctx, cfg.router, append(cfg.apOpts, hap.SSID(ssid)), cfg.secConf, false, false)
		if err != nil {
			s.Fatal("Failed to configure the AP1: ", err)
		}
		defer func(ctx context.Context) {
			if err := tf.DeconfigAP(ctx, ap); err != nil {
				s.Error("Failed to deconfig the AP: ", err)
			}
		}(ctx)
		ctx, cancel = tf.ReserveForDeconfigAP(ctx, ap)
		defer cancel()

		if err := tf.WifiClient().DiscoverBSSID(ctx, ap.Config().BSSID, clientIface, []byte(ssid)); err != nil {
			s.Fatal("Failed to discover AP: ", err)
		}
		if srvcPath == "" {
			srvcPath, err = getServicePath(ctx, tf, ssid, &cfg.secConf)
			if err != nil {
				s.Fatal("Failed to get service path: ", err)
			}
		}
		rsp, err := tf.WifiClient().QueryServiceWithPath(ctx, srvcPath)
		if err != nil {
			s.Fatal("Failed to get service properties: ", err)
		}
		security = rsp.GetWifi().GetSecurity()
		s.Log("Current service security: ", security)
		if security != cfg.expectedSecurity {
			s.Fatalf("Wrong security: got %s, want %s", security, cfg.expectedSecurity)
		}
	}
	rsp, err := tf.ConnectWifiAPFromDUT(ctx, wificell.DefaultDUT, ap)
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

	if srvcPath != rsp.ServicePath {
		s.Logf("Service paths differ: got %s, want %s", srvcPath, rsp.ServicePath)
	}

	srvcResp, err := tf.WifiClient().QueryService(ctx)
	if err != nil {
		s.Fatal("Failed to get service properties: ", err)
	}
	s.Log("Connected with Security: ", srvcResp.Wifi.Security)
	if srvcResp.Wifi.Security != security {
		s.Fatalf("Wrong service security: got %s, want %s", srvcResp.Wifi.Security, security)
	}
}

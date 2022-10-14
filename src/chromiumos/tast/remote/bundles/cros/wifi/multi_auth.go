// Copyright 2020 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wifi

import (
	"context"
	"fmt"

	"chromiumos/tast/common/crypto/certificate"
	"chromiumos/tast/common/wifi/security"
	"chromiumos/tast/common/wifi/security/wpa"
	"chromiumos/tast/common/wifi/security/wpaeap"
	"chromiumos/tast/remote/wificell"
	"chromiumos/tast/remote/wificell/hostapd"
	"chromiumos/tast/testing"
)

type multiAuthParam struct {
	name []string
	cfg  []security.ConfigFactory
}

// EAP certs/keys for EAP tests.
var (
	multiAuthCert = certificate.TestCert1()
)

func init() {
	testing.AddTest(&testing.Test{
		Func: MultiAuth,
		Desc: "Tests the ability to select network correctly among APs with similar network configurations, by configuring two APs with the same SSID/channel/mode but different security config and connecting to each in turn",
		Contacts: []string{
			"chromeos-wifi-champs@google.com", // WiFi oncall rotation; or http://b/new?component=893827
		},
		Attr:        []string{"group:wificell", "wificell_func", "wificell_unstable"},
		ServiceDeps: []string{wificell.TFServiceName},
		Fixture:     "wificellFixtWithCapture",
	})
}

/*
This test is to check the ability to identify the target network from APs with
similar network configurations and connect to the target network correctly, even
if it may be less secure.
*/

func MultiAuth(ctx context.Context, s *testing.State) {
	testOnce := func(ctx context.Context, s *testing.State, param multiAuthParam) {
		tf := s.FixtValue().(*wificell.TestFixture)

		apOpts := []hostapd.Option{hostapd.SSID(hostapd.RandomSSID("TAST_TEST_MultiAuth")), hostapd.Mode(hostapd.Mode80211g), hostapd.Channel(1)}

		s.Logf("Configuring AP 0 (%s)", param.name[0])
		ap0, err := tf.ConfigureAP(ctx, apOpts, param.cfg[0])
		if err != nil {
			s.Fatal("Failed to configure AP 0: ", err)
		}
		defer func(ctx context.Context) {
			if err := tf.DeconfigAP(ctx, ap0); err != nil {
				s.Error("Failed to deconfig AP 0: ", err)
			}
		}(ctx)
		ctx, cancel := tf.ReserveForDeconfigAP(ctx, ap0)
		defer cancel()

		s.Logf("Configuring AP 1 (%s)", param.name[1])
		ap1, err := tf.ConfigureAP(ctx, apOpts, param.cfg[1])
		if err != nil {
			s.Fatal("Failed to configure AP 1: ", err)
		}
		defer func(ctx context.Context) {
			if err := tf.DeconfigAP(ctx, ap1); err != nil {
				s.Error("Failed to deconfig AP 1: ", err)
			}
		}(ctx)
		ctx, cancel = tf.ReserveForDeconfigAP(ctx, ap1)
		defer cancel()

		s.Log("Connecting to AP 0")
		if _, err := tf.ConnectWifiAP(ctx, ap0); err != nil {
			s.Fatal("Failed to connect to AP 0: ", err)
		}
		defer func(ctx context.Context) {
			if err := tf.CleanDisconnectWifi(ctx); err != nil {
				s.Error("Failed to disconnect WiFi: ", err)
			}
		}(ctx)
		ctx, cancel = tf.ReserveForDisconnect(ctx)
		defer cancel()
		s.Log("Verifying connection to AP 0")
		if err := tf.VerifyConnection(ctx, ap0); err != nil {
			s.Fatal("Failed to verify connection: ", err)
		}

		s.Log("Connecting to AP 1")
		if _, err := tf.ConnectWifiAP(ctx, ap1); err != nil {
			s.Fatal("Failed to connect to AP 1: ", err)
		}
		s.Log("Verifying connection to AP 1")
		if err := tf.VerifyConnection(ctx, ap1); err != nil {
			s.Fatal("Failed to verify connection: ", err)
		}
	}

	testcases := []multiAuthParam{
		{
			name: []string{"Open", "WPA"},
			cfg:  []security.ConfigFactory{nil, wpa.NewConfigFactory("chromeos", wpa.Mode(wpa.ModePureWPA), wpa.Ciphers(wpa.CipherCCMP))},
		},
		{
			// In WPA-PSK, all stations share the same passphrase, and its
			// leakage may result in attack. WPA-EAP reduces the risk by
			// adopting a RADIUS server to authenticate the clients.
			name: []string{"WPA", "WPA-EAP"},
			cfg: []security.ConfigFactory{wpa.NewConfigFactory("chromeos", wpa.Mode(wpa.ModePureWPA), wpa.Ciphers(wpa.CipherCCMP)),
				wpaeap.NewConfigFactory(multiAuthCert.CACred.Cert, multiAuthCert.ServerCred, wpaeap.ClientCACert(multiAuthCert.CACred.Cert), wpaeap.ClientCred(multiAuthCert.ClientCred))},
		},
	}
	for i, tc := range testcases {
		subtest := func(ctx context.Context, s *testing.State) {
			testOnce(ctx, s, tc)
		}
		if !s.Run(ctx, fmt.Sprintf("Testcase #%d (%s vs %s)", i, tc.name[0], tc.name[1]), subtest) {
			// Stop if one of the subtest's parameter set fails the test.
			return
		}
	}
}

// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wifi

import (
	"context"
	"time"

	"chromiumos/tast/common/crypto/certificate"
	"chromiumos/tast/common/shillconst"
	"chromiumos/tast/common/wifi/security"
	"chromiumos/tast/common/wifi/security/wep"
	"chromiumos/tast/common/wifi/security/wpa"
	"chromiumos/tast/common/wifi/security/wpaeap"
	"chromiumos/tast/remote/wificell"
	"chromiumos/tast/remote/wificell/hostapd"
	"chromiumos/tast/services/cros/wifi"
	"chromiumos/tast/testing"
)

type roamTestcase struct {
	apOpts1    []hostapd.Option
	apOpts2    []hostapd.Option
	secConfFac security.ConfigFactory
}

// EAP certs/keys for EAP tests.
var (
	roamCert = certificate.TestCert1()
)

func init() {
	testing.AddTest(&testing.Test{
		Func: RoamAPGone,
		Desc: "Tests roaming to an AP that disappears while the client is awake",
		Contacts: []string{
			"chromeos-wifi-champs@google.com", // WiFi oncall rotation; or http://b/new?component=893827
		},
		Attr:        []string{"group:wificell", "wificell_func"},
		ServiceDeps: []string{wificell.TFServiceName},
		Fixture:     "wificellFixtWithCapture",
		Params: []testing.Param{
			{
				ExtraAttr: []string{"wificell_cq"},
				// Verifies that DUT can roam between two APs in full view of it.
				Val: roamTestcase{
					apOpts1:    []hostapd.Option{hostapd.Mode(hostapd.Mode80211nPure), hostapd.Channel(1), hostapd.HTCaps(hostapd.HTCapHT20)},
					apOpts2:    []hostapd.Option{hostapd.Mode(hostapd.Mode80211nPure), hostapd.Channel(48), hostapd.HTCaps(hostapd.HTCapHT20)},
					secConfFac: nil,
				},
			}, {
				// Verifies that DUT can roam between two WPA APs in full view of it.
				Name: "wpa",
				Val: roamTestcase{
					apOpts1:    []hostapd.Option{hostapd.Mode(hostapd.Mode80211nPure), hostapd.Channel(1), hostapd.HTCaps(hostapd.HTCapHT20)},
					apOpts2:    []hostapd.Option{hostapd.Mode(hostapd.Mode80211nPure), hostapd.Channel(48), hostapd.HTCaps(hostapd.HTCapHT20)},
					secConfFac: wpa.NewConfigFactory("chromeos", wpa.Mode(wpa.ModePureWPA2), wpa.Ciphers2(wpa.CipherCCMP)),
				},
			}, {
				// Verifies that DUT can roam between two WEP APs in full view of it.
				Name: "wep",
				Val: roamTestcase{
					apOpts1:    []hostapd.Option{hostapd.Mode(hostapd.Mode80211nMixed), hostapd.Channel(1), hostapd.HTCaps(hostapd.HTCapHT20)},
					apOpts2:    []hostapd.Option{hostapd.Mode(hostapd.Mode80211nMixed), hostapd.Channel(48), hostapd.HTCaps(hostapd.HTCapHT20)},
					secConfFac: wep.NewConfigFactory([]string{"abcde", "fedcba9876", "ab\xe4\xb8\x89", "\xe4\xb8\x89\xc2\xa2"}, wep.DefaultKey(0), wep.AuthAlgs(wep.AuthAlgoOpen)),
				},
			}, {
				// Verifies that DUT can roam between two WPA-EAP APs in full view of it.
				Name: "8021xwpa",
				Val: roamTestcase{
					apOpts1:    []hostapd.Option{hostapd.Mode(hostapd.Mode80211nPure), hostapd.Channel(1), hostapd.HTCaps(hostapd.HTCapHT20)},
					apOpts2:    []hostapd.Option{hostapd.Mode(hostapd.Mode80211nPure), hostapd.Channel(48), hostapd.HTCaps(hostapd.HTCapHT20)},
					secConfFac: wpaeap.NewConfigFactory(roamCert.CACred.Cert, roamCert.ServerCred, wpaeap.ClientCACert(roamCert.CACred.Cert), wpaeap.ClientCred(roamCert.ClientCred)),
				},
			},
		},
	})
}

func RoamAPGone(ctx context.Context, s *testing.State) {
	// This test associates a device to an AP, and then configures another AP on the same SSID.
	// It verifies that, after we deconfigure the first AP, the DUT eventually associates to
	// the second AP."
	tf := s.FixtValue().(*wificell.TestFixture)

	// Configure the initial AP.
	param := s.Param().(roamTestcase)
	ap1, err := tf.ConfigureAP(ctx, param.apOpts1, param.secConfFac)
	if err != nil {
		s.Fatal("Failed to configure ap, err: ", err)
	}
	ssid := ap1.Config().SSID
	defer func(ctx context.Context) {
		if ap1 == nil {
			// ap1 is already deconfigured.
			return
		}
		if err := tf.DeconfigAP(ctx, ap1); err != nil {
			s.Error("Failed to deconfig ap1, err: ", err)
		}
	}(ctx)
	ctx, cancel := tf.ReserveForDeconfigAP(ctx, ap1)
	defer cancel()
	s.Log("AP1 setup done")

	// Schedule defer for AP2 before connection. Otherwise, we
	// will teardown AP2 before disconnect, and the DUT might
	// get disconnected due to inactivity and causing flaky
	// Disconnect failure.
	var ap2 *wificell.APIface
	defer func(ctx context.Context) {
		if ap2 == nil {
			return
		}
		if err := tf.DeconfigAP(ctx, ap2); err != nil {
			s.Error("Failed to deconfig ap2, err: ", err)
		}
	}(ctx)
	// We don't have ap2 yet, borrow the reserve of ap1.
	ctx, cancel = tf.ReserveForDeconfigAP(ctx, ap1)
	defer cancel()

	// Connect to the initial AP.
	var servicePath string
	if resp, err := tf.ConnectWifiAP(ctx, ap1); err != nil {
		s.Fatal("Failed to connect to WiFi, err: ", err)
	} else {
		servicePath = resp.ServicePath
	}
	defer func(ctx context.Context) {
		if err := tf.CleanDisconnectWifi(ctx); err != nil {
			s.Error("Failed to disconnect WiFi, err: ", err)
		}
	}(ctx)
	ctx, cancel = tf.ReserveForDisconnect(ctx)
	defer cancel()
	s.Log("Connected to AP1")

	if err := tf.VerifyConnection(ctx, ap1); err != nil {
		s.Fatal("Failed to verify connection: ", err)
	}

	// Generate the BSSID for second AP.
	mac, err := hostapd.RandomMAC()
	if err != nil {
		s.Fatal("Failed to generate random BSSID: ", err)
	}
	ap2BSSID := mac.String()

	props := []*wificell.ShillProperty{
		&wificell.ShillProperty{
			Property:       shillconst.ServicePropertyWiFiBSSID,
			ExpectedValues: []interface{}{ap2BSSID},
			Method:         wifi.ExpectShillPropertyRequest_ON_CHANGE,
		},
	}

	waitCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	waitForProps, err := tf.ExpectShillProperty(waitCtx, servicePath, props, nil)
	if err != nil {
		s.Fatal("DUT: failed to create a property watcher, err: ", err)
	}

	// Configure the second AP.
	var ops []hostapd.Option
	ops = append(ops, param.apOpts2...)
	// Override SSID and BSSID as we need the same SSID as the first AP
	// and the BSSID that we're waiting.
	ops = append(ops, hostapd.SSID(ssid), hostapd.BSSID(ap2BSSID))
	ap2, err = tf.ConfigureAP(ctx, ops, param.secConfFac)
	if err != nil {
		s.Fatal("Failed to configure ap, err: ", err)
	}
	// defer deconfig already scheduled above.
	s.Log("AP2 setup done")

	// Deconfigure the initial AP.
	if err := tf.DeconfigAP(ctx, ap1); err != nil {
		s.Error("Failed to deconfig ap, err: ", err)
	}
	ap1 = nil
	s.Log("Deconfigured AP1")

	if _, err := waitForProps(); err != nil {
		s.Fatal("DUT: failed to wait for the properties, err: ", err)
	}
	s.Log("DUT: roamed")

	if err := testing.Poll(ctx, func(ctx context.Context) error {
		return tf.VerifyConnection(ctx, ap2)
	}, &testing.PollOptions{
		Timeout:  20 * time.Second,
		Interval: time.Second,
	}); err != nil {
		s.Fatal("Failed to verify connection: ", err)
	}
}

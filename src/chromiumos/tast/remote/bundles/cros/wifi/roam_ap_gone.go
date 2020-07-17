// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wifi

import (
	"context"
	"time"

	"chromiumos/tast/common/shillconst"
	"chromiumos/tast/common/wifi/security"
	"chromiumos/tast/common/wifi/security/wep"
	"chromiumos/tast/common/wifi/security/wpa"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/remote/wificell"
	"chromiumos/tast/remote/wificell/hostapd"
	"chromiumos/tast/services/cros/network"
	"chromiumos/tast/testing"
)

type roamTestcase struct {
	apOpts1    []hostapd.Option
	apOpts2    []hostapd.Option
	secConfFac security.ConfigFactory
}

const (
	ap1BSSID = "00:11:22:33:44:55"
	ap2BSSID = "00:11:22:33:44:56"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:        RoamAPGone,
		Desc:        "Tests roaming to an AP that disappears while the client is awake",
		Contacts:    []string{"arowa@google.com", "chromeos-platform-connectivity@google.com"},
		Attr:        []string{"group:wificell", "wificell_func", "wificell_unstable"},
		ServiceDeps: []string{wificell.TFServiceName},
		Pre:         wificell.TestFixturePre(),
		Vars:        []string{"router", "pcap"},
		Params: []testing.Param{
			{
				// Verifies that DUT can roam between two APs in full view of it.
				Val: roamTestcase{
					apOpts1:    []hostapd.Option{hostapd.Mode(hostapd.Mode80211nPure), hostapd.Channel(1), hostapd.HTCaps(hostapd.HTCapHT20), hostapd.BSSID(ap1BSSID)},
					apOpts2:    []hostapd.Option{hostapd.Mode(hostapd.Mode80211nPure), hostapd.Channel(48), hostapd.HTCaps(hostapd.HTCapHT20), hostapd.BSSID(ap2BSSID)},
					secConfFac: nil,
				},
			}, {
				// Verifies that DUT can roam between two WPA APs in full view of it.
				Name: "wpa",
				Val: roamTestcase{
					apOpts1:    []hostapd.Option{hostapd.Mode(hostapd.Mode80211nPure), hostapd.Channel(1), hostapd.HTCaps(hostapd.HTCapHT20), hostapd.BSSID(ap1BSSID)},
					apOpts2:    []hostapd.Option{hostapd.Mode(hostapd.Mode80211nPure), hostapd.Channel(48), hostapd.HTCaps(hostapd.HTCapHT20), hostapd.BSSID(ap2BSSID)},
					secConfFac: wpa.NewConfigFactory("chromeos", wpa.Mode(wpa.ModePureWPA2), wpa.Ciphers2(wpa.CipherCCMP)),
				},
			}, {
				// Verifies that DUT can roam between two WEP APs in full view of it.
				Name: "wep",
				Val: roamTestcase{
					apOpts1:    []hostapd.Option{hostapd.Mode(hostapd.Mode80211b), hostapd.Channel(1), hostapd.BSSID(ap1BSSID)},
					apOpts2:    []hostapd.Option{hostapd.Mode(hostapd.Mode80211a), hostapd.Channel(48), hostapd.BSSID(ap2BSSID)},
					secConfFac: wep.NewConfigFactory([]string{"abcde", "fedcba9876", "ab\xe4\xb8\x89", "\xe4\xb8\x89\xc2\xa2"}, wep.DefaultKey(0), wep.AuthAlgs(wep.AuthAlgoOpen)),
				},
			},
			// TODO(b:161550825): Add a test case for EAP-TLS configuration to verify that DUT can roam between
			// two 802.1x EAP-TLS APs in full view of it. In the meantime, the EAP-TLS is blocked by hwsec's
			// NetCertStore component.
		},
	})
}

func RoamAPGone(ctx context.Context, s *testing.State) {
	tf := s.PreValue().(*wificell.TestFixture)
	defer func(ctx context.Context) {
		if err := tf.CollectLogs(ctx); err != nil {
			s.Log("Error collecting logs, err: ", err)
		}
	}(ctx)
	ctx, cancel := ctxutil.Shorten(ctx, time.Second)
	defer cancel()

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
			s.Error("Failed to deconfig ap, err: ", err)
		}
	}(ctx)
	ctx, cancel = tf.ReserveForDeconfigAP(ctx, ap1)
	defer cancel()
	s.Log("AP1 setup done")

	// Connect to the initial AP.
	if _, err := tf.ConnectWifiAP(ctx, ap1); err != nil {
		s.Fatal("Failed to connect to WiFi, err: ", err)
	}
	defer func(ctx context.Context) {
		if err := tf.DisconnectWifi(ctx); err != nil {
			s.Error("Failed to disconnect WiFi, err: ", err)
		}
		req := &network.DeleteEntriesForSSIDRequest{Ssid: []byte(ssid)}
		if _, err := tf.WifiClient().DeleteEntriesForSSID(ctx, req); err != nil {
			s.Errorf("Failed to remove entries for ssid=%s, err: %v", ssid, err)
		}
	}(ctx)
	ctx, cancel = ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()
	s.Log("Connected to AP1")

	if err := tf.VerifyConnection(ctx, ap1); err != nil {
		s.Fatal("Failed to verify connection: ", err)
	}

	props := []*wificell.ShillProperty{
		&wificell.ShillProperty{
			Property:         shillconst.ServicePropertyState,
			ExpectedValues:   []interface{}{shillconst.ServiceStateConfiguration},
			UnexpectedValues: []interface{}{},
			Method:           network.ExpectShillPropertyRequest_ON_CHANGE,
		},
		&wificell.ShillProperty{
			Property:         shillconst.ServicePropertyIsConnected,
			ExpectedValues:   []interface{}{true},
			UnexpectedValues: []interface{}{},
			Method:           network.ExpectShillPropertyRequest_ON_CHANGE,
		},
		&wificell.ShillProperty{
			Property:         shillconst.ServicePropertyWiFiBSSID,
			ExpectedValues:   []interface{}{ap2BSSID},
			UnexpectedValues: []interface{}{},
			Method:           network.ExpectShillPropertyRequest_CHECK_ONLY,
		},
	}

	waitForProps, err := tf.ExpectShillProperty(ctx, props)
	if err != nil {
		s.Fatal("DUT: failed to create a property watcher, err: ", err)
	}

	// Configure the second AP.
	ops := append([]hostapd.Option{hostapd.SSID(ssid)}, param.apOpts2...)
	ap2, err := tf.ConfigureAP(ctx, ops, param.secConfFac)
	if err != nil {
		s.Fatal("Failed to configure ap, err: ", err)
	}
	defer func(ctx context.Context) {
		if err := tf.DeconfigAP(ctx, ap2); err != nil {
			s.Error("Failed to deconfig ap, err: ", err)
		}
	}(ctx)
	ctx, cancel = tf.ReserveForDeconfigAP(ctx, ap1)
	defer cancel()
	s.Log("AP2 setup done")

	// Deconfigure the initial AP.
	if err := tf.DeconfigAP(ctx, ap1); err != nil {
		s.Error("Failed to deconfig ap, err: ", err)
	}
	ap1 = nil
	s.Log("Deconfigured AP1")

	if err := waitForProps(); err != nil {
		s.Fatal("DUT: failed to wait for the properties, err: ", err)
	}

	s.Log("DUT: roamed")

	if err := tf.VerifyConnection(ctx, ap2); err != nil {
		s.Fatal("Failed to verify connection: ", err)
	}

}

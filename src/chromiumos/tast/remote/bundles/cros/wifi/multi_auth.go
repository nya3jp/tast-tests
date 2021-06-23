// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wifi

import (
	"context"

	"chromiumos/tast/common/wifi/security/wpa"
	"chromiumos/tast/remote/wificell"
	"chromiumos/tast/remote/wificell/hostapd"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: MultiAuth,
		Desc: "Tests the ability to select network correctly among APs with similar network configurations, by configuring two APs with the same SSID/channel/mode but different security config and connecting to each in turn",
		Contacts: []string{
			"chromeos-wifi-champs@google.com", // WiFi oncall rotation; or http://b/new?component=893827
		},
		Attr:        []string{"group:wificell", "wificell_func"},
		ServiceDeps: []string{wificell.TFServiceName},
		Fixture:     "wificellFixtWithCapture",
	})
}

func MultiAuth(ctx context.Context, s *testing.State) {
	tf := s.FixtValue().(*wificell.TestFixture)

	apOpts := []hostapd.Option{hostapd.SSID("an ssid"), hostapd.Mode(hostapd.Mode80211g), hostapd.Channel(1)}
	wpaCfg := wpa.NewConfigFactory("chromeos", wpa.Mode(wpa.ModePureWPA), wpa.Ciphers(wpa.CipherCCMP))

	s.Log("Configuring AP 0 (Open)")
	ap0, err := tf.ConfigureAP(ctx, apOpts, nil)
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

	s.Log("Configuring AP 1 (WPA)")
	ap1, err := tf.ConfigureAP(ctx, apOpts, wpaCfg)
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

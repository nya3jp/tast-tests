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
		Func:        MultiAuth,
		Desc:        "Select between two networks with identical SSIDs but different security parameters",
		Contacts:    []string{"wgd@google.com", "chromeos-platform-connectivity@google.com"},
		Attr:        []string{"group:wificell", "wificell_func", "wificell_unstable"},
		ServiceDeps: []string{wificell.TFServiceName},
		Pre:         wificell.TestFixturePreWithCapture(),
		Vars:        []string{"router", "pcap"},
	})
}

// MultiAuth configures two APs with the same SSID/channel/mode but different security, and attempts to connect to each.
func MultiAuth(ctx context.Context, s *testing.State) {
	tf := s.PreValue().(*wificell.TestFixture)
	defer func(ctx context.Context) {
		if err := tf.CollectLogs(ctx); err != nil {
			s.Log("Error collecting logs, err: ", err)
		}
	}(ctx)
	ctx, cancel := tf.ReserveForCollectLogs(ctx)
	defer cancel()

	apOpts := []hostapd.Option{hostapd.SSID("an ssid"), hostapd.Mode(hostapd.Mode80211g), hostapd.Channel(1)}
	wpaCfg := wpa.NewConfigFactory("chromeos", wpa.Mode(wpa.ModePureWPA), wpa.Ciphers(wpa.CipherCCMP))

	ap0, err := tf.ConfigureAP(ctx, apOpts, nil)
	if err != nil {
		s.Fatal("Failed to configure AP0: ", err)
	}
	defer func(ctx context.Context) {
		if err := tf.DeconfigAP(ctx, ap0); err != nil {
			s.Error("Failed to deconfig AP0: ", err)
		}
	}(ctx)
	ctx, cancel = tf.ReserveForDeconfigAP(ctx, ap0)
	defer cancel()
	s.Log("Configured AP0: ", ap0)

	ap1, err := tf.ConfigureAP(ctx, apOpts, wpaCfg)
	if err != nil {
		s.Fatal("Failed to configure AP1: ", err)
	}
	s.Log("Configured AP1: ", ap1)
	defer func(ctx context.Context) {
		if err := tf.DeconfigAP(ctx, ap1); err != nil {
			s.Error("Failed to deconfig AP1: ", err)
		}
	}(ctx)
	ctx, cancel = tf.ReserveForDeconfigAP(ctx, ap1)
	defer cancel()

	s.Log("Connecting to AP0")
	if _, err := tf.ConnectWifiAP(ctx, ap0); err != nil {
		s.Fatal("Failed to connect to AP0: ", err)
	}
	defer func(ctx context.Context) {
		if err := tf.CleanDisconnectWifi(ctx); err != nil {
			s.Error("Failed to disconnect WiFi: ", err)
		}
	}(ctx)
	ctx, cancel = tf.ReserveForDisconnect(ctx)
	defer cancel()
	s.Log("Verifying connection to AP0")
	if err := tf.VerifyConnection(ctx, ap0); err != nil {
		s.Fatal("Failed to verify connection: ", err)
	}

	s.Log("Connecting to AP1")
	if _, err := tf.ConnectWifiAP(ctx, ap1); err != nil {
		s.Fatal("Failed to connect to AP1: ", err)
	}
	s.Log("Verifying connection to AP0")
	if err := tf.VerifyConnection(ctx, ap1); err != nil {
		s.Fatal("Failed to verify connection: ", err)
	}
}

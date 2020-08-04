// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wifi

import (
	"context"

	"chromiumos/tast/common/network/arping"
	"chromiumos/tast/common/wifi/security/wpa"
	"chromiumos/tast/remote/wificell"
	"chromiumos/tast/remote/wificell/hostapd"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:        GTK,
		Desc:        "Verifies that we can continue to decrypt broadcast traffic while going through group temporal key (GTK) rekeys",
		Contacts:    []string{"chharry@google.com", "chromeos-platform-connectivity@google.com"},
		Attr:        []string{"group:wificell", "wificell_func", "wificell_unstable"},
		ServiceDeps: []string{wificell.TFServiceName},
		Pre:         wificell.TestFixturePre(),
		Vars:        []string{"router", "pcap"},
	})
}

func GTK(ctx context.Context, s *testing.State) {
	// The settings gives us around 20 seconds to arping, which covers about 4 GTK rekeys.
	const (
		gtkRekeyPeriod = 5
		gmkRekeyPeriod = 7
		arpingCount    = 20
	)

	tf := s.PreValue().(*wificell.TestFixture)
	defer func(ctx context.Context) {
		if err := tf.CollectLogs(ctx); err != nil {
			s.Log("Error collecting logs, err: ", err)
		}
	}(ctx)
	ctx, cancel := tf.ReserveForCollectLogs(ctx)
	defer cancel()

	apOps := []hostapd.Option{
		hostapd.Mode(hostapd.Mode80211g),
		hostapd.Channel(1),
	}
	secConfFac := wpa.NewConfigFactory(
		"chromeos", wpa.Mode(wpa.ModeMixed),
		wpa.Ciphers(wpa.CipherTKIP, wpa.CipherCCMP),
		wpa.Ciphers2(wpa.CipherCCMP),
		wpa.UseStrictRekey(true),
		wpa.GTKRekeyPeriod(gtkRekeyPeriod),
		wpa.GMKRekeyPeriod(gmkRekeyPeriod),
	)
	ap, err := tf.ConfigureAP(ctx, apOps, secConfFac)
	if err != nil {
		s.Fatal("Failed to configure ap: ", err)
	}
	defer func(ctx context.Context) {
		if err := tf.DeconfigAP(ctx, ap); err != nil {
			s.Error("Failed to deconfig ap: ", err)
		}
	}(ctx)
	ctx, cancel = tf.ReserveForDeconfigAP(ctx, ap)
	defer cancel()

	s.Log("AP setup done")

	if _, err := tf.ConnectWifiAP(ctx, ap, nil); err != nil {
		s.Fatal("Failed to connect to WiFi: ", err)
	}
	defer func(ctx context.Context) {
		if err := tf.CleanDisconnectWifi(ctx); err != nil {
			s.Error("Failed to disconnect WiFi: ", err)
		}
	}(ctx)
	ctx, cancel = tf.ReserveForDisconnect(ctx)
	defer cancel()
	s.Log("Connected")

	if err := tf.PingFromDUT(ctx, ap.ServerIP().String()); err != nil {
		s.Fatal("Failed to ping from the DUT: ", err)
	}

	// Test that network traffic goes through.
	if err := tf.ArpingFromDUT(ctx, ap.ServerIP().String(), arping.Count(arpingCount)); err != nil {
		s.Error("Failed to send broadcast packets to server: ", err)
	}
	if err := tf.ArpingFromServer(ctx, ap.Interface(), arping.Count(arpingCount)); err != nil {
		s.Error("Failed to receive broadcast packets from server: ", err)
	}

	s.Log("Deconfiguring")
}

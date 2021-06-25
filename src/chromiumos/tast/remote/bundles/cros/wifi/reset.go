// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wifi

import (
	"context"
	"time"

	"chromiumos/tast/remote/wificell"
	"chromiumos/tast/remote/wificell/hostapd"
	"chromiumos/tast/services/cros/wifi"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: Reset,
		Desc: "Test that the WiFi interface can be reset successfully, and that WiFi comes back up properly",
		Contacts: []string{
			"chharry@google.com",              // Test author
			"chromeos-wifi-champs@google.com", // WiFi oncall rotation; or http://b/new?component=893827
		},
		Attr:        []string{"group:wificell", "wificell_suspend"},
		ServiceDeps: []string{wificell.TFServiceName},
		Fixture:     "wificellFixt",
		// For some Marvell DUT, this test may take more than 25 minutes.
		// For WCN3990 device, this test may take more than 39 minutes.
		Timeout: time.Minute * 45,
		// We only support reset on Intel/Marvell/QCA WiFi (iwlwifi/mwifiex/ath10k).
		// TODO(chromium:1070299): These models are chosen manually by finding the models that are always failing with NA-error on Autotest network_WiFi_Reset. Replace them with more proper hwdep in the future.
		HardwareDeps: hwdep.D(hwdep.SkipOnModel("barla", "blooglet", "dirinboz", "ezkinil", "vilboz")),
		Params: []testing.Param{
			{
				// Default AP settings ported from Autotest.
				ExtraAttr: []string{"wificell_unstable"},
				Val:       []hostapd.Option{hostapd.Mode(hostapd.Mode80211b), hostapd.Channel(1)},
			},
			{
				// The target protocol and channel settings, as this is more widely used nowadays.
				// TODO(b/175602523): Replace the default with this once the issue is fixed.
				Name:      "80211n_ch48",
				ExtraAttr: []string{"wificell_unstable"},
				Val:       wificell.DefaultOpenNetworkAPOptions(),
			},
		},
	})
}

func Reset(ctx context.Context, s *testing.State) {
	tf := s.FixtValue().(*wificell.TestFixture)

	apOps := s.Param().([]hostapd.Option)
	ap, err := tf.ConfigureAP(ctx, apOps, nil)
	if err != nil {
		s.Fatal("Failed to configure the AP: ", err)
	}
	defer func(ctx context.Context) {
		if err := tf.DeconfigAP(ctx, ap); err != nil {
			s.Error("Failed to deconfigure the AP: ", err)
		}
	}(ctx)
	ctx, cancel := tf.ReserveForDeconfigAP(ctx, ap)
	defer cancel()

	ctxForDisconnectWiFi := ctx
	ctx, cancel = tf.ReserveForDisconnect(ctx)
	defer cancel()
	resp, err := tf.ConnectWifiAP(ctx, ap)
	if err != nil {
		s.Fatal("Failed to connect to the AP: ", err)
	}
	defer func(ctx context.Context) {
		if err := tf.CleanDisconnectWifi(ctx); err != nil {
			s.Error("Failed to disconnect from the AP: ", err)
		}
	}(ctxForDisconnectWiFi)

	if err := tf.PingFromDUT(ctx, ap.ServerIP().String()); err != nil {
		s.Fatal("Failed to ping from the DUT: ", err)
	}

	if _, err := tf.WifiClient().ResetTest(ctx, &wifi.ResetTestRequest{
		ServicePath: resp.ServicePath,
		ServerIp:    ap.ServerIP().String(),
	}); err != nil {
		s.Fatal("gRPC command ResetTest failed: ", err)
	}
}

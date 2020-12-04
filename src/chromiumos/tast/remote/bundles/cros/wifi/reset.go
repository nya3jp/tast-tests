// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wifi

import (
	"context"
	"time"

	"chromiumos/tast/remote/wificell"
	"chromiumos/tast/services/cros/network"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:        Reset,
		Desc:        "Test that the WiFi interface can be reset successfully, and that WiFi comes back up properly",
		Contacts:    []string{"chharry@google.com", "chromeos-platform-connectivity@google.com"},
		Attr:        []string{"group:wificell", "wificell_func", "wificell_unstable"},
		ServiceDeps: []string{wificell.TFServiceName},
		Pre:         wificell.TestFixturePre(),
		Vars:        []string{"router", "pcap"},
		// For some Marvell DUT, this test may take more than 15 minutes.
		Timeout: time.Minute * 18,
		// We only support reset on Intel/Marvell/QCA WiFi (iwlwifi/mwifiex/ath10k).
		// TODO(chromium:1070299): These models are chosen manually by finding the models that are always failing with NA-error on Autotest network_WiFi_Reset. Replace them with more proper hwdep in the future.
		HardwareDeps: hwdep.D(hwdep.SkipOnModel("blooglet", "dalboz", "ezkinil", "trembyle")),
	})
}

func Reset(ctx context.Context, s *testing.State) {
	tf := s.PreValue().(*wificell.TestFixture)
	defer func(ctx context.Context) {
		if err := tf.CollectLogs(ctx); err != nil {
			s.Log("Error collecting logs, err: ", err)
		}
	}(ctx)
	ctx, cancel := tf.ReserveForCollectLogs(ctx)
	defer cancel()

	ap, err := tf.DefaultOpenNetworkAP(ctx)
	if err != nil {
		s.Fatal("Failed to configure the AP: ", err)
	}
	defer func(ctx context.Context) {
		if err := tf.DeconfigAP(ctx, ap); err != nil {
			s.Error("Failed to deconfigure the AP: ", err)
		}
	}(ctx)
	ctx, cancel = tf.ReserveForDeconfigAP(ctx, ap)
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

	if _, err := tf.WifiClient().ResetTest(ctx, &network.ResetTestRequest{
		ServicePath: resp.ServicePath,
		ServerIp:    ap.ServerIP().String(),
	}); err != nil {
		s.Fatal("gRPC command ResetTest failed: ", err)
	}
}

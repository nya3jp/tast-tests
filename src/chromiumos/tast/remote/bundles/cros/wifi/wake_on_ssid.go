// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wifi

import (
	"context"
	"time"

	"chromiumos/tast/common/shillconst"
	"chromiumos/tast/errors"
	"chromiumos/tast/remote/bundles/cros/wifi/wifiutil"
	"chromiumos/tast/remote/wificell"
	"chromiumos/tast/remote/wificell/hostapd"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: WakeOnSSID,
		Desc: "Verifies that the DUT can wake up when a known SSID is discovered",
		Contacts: []string{
			"yenlinlai@google.com",            // Test author.
			"chromeos-wifi-champs@google.com", // WiFi oncall rotation; or http://b/new?component=893827
		},
		// TODO(b/187362093): Add a SoftwareDep for wake_on_wifi.
		// TODO(b/187362093): Give WoWiFi a separate suite.
		Attr:        []string{"group:wificell", "wificell_suspend", "wificell_unstable"},
		ServiceDeps: []string{wificell.TFServiceName},
		Pre:         wificell.TestFixturePre(),
		Vars:        []string{"router", "pcap"},
	})
}

func WakeOnSSID(ctx context.Context, s *testing.State) {
	tf := s.PreValue().(*wificell.TestFixture)
	defer func(ctx context.Context) {
		if err := tf.CollectLogs(ctx); err != nil {
			s.Log("Error collecting logs, err: ", err)
		}
	}(ctx)
	ctx, cancel := tf.ReserveForCollectLogs(ctx)
	defer cancel()

	const (
		netDetectScanPeriod = 15 // In seconds.
	)

	features := shillconst.WakeOnWiFiFeaturesDarkConnect
	wakeOnWifiOps := []wificell.SetWakeOnWifiOption{
		wificell.WakeOnWifiFeatures(features),
		wificell.WakeOnWifiNetDetectScanPeriod(netDetectScanPeriod),
	}
	ctx, restoreWakeOnWiFi, err := tf.SetWakeOnWifi(ctx, wakeOnWifiOps...)
	if err != nil {
		s.Fatal("Failed to set up wake on WiFi: ", err)
	}
	defer func() {
		if err := restoreWakeOnWiFi(); err != nil {
			s.Error("Failed to restore wake on WiFi setting: ", err)
		}
	}()

	// Set up the AP and connect.
	apOpts := []hostapd.Option{
		hostapd.SSID("TAST_TEST_WakeOnSSID"),
		hostapd.Mode(hostapd.Mode80211nMixed),
		hostapd.Channel(1),
		hostapd.HTCaps(hostapd.HTCapHT40),
	}

	ap, err := tf.ConfigureAP(ctx, apOpts, nil)
	if err != nil {
		s.Fatal("Failed to configure AP: ", err)
	}
	defer func(ctx context.Context) {
		if ap == nil {
			// Already de-configured.
			return
		}
		if err := tf.DeconfigAP(ctx, ap); err != nil {
			s.Error("Failed to deconfig AP: ", err)
		}
	}(ctx)
	ctx, cancel = tf.ReserveForDeconfigAP(ctx, ap)
	defer cancel()
	s.Log("AP setup done")

	connectResp, err := tf.ConnectWifiAP(ctx, ap)
	if err != nil {
		s.Fatal("Failed to connect to WiFi: ", err)
	}
	defer func(ctx context.Context) {
		// If AP is already deconfigured, let's just wait for service idle.
		// Otherwise, explicitly disconnect.
		if ap == nil {
			if err := wifiutil.WaitServiceIdle(ctx, tf, connectResp.ServicePath); err != nil {
				s.Error("Failed to wait for DUT disconnected: ", err)
			}
		} else if err := tf.DisconnectWifi(ctx); err != nil {
			s.Error("Failed to disconnect: ", err)
		}
	}(ctx)
	// This is over-reserving time, but for safety.
	ctx, cancel = tf.ReserveForDisconnect(ctx)
	defer cancel()
	ctx, cancel = wifiutil.ReserveForWaitServiceIdle(ctx)
	defer cancel()
	s.Log("Connected")

	// Now deconfig AP so DUT is not connected before suspend.
	if err := tf.DeconfigAP(ctx, ap); err != nil {
		s.Fatal("Failed to deconfig AP: ", err)
	}
	ap = nil
	if err := wifiutil.WaitServiceIdle(ctx, tf, connectResp.ServicePath); err != nil {
		s.Fatal("Failed to wait for DUT disconnected: ", err)
	}
	s.Log("Disconnected")

	triggerFunc := func(ctx context.Context) error {
		// Re-spawn the AP. Notice the scope of "ap" so that the defer above can
		// clean it up.
		ap, err = tf.ConfigureAP(ctx, apOpts, nil)
		if err != nil {
			return errors.Wrap(err, "failed to configure AP")
		}
		return nil
	}

	// Expect the DUT can discover the service in one scan, so 2*netDetectScanPeriod should
	// be enough.
	if err := wifiutil.VerifyWakeOnWifiReason(ctx, tf, s.DUT(), 2*netDetectScanPeriod*time.Second,
		shillconst.WakeOnWiFiReasonSSID, triggerFunc); err != nil {
		s.Fatal("Verification failed: ", err)
	}
}

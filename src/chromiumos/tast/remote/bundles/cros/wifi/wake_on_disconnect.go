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
		Func: WakeOnDisconnect,
		Desc: "Verifies that the DUT can wake up when disconnected from AP",
		Contacts: []string{
			"yenlinlai@google.com",            // Test author.
			"chromeos-wifi-champs@google.com", // WiFi oncall rotation; or http://b/new?component=893827
		},
		// TODO(b/187362093): Add a SoftwareDep for wake_on_wifi.
		Attr:        []string{"group:wificell", "wificell_suspend", "wificell_unstable"},
		ServiceDeps: []string{wificell.TFServiceName},
		Fixture:     "wificellFixt",
	})
}

func WakeOnDisconnect(ctx context.Context, s *testing.State) {
	tf := s.FixtValue().(*wificell.TestFixture)

	features := shillconst.WakeOnWiFiFeaturesDarkConnect
	ctx, restoreWakeOnWiFi, err := tf.SetWakeOnWifi(ctx, wificell.WakeOnWifiFeatures(features))
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
		hostapd.SSID(hostapd.RandomSSID("TAST_TEST_WakeOnDisconnect")),
		hostapd.Mode(hostapd.Mode80211acMixed),
		hostapd.Channel(48),
		hostapd.HTCaps(hostapd.HTCapHT40),
		hostapd.VHTChWidth(hostapd.VHTChWidth20Or40),
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
	ctx, cancel := tf.ReserveForDeconfigAP(ctx, ap)
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

	triggerFunc := func(ctx context.Context) error {
		if err := tf.DeconfigAP(ctx, ap); err != nil {
			return errors.Wrap(err, "failed to deconfig AP")
		}
		ap = nil
		return nil
	}
	// Assume DUT could notice AP gone in 10 seconds. (by deauth or missing beason)
	if err := wifiutil.VerifyWakeOnWifiReason(ctx, tf, s.DUT(), 10*time.Second,
		shillconst.WakeOnWiFiReasonDisconnect, triggerFunc); err != nil {
		s.Fatal("Verification failed: ", err)
	}
}

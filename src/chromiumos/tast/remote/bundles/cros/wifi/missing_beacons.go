// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wifi

import (
	"context"
	"time"

	remoteiw "chromiumos/tast/remote/network/iw"
	"chromiumos/tast/remote/wificell"
	"chromiumos/tast/services/cros/wifi"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: MissingBeacons,
		Desc: "Test how a DUT behaves when an AP disappears suddenly",
		Contacts: []string{
			"chromeos-wifi-champs@google.com", // WiFi oncall rotation; or http://b/new?component=893827
		},
		Attr:        []string{"group:wificell", "wificell_func"},
		ServiceDeps: []string{wificell.TFServiceName},
		Fixture:     "wificellFixt",
		Params: []testing.Param{
			// Two subtests:
			// 1. MissingBeacons does not perform a scan before taking down the AP.
			// 2. MissingBeacons.scan performs a scan before taking down the AP.
			{
				Val: false,
			}, {
				Name: "scan",
				Val:  true,
			},
		},
	})
}

func MissingBeacons(ctx context.Context, s *testing.State) {
	// Connects a DUT to an AP, then kills the AP in such a way that no de-auth
	// message is sent.  Asserts that the DUT marks itself as disconnected from
	// the AP within maxDisconnectTime.
	tf := s.FixtValue().(*wificell.TestFixture)

	legacyRouter, err := tf.LegacyRouter()
	if err != nil {
		s.Fatal("Failed to get legacy router: ", err)
	}

	ap, err := tf.DefaultOpenNetworkAP(ctx)
	if err != nil {
		s.Fatal("Failed to configure the AP: ", err)
	}
	defer func(ctx context.Context) {
		if err := tf.DeconfigAP(ctx, ap); err != nil {
			s.Error("Failed to deconfig the AP: ", err)
		}
	}(ctx)
	ctx, cancel := tf.ReserveForDeconfigAP(ctx, ap)
	defer cancel()

	var servicePath string
	if resp, err := tf.ConnectWifiAP(ctx, ap); err != nil {
		s.Fatal("DUT: failed to connect to WiFi: ", err)
	} else {
		servicePath = resp.ServicePath
	}

	apSSID := ap.Config().SSID

	defer func(ctx context.Context) {
		if err := tf.DisconnectWifi(ctx); err != nil {
			// Do not fail on this error as we're triggering some
			// disconnection in this test and the service can be
			// inactive at this point.
			s.Log("Failed to disconnect WiFi (The service might have been already idle, as the test is triggering some disconnection): ", err)
		}
		// Explicitly delete service entries here because it could have
		// no active service here so calling tf.CleanDisconnectWifi()
		// would fail.
		if _, err := tf.WifiClient().DeleteEntriesForSSID(ctx, &wifi.DeleteEntriesForSSIDRequest{Ssid: []byte(apSSID)}); err != nil {
			s.Errorf("Failed to remove entries for ssid=%s, err: %v", apSSID, err)
		}
	}(ctx)
	ctx, cancel = tf.ReserveForDisconnect(ctx)
	defer cancel()

	if err := tf.PingFromDUT(ctx, ap.ServerIP().String()); err != nil {
		s.Fatal("DUT: failed to ping from the DUT: ", err)
	}

	// A stale scan can cause wpa_supplicant to attempt to fast-reconnect to an AP even if it is no longer available.
	// The purpose of the scan is to ensure that the AP is definitely in the scan results before the disconnect and
	// subsequent fast-reconnect happens. When the next scan is triggered (because the client is attempting to connect),
	// we expect the AP, which is no longer up and thus should no longer be in the scan results, to be purged from the results.
	// The functionality to purge all scan results older than the most recent scan request time for fast-reconnect was added in
	// this patch: http://crrev.com/c/7437.
	if s.Param().(bool) {
		iface, err := tf.ClientInterface(ctx)
		if err != nil {
			s.Fatal("DUT: failed to get the client interface: ", err)
		}

		// There is no need to stop the bgscan before running the scan, because the bgscan is not set when the DUT is connecting
		// to an SSID that has only one AP. The test is configuring one AP, so that is not going to set the bgscan.
		res, err := remoteiw.NewRemoteRunner(s.DUT().Conn()).TimedScan(ctx, iface, nil, nil)
		if err != nil {
			s.Fatal("TimedScan failed: ", err)
		}

		foundSSID := false
		for _, data := range res.BSSList {
			if apSSID == data.SSID {
				foundSSID = true
				break
			}
		}

		if !foundSSID {
			s.Errorf("DUT: failed to find the ssid=%s in the scan", apSSID)
		}
	}

	// Take down the AP interface, which looks like the AP "disappeared" from the DUT's point of view.
	// This is also much faster than actually tearing down the AP, which allows us to watch for the client
	// reporting itself as disconnected.
	if err := legacyRouter.SetAPIfaceDown(ctx, ap.Interface()); err != nil {
		s.Fatal("DUT: failed to set the AP interface down: ", err)
	}

	const maxDisconnectTime = 20 * time.Second

	testing.ContextLogf(ctx, "Waiting %s for client to notice the missing AP", maxDisconnectTime)

	if err := tf.AssureDisconnect(ctx, servicePath, maxDisconnectTime); err != nil {
		s.Fatalf("DUT: failed to disconnect in %s: %v", maxDisconnectTime, err)
	}
}

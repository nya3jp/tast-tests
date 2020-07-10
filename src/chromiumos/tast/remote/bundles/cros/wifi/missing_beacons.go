// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wifi

import (
	"context"
	"time"

	remoteiw "chromiumos/tast/remote/network/iw"
	"chromiumos/tast/remote/wificell"
	"chromiumos/tast/services/cros/network"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:        MissingBeacons,
		Desc:        "Test how a DUT behaves when an AP disappears suddenly",
		Contacts:    []string{"arowa@google.com", "chromeos-platform-connectivity@google.com"},
		Attr:        []string{"group:wificell", "wificell_func"},
		ServiceDeps: []string{"tast.cros.network.WifiService"},
		Vars:        []string{"router"},
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

func MissingBeacons(fullCtx context.Context, s *testing.State) {
	// Connects a DUT to an AP, then kills the AP in such a way that no de-auth
	// message is sent.  Asserts that the DUT marks itself as disconnected from
	// the AP within maxDisconnectTime.
	router, _ := s.Var("router")
	tf, err := wificell.NewTestFixture(fullCtx, fullCtx, s.DUT(), s.RPCHint(), wificell.TFRouter(router))
	if err != nil {
		s.Fatal("Failed to set up test fixture: ", err)
	}
	defer func() {
		if err := tf.Close(fullCtx); err != nil {
			s.Log("Failed to tear down test fixture: ", err)
		}
	}()

	tfCtx, cancel := tf.ReserveForClose(fullCtx)
	defer cancel()

	ap, err := tf.DefaultOpenNetworkAP(tfCtx)
	if err != nil {
		s.Fatal("Failed to configure the AP: ", err)
	}
	defer func() {
		if err := tf.DeconfigAP(tfCtx, ap); err != nil {
			s.Error("Failed to deconfig the AP: ", err)
		}
	}()

	ctx, cancel := tf.ReserveForDeconfigAP(tfCtx, ap)
	defer cancel()

	if _, err := tf.ConnectWifiAP(ctx, ap); err != nil {
		s.Fatal("DUT: failed to connect to WiFi: ", err)
	}

	apSSID := ap.Config().SSID

	defer func() {
		if err := tf.DisconnectWifi(ctx); err != nil {
			// Do not fail on this error as we're triggering some
			// disconnection in this test and the service can be
			// inactive at this point.
			s.Log("Failed to disconnect WiFi (The service might have been already idle, as the test is triggering some disconnection): ", err)
		}
		if _, err := tf.WifiClient().DeleteEntriesForSSID(ctx, &network.DeleteEntriesForSSIDRequest{Ssid: []byte(apSSID)}); err != nil {
			s.Errorf("Failed to remove entries for ssid=%s, err: %v", apSSID, err)
		}
	}()

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
	if err := tf.Router().SetAPIfaceDown(ctx, ap.Interface()); err != nil {
		s.Fatal("DUT: failed to set the AP interface down: ", err)
	}

	const maxDisconnectTime = 20 * time.Second

	testing.ContextLogf(ctx, "Waiting %s for client to notice the missing AP", maxDisconnectTime)

	if err := tf.AssureDisconnect(ctx, maxDisconnectTime); err != nil {
		s.Fatalf("DUT: failed to disconnect in %s: %v", maxDisconnectTime, err)
	}

}

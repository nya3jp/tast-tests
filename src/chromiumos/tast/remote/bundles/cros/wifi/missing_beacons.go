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
		Attr:        []string{"group:wificell", "wificell_func", "wificell_unstable"},
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
	defer func() {
		if err := tf.DisconnectWifi(ctx); err != nil {
			// Do not fail on this error as we're triggering some
			// disconnection in this test and the service can be
			// inactive at this point.
			s.Log("Failed to disconnect WiFi: ", err)
		}
		if _, err := tf.WifiClient().DeleteEntriesForSSID(ctx, &network.DeleteEntriesForSSIDRequest{Ssid: []byte(ap.Config().Ssid)}); err != nil {
			s.Errorf("Failed to remove entries for ssid=%s, err: %v", ap.Config().Ssid, err)
		}
	}()

	if err := tf.PingFromDUT(ctx, ap.ServerIP().String()); err != nil {
		s.Fatal("DUT: failed to ping from the DUT: ", err)
	}

	// TODO(b:158874753) Running a scan before taking down the AP doesn't seem to
	// have a significant impact on the disconnect time.
	if s.Param().(bool) {
		iface, err := tf.ClientInterface(ctx)
		if err != nil {
			s.Fatal("DUT: failed to get the client interface: ", err)
		}

		if _, err := remoteiw.NewRemoteRunner(s.DUT().Conn()).TimedScan(ctx, iface, nil, nil); err != nil {
			s.Fatal("TimedScan failed: ", err)
		}
	}

	// Take down the AP interface, which looks like the AP "disappeared" from the DUT's point of view.
	// This is also much faster than actually tearing down the AP, which allows us to watch for the client
	// reporting itself as disconnected.
	if err := tf.Router().SetAPIfaceDown(ctx, ap.Interface()); err != nil {
		s.Fatal("DUT: failed to set the AP interface down: ", err)
	}

	const maxDisconnectTime = 20 * time.Second

	// Leave some margin of seconds to check how long it actually takes to disconnect should we fail to disconnect in time.
	timeout := maxDisconnectTime + 10*time.Second
	testing.ContextLogf(ctx, "Waiting %s for client to notice the missing AP", timeout)

	start := time.Now()
	if err := tf.AssureDisconnect(ctx, timeout); err != nil {
		s.Fatalf("DUT: failed to disconnect in %s: %v", maxDisconnectTime, err)
	}
	elapsed := time.Since(start)

	if elapsed > maxDisconnectTime {
		s.Errorf("DUT: time exceeded disconnecting from the SSID: got %s, want < %s", elapsed, maxDisconnectTime)
	}

}

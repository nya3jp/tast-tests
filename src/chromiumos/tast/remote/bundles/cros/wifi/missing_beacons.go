// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wifi

import (
	"context"

	remote_iw "chromiumos/tast/remote/network/iw"
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
	})
}

const maxDisconnectTimeSeconds = 20

func MissingBeacons(fullCtx context.Context, s *testing.State) {
	// Connects a DUT to an AP, then kills the AP in such a way that no de-auth
	// message is sent.  Asserts that the DUT marks itself as disconnected from
	// the AP within MAX_DISCONNECT_TIME_SECONDS.
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

	testOnce := func(tfCtx context.Context, doScan bool) {
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

		if doScan {
			iface, err := tf.ClientInterface(ctx)
			if err != nil {
				s.Fatal("DUT: failed to get the client interface: ", err)
			}

			if _, err := remote_iw.NewRemoteRunner(s.DUT().Conn()).TimedScan(ctx, iface, []int{}, []string{}); err != nil {
				s.Fatal("TimedScan failed: ", err)
			}
		}

		// Take down the AP interface, which looks like the AP "disappeared" from the DUT's point of view.
		// This is also much faster than actually tearing down the AP, which allows us to watch for the client
		// reporting itself as disconnected.
		if err := tf.SetAPInterfaceDown(ctx, ap); err != nil {
			s.Fatal("DUT: failed to set the AP interface down: ", err)
		}

		// Leave some margin of seconds to check how long it actually takes to disconnect should we fail to disconnect in time.
		timeout := maxDisconnectTimeSeconds + 10
		testing.ContextLogf(ctx, "Waiting %d seconds for client to notice the missing AP", timeout)

		if err := tf.AssureDisconnect(ctx, timeout); err != nil {
			s.Fatalf("DUT: failed to disconnect in %d seconds: %v", maxDisconnectTimeSeconds, err)
		}
	}

	testOnce(tfCtx, false)
	s.Log("Repeating test with a client scan just before AP death")
	testOnce(tfCtx, true)
}

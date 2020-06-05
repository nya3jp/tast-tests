// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wifi

import (
	"context"
	"time"

	"chromiumos/tast/errors"
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

const maxDisconnectTimeSeconds = 20.0

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

	ap, err := tf.DefaultOpenNetworkAP(tfCtx)
	if err != nil {
		s.Fatal("Failed to configure the AP: ", err)
	}

	if _, err := tf.ConnectWifiAP(tfCtx, ap); err != nil {
		s.Fatal("DUT: failed to connect to WiFi: ", err)
	}

	if err := tf.PingFromDUT(tfCtx, ap.ServerIP().String()); err != nil {
		s.Fatal("DUT: failed to ping from the DUT: ", err)
	}

	// Take down the AP interface, which looks like the AP "disappeared" from the DUT's point of view.
	// This is also much faster than actually tearing down the AP, which allows us to watch for the client
	// reporting itself as disconnected.
	if err := tf.SetAPInterfaceDown(tfCtx, ap); err != nil {
		s.Fatal("DUT: failed to set the AP interface down: ", err)
	}

	if err := assureDisconnect(tfCtx, tf, ap.Config().Ssid); err != nil {
		s.Fatalf("DUT: failed to disconnect from SSID in %d seconds: %v", maxDisconnectTimeSeconds, err)
	}

	if _, err := tf.WifiClient().DeleteEntriesForSSID(tfCtx, &network.DeleteEntriesForSSIDRequest{Ssid: []byte(ap.Config().Ssid)}); err != nil {
		s.Errorf("Failed to remove entries for ssid=%s, err: %v", ap.Config().Ssid, err)
	}

	if err := tf.DeconfigAP(tfCtx, ap); err != nil {
		s.Error("Failed to deconfig the AP: ", err)
	}

	s.Log("Repeating test with a client scan just before AP death")

	ap, err = tf.DefaultOpenNetworkAP(tfCtx)
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
		if _, err := tf.WifiClient().DeleteEntriesForSSID(ctx, &network.DeleteEntriesForSSIDRequest{Ssid: []byte(ap.Config().Ssid)}); err != nil {
			s.Errorf("Failed to remove entries for ssid=%s, err: %v", ap.Config().Ssid, err)
		}
	}()

	if err := tf.PingFromDUT(ctx, ap.ServerIP().String()); err != nil {
		s.Fatal("DUT: failed to ping from the DUT: ", err)
	}

	iface, err := tf.ClientInterface(ctx)
	if err != nil {
		s.Fatal("DUT: failed to get the client interface: ", err)
	}

	if _, err := remote_iw.NewRemoteRunner(s.DUT().Conn()).TimedScan(ctx, iface, []int{}, []string{}); err != nil {
		s.Fatal("TimedScan failed: ", err)
	}

	if err := tf.SetAPInterfaceDown(tfCtx, ap); err != nil {
		s.Fatal("DUT: failed to set the AP interface down: ", err)
	}

	if err := assureDisconnect(ctx, tf, ap.Config().Ssid); err != nil {
		s.Fatalf("DUT: failed to disconnect from SSID in %f seconds: %v", maxDisconnectTimeSeconds, err)
	}
}

// assureDisconnect asserts that we disconnect from the SSID in maxDisconnectTimeSeconds or less.
func assureDisconnect(ctx context.Context, tf *wificell.TestFixture, ssid string) error {
	// Leave some margin of seconds to check how long it actually takes to disconnect should we fail to disconnect in time.
	timeout := maxDisconnectTimeSeconds + 10.0
	testing.ContextLogf(ctx, "Waiting %d seconds for client to notice the missing AP", timeout)

	start := time.Now()
	if err := tf.AssureDisconnectSSID(ctx, ssid); err != nil {
		return err
	}
	t := time.Now()
	elapsed := t.Sub(start)

	if elapsed.Seconds() > timeout {
		return errors.Errorf("timeout while disconnecting the SSID, it took %f seconds", elapsed.Seconds())
	}

	return nil
}

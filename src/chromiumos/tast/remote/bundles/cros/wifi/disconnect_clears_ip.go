// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wifi

import (
	"context"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/remote/wificell"
	"chromiumos/tast/services/cros/network"
	"chromiumos/tast/testing"
	"chromiumos/tast/timing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:        DisconnectClearsIP,
		Desc:        "Check that the DUT removes the IP after disconnecting from a WiFi network",
		Contacts:    []string{"arowa@google.com", "chromeos-platform-connectivity@google.com"},
		Attr:        []string{"group:wificell", "wificell_func"},
		ServiceDeps: []string{"tast.cros.network.WifiService"},
		Vars:        []string{"router"},
	})
}

func DisconnectClearsIP(fullCtx context.Context, s *testing.State) {
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
	s.Log("AP setup done")

	if _, err := tf.ConnectWifiAP(ctx, ap); err != nil {
		s.Fatal("DUT: failed to connect to WiFi: ", err)
	}
	defer func() {
		if _, err := tf.WifiClient().DeleteEntriesForSSID(ctx, &network.DeleteEntriesForSSIDRequest{Ssid: []byte(ap.Config().SSID)}); err != nil {
			s.Errorf("Failed to remove entries for ssid=%s, err: %v", ap.Config().SSID, err)
		}
	}()
	s.Log("Connected")

	if err := tf.PingFromDUT(ctx, ap.ServerIP().String()); err != nil {
		s.Fatal("Failed to ping from the DUT: ", err)
	}

	addr, err := tf.ClientIPv4Addrs(ctx)
	if err != nil {
		s.Fatal("DUT: failed to get the IP address: ", err)
	}

	if len(addr) == 0 {
		s.Fatal("DUT: expect an IPv4 address")
	}

	s.Logf("Connected with IP address: %s. Disconnecting WiFi", addr)

	if err := tf.DisconnectWifi(ctx); err != nil {
		s.Fatal("DUT: failed to disconnect WiFi: ", err)
	}

	// Wait for IP to be cleared.
	s.Log("Disconnected. Wait for the IP address to be cleared")
	wCtx, st := timing.Start(ctx, "waitIPGone")
	defer st.End()
	if err := testing.Poll(wCtx, func(wCtx context.Context) error {
		addr, err := tf.ClientIPv4Addrs(wCtx)
		if err != nil {
			s.Fatal("DUT: failed to get the IP address: ", err)
		}
		if len(addr) != 0 {
			return errors.Errorf("DUT: expect no IPv4 address, got: %s", addr)
		}
		return nil
	}, &testing.PollOptions{Timeout: 10 * time.Second, Interval: 200 * time.Millisecond}); err != nil {
		s.Fatal("Failed to clear the IP after WiFi disconnected: ", err)
	}

}

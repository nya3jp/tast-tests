// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wifi

import (
	"context"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/shill"
	"chromiumos/tast/remote/wificell"
	"chromiumos/tast/remote/wificell/hostapd"
	"chromiumos/tast/services/cros/network"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:        DisconnectClearsIP,
		Desc:        "Check that we remove our IP after disconnection from a WiFi network",
		Contacts:    []string{"arowa@google.com", "chromeos-kernel-wifi@google.com"},
		Attr:        []string{"group:wificell", "wificell_func", "wificell_unstable"},
		ServiceDeps: []string{"tast.cros.network.Wifi"},
		Vars:        []string{"router"},
	})
}

func DisconnectClearsIP(ctx context.Context, s *testing.State) {
	router, _ := s.Var("router")
	tf, err := wificell.NewTestFixture(ctx, s.DUT(), s.RPCHint(), router)
	if err != nil {
		s.Fatal("Failed to set up test fixture: ", err)
	}
	defer func() {
		if err := tf.Close(ctx); err != nil {
			s.Log("Failed to tear down test fixture: ", err)
		}
	}()

	options := []hostapd.Option{hostapd.Mode(hostapd.Mode80211nPure), hostapd.Channel(48), hostapd.HTCaps(hostapd.HTCapHT20)}

	ap, err := tf.ConfigureAP(ctx, options...)
	if err != nil {
		s.Fatal("Failed to configure the AP: ", err)
	}
	defer func() {
		if err := tf.DeconfigAP(ctx, ap); err != nil {
			s.Error("Failed to deconfig ap: ", err)
		}
	}()
	s.Log("AP setup done")

	if err := tf.ConnectWifi(ctx, ap); err != nil {
		s.Fatal("Failed to connect to WiFi: ", err)
	}

	if err := tf.PingFromDUT(ctx); err != nil {
		s.Fatal("Failed to ping from the DUT: ", err)
	}

	clIface, err := tf.NewClientInterface(ctx, shill.TechnologyWifi)
	if err != nil {
		s.Fatal("Failed to get the client Wifi interface: ", err)
	}

	addr, err := clIface.Addresses(ctx)
	if err != nil {
		s.Fatal("Failed to get the IP addresses after connection: ", err)
	}

	if addr.IPv4 == "" {
		s.Fatal("Failed the IP address doesn't exists after connection")
	}

	if err := tf.DisconnectWifi(ctx); err != nil {
		s.Error("Failed to disconnect WiFi: ", err)
	}

	if s.HasError() {
		if _, err := tf.WifiClient().DeleteEntriesForSSID(ctx, &network.SSID{Ssid: ap.Config().Ssid}); err != nil {
			s.Errorf("Failed to remove entries for ssid=%s, err: %v", ap.Config().Ssid, err)
		}
	}

	s.Log("Successfully disconnected")

	// Wait for IP to be cleared.
	s.Log("Wait for the IP address to be cleared")
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		add, err := clIface.Addresses(ctx)
		if err != nil {
			s.Fatal("Failed to get the interface addresses: ", err)
		}
		if add.IPv4 != "" {
			return errors.Errorf("failed the IP address is still set %s", add.IPv4)
		}
		return nil
	}, &testing.PollOptions{Timeout: 10 * time.Second, Interval: 10 * time.Second}); err != nil {
		s.Fatal("Failed to clear the IP address: ", err)
	}

}

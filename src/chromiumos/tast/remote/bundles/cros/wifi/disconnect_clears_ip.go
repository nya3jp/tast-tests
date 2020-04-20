// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wifi

import (
	"context"
	"time"

	"chromiumos/tast/errors"
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
			s.Log("Failed to tear down test fixture, err: ", err)
		}
	}()

	f, err := hostapd.FrequencyToChannel(2412)
	if err != nil {
		s.Fatal("Failed to get the channel number from the frequency, err: ", err)
	}

	options := []hostapd.Option{hostapd.Mode(hostapd.Mode80211g), hostapd.Channel(f)}

	ap, err := tf.ConfigureAP(ctx, options...)
	if err != nil {
		s.Fatal("Failed to configure ap, err: ", err)
	}
	defer func() {
		if err := tf.DeconfigAP(ctx, ap); err != nil {
			s.Error("Failed to deconfig ap, err: ", err)
		}
	}()
	s.Log("AP setup done")

	if err := tf.ConnectWifi(ctx, ap); err != nil {
		s.Fatal("Failed to connect to WiFi, err: ", err)
	}

	ping := func(ctx context.Context) error {
		return tf.PingFromDUT(ctx)
	}

	if err := tf.AssertNoDisconnect(ctx, ping); err != nil {
		s.Fatal("Failed to ping from DUT, err: ", err)
	}

	clIface, err := tf.NewClinetInterface(ctx)
	if err != nil {
		s.Fatal("Failed to get the client Wifi interface, err: ", err)
	}

	addrIPv4, err := clIface.IPv4AddressAndPrefix(ctx)
	if err != nil {
		s.Fatal("Failed to get the IP addresses after connection, err: ", err)
	}

	if addrIPv4 == "" {
		s.Fatal("Failed the IP address doesn't exists after connection")
	}

	if err := tf.DisconnectWifi(ctx); err != nil {
		s.Error("Failed to disconnect WiFi, err: ", err)
	}

	if _, err := tf.WifiClient().DeleteEntriesForSSID(ctx, &network.SSID{Ssid: ap.Config().Ssid}); err != nil {
		s.Errorf("Failed to remove entries for ssid=%s, err: %v", ap.Config().Ssid, err)
	}

	s.Log("Successfully disconnected")

	// Wait for IP to be cleared.
	s.Log("Wait for the IP address to be cleared")
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		addIPv4, err := clIface.IPv4AddressAndPrefix(ctx)
		if err != nil {
			s.Fatal("Failed to get the interface addresses, err: ", err)
		}
		if addIPv4 != "" {
			testing.Sleep(ctx, 1*time.Second)
			return errors.Errorf("failed the IP address is still set %s", addIPv4)
		}
		return nil
	}, &testing.PollOptions{Timeout: 100 * time.Second}); err != nil {
		s.Fatal("Failed to clear the IP address, err: ", err)
	}

}

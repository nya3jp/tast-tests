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
		Attr:        []string{"group:wificell", "wificell_func", "wificell_unstable"},
		ServiceDeps: []string{"tast.cros.network.Wifi"},
		Vars:        []string{"router"},
	})
}

func DisconnectClearsIP(fullctx context.Context, s *testing.State) {
	router, _ := s.Var("router")
	tf, err := wificell.NewTestFixture(fullctx, fullctx, s.DUT(), s.RPCHint(), wificell.TFRouter(router))
	if err != nil {
		s.Fatal("Failed to set up test fixture: ", err)
	}
	defer func() {
		if err := tf.Close(fullctx); err != nil {
			s.Log("Failed to tear down test fixture: ", err)
		}
	}()

	ap, err := tf.DefaultOpenNetworkAP(fullctx)
	if err != nil {
		s.Fatal("Failed to configure the AP: ", err)
	}
	defer func() {
		if err := tf.DeconfigAP(fullctx, ap); err != nil {
			s.Error("Failed to deconfig the AP: ", err)
		}
	}()

	if err := tf.ConnectWifi(fullctx, ap); err != nil {
		s.Fatal("DUT: failed to connect to WiFi: ", err)
	}
	defer func() {
		if _, err := tf.WifiClient().DeleteEntriesForSSID(fullctx, &network.SSID{Ssid: ap.Config().Ssid}); err != nil {
			s.Errorf("Failed to remove entries for ssid=%s, err: %v", ap.Config().Ssid, err)
		}
	}()

	if err := tf.PingFromDUT(fullctx); err != nil {
		s.Fatal("Failed to ping from the DUT: ", err)
	}

	addr, err := ipv4Addrs(fullctx, tf)
	if err != nil {
		s.Fatal("DUT: failed to get the IP address: ", err)
	}

	if len(addr) == 0 {
		s.Fatal("DUT: expect an IPv4 address")
	}

	s.Logf("Connected with IP address: %s. Disconnecting WiFi", addr)

	if err := tf.DisconnectWifi(fullctx); err != nil {
		s.Fatal("DUT: failed to disconnect WiFi: ", err)
	}

	// Wait for IP to be cleared.
	s.Log("Disconnected. Wait for the IP address to be cleared")
	waitIPGone := func() {
		fullctx, st := timing.Start(fullctx, "waitIPGone")
		defer st.End()
		if err := testing.Poll(fullctx, func(fullctx context.Context) error {
			addr, err := ipv4Addrs(fullctx, tf)
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

	waitIPGone()
}

// ipv4Addrs returns the IPv4 addresses for the network interface.
func ipv4Addrs(fullctx context.Context, tf *wificell.TestFixture) ([]string, error) {
	iface, err := tf.ClientInterface(fullctx)
	if err != nil {
		return nil, errors.Wrap(err, "DUT: failed to get the client WiFi interface")
	}
	netIface := &network.IPv4AddrsRequest{
		InterfaceName: iface,
	}
	addr, err := tf.WifiClient().GetIPv4Addrs(fullctx, netIface)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get the IPv4 addresses")
	}

	return addr.Ipv4, nil
}

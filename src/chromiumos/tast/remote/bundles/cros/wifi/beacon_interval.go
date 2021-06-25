// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wifi

import (
	"context"
	"strconv"

	"chromiumos/tast/dut"
	"chromiumos/tast/errors"
	"chromiumos/tast/remote/network/iw"
	"chromiumos/tast/remote/wificell"
	"chromiumos/tast/remote/wificell/hostapd"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: BeaconInterval,
		Desc: "Verifies that the beacon interval set on the AP is successfully adopted by the DUT",
		Contacts: []string{
			"chromeos-wifi-champs@google.com", // WiFi oncall rotation; or http://b/new?component=893827
		},
		Attr:        []string{"group:wificell", "wificell_func", "wificell_cq"},
		ServiceDeps: []string{wificell.TFServiceName},
		Fixture:     "wificellFixt",
	})
}

func BeaconInterval(ctx context.Context, s *testing.State) {
	tf := s.FixtValue().(*wificell.TestFixture)

	// The value of beacon interval to be set in hostapd config
	// and checked from DUT.
	const expectBeaconInt = 200

	s.Log("Setting up AP")
	apOps := []hostapd.Option{
		hostapd.Mode(hostapd.Mode80211nMixed),
		hostapd.Channel(6),
		hostapd.HTCaps(hostapd.HTCapHT40),
		hostapd.BeaconInterval(expectBeaconInt),
	}
	ap, err := tf.ConfigureAP(ctx, apOps, nil)
	if err != nil {
		s.Fatal("Failed to configure ap: ", err)
	}
	defer func(ctx context.Context) {
		if err := tf.DeconfigAP(ctx, ap); err != nil {
			s.Error("Failed to deconfig ap: ", err)
		}
	}(ctx)
	ctx, cancel := tf.ReserveForDeconfigAP(ctx, ap)
	defer cancel()

	s.Log("Connecting to WiFi")
	if _, err := tf.ConnectWifiAP(ctx, ap); err != nil {
		s.Fatal("Failed to connect to WiFi: ", err)
	}
	defer func(ctx context.Context) {
		if err := tf.CleanDisconnectWifi(ctx); err != nil {
			s.Error("Failed to disconnect WiFi: ", err)
		}
	}(ctx)
	ctx, cancel = tf.ReserveForDisconnect(ctx)
	defer cancel()

	s.Log("Start verification")
	// Check the beacon interval setting.
	iface, err := tf.ClientInterface(ctx)
	if err != nil {
		s.Fatal("Failed to get DUT's WiFi interface: ", err)
	}
	bi, err := ifaceBeaconInt(ctx, s.DUT(), iface)
	if err != nil {
		s.Fatal("Failed to get beacon interval: ", err)
	}
	if bi != expectBeaconInt {
		s.Fatalf("Unexpected beacon interval, got %d, want %d", bi, expectBeaconInt)
	}
	// Check connectivity.
	if err := tf.PingFromDUT(ctx, ap.ServerIP().String()); err != nil {
		s.Fatal("Failed to ping from DUT: ", err)
	}

	s.Log("Verified; tearing down")
}

func ifaceBeaconInt(ctx context.Context, dut *dut.DUT, iface string) (int, error) {
	iwr := iw.NewRemoteRunner(dut.Conn())
	val, err := iwr.LinkValue(ctx, iface, "beacon int")
	if err != nil {
		return 0, err
	}
	bi, err := strconv.Atoi(val)
	if err != nil {
		return 0, errors.Wrapf(err, "beacon int %q not a number", val)
	}
	return bi, nil
}

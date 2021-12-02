// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wifi

import (
	"context"
	"net"

	"chromiumos/tast/remote/bundles/cros/wifi/wifiutil"
	"chromiumos/tast/remote/network/ip"
	"chromiumos/tast/remote/wificell"
	"chromiumos/tast/remote/wificell/hostapd"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: RandomMACAddress,
		Desc: "Verifies that the MAC address is randomized (or not) according to the setting when we toggle it on/off",
		Contacts: []string{
			"yenlinlai@google.com",            // Test author
			"chromeos-wifi-champs@google.com", // WiFi oncall rotation; or http://b/new?component=893827
		},
		Attr:         []string{"group:wificell", "wificell_func"},
		ServiceDeps:  []string{wificell.TFServiceName},
		Fixture:      "wificellFixt",
		HardwareDeps: hwdep.D(hwdep.WifiMACAddrRandomize()),
	})
}

func RandomMACAddress(ctx context.Context, s *testing.State) {
	// Notice that this test aggressively scans all probe requests captured so when
	// run in open air environment, it is very probable to fail due to the packets
	// from other devices. (esp. the mac randomization disabled case)

	tf := s.FixtValue().(*wificell.TestFixture)

	// Use 2.4GHz channel 1 as some devices sets no_IR on 5GHz channels. See http://b/173633813.
	apOps := []hostapd.Option{
		hostapd.Mode(hostapd.Mode80211nPure),
		hostapd.Channel(1),
		hostapd.HTCaps(hostapd.HTCapHT20),
	}
	ap, err := tf.ConfigureAP(ctx, apOps, nil)
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

	// Get the MAC address of WiFi interface.
	iface, err := tf.ClientInterface(ctx)
	if err != nil {
		s.Fatal("Failed to get WiFi interface of DUT: ", err)
	}
	ipr := ip.NewRemoteRunner(s.DUT().Conn())
	mac, err := ipr.MAC(ctx, iface)
	if err != nil {
		s.Fatal("Failed to get MAC of WiFi interface: ", err)
	}

	// Test both enabled and disabled cases.
	testcases := []struct {
		name    string
		enabled bool
	}{
		{
			name:    "randomize_enabled",
			enabled: true,
		},
		{
			name:    "randomize_disabled",
			enabled: false,
		},
	}

	for _, tc := range testcases {
		if !s.Run(ctx, tc.name, func(ctx context.Context, s *testing.State) {
			err := wifiutil.VerifyMACUsedForScan(ctx, tf, ap, tc.name, tc.enabled, []net.HardwareAddr{mac})
			if err != nil {
				s.Fatal("Subtest failed: ", err)
			}
		}) {
			// Stop if any of the testcase failed.
			return
		}
	}

	s.Log("Verified; tearing down")
}

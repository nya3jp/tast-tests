// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wifi

import (
	"bytes"
	"context"
	"net"

	cip "chromiumos/tast/common/network/ip"
	"chromiumos/tast/common/shillconst"
	"chromiumos/tast/remote/bundles/cros/wifi/wifiutil"
	"chromiumos/tast/remote/network/ip"
	"chromiumos/tast/remote/wificell"
	"chromiumos/tast/remote/wificell/dutcfg"
	"chromiumos/tast/remote/wificell/hostapd"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: ConnectRandomizeMAC,
		Desc: "Verifies that during connection the MAC address is randomized (or not) according to the setting",
		Contacts: []string{
			"andrzejo@google.com",             // Test author
			"chromeos-wifi-champs@google.com", // WiFi oncall rotation; or http://b/new?component=893827
		},
		Attr:         []string{"group:wificell", "wificell_func", "wificell_unstable"},
		ServiceDeps:  []string{wificell.TFServiceName},
		Fixture:      "wificellFixt",
		HardwareDeps: hwdep.D(hwdep.WifiMACAddrRandomize()),
	})
}

func ConnectRandomizeMAC(ctx context.Context, s *testing.State) {
	tf := s.FixtValue().(*wificell.TestFixture)

	// Use 2.4GHz channel 1 as some devices sets no_IR on 5GHz channels. See http://b/173633813.
	apOps := []hostapd.Option{
		hostapd.Mode(hostapd.Mode80211nPure),
		hostapd.Channel(1),
		hostapd.HTCaps(hostapd.HTCapHT20),
	}
	ap1, err := tf.ConfigureAP(ctx, apOps, nil)
	if err != nil {
		s.Fatal("Failed to configure the AP: ", err)
	}
	defer func(ctx context.Context) {
		if err := tf.DeconfigAP(ctx, ap1); err != nil {
			s.Error("Failed to deconfig the AP: ", err)
		}
	}(ctx)
	ctx, cancel := tf.ReserveForDeconfigAP(ctx, ap1)
	defer cancel()

	// Get the MAC address of WiFi interface.
	iface, err := tf.ClientInterface(ctx)
	if err != nil {
		s.Fatal("Failed to get WiFi interface of DUT: ", err)
	}
	ipr := ip.NewRemoteRunner(s.DUT().Conn())
	hwMac, err := ipr.MAC(ctx, iface)
	if err != nil {
		s.Fatal("Failed to get MAC of WiFi interface: ", err)
	}
	s.Log("Read HW MAC: ", hwMac)
	defer func(ctx context.Context, iface string, mac net.HardwareAddr) {
		if err := ipr.SetLinkDown(ctx, iface); err != nil {
			s.Error("Failed to set the interface down: ", err)
		}
		if err = ipr.SetMAC(ctx, iface, mac); err != nil {
			s.Error("Failed to revert the original MAC: ", err)
		}
		if err = ipr.SetLinkUp(ctx, iface); err != nil {
			s.Error("Failed to set the interface up: ", err)
		}
	}(ctx, iface, hwMac)
	// Make sure the device is up
	link, err := ipr.State(ctx, iface)
	if err != nil {
		s.Fatal("Failed to get link state")
	}
	if link != cip.LinkStateUp {
		if err = ipr.SetLinkUp(ctx, iface); err != nil {
			s.Error("Failed to set the interface up: ", err)
		}
	}

	// Routine to connect with PersistentRandom policy and get current MAC address
	connectAndGetMAC := func(ctx context.Context, ap *wificell.APIface) (net.HardwareAddr, string) {
		configProps := map[string]interface{}{
			shillconst.ServicePropertyWiFiRandomMACPolicy: shillconst.MacPolicyPersistentRandom,
		}
		resp, err := tf.ConnectWifiAP(ctx, ap, dutcfg.ConnProperties(configProps))
		if err != nil {
			s.Fatal("Failed to connect to WiFi: ", err)
		}
		defer func(ctx context.Context) {
			if err := tf.DisconnectWifi(ctx); err != nil {
				s.Fatal("Failed to disconnect WiFi: ", err)
			}
		}(ctx)
		ctx, cancel = tf.ReserveForDisconnect(ctx)
		defer cancel()
		s.Log("Connected to service: ", resp.ServicePath)
		macAddr, err := ipr.MAC(ctx, iface)
		if err != nil {
			s.Fatal("Failed to get MAC of WiFi interface: ", err)
		}
		return macAddr, resp.ServicePath
	}

	// Connect to AP1 and check that MAC has changed
	connMac, servicePath := connectAndGetMAC(ctx, ap1)
	s.Log("MAC after connection: ", connMac)
	if bytes.Equal(hwMac, connMac) {
		s.Fatal("Failed to randomize MAC during connection")
	}

	// Reconnect to the same network and check that MAC is kept the same
	reconnMac, _ := connectAndGetMAC(ctx, ap1)
	s.Log("MAC after re-connection: ", reconnMac)
	if !bytes.Equal(reconnMac, connMac) {
		s.Fatalf("Failed to keep the MAC during re-connection: %s vs %s", reconnMac, connMac)
	}

	// Switch to AP2 (also with randomization turned on) and check
	// that MAC has changed.
	ap2, err := tf.ConfigureAP(ctx, apOps, nil)
	if err != nil {
		s.Fatal("Failed to configure the AP: ", err)
	}
	defer func(ctx context.Context) {
		if err := tf.DeconfigAP(ctx, ap2); err != nil {
			s.Error("Failed to deconfig the AP: ", err)
		}
	}(ctx)
	ctx, cancel = tf.ReserveForDeconfigAP(ctx, ap2)
	defer cancel()

	connMac2, _ := connectAndGetMAC(ctx, ap2)
	s.Log("MAC after connection to AP2: ", connMac2)
	// This should be a new address - no previous one should be used
	for _, prevMac := range []net.HardwareAddr{hwMac, connMac} {
		if bytes.Equal(prevMac, connMac2) {
			s.Fatal("Used previous MAC address: ", prevMac)
		}
	}

	// Go back to AP1 and check if we still have the same MAC as
	// before (we are using the PersistentRandom policy).
	connMac1, servicePath1 := connectAndGetMAC(ctx, ap1)
	if servicePath1 != servicePath {
		s.Fatal("Different service used during reconnection for AP1")
	}
	s.Log("MAC after going back to AP1: ", connMac1)
	if !bytes.Equal(connMac, connMac1) {
		s.Fatalf("MAC used for AP1 changed after switching AP: %s vs %s", connMac1, connMac)
	}

	// Check that randomization for scans still works as expected.
	err = wifiutil.VerifyMACUsedForScan(ctx, tf, ap1, "disconnected-randomized", true,
		[]net.HardwareAddr{hwMac, connMac1, connMac2})
	if err != nil {
		s.Fatal("Failed to verify correct MAC used during scanning: ", err)
	}

	s.Log("Completed successfully, cleaning up")
}

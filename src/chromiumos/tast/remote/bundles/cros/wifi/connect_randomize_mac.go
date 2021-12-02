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

	// Connect with PersistentRandom policy and ...
	configProps := map[string]interface{}{
		shillconst.ServicePropertyWiFiRandomMACPolicy: shillconst.MacPolicyPersistentRandom,
	}
	resp, err := tf.ConnectWifiAP(ctx, ap1, dutcfg.ConnProperties(configProps))
	if err != nil {
		s.Fatal("Failed to connect to WiFi: ", err)
	}
	servicePath := resp.ServicePath
	defer func(ctx context.Context) {
		if err := tf.CleanDisconnectWifi(ctx); err != nil {
			s.Error("Failed to disconnect WiFi: ", err)
		}
	}(ctx)
	ctx, cancel = tf.ReserveForDisconnect(ctx)
	defer cancel()
	s.Log("Connected to service: ", servicePath)
	// ... check that MAC has changed
	connMac, err := ipr.MAC(ctx, iface)
	if err != nil {
		s.Fatal("Failed to get MAC of WiFi interface: ", err)
	}
	s.Log("MAC after connection: ", connMac)
	if bytes.Equal(hwMac, connMac) {
		s.Fatal("Failed to randomize MAC during connection")
	}

	// Reconnect to the same network and ...
	err = tf.DisconnectWifi(ctx)
	if err != nil {
		s.Fatal("Failed to disconnect: ", err)
	}
	_, err = tf.ConnectWifiAP(ctx, ap1, dutcfg.ConnProperties(configProps))
	if err != nil {
		s.Fatal("Failed to reconnect to WiFi: ", err)
	}
	// ... check that MAC is kept the same
	reconnMac, err := ipr.MAC(ctx, iface)
	if err != nil {
		s.Fatal("Failed to get MAC of WiFi interface: ", err)
	}
	s.Log("MAC after re-connection: ", reconnMac)
	if !bytes.Equal(reconnMac, connMac) {
		s.Fatal("Failed to keep the same random MAC during re-connection")
	}

	// Switch to AP2 (also with randomization turned on) and check
	// that MAC has changed.
	err = tf.DisconnectWifi(ctx)
	if err != nil {
		s.Fatal("Failed to disconnect: ", err)
	}
	ap2, err := tf.ConfigureAP(ctx, apOps, nil)
	if err != nil {
		s.Fatal("Failed to configure the AP: ", err)
	}
	defer func(ctx context.Context) {
		if err := tf.DeconfigAP(ctx, ap1); err != nil {
			s.Error("Failed to deconfig the AP: ", err)
		}
	}(ctx)
	ctx, cancel = tf.ReserveForDeconfigAP(ctx, ap2)
	defer cancel()
	_, err = tf.ConnectWifiAP(ctx, ap2, dutcfg.ConnProperties(configProps))
	if err != nil {
		s.Fatal("Failed to connect to the 2nd AP: ", err)
	}
	connMac2, err := ipr.MAC(ctx, iface)
	if err != nil {
		s.Fatal("Failed to get MAC of WiFi interface: ", err)
	}
	s.Log("MAC after connection to AP2: ", connMac2)
	// This should be a new address - no previous one should be used
	for _, prevMac := range []net.HardwareAddr{hwMac, connMac} {
		if bytes.Equal(prevMac, connMac2) {
			s.Fatal("Used previous MAC address")
		}
	}

	// Go back to AP1 and check if we have the same MAC as before
	// (we are using the PersistentRandom policy).
	err = tf.DisconnectWifi(ctx)
	if err != nil {
		s.Fatal("Failed to disconnect: ", err)
	}
	resp, err = tf.ConnectWifiAP(ctx, ap1, dutcfg.ConnProperties(configProps))
	if err != nil {
		s.Fatal("Failed to connect to the 2nd AP: ", err)
	}
	if resp.ServicePath != servicePath {
		s.Fatal("Different service used during reconnection for AP1")
	}
	connMac1, err := ipr.MAC(ctx, iface)
	if err != nil {
		s.Fatal("Failed to get MAC of WiFi interface: ", err)
	}
	s.Log("MAC after going back to AP1: ", connMac1)
	if !bytes.Equal(connMac, connMac1) {
		s.Fatal("MAC used for AP1 changed after switching AP")
	}

	// When we are connected current MAC is used in scanning probes,
	// so let's disconnect and check that randomization for scans
	// still works as expected.
	if err = tf.DisconnectWifi(ctx); err != nil {
		s.Fatal("Failed to disconnect: ", err)
	}
	err = wifiutil.VerifyMACUsedForScan(ctx, tf, ap1, "disconnected-randomized", true,
		[]net.HardwareAddr{hwMac, connMac1, connMac2})
	if err != nil {
		s.Fatal("Failed to verify correct MAC used during scanning: ", err)
	}

	s.Log("Completed successfully, cleaning up")
}

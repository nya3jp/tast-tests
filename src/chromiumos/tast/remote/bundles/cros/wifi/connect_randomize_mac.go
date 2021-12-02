// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wifi

import (
	"bytes"
	"context"
	"net"
	"time"

	"github.com/golang/protobuf/ptypes/empty"
	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"

	"chromiumos/tast/common/shillconst"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/remote/bundles/cros/wifi/wifiutil"
	"chromiumos/tast/remote/network/ip"
	"chromiumos/tast/remote/wificell"
	"chromiumos/tast/remote/wificell/dutcfg"
	"chromiumos/tast/remote/wificell/hostapd"
	"chromiumos/tast/remote/wificell/pcap"
	"chromiumos/tast/services/cros/wifi"
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
		Attr:         []string{"group:wificell", "wificell_func"},
		ServiceDeps:  []string{wificell.TFServiceName},
		Fixture:      "wificellFixt",
		HardwareDeps: hwdep.D(hwdep.WifiMACAddrRandomize()),
	})
}

func verifyRandomMACIsUsedForScan(ctx context.Context, s *testing.State, ap *wificell.APIface, name string, macs []net.HardwareAddr) {
	tf := s.FixtValue().(*wificell.TestFixture)

	resp, err := tf.WifiClient().SetMACRandomize(ctx, &wifi.SetMACRandomizeRequest{Enable: true})
	if err != nil {
		s.Fatal("Failed to set MAC randomization: ", err)
	}
	if resp.OldSetting != true {
		s.Log("Enabled MAC randomization for scans")
	}
	// Always restore the setting on leaving.
	defer func(ctx context.Context, restore bool) {
		if _, err := tf.WifiClient().SetMACRandomize(ctx, &wifi.SetMACRandomizeRequest{Enable: restore}); err != nil {
			s.Errorf("Failed to restore MAC randomization setting back to %t: %v", restore, err)
		}
	}(ctx, resp.OldSetting)
	ctx, cancel := ctxutil.Shorten(ctx, time.Second)
	defer cancel()

	// Wait current scan to be done if available to avoid possible scan started
	// before our setting.
	if _, err := tf.WifiClient().WaitScanIdle(ctx, &empty.Empty{}); err != nil {
		s.Fatal("Failed to wait for current scan to be done: ", err)
	}

	timeoutCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	pcapPath, err := wifiutil.ScanAndCollectPcap(timeoutCtx, tf, name, 5, ap.Config().Channel)
	if err != nil {
		s.Fatal("Failed to collect pcap: ", err)
	}

	s.Log("Start analyzing pcap")
	filters := []pcap.Filter{
		pcap.RejectLowSignal(),
		pcap.Dot11FCSValid(),
		pcap.TypeFilter(
			layers.LayerTypeDot11MgmtProbeReq,
			func(layer gopacket.Layer) bool {
				ssid, err := pcap.ParseProbeReqSSID(layer.(*layers.Dot11MgmtProbeReq))
				if err != nil {
					s.Logf("skip malformed probe request %v: %v", layer, err)
					return false
				}
				// Take the ones with wildcard SSID or SSID of the AP.
				if ssid == "" || ssid == ap.Config().SSID {
					return true
				}
				return false
			},
		),
	}
	packets, err := pcap.ReadPackets(pcapPath, filters...)
	if err != nil {
		s.Fatal("Failed to read packets: ", err)
	}
	if len(packets) == 0 {
		s.Fatal("No probe request found in pcap")
	}
	s.Logf("Total %d probe requests found", len(packets))

	for _, p := range packets {
		// Get sender address.
		layer := p.Layer(layers.LayerTypeDot11)
		if layer == nil {
			s.Fatalf("ProbeReq packet %v does not have Dot11 layer", p)
		}
		dot11, ok := layer.(*layers.Dot11)
		if !ok {
			s.Fatalf("Dot11 layer output %v not *layers.Dot11", p)
		}
		sender := dot11.Address2

		// Verify that sender address has not been used before.
		for _, mac := range macs {
			if bytes.Equal(sender, mac) {
				s.Fatal("Expected randomized MAC but found probe request with a known MAC")
			}
		}
	}
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

	// Connect with PersistentRandom policy and ...
	configProps := map[string]interface{}{
		shillconst.ServicePropertyWiFiRandomMACPolicy: shillconst.MacPolicyPersistentRandom,
	}
	resp, err := tf.ConnectWifiAP(ctx, ap1, dutcfg.ConnProperties(configProps))
	if err != nil {
		s.Fatal("Failed to connect to WiFi: ", err)
	}
	servicePath := resp.ServicePath
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

	// Now switch to AP2 (also with randomization turned on) and
	// check that MAC has changed.
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
	for _, mac := range []net.HardwareAddr{hwMac, connMac} {
		if bytes.Equal(mac, connMac2) {
			s.Fatal("Used previous MAC address")
		}
	}

	// Now go back to AP1 and check if we have the same MAC as
	// before (we are using the PersistentRandom policy).
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
	err = tf.DisconnectWifi(ctx)
	if err != nil {
		s.Fatal("Failed to disconnect: ", err)
	}
	verifyRandomMACIsUsedForScan(ctx, s, ap1, "disconnected-randomized",
		[]net.HardwareAddr{hwMac, connMac1, connMac2})

	s.Log("Completed successfully, cleaning up")
}

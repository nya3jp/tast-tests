// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wifi

import (
	"bytes"
	"context"
	"net"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"

	cip "chromiumos/tast/common/network/ip"
	"chromiumos/tast/common/shillconst"
	"chromiumos/tast/errors"
	"chromiumos/tast/remote/bundles/cros/wifi/wifiutil"
	"chromiumos/tast/remote/network/ip"
	"chromiumos/tast/remote/wificell"
	"chromiumos/tast/remote/wificell/dutcfg"
	"chromiumos/tast/remote/wificell/hostapd"
	"chromiumos/tast/remote/wificell/pcap"
	"chromiumos/tast/remote/wificell/router"
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

type macsAllowed int

const (
	noneOf   = 0
	allEqual = 1
)

func isBroadcastMAC(mac net.HardwareAddr) bool {
	return bytes.Compare(mac, []byte{0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF}) == 0
}

func verifyMACUsed(ctx context.Context, macAaddr net.HardwareAddr, pcapPath string,
	mode macsAllowed, macs []net.HardwareAddr) error {
	var prevMacUsed net.HardwareAddr  // used when we expect new MAC and we find (one of) previous MACs used
	var wrongMacUsed net.HardwareAddr // used when we expect the MAC be kept but it changes

	// This function returns bool so that it can be used also inside packet filter
	checkFailed := func(mac net.HardwareAddr, toDS bool) bool {
		switch mode {
		case allEqual:
			if !bytes.Equal(mac, macs[0]) && toDS {
				if wrongMacUsed == nil {
					wrongMacUsed = mac
				}
				return true
			}
		case noneOf:
			for _, prevMac := range macs {
				if bytes.Equal(mac, prevMac) {
					if prevMacUsed == nil {
						prevMacUsed = prevMac
					}
					return true
				}
			}
		}
		return false
	}
	// First check the MAC set on the wlan interface ...
	if checkFailed(macAaddr, true) {
		if mode == allEqual {
			return errors.Errorf("hardware address changed: got %s, want %s", wrongMacUsed, macs[0])
		}
		return errors.New("used previous MAC address: " + prevMacUsed.String())
	}
	// ... and now let's find packet in pcap using wrong MAC
	filters := []pcap.Filter{
		pcap.RejectLowSignal(),
		pcap.Dot11FCSValid(),
		pcap.TypeFilter(
			layers.LayerTypeDot11,
			func(layer gopacket.Layer) bool {
				dot11 := layer.(*layers.Dot11)
				if dot11.Flags.ToDS() {
					return checkFailed(dot11.Address2, true)
				} else if dot11.Flags.FromDS() {
					return (!isBroadcastMAC(dot11.Address1) && checkFailed(dot11.Address1, false)) ||
						(!isBroadcastMAC(dot11.Address3) && checkFailed(dot11.Address3, false))
				}
				return false
			},
		),
	}
	packets, err := pcap.ReadPackets(pcapPath, filters...)
	if err != nil {
		return errors.Wrap(err, "failed to read packets")
	}
	if len(packets) > 0 {
		testing.ContextLogf(ctx, "Found %d packets with incorrect MAC", len(packets))
		if mode == allEqual {
			return errors.New("found packet using wrong MAC: " + wrongMacUsed.String())
		}
		return errors.New("found packet with previously used MAC: " + prevMacUsed.String())
	}

	return nil
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

	// We want control over capturer start/stop so we don't use fixture with
	// pcap but spawn it here and use manually.
	pcapRouter, ok := tf.Pcap().(router.SupportCapture)
	if !ok {
		s.Fatal("Device without capture support - device type: ", tf.Pcap().RouterType())
	}

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
		if err := ipr.SetMAC(ctx, iface, mac); err != nil {
			s.Error("Failed to revert the original MAC: ", err)
		}
		if err := ipr.SetLinkUp(ctx, iface); err != nil {
			s.Error("Failed to set the interface up: ", err)
		}
	}(ctx, iface, hwMac)
	// Make sure the device is up
	link, err := ipr.State(ctx, iface)
	if err != nil {
		s.Fatal("Failed to get link state")
	}
	if link != cip.LinkStateUp {
		if err := ipr.SetLinkUp(ctx, iface); err != nil {
			s.Error("Failed to set the interface up: ", err)
		}
	}

	// Routine to connect with PersistentRandom policy and get current MAC address together with captured packets
	connectAndGetConnData := func(ctx context.Context, ap *wificell.APIface, name string) (net.HardwareAddr, string, string) {
		freqOps, err := ap.Config().PcapFreqOptions()
		if err != nil {
			s.Fatal("Failed to get frequency options for Pcap: ", err)
		}
		capturer, err := pcapRouter.StartCapture(ctx, name, ap.Config().Channel, freqOps)
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
		ctx, cancel := tf.ReserveForDisconnect(ctx)
		defer cancel()
		s.Log("Connected to service: ", resp.ServicePath)
		macAddr, err := ipr.MAC(ctx, iface)
		if err != nil {
			s.Fatal("Failed to get MAC of WiFi interface: ", err)
		}
		if err := capturer.Close(ctx); err != nil {
			s.Fatal("Failed to close and download packet capture: ", err)
		}
		packetPath, err := capturer.PacketPath(ctx)
		if err != nil {
			s.Fatal("Failed to get packet capture path: ", err)
		}
		return macAddr, packetPath, resp.ServicePath
	}

	// Connect to AP1 and check that MAC has changed
	connMac, ap1pcap1, servicePath := connectAndGetConnData(ctx, ap1, "ap1-connect")
	s.Log("MAC after connection: ", connMac)
	if err := verifyMACUsed(ctx, connMac, ap1pcap1, noneOf, []net.HardwareAddr{hwMac}); err != nil {
		s.Fatal("Failed to randomize MAC during connection: ", err)
	}

	// Reconnect to the same network and check that MAC is kept the same
	reconnMac, ap1pcap2, _ := connectAndGetConnData(ctx, ap1, "ap1-reconnect")
	s.Log("MAC after re-connection: ", reconnMac)
	if err := verifyMACUsed(ctx, reconnMac, ap1pcap2, allEqual, []net.HardwareAddr{connMac}); err != nil {
		s.Fatal("Failed to keep the MAC during re-connection: ", err)
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

	connMac2, ap2pcap, servicePath2 := connectAndGetConnData(ctx, ap2, "ap2-connect")
	s.Log("MAC after connection to AP2: ", connMac2)
	if servicePath == servicePath2 {
		s.Fatal("The same service used for both AP1 and AP2: ", servicePath)
	}
	// This should be a new address - no previous one should be used
	if err := verifyMACUsed(ctx, connMac2, ap2pcap, noneOf, []net.HardwareAddr{hwMac, connMac}); err != nil {
		s.Fatal("Failed to change MAC for AP2: ", err)
	}

	// Go back to AP1 and check if we still have the same MAC as
	// before (we are using the PersistentRandom policy).
	connMac1, ap1pcap3, servicePath1 := connectAndGetConnData(ctx, ap1, "ap1-return")
	if servicePath1 != servicePath {
		s.Fatalf("Different service used during reconnection for AP1: got %s, want %s", servicePath1, servicePath)
	}
	s.Log("MAC after going back to AP1: ", connMac1)
	if err := verifyMACUsed(ctx, connMac1, ap1pcap3, allEqual, []net.HardwareAddr{connMac}); err != nil {
		s.Fatal("Failed to keep the MAC for AP1 after switching back: ", err)
	}

	// Check that randomization for scans still works as expected.
	err = wifiutil.VerifyMACUsedForScan(ctx, tf, ap1, "disconnected-randomized", true,
		[]net.HardwareAddr{hwMac, connMac1, connMac2})
	if err != nil {
		s.Fatal("Failed to verify correct MAC used during scanning: ", err)
	}

	s.Log("Completed successfully, cleaning up")
}

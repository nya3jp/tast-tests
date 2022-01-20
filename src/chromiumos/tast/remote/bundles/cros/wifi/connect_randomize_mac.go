// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wifi

import (
	"bytes"
	"context"
	"net"

	"chromiumos/tast/remote/wificell/router/common/support"
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
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: ConnectRandomizeMAC,
		Desc: "Verifies that during connection the MAC address is randomized (or not) according to the setting",
		Contacts: []string{
			"amo@semihalf.com",                // Test author
			"chromeos-wifi-champs@google.com", // WiFi oncall rotation; or http://b/new?component=893827
		},
		Attr:         []string{"group:wificell", "wificell_func", "wificell_unstable"},
		ServiceDeps:  []string{wificell.TFServiceName},
		Fixture:      "wificellFixt",
		HardwareDeps: hwdep.D(hwdep.WifiMACAddrRandomize()),
	})
}

func isBroadcastMAC(mac net.HardwareAddr) bool {
	return bytes.Compare(mac, []byte{0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF}) == 0
}

// findWrongPackets is a simple filter returning all packets from capture
// indicated by pcapPath that fail to pass the wrongMAC check, that is when
// wrongMAC function returns true.
func findWrongPackets(pcapPath string, wrongMAC func(mac net.HardwareAddr, toDS bool) bool) ([]gopacket.Packet, error) {
	filters := []pcap.Filter{
		pcap.RejectLowSignal(),
		pcap.Dot11FCSValid(),
		pcap.TypeFilter(
			layers.LayerTypeDot11,
			func(layer gopacket.Layer) bool {
				dot11 := layer.(*layers.Dot11)
				if dot11.Flags.ToDS() {
					return wrongMAC(dot11.Address2, true)
				}
				if dot11.Flags.FromDS() {
					return (!isBroadcastMAC(dot11.Address1) && wrongMAC(dot11.Address1, false)) ||
						(!isBroadcastMAC(dot11.Address3) && wrongMAC(dot11.Address3, false))
				}
				return false
			},
		),
	}
	return pcap.ReadPackets(pcapPath, filters...)
}

// verifyMACIsKept checks whether currently used MAC address (macAddr) is the
// same as the original one (origMAC) and that this address is used in all
// communication (by parsing captured packets in pcapPath).
func verifyMACIsKept(ctx context.Context, macAddr net.HardwareAddr, pcapPath string, origMAC net.HardwareAddr) error {
	var wrongMACUsed net.HardwareAddr
	wrongMAC := func(mac net.HardwareAddr, toDS bool) bool {
		// When checking that all packets use original MAC we have to
		// confine search to only 'toDS' direction because of high
		// probability of getting unrelated packet from the environment.
		if !bytes.Equal(mac, origMAC) && toDS {
			if wrongMACUsed == nil {
				wrongMACUsed = mac
			}
			return true
		}
		return false
	}
	if wrongMAC(macAddr, true) {
		return errors.Errorf("hardware address changed: got %s, want %s", macAddr, origMAC)
	}
	packets, err := findWrongPackets(pcapPath, wrongMAC)
	if err != nil {
		return errors.Wrap(err, "failed to read packets")
	}
	if len(packets) > 0 {
		testing.ContextLogf(ctx, "Found %d packets with incorrect MAC", len(packets))
		return errors.New("found packet using wrong MAC: " + wrongMACUsed.String())
	}
	return nil
}

// verifyMACIsChanged checks whether currently used MAC address (macAddr) is
// different from any previously used addresses (indicated in prevMACs).
// It also verifies that none of the previously used MAC addresses is used in
// communication (by parsing captured packets in pcapPath).
func verifyMACIsChanged(ctx context.Context, macAddr net.HardwareAddr, pcapPath string, prevMACs []net.HardwareAddr) error {
	var prevMACUsed net.HardwareAddr
	wrongMAC := func(mac net.HardwareAddr, _ bool) bool {
		for _, prevMAC := range prevMACs {
			if bytes.Equal(mac, prevMAC) {
				if prevMACUsed == nil {
					prevMACUsed = prevMAC
				}
				return true
			}
		}
		return false
	}
	if wrongMAC(macAddr, true) {
		return errors.New("used previous MAC address: " + prevMACUsed.String())
	}
	packets, err := findWrongPackets(pcapPath, wrongMAC)
	if err != nil {
		return errors.Wrap(err, "failed to read packets")
	}
	if len(packets) > 0 {
		testing.ContextLogf(ctx, "Found %d packets with incorrect MAC", len(packets))
		return errors.New("found packet with previously used MAC: " + prevMACUsed.String())
	}
	return nil
}

func ConnectRandomizeMAC(ctx context.Context, s *testing.State) {
	tf := s.FixtValue().(*wificell.TestFixture)

	// Use 2.4GHz channel 1 for AP1.
	apOps := []hostapd.Option{
		hostapd.Mode(hostapd.Mode80211nPure),
		hostapd.Channel(1),
		hostapd.HTCaps(hostapd.HTCapHT20),
	}
	ap1, err := tf.ConfigureAP(ctx, apOps, nil)
	if err != nil {
		s.Fatal("Failed to configure the AP: ", err)
	}
	cleanUpCtx := ctx
	ctx, cancel := tf.ReserveForDeconfigAP(ctx, ap1)
	defer cancel()
	defer func(ctx context.Context) {
		if err := tf.DeconfigAP(ctx, ap1); err != nil {
			s.Error("Failed to deconfig the AP: ", err)
		}
	}(cleanUpCtx)

	// We want control over capturer start/stop so we don't use fixture with
	// pcap but spawn it here and use manually.
	pcapRouter, err := support.WithCapture(tf.Pcap())
	if err != nil {
		s.Fatal("Device without capture support - device type: ", tf.Pcap().RouterType())
	}

	// Get the MAC address of WiFi interface.
	iface, err := tf.ClientInterface(ctx)
	if err != nil {
		s.Fatal("Failed to get WiFi interface of DUT: ", err)
	}
	ipr := ip.NewRemoteRunner(s.DUT().Conn())
	hwMAC, err := ipr.MAC(ctx, iface)
	if err != nil {
		s.Fatal("Failed to get MAC of WiFi interface: ", err)
	}
	s.Log("Read HW MAC: ", hwMAC)
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
	}(ctx, iface, hwMAC)
	// Make sure the device is up.
	link, err := ipr.State(ctx, iface)
	if err != nil {
		s.Fatal("Failed to get link state")
	}
	if link != cip.LinkStateUp {
		if err := ipr.SetLinkUp(ctx, iface); err != nil {
			s.Error("Failed to set the interface up: ", err)
		}
	}

	// Routine to connect with PersistentRandom policy and get current MAC address together with captured packets.
	connectAndGetConnData := func(ctx context.Context, ap *wificell.APIface, name string) (macAddr net.HardwareAddr, pcapPath, servicePath string) {
		freqOps, err := ap.Config().PcapFreqOptions()
		if err != nil {
			s.Fatal("Failed to get frequency options for Pcap: ", err)
		}
		action := func(ctx context.Context) error {
			configProps := map[string]interface{}{
				shillconst.ServicePropertyWiFiRandomMACPolicy: shillconst.MacPolicyPersistentRandom,
			}
			resp, err := tf.ConnectWifiAP(ctx, ap, dutcfg.ConnProperties(configProps))
			if err != nil {
				return errors.Wrap(err, "failed to connect to WiFi")
			}
			servicePath = resp.ServicePath
			cleanUpCtx := ctx
			ctx, cancel := tf.ReserveForDisconnect(ctx)
			defer cancel()
			defer func(ctx context.Context) {
				if err := tf.DisconnectWifi(ctx); err != nil {
					testing.ContextLog(ctx, "Failed to disconnect WiFi: ", err)
				}
			}(cleanUpCtx)
			testing.ContextLog(ctx, "Connected to service: ", servicePath)
			macAddr, err = ipr.MAC(ctx, iface)
			if err != nil {
				return errors.Wrap(err, "failed to get MAC of WiFi interface")
			}
			return nil
		}
		pcapPath, err = wifiutil.CollectPcapForAction(ctx, pcapRouter, name, ap.Config().Channel, freqOps, action)
		if err != nil {
			s.Fatal("Failed to get packet capture path: ", err)
		}
		return macAddr, pcapPath, servicePath
	}

	// Connect to AP1 and check that MAC has changed.
	connMac, ap1pcap1, servicePath := connectAndGetConnData(ctx, ap1, "ap1-connect")
	s.Log("MAC after connection: ", connMac)
	if err := verifyMACIsChanged(ctx, connMac, ap1pcap1, []net.HardwareAddr{hwMAC}); err != nil {
		s.Fatal("Failed to randomize MAC during connection: ", err)
	}

	// Reconnect to the same network and check that MAC is kept the same.
	reconnMac, ap1pcap2, _ := connectAndGetConnData(ctx, ap1, "ap1-reconnect")
	s.Log("MAC after re-connection: ", reconnMac)
	if err := verifyMACIsKept(ctx, reconnMac, ap1pcap2, connMac); err != nil {
		s.Fatal("Failed to keep the MAC during re-connection: ", err)
	}

	// Switch to AP2 (also with randomization turned on) and check
	// that MAC has changed.  For AP2 use 5GHz channel 48.
	apOps[1] = hostapd.Channel(48)
	ap2, err := tf.ConfigureAP(ctx, apOps, nil)
	if err != nil {
		s.Fatal("Failed to configure the AP: ", err)
	}
	cleanUpCtx = ctx
	ctx, cancel = tf.ReserveForDeconfigAP(ctx, ap2)
	defer cancel()
	defer func(ctx context.Context) {
		if err := tf.DeconfigAP(ctx, ap2); err != nil {
			s.Error("Failed to deconfig the AP: ", err)
		}
	}(cleanUpCtx)

	connMac2, ap2pcap, servicePath2 := connectAndGetConnData(ctx, ap2, "ap2-connect")
	s.Log("MAC after connection to AP2: ", connMac2)
	if servicePath == servicePath2 {
		s.Fatal("The same service used for both AP1 and AP2: ", servicePath)
	}
	// This should be a new address - no previous one should be used.
	if err := verifyMACIsChanged(ctx, connMac2, ap2pcap, []net.HardwareAddr{hwMAC, connMac}); err != nil {
		s.Fatal("Failed to change MAC for AP2: ", err)
	}

	// Go back to AP1 and check if we still have the same MAC as
	// before (we are using the PersistentRandom policy).
	connMac1, ap1pcap3, servicePath1 := connectAndGetConnData(ctx, ap1, "ap1-return")
	if servicePath1 != servicePath {
		s.Fatalf("Different service used during reconnection for AP1: got %s, want %s", servicePath1, servicePath)
	}
	s.Log("MAC after going back to AP1: ", connMac1)
	if err := verifyMACIsKept(ctx, connMac1, ap1pcap3, connMac); err != nil {
		s.Fatal("Failed to keep the MAC for AP1 after switching back: ", err)
	}

	// Check that randomization for scans still works as expected.
	err = wifiutil.VerifyMACUsedForScan(ctx, tf, ap1, "disconnected-randomized", true,
		[]net.HardwareAddr{hwMAC, connMac1, connMac2})
	if err != nil {
		s.Fatal("Failed to verify correct MAC used during scanning: ", err)
	}

	s.Log("Completed successfully, cleaning up")
}

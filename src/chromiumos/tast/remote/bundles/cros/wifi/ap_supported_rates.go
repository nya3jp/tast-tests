// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wifi

import (
	"bytes"
	"context"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"

	"chromiumos/tast/remote/bundles/cros/wifi/wifiutil"
	"chromiumos/tast/remote/network/ip"
	"chromiumos/tast/remote/wificell"
	"chromiumos/tast/remote/wificell/hostapd"
	"chromiumos/tast/remote/wificell/pcap"
	"chromiumos/tast/testing"
)

type supportedRatesCase struct {
	apOpts  []hostapd.Option
	minRate uint8
}

func init() {
	testing.AddTest(&testing.Test{
		Func: APSupportedRates,
		Desc: "Verifies that we avoid legacy bitrates on APs that disable them",
		Contacts: []string{
			"briannorris@chromium.org",        // Test author
			"chromeos-wifi-champs@google.com", // WiFi oncall rotation; or http://b/new?component=893827
		},
		Attr:        []string{"group:wificell", "wificell_func", "wificell_unstable"},
		ServiceDeps: []string{wificell.TFServiceName},
		Pre:         wificell.TestFixturePre(),
		Vars:        []string{"router", "pcap"},
		// See b/138406224. ath10k only supports this on CrOS kernels >=4.14
		SoftwareDeps: []string{"no_ath10k_4_4"},
		Params: []testing.Param{
			{
				Name: "11g",
				Val: supportedRatesCase{
					apOpts: []hostapd.Option{
						hostapd.Mode(hostapd.Mode80211g), hostapd.Channel(1),
						hostapd.BasicRates(24.0), hostapd.SupportedRates(24.0, 36.0, 48.0, 54.0),
					},
					minRate: 24,
				},
			},
			{
				Name: "11ac",
				Val: supportedRatesCase{
					apOpts: []hostapd.Option{
						hostapd.Mode(hostapd.Mode80211acMixed), hostapd.Channel(157), hostapd.VHTCenterChannel(155),
						hostapd.HTCaps(hostapd.HTCapHT40Plus), hostapd.VHTCaps(hostapd.VHTCapSGI80), hostapd.VHTChWidth(hostapd.VHTChWidth80),
						hostapd.BasicRates(36.0), hostapd.SupportedRates(36.0, 48.0, 54.0),
					},
					minRate: 36,
				},
			},
		},
	})
}

func APSupportedRates(ctx context.Context, s *testing.State) {
	tf := s.PreValue().(*wificell.TestFixture)
	defer func(ctx context.Context) {
		if err := tf.CollectLogs(ctx); err != nil {
			s.Log("Error collecting logs, err: ", err)
		}
	}(ctx)
	ctx, cancel := tf.ReserveForCollectLogs(ctx)
	defer cancel()

	param := s.Param().(supportedRatesCase)
	apOpts := param.apOpts

	ap, err := tf.ConfigureAP(ctx, apOpts, nil)
	if err != nil {
		s.Fatal("Failed to configure the AP: ", err)
	}
	defer func(ctx context.Context) {
		if err := tf.DeconfigAP(ctx, ap); err != nil {
			s.Error("Failed to deconfig the AP: ", err)
		}
	}(ctx)
	ctx, cancel = tf.ReserveForDeconfigAP(ctx, ap)
	defer cancel()

	// Get the MAC address of WiFi interface.
	iface, err := tf.ClientInterface(ctx)
	if err != nil {
		s.Fatal("Failed to get WiFi interface of DUT")
	}
	ipr := ip.NewRemoteRunner(s.DUT().Conn())
	mac, err := ipr.MAC(ctx, iface)
	if err != nil {
		s.Fatal("Failed to get MAC of WiFi interface")
	}

	// Operations to perform while monitoring via packet capture.
	testAction := func(ctx context.Context) error {
		cleanupCtx := ctx
		ctx, cancel = tf.ReserveForDisconnect(ctx)
		defer cancel()

		if _, err := tf.ConnectWifiAP(ctx, ap); err != nil {
			return err
		}
		defer func(ctx context.Context) {
			if err := tf.CleanDisconnectWifi(ctx); err != nil {
				s.Error("Failed to disconnect WiFi: ", err)
			}
		}(cleanupCtx)

		if err := tf.PingFromDUT(ctx, ap.ServerIP().String()); err != nil {
			s.Fatal("Failed to ping from the DUT: ", err)
		}

		if err := tf.PingFromServer(ctx); err != nil {
			s.Fatal("Failed to ping from the Server: ", err)
		}

		return nil
	}

	freqOpts, err := ap.Config().PcapFreqOptions()
	if err != nil {
		s.Fatal("Failed to get pcap freqency options: ", err)
	}
	legacyPcap, err := tf.LegacyPcap()
	if err != nil {
		s.Fatal("Unable to get legacy pcap: ", err)
	}
	pcapPath, err := wifiutil.CollectPcapForAction(ctx, legacyPcap, "connect", ap.Config().Channel, freqOpts, testAction)
	if err != nil {
		s.Fatal("Failed to collect pcap or perform action: ", err)
	}

	s.Log("Start analyzing pcap")
	filters := []pcap.Filter{
		// Use TA (not SA), because multicast may retransmit our
		// "Source-Addressed" frames at rates we don't control.
		pcap.TransmitterAddress(mac),
		// Some chips use self-addressed (receiver==self) frames to tune channel
		// performance. They don't carry host-generated traffic, so filter them out.
		pcap.TypeFilter(
			layers.LayerTypeDot11,
			func(layer gopacket.Layer) bool {
				dot11 := layer.(*layers.Dot11)
				// Skip receiver == MAC.
				return !bytes.Equal(dot11.Address1, mac)
			},
		),
		// (QoS) null filter: these frames are short (no data payload), and it's more
		// important that they be reliable (e.g., for PS transitions) than fast. See
		// b/132825853#comment40, for example.
		func(p gopacket.Packet) bool {
			qosLayer := p.Layer(layers.LayerTypeDot11DataQOSNull)
			nullLayer := p.Layer(layers.LayerTypeDot11DataNull)
			// Skip QoS null data or null data.
			return qosLayer == nil && nullLayer == nil
		},
		// TODO: skip ACKs, BlockAcks, etc.? The original test did so (see
		// https://crrev.com/c/1679995) because our test APs don't always (as of 2019-06-28)
		// respect the Supported Rates IEs that we're configuring, and so DUT ACKs may match
		// the (incorrect) rate that the AP is using. We may not want to penalize the DUT
		// for that.
	}
	packets, err := pcap.ReadPackets(pcapPath, filters...)
	if err != nil {
		s.Fatal("Failed to read packets: ", err)
	}
	if len(packets) == 0 {
		s.Fatal("No valid frames found in pcap")
	}
	s.Logf("Total %d candidate frames found", len(packets))

	var bad []gopacket.Packet
	badRates := make(map[uint8]interface{})
	for _, p := range packets {
		// Get sender address.
		layer := p.Layer(layers.LayerTypeRadioTap)
		if layer == nil {
			// Not all frames will have radiotap?
			continue
		}
		radioTap, ok := layer.(*layers.RadioTap)
		if !ok {
			s.Fatalf("RadioTap layer output %v not *layers.RadioTap", p)
		}
		if !radioTap.Present.Rate() {
			// No rate? Might be non-legacy (e.g., HT), which is a "pass."
			continue
		}
		// Rate field is in units of Mbps*2.
		rate := uint8(radioTap.Rate) / 2
		if rate < param.minRate {
			bad = append(bad, p)
			badRates[rate] = true
		}
	}

	if len(bad) != 0 {
		for i, p := range bad {
			s.Logf("Bad frame %d: %v", i, p)
		}
		var list []uint8
		for r := range badRates {
			list = append(list, r)
		}
		s.Fatalf("Expected rates >= %d; saw: %v", param.minRate, list)
	}

	s.Log("Verified; tearing down")
}

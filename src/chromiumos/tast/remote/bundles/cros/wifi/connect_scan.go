// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wifi

import (
	"bytes"
	"context"
	"fmt"
	"reflect"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"

	"chromiumos/tast/remote/bundles/cros/wifi/wifiutil"
	"chromiumos/tast/remote/network/ip"
	"chromiumos/tast/remote/wificell"
	"chromiumos/tast/remote/wificell/hostapd"
	"chromiumos/tast/remote/wificell/pcap"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: ConnectScan,
		Desc: "Verifies that the 802.11 probe frames with expected SSIDs are seen over-the-air when connecting to WiFi",
		Contacts: []string{
			"chromeos-wifi-champs@google.com", // WiFi oncall rotation; or http://b/new?component=893827
		},
		Attr:        []string{"group:wificell", "wificell_func"},
		ServiceDeps: []string{wificell.TFServiceName},
		Fixture:     "wificellFixtWithCapture",
		Params: []testing.Param{
			{
				Name: "hidden",
				Val: []hostapd.Option{
					// Use 2.4G band as some devices sets no_IR on 5GHz channels. See http://b/173633813.
					hostapd.Channel(1),
					hostapd.Mode(hostapd.Mode80211nPure),
					hostapd.HTCaps(hostapd.HTCapHT40),
					hostapd.Hidden(),
				},
			},
			{
				Name: "visible",
				Val: []hostapd.Option{
					hostapd.Channel(1), // We have visible_vht for 5G band, use 2.4G band here.
					hostapd.Mode(hostapd.Mode80211nPure),
					hostapd.HTCaps(hostapd.HTCapHT40),
				},
			},
			{
				// For coverage of 5G and VHT setting.
				Name: "visible_vht",
				Val: []hostapd.Option{
					hostapd.Channel(149),
					hostapd.Mode(hostapd.Mode80211acPure),
					hostapd.VHTChWidth(hostapd.VHTChWidth80),
					hostapd.VHTCenterChannel(155),
					hostapd.VHTCaps(hostapd.VHTCapSGI80),
					hostapd.HTCaps(hostapd.HTCapHT40Plus),
				},
			},
		},
	})
}

func ConnectScan(ctx context.Context, s *testing.State) {
	tf := s.FixtValue().(*wificell.TestFixture)

	ctx, restore, err := tf.DisableMACRandomize(ctx)
	if err != nil {
		s.Fatal("Failed to disable MAC randomization: ", err)
	}
	defer func() {
		if err := restore(); err != nil {
			s.Error("Failed to restore MAC randomization: ", err)
		}
	}()

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

	apOps := s.Param().([]hostapd.Option)
	pcapPath, apConf, err := wifiutil.ConnectAndCollectPcap(ctx, tf, apOps)
	if err != nil {
		s.Fatal("Failed to collect packet: ", err)
	}

	s.Log("Start analyzing pcap")
	filters := []pcap.Filter{
		pcap.Dot11FCSValid(),
		pcap.TypeFilter(
			layers.LayerTypeDot11,
			func(layer gopacket.Layer) bool {
				dot11 := layer.(*layers.Dot11)
				// Filter sender == MAC of DUT.
				return bytes.Equal(dot11.Address2, mac)
			},
		),
		pcap.TypeFilter(layers.LayerTypeDot11MgmtProbeReq, nil),
	}
	packets, err := pcap.ReadPackets(pcapPath, filters...)
	if err != nil {
		s.Fatal("Failed to read packets: ", err)
	}
	s.Logf("Total %d probe requests found", len(packets))

	ssidSet := make(map[string]struct{})
	for _, p := range packets {
		layer := p.Layer(layers.LayerTypeDot11MgmtProbeReq)
		if layer == nil {
			s.Fatal("Found packet without PrboeReq layer")
		}
		ssid, err := pcap.ParseProbeReqSSID(layer.(*layers.Dot11MgmtProbeReq))
		if err != nil {
			continue
		}
		ssidSet[ssid] = struct{}{}
	}

	expectedSSIDs := map[string]struct{}{
		"":          {},
		apConf.SSID: {},
	}
	if apConf.Hidden {
		// For hidden network, we expect both SSIDs.
		if !reflect.DeepEqual(ssidSet, expectedSSIDs) {
			formatMapKeys := func(m map[string]struct{}) string {
				var keys []string
				for k := range m {
					keys = append(keys, k)
				}
				return fmt.Sprintf("%q", keys)
			}
			s.Fatalf("Got set of SSIDs %s, want %s", formatMapKeys(ssidSet), formatMapKeys(expectedSSIDs))
		}
	} else {
		// For visible network, we expect wildcard SSID, but it is not guaranteed.
		for ssid := range ssidSet {
			if _, ok := expectedSSIDs[ssid]; !ok {
				s.Errorf("Found unexpected SSID=%q", ssid)
			}
		}
	}
}

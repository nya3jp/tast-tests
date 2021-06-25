// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wifi

import (
	"bytes"
	"context"
	"time"

	"github.com/golang/protobuf/ptypes/empty"
	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/remote/bundles/cros/wifi/wifiutil"
	"chromiumos/tast/remote/network/ip"
	"chromiumos/tast/remote/wificell"
	"chromiumos/tast/remote/wificell/hostapd"
	"chromiumos/tast/remote/wificell/pcap"
	"chromiumos/tast/services/cros/wifi"
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
		s.Fatal("Failed to get WiFi interface of DUT")
	}
	ipr := ip.NewRemoteRunner(s.DUT().Conn())
	mac, err := ipr.MAC(ctx, iface)
	if err != nil {
		s.Fatal("Failed to get MAC of WiFi interface")
	}

	testOnce := func(ctx context.Context, s *testing.State, name string, enabled bool) {
		resp, err := tf.WifiClient().SetMACRandomize(ctx, &wifi.SetMACRandomizeRequest{Enable: enabled})
		if err != nil {
			s.Fatalf("Failed to set MAC randomization to %t: %v", enabled, err)
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

			// Verify sender address.
			sameAddr := bytes.Equal(sender, mac)
			if enabled && sameAddr {
				s.Fatal("Expect randomized MAC but found probe request with hardware MAC")
			} else if !enabled && !sameAddr {
				s.Fatal("Expect non-randomized MAC but found probe request with non-hardware MAC")
			}
		}
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
			testOnce(ctx, s, tc.name, tc.enabled)
		}) {
			// Stop if any of the testcase failed.
			return
		}
	}

	s.Log("Verified; tearing down")
}

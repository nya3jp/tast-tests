// Copyright 2020 The Chromium OS Authors. All rights reserved.
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
	"chromiumos/tast/remote/wificell/pcap"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:        ProbeReqFormat,
		Desc:        "Verifies that the DUT does not send out malformed probe requests",
		Contacts:    []string{"yenlinlai@google.com", "chromeos-platform-connectivity@google.com"},
		Attr:        []string{"group:wificell", "wificell_func", "wificell_unstable"},
		ServiceDeps: []string{wificell.TFServiceName},
		Pre:         wificell.TestFixturePreWithCapture(),
		Vars:        []string{"router", "pcap"},
	})
}

func ProbeReqFormat(ctx context.Context, s *testing.State) {
	// Trigger active scans and verify the format of probe requests in pcap.
	// In b/169117094#c3, we found that on some DUTs, the probe requests
	// captured in pcap have good checksum but malformed body. It is still
	// not clear if this is a problem on the DUT side or pcap side.
	tf := s.PreValue().(*wificell.TestFixture)
	defer func(ctx context.Context) {
		if err := tf.CollectLogs(ctx); err != nil {
			s.Log("Error collecting logs, err: ", err)
		}
	}(ctx)
	ctx, cancel := tf.ReserveForCollectLogs(ctx)
	defer cancel()

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

	// Collect probe requests on channel 1.
	pcapPath, err := wifiutil.ScanAndCollectPcap(ctx, tf, "malfromed_probe", 10, 1)
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
	if len(packets) == 0 {
		s.Fatal("No probe request found")
	}

	for _, p := range packets {
		layer := p.Layer(layers.LayerTypeDot11MgmtProbeReq)
		if layer == nil {
			s.Fatal("Found packet without ProbeReq layer")
		}
		// Parse the frame body into IEs.
		content := layer.LayerContents()
		e := gopacket.NewPacket(content, layers.LayerTypeDot11InformationElement, gopacket.NoCopy)
		if err := e.ErrorLayer(); err != nil {
			s.Errorf("Malformed probe request %v: %v", layer, err.Error())
		}
	}
}

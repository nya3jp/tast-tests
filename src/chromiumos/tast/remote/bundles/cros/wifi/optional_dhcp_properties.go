// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wifi

import (
	"bytes"
	"context"
	"time"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/remote/bundles/cros/wifi/wifiutil"
	"chromiumos/tast/remote/network/ip"
	"chromiumos/tast/remote/wificell"
	"chromiumos/tast/remote/wificell/pcap"
	"chromiumos/tast/services/cros/network"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:        OptionalDHCPProperties,
		Desc:        "Verifies that optional DHCP properties set on the DUT are used as parameters in DHCP requests",
		Contacts:    []string{"yenlinlai@google.com", "chromeos-platform-connectivity@google.com"},
		Attr:        []string{"group:wificell", "wificell_func", "wificell_unstable"},
		ServiceDeps: []string{wificell.TFServiceName},
		Pre:         wificell.TestFixturePreWithCapture(),
		Vars:        []string{"router", "pcap"},
	})
}

func OptionalDHCPProperties(ctx context.Context, s *testing.State) {
	tf := s.PreValue().(*wificell.TestFixture)
	defer func(ctx context.Context) {
		if err := tf.CollectLogs(ctx); err != nil {
			s.Log("Error collecting logs, err: ", err)
		}
	}(ctx)
	ctx, cancel := tf.ReserveForCollectLogs(ctx)
	defer cancel()

	const vendorClass = "testVendorClass"
	const hostname = "testHostname"

	req := &network.SetDHCPPropertiesRequest{
		Props: &network.DHCPProperties{
			Hostname:    hostname,
			VendorClass: vendorClass,
		},
	}
	resp, err := tf.WifiClient().SetDHCPProperties(ctx, req)
	if err != nil {
		s.Fatal("Failed to set DHCP properties: ", err)
	}
	defer func(ctx context.Context, p *network.DHCPProperties) {
		req := &network.SetDHCPPropertiesRequest{Props: p}
		if _, err := tf.WifiClient().SetDHCPProperties(ctx, req); err != nil {
			s.Error("Failed to restore DHCP properties: ", err)
		}
	}(ctx, resp.Props)
	ctx, cancel = ctxutil.Shorten(ctx, 2*time.Second)
	defer cancel()

	// Get the MAC address of WiFi interface.
	iface, err := tf.ClientInterface(ctx)
	if err != nil {
		s.Fatal("Failed to get WiFi interface of DUT")
	}

	ipr := ip.NewRemoteRunner(s.DUT().Conn())
	mac, err := ipr.MAC(ctx, iface)
	if err != nil {
		s.Fatal("Failed to get MAC of the WiFi interface")
	}

	apOps := tf.DefaultOpenNetworkAPOptions()
	pcapPath, _, err := wifiutil.ConnectAndCollectPcap(ctx, tf, "pcap", apOps)
	if err != nil {
		s.Fatal("Failed to collect packet: ", err)
	}

	s.Log("Start analyzing pcap")
	// Filter the DHCP packets from DUT.
	filters := []pcap.Filter{
		pcap.TypeFilter(
			layers.LayerTypeDot11,
			func(layer gopacket.Layer) bool {
				dot11 := layer.(*layers.Dot11)
				// Filter sender == MAC of DUT.
				return bytes.Equal(dot11.Address2, mac)
			},
		),
		pcap.TypeFilter(layers.LayerTypeDHCPv4, nil),
	}
	packets, err := pcap.ReadPackets(pcapPath, filters...)
	if err != nil {
		s.Fatal("Failed to read packets: ", err)
	}

	// Go through the DHCP packets and check.
	// Notice: The packet/layer string is usually quite long. Given that
	// pcap is already saved in OutDir, avoid printing the full packet
	// in the checking below.
	dhcpReqCount := 0
packetLoop:
	for _, p := range packets {
		layer := p.Layer(layers.LayerTypeDHCPv4)
		if layer == nil {
			s.Fatal("Non DHCPv4 packet passed type filter")
		}
		dhcp := layer.(*layers.DHCPv4)
		// Map for the options we're interested.
		optMap := map[layers.DHCPOpt]*layers.DHCPOption{
			layers.DHCPOptMessageType: nil,
			layers.DHCPOptClassID:     nil,
			layers.DHCPOptHostname:    nil,
		}
		for i, opt := range dhcp.Options {
			if prev, ok := optMap[opt.Type]; !ok {
				// Not interested.
				continue
			} else if prev != nil {
				// In https://tools.ietf.org/html/rfc2131#section-4.1, it says:
				// "Options may appear only once, unless otherwise specified in the
				// options document. The client concatenates the values of multiple
				// instances of the same option into a single parameter list for
				// configuration."
				// However, the options we're interested in are not expected to be
				// a parameter list. Let's be strict here.
				s.Errorf("Malformed DHCP packet with duplicate %v option", opt.Type)
				continue packetLoop
			}
			optMap[opt.Type] = &dhcp.Options[i]
		}
		// Filter DHCP Request.
		if opt := optMap[layers.DHCPOptMessageType]; opt == nil {
			// Message type option must be included in every DHCP message.
			// See: https://tools.ietf.org/html/rfc2131#section-3
			s.Error("Malformed DHCP packet without message type")
			continue
		} else if len(opt.Data) != 1 {
			// Data size should be 1.
			// See: https://tools.ietf.org/html/rfc1533#section-9.4
			s.Errorf("Malformed DHCP packet with message type data length = %d, want 1", len(opt.Data))
			continue
		} else if layers.DHCPMsgType(opt.Data[0]) != layers.DHCPMsgTypeRequest {
			continue
		}

		dhcpReqCount++
		// Check vendor class ID.
		if opt := optMap[layers.DHCPOptClassID]; opt == nil {
			s.Error("Found DHCP Request without vendor class option")
		} else if vc := string(opt.Data); vc != vendorClass {
			s.Errorf("Unexpected vendor class; got %q, want %q", vc, vendorClass)
		}
		// Check hostname.
		if opt := optMap[layers.DHCPOptHostname]; opt == nil {
			s.Error("Found DHCP Request without hostname option")
		} else if name := string(opt.Data); name != hostname {
			s.Errorf("Unexpected hostname; got %q, want %q", name, hostname)
		}
	}
	if dhcpReqCount != 1 {
		s.Fatalf("Found %d DHCP Request(s), expect 1", dhcpReqCount)
	}
}

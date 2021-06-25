// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wifi

import (
	"context"
	"time"

	"github.com/google/gopacket/layers"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/remote/wificell"
	"chromiumos/tast/remote/wificell/pcap"
	"chromiumos/tast/services/cros/wifi"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: OptionalDHCPProperties,
		Desc: "Verifies that optional DHCP properties set on the DUT are used as parameters in DHCP requests",
		Contacts: []string{
			"chromeos-wifi-champs@google.com", // WiFi oncall rotation; or http://b/new?component=893827
		},
		Attr:        []string{"group:wificell", "wificell_func", "wificell_cq"},
		ServiceDeps: []string{wificell.TFServiceName},
		Fixture:     "wificellFixt",
	})
}

func OptionalDHCPProperties(ctx context.Context, s *testing.State) {
	tf := s.FixtValue().(*wificell.TestFixture)

	legacyRouter, err := tf.LegacyRouter()
	if err != nil {
		s.Fatal("Failed to get legacy router: ", err)
	}

	const vendorClass = "testVendorClass"
	const hostname = "testHostname"

	req := &wifi.SetDHCPPropertiesRequest{
		Props: &wifi.DHCPProperties{
			Hostname:    hostname,
			VendorClass: vendorClass,
		},
	}
	resp, err := tf.WifiClient().SetDHCPProperties(ctx, req)
	if err != nil {
		s.Fatal("Failed to set DHCP properties: ", err)
	}
	defer func(ctx context.Context, p *wifi.DHCPProperties) {
		req := &wifi.SetDHCPPropertiesRequest{Props: p}
		if _, err := tf.WifiClient().SetDHCPProperties(ctx, req); err != nil {
			s.Error("Failed to restore DHCP properties: ", err)
		}
	}(ctx, resp.Props)
	ctx, cancel := ctxutil.Shorten(ctx, 2*time.Second)
	defer cancel()

	// Connect and capture L3 packets on the interface of AP.
	// As we'll collect pcap file after Capturer closed, run it in
	// an inner function so that we can clean up easier with defer.
	capturer, err := func(ctx context.Context) (ret *pcap.Capturer, retErr error) {
		collectFirstErr := func(err error) {
			if retErr == nil {
				ret = nil
				retErr = err
			}
			testing.ContextLog(ctx, "Error when connect and collect packets: ", err)
		}

		testing.ContextLog(ctx, "Configuring WiFi to connect")
		ap, err := tf.DefaultOpenNetworkAP(ctx)
		if err != nil {
			return nil, errors.Wrap(err, "failed to configure AP")
		}
		defer func(ctx context.Context) {
			if err := tf.DeconfigAP(ctx, ap); err != nil {
				collectFirstErr(errors.Wrap(err, "failed to deconfig AP"))
			}
		}(ctx)
		ctx, cancel := tf.ReserveForDeconfigAP(ctx, ap)
		defer cancel()

		capturer, err := legacyRouter.StartRawCapturer(ctx, "dhcp", ap.Interface())
		if err != nil {
			return nil, errors.Wrap(err, "failed to start capturer")
		}
		defer func(ctx context.Context) {
			if err := legacyRouter.StopRawCapturer(ctx, capturer); err != nil {
				collectFirstErr(errors.Wrap(err, "failed to close capturer"))
			}
		}(ctx)
		ctx, cancel = legacyRouter.ReserveForStopRawCapturer(ctx, capturer)
		defer cancel()

		testing.ContextLog(ctx, "Connecting to WiFi")
		if _, err := tf.ConnectWifiAP(ctx, ap); err != nil {
			return nil, err
		}
		defer func(ctx context.Context) {
			if err := tf.CleanDisconnectWifi(ctx); err != nil {
				collectFirstErr(errors.Wrap(err, "failed to disconnect"))
			}
		}(ctx)
		// We're done after get connected, start tearing down.
		return capturer, nil
	}(ctx)
	if err != nil {
		s.Fatal("Failed to connect and collect packets: ", err)
	}
	pcapPath, err := capturer.PacketPath(ctx)
	if err != nil {
		s.Fatal("Failed to get path to packets")
	}

	s.Log("Start analyzing pcap")
	// Filter the DHCP packets from DUT.
	filters := []pcap.Filter{
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
	s.Logf("Found %d DHCP Requests", dhcpReqCount)
	if dhcpReqCount == 0 {
		s.Fatal("No DHCP Request found")
	}
}

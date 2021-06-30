// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wifi

import (
	"bytes"
	"context"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"

	"chromiumos/tast/errors"
	"chromiumos/tast/remote/bundles/cros/wifi/wifiutil"
	"chromiumos/tast/remote/network/ip"
	"chromiumos/tast/remote/wificell"
	"chromiumos/tast/remote/wificell/hostapd"
	"chromiumos/tast/remote/wificell/pcap"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: ConnectMBO,
		Desc: "Verifies that the MBO IE and other MBO-related capability bits are set",
		Contacts: []string{
			"matthewmwang@google.com",
			"chromeos-wifi-champs@google.com", // WiFi oncall rotation; or http://b/new?component=893827
		},
		Attr:         []string{"group:wificell", "wificell_func", "wificell_unstable"},
		ServiceDeps:  []string{wificell.TFServiceName},
		Pre:          wificell.TestFixturePreWithFeatures(wificell.TFFeaturesCapture | wificell.TFFeaturesRouterAsCapture),
		Vars:         []string{"router", "pcap"},
		SoftwareDeps: []string{"mbo", "rrm_support"},
	})
}

func ConnectMBO(ctx context.Context, s *testing.State) {
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

	apOps := []hostapd.Option{
		hostapd.MBO(),
		hostapd.RRMBeaconReport(),
		hostapd.Channel(1),
		hostapd.Mode(hostapd.Mode80211acMixed),
		hostapd.HTCaps(hostapd.HTCapHT40),
		hostapd.VHTChWidth(hostapd.VHTChWidth20Or40),
	}
	pcapPath, _, err := wifiutil.ConnectAndCollectPcap(ctx, tf, apOps)
	if err != nil {
		s.Fatal("Failed to collect packet: ", err)
	}

	s.Log("Start analyzing pcap")
	filters := []pcap.Filter{
		pcap.Dot11FCSValid(),
		pcap.TransmitterAddress(mac),
	}
	probePackets, err := pcap.ReadPackets(pcapPath, append(filters, pcap.TypeFilter(layers.LayerTypeDot11MgmtProbeReq, nil))...)
	if err != nil {
		s.Fatal("Failed to read probe request packets: ", err)
	}
	s.Logf("Total %d probe requests found", len(probePackets))
	assocPackets, err := pcap.ReadPackets(pcapPath, append(filters, pcap.TypeFilter(layers.LayerTypeDot11MgmtAssociationReq, nil))...)
	if err != nil {
		s.Fatal("Failed to read association request packets: ", err)
	}
	s.Logf("Total %d assoc requests found", len(assocPackets))

	checkIEs := func(p gopacket.Packet, isProbe bool) error {
		containsExt := false
		containsMBO := false
		containsRM := false
		for _, l := range p.Layers() {
			element, ok := l.(*layers.Dot11InformationElement)
			if !ok {
				continue
			}
			if element.ID == layers.Dot11InformationElementIDExtCapability {
				containsExt = true
				if int(element.Length) < 3 {
					return errors.New("Extended Capability IE not long enough")
				}
				if (element.Info[2] & 0x08) == 0 {
					return errors.New("Extended Capability IE does not contain BSS Transition capability")
				}
			}
			if element.ID == layers.Dot11InformationElementIDVendor {
				if int(element.Length) < 7 ||
					bytes.Compare(element.OUI[:3], []byte{0x50, 0x6F, 0x9A}) != 0 ||
					element.OUI[3] != 0x16 {
					continue
				}
				for i := 0; i < len(element.Info); {
					attrID := element.Info[i]
					attrLen := element.Info[i+1]
					// Check that the Cellular Data Capabilities attribute is present
					if attrID == 0x03 && attrLen == 1 && element.Info[i+2] >= 0x01 && element.Info[i+2] <= 0x03 {
						containsMBO = true
						break
					}
					i += 2 + int(attrLen)
				}
			}
			if element.ID == layers.Dot11InformationElementIDRMEnabledCapabilities {
				containsRM = true
				if int(element.Length) < 1 {
					return errors.New("RM Enabled Capabilities IE not long enough")
				}
				if (element.Info[0] & 0x10) == 0 {
					return errors.New("RM Enabled Capabilities IE missing Passive Measurement support")
				}
				if (element.Info[0] & 0x20) == 0 {
					return errors.New("RM Enabled Capabilities IE missing Active Measurement support")
				}
				if (element.Info[0] & 0x40) == 0 {
					return errors.New("RM Enabled Capabilities IE missing Table Measurement support")
				}
			}
		}
		if !containsExt {
			return errors.New("Extended Capabilities IE missing")
		} else if !containsMBO {
			return errors.New("MBO-OCE IE missing")
		} else if !isProbe && !containsRM {
			return errors.New("RM Enabled Capabilities IE missing")
		}
		return nil
	}
	s.Log("Checking probe request packets")
	for _, p := range probePackets {
		layer := p.Layer(layers.LayerTypeDot11MgmtProbeReq)
		if layer == nil {
			s.Fatal("Found packet without ProbeReq layer")
		}
		req := layer.(*layers.Dot11MgmtProbeReq)
		content := req.LayerContents()
		e := gopacket.NewPacket(content, layers.LayerTypeDot11InformationElement, gopacket.NoCopy)
		if err := e.ErrorLayer(); err != nil {
			s.Log("Error: ", err)
			continue
		}
		if err := checkIEs(e, true); err != nil {
			s.Fatal("Probe request IEs missing: ", err)
		}
	}
	s.Log("Checking assoc request packets")
	for _, p := range assocPackets {
		if err := checkIEs(p, false); err != nil {
			s.Fatal("Assoc request IEs missing: ", err)
		}
	}
}

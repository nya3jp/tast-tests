// Copyright 2021 The Chromium OS Authors. All rights reserved.
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

	"chromiumos/tast/common/network/wpacli"
	"chromiumos/tast/errors"
	"chromiumos/tast/remote/bundles/cros/wifi/wifiutil"
	"chromiumos/tast/remote/network/cmd"
	"chromiumos/tast/remote/network/ip"
	"chromiumos/tast/remote/wificell"
	"chromiumos/tast/remote/wificell/hostapd"
	"chromiumos/tast/remote/wificell/pcap"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: NonPrefChan,
		Desc: "Verifies that the MBO-OCE IEs set non preferred channel reports as expected",
		Contacts: []string{
			"matthewmwang@google.com",
			"chromeos-wifi-champs@google.com", // WiFi oncall rotation; or http://b/new?component=893827
		},
		Attr:         []string{"group:wificell", "wificell_func", "wificell_unstable"},
		ServiceDeps:  []string{wificell.TFServiceName},
		Pre:          wificell.TestFixturePre(),
		Vars:         []string{"router", "pcap"},
		SoftwareDeps: []string{"mbo"},
	})
}

var wfaOUI = []byte{0x50, 0x6F, 0x9A}

func NonPrefChan(ctx context.Context, s *testing.State) {
	/*
	  In this test, we verify that a DUT can set non-preferred channels
	  properly. We test three things:

	  1. The association request contains the Supported Operating Class IE,
	     which should indicate that the DUT supports both 2.4GHz and 5GHz
	     channels.
	  2. The association request contains an MBO-OCE IE with the
	     non-preferred channels we have preset.
	  3. Setting non-preferred channels on the DUT after association
	     triggers a WNM notification to be sent to the AP containing the
	     updated non-preferred channels.

	  Note that we don't expect a certain behavior from the DUT or the AP.
	  The AP can use the non-preferred channel information at its
	  discretion.
	*/
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

	wpa := wpacli.NewRunner(&cmd.RemoteCmdRunner{Host: s.DUT().Conn()})
	nonPrefChans := []wpacli.NonPrefChan{{
		OpClass: 0x73,
		Channel: 0x30,
		Pref:    0x00,
		Reason:  0x00,
	}, {
		OpClass: 0x73,
		Channel: 0x2C,
		Pref:    0x01,
		Reason:  0x00,
	}}
	setNonPrefChans := func(chans ...wpacli.NonPrefChan) func(context.Context) error {
		return func(ctx context.Context) error {
			nonPrefChanStr := wpacli.SerializeNonPrefChans(chans...)
			return wpa.Set(ctx, wpacli.PropertyNonPrefChan, nonPrefChanStr)
		}
	}
	if err := setNonPrefChans(nonPrefChans...)(ctx); err != nil {
		s.Fatal("Failed to set non-preferred channels: ", err)
	}

	s.Log("Configuring AP")
	channel := 36
	testSSID := hostapd.RandomSSID("NON_PREF_CHAN_")
	apOps := []hostapd.Option{
		hostapd.SSID(testSSID),
		hostapd.MBO(),
		hostapd.Channel(channel),
		hostapd.Mode(hostapd.Mode80211acMixed),
		hostapd.HTCaps(hostapd.HTCapHT40),
		hostapd.VHTChWidth(hostapd.VHTChWidth20Or40),
	}
	ap, err := tf.ConfigureAP(ctx, apOps, nil)
	if err != nil {
		s.Fatal("Failed to configure AP: ", err)
	}
	defer func(ctx context.Context) {
		if err := tf.DeconfigAP(ctx, ap); err != nil {
			s.Error("Failed to deconfig AP: ", err)
		}
	}(ctx)
	ctx, cancel = tf.ReserveForDeconfigAP(ctx, ap)
	defer cancel()

	s.Log("Attempting to connect to AP")
	cleanupCtx := ctx
	ctx, cancel = tf.ReserveForDisconnect(ctx)
	defer cancel()
	connect := func(ctx context.Context) error {
		if _, err := tf.ConnectWifiAP(ctx, ap); err != nil {
			return err
		}
		return nil
	}
	pcapPath, connectSuccessful, err := wifiutil.CollectPcapForAction(ctx, tf.Router(), "connect", channel, connect)
	if connectSuccessful {
		defer func(ctx context.Context) {
			if err := tf.CleanDisconnectWifi(ctx); err != nil {
				s.Error("Failed to disconnect WiFi: ", err)
			}
		}(cleanupCtx)
	}
	if err != nil {
		s.Fatal("Failed to collect packet: ", err)
	}

	s.Log("Start analyzing assoc requests")
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
	}
	assocPackets, err := pcap.ReadPackets(pcapPath, append(filters, pcap.TypeFilter(layers.LayerTypeDot11MgmtAssociationReq, nil))...)
	if err != nil {
		s.Fatal("Failed to read association request packets: ", err)
	}
	if len(assocPackets) == 0 {
		s.Fatal("No association request packets found")
	}
	s.Logf("Total %d assoc requests found", len(assocPackets))

	checkIEs := func(p gopacket.Packet, chans ...wpacli.NonPrefChan) error {
		containsSuppOp := false
		containsMBO := false
		for _, l := range p.Layers() {
			element, ok := l.(*layers.Dot11InformationElement)
			if !ok {
				continue
			}
			if element.ID == layers.Dot11InformationElementIDSuppOperatingClass {
				containsSuppOp = true
				supports2GHz := false
				supports5GHz := false
				for i := 1; i < int(element.Length); i++ {
					if element.Info[i] == 0x51 {
						supports2GHz = true
					} else if element.Info[i] == 0x73 {
						supports5GHz = true
					}
				}
				if !supports2GHz {
					return errors.New("Device does not indicate 2.4GHz support")
				}
				if !supports5GHz {
					return errors.New("Device does not indicate 5GHz support")
				}
			}
			if element.ID == layers.Dot11InformationElementIDVendor {
				if int(element.Length) < 19 ||
					!bytes.Equal(element.OUI[:3], wfaOUI) ||
					element.OUI[3] != 0x16 {
					continue
				}
				containsMBO = true
				expectedChanMap := make(map[uint8]wpacli.NonPrefChan)
				for _, ch := range chans {
					expectedChanMap[ch.Channel] = ch
				}
				actualChanMap := make(map[uint8]wpacli.NonPrefChan)
				for i := 0; i < len(element.Info); {
					attrID := element.Info[i]
					attrLen := element.Info[i+1]
					i += 2
					// Check for a well-formatted Channel Report subelement
					if attrID == 0x02 && attrLen == 4 {
						actualChanMap[element.Info[i+1]] = wpacli.NonPrefChan{
							OpClass: element.Info[i],
							Channel: element.Info[i+1],
							Pref:    element.Info[i+2],
							Reason:  element.Info[i+3],
						}
					}
					i += int(attrLen)
				}
				if !reflect.DeepEqual(expectedChanMap, actualChanMap) {
					return errors.New("Non-preferred channel report does not match expected report")
				}
			}
		}
		if !containsSuppOp {
			return errors.New("Supported Operating Classes IE missing")
		} else if !containsMBO {
			return errors.New("MBO-OCE IE missing")
		}
		return nil
	}
	s.Log("Checking assoc request packets")
	for _, p := range assocPackets {
		if err := checkIEs(p, nonPrefChans...); err != nil {
			s.Fatal("Assoc request IEs missing: ", err)
		}
	}

	for tc, chans := range [][]wpacli.NonPrefChan{
		{
			// Test that both channels are present in the report
			{
				OpClass: 0x73,
				Channel: 0x28,
				Pref:    0x01,
				Reason:  0x00,
			}, {
				OpClass: 0x73,
				Channel: 0x2C,
				Pref:    0x01,
				Reason:  0x00,
			},
		}, {
			// Test that no channels are present in the report
		},
	} {
		s.Log("Running test case: ", tc)
		pcapPath, _, err = wifiutil.CollectPcapForAction(ctx, tf.Router(), fmt.Sprintf("setNonPrefChans%d", tc), channel, setNonPrefChans(chans...))
		if err != nil {
			s.Fatal("Failed to reset non-preferred channels: ", err)
		}
		actionPackets, err := pcap.ReadPackets(pcapPath, append(filters, pcap.TypeFilter(layers.LayerTypeDot11MgmtAction, nil))...)
		if err != nil {
			s.Fatal("Failed to read action packets: ", err)
		}
		expectedChanMap := make(map[uint8]wpacli.NonPrefChan)
		for _, ch := range chans {
			expectedChanMap[ch.Channel] = ch
		}
		foundWNM := false
		for _, p := range actionPackets {
			layer := p.Layer(layers.LayerTypeDot11MgmtAction)
			if layer == nil {
				s.Fatal("Found packet without Action layer")
			}
			action := layer.(*layers.Dot11MgmtAction)
			if action.Contents[0] != 10 { // WNM
				continue
			}
			foundWNM = true
			actualNonPrefChans := make(map[uint8]wpacli.NonPrefChan)
			for i := 4; i < len(action.Contents); {
				tagNum := action.Contents[i]
				tagLen := int(action.Contents[i+1])
				i += 2
				// Check for the vendor-specific tag number, the WFA OUI, and the correct OUI type
				if tagNum != 0xdd || !bytes.Equal(action.Contents[i:i+3], wfaOUI) || action.Contents[i+3] != 0x02 {
					s.Fatal("Unexpected action packet contents")
				}
				if tagLen < 8 {
					// No channels found in this report
					i += tagLen
					continue
				}
				opClass := action.Contents[i+4]
				pref := action.Contents[i+tagLen-2]   // second to last byte
				reason := action.Contents[i+tagLen-1] // last byte
				// There are 7 fixed bytes in the tag. All additional
				// bytes are taken up by a list of channels. Iterate
				// through this list and insert the channels into a map.
				for j := 0; j < tagLen-7; j++ {
					ch := action.Contents[i+5+j]
					if _, chanExists := actualNonPrefChans[ch]; chanExists {
						s.Fatalf("Malformed non-preferred channel report. Channel %d reported multiple times", ch)
					}
					actualNonPrefChans[ch] = wpacli.NonPrefChan{
						OpClass: opClass,
						Channel: ch,
						Pref:    pref,
						Reason:  reason,
					}
				}
				i += tagLen
			}
			if !reflect.DeepEqual(expectedChanMap, actualNonPrefChans) {
				s.Fatal("WNM Notification does not contain expected non-preferred channel report")
			}
		}
		if !foundWNM {
			s.Fatal("No WNM Notifications found in packet capture")
		}
	}
}

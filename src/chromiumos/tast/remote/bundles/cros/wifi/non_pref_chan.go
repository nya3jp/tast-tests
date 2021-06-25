// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wifi

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"reflect"
	"time"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"

	"chromiumos/tast/common/network/wpacli"
	"chromiumos/tast/common/wifi/ieee80211"
	"chromiumos/tast/ctxutil"
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
		Fixture:      "wificellFixt",
		SoftwareDeps: []string{"mbo"},
	})
}

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
	const (
		OUITypeMBO               = 0x16
		OUITypeNonPrefChanReport = 0x02
		ChanReportSubelem        = 0x02
		WNMCategoryCode          = 0x0A
		TagNumVendor             = 0xDD
	)
	type actionHeader struct {
		Category uint8
		_        [3]byte
	}
	type elemHeader struct {
		ID  uint8
		Len uint8
	}
	type nonPrefChanPreData struct {
		OUI     [3]uint8
		OUIType uint8
		OpClass uint8
	}
	type nonPrefChanPostData struct {
		Pref   uint8
		Reason uint8
	}
	var (
		nonPrefChanMinTagSz  = binary.Size(nonPrefChanPreData{}) + binary.Size(nonPrefChanPostData{})
		nonPrefChanSubelemSz = binary.Size(wpacli.NonPrefChan{})
	)

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

	wpa := wpacli.NewRunner(&cmd.RemoteCmdRunner{Host: s.DUT().Conn()})
	nonPrefChans := []wpacli.NonPrefChan{{
		OpClass: ieee80211.OpClass5GHz,
		Channel: 0x30,
		Pref:    0x00,
		Reason:  0x00,
	}, {
		OpClass: ieee80211.OpClass5GHz,
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
	defer func(ctx context.Context) {
		if err := setNonPrefChans()(ctx); err != nil {
			s.Error("Failed to reset non-preferred channels: ", err)
		}
	}(ctx)
	ctx, cancel := ctxutil.Shorten(ctx, time.Second)
	defer cancel()

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

	freqOpts, err := ap.Config().PcapFreqOptions()
	if err != nil {
		s.Fatal("Failed to get pcap frequency options: ", err)
	}

	s.Log("Attempting to connect to AP")
	cleanupCtx := ctx
	ctx, cancel = tf.ReserveForDisconnect(ctx)
	defer cancel()
	connectSuccessful := false
	connect := func(ctx context.Context) error {
		if _, err := tf.ConnectWifiAP(ctx, ap); err != nil {
			return err
		}
		connectSuccessful = true
		return nil
	}
	legacyRouter, err := tf.LegacyRouter()
	if err != nil {
		s.Fatal("Unable to get legacy router: ", err)
	}
	pcapPath, err := wifiutil.CollectPcapForAction(ctx, legacyRouter, "connect", channel, freqOpts, connect)
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
					if element.Info[i] == ieee80211.OpClass2GHz {
						supports2GHz = true
					} else if element.Info[i] == ieee80211.OpClass5GHz {
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
				if !bytes.Equal(element.OUI, append(ieee80211.WFAOUI, OUITypeMBO)) {
					continue
				}
				containsMBO = true
				expectedChanMap := make(map[uint8]wpacli.NonPrefChan)
				for _, ch := range chans {
					expectedChanMap[ch.Channel] = ch
				}
				actualChanMap := make(map[uint8]wpacli.NonPrefChan)
				r := bytes.NewReader(element.Info)
				for r.Len() > 0 {
					var header elemHeader
					if binary.Read(r, binary.LittleEndian, &header); err != nil {
						s.Fatal("Unable to read subelement header: ", err)
					}
					// Check for a well-formatted Channel Report subelement
					var ch wpacli.NonPrefChan
					if header.ID == ChanReportSubelem && int(header.Len) == nonPrefChanSubelemSz {
						if err := binary.Read(r, binary.LittleEndian, &ch); err != nil {
							s.Fatal("Unable to read non pref chan: ", err)
						}
						actualChanMap[ch.Channel] = ch
					} else if header.Len > 0 {
						if _, err := r.Seek(int64(header.Len), io.SeekCurrent); err != nil {
							s.Fatal("Unable to seek: ", err)
						}
					}
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
				OpClass: ieee80211.OpClass5GHz,
				Channel: 0x28,
				Pref:    0x01,
				Reason:  0x00,
			}, {
				OpClass: ieee80211.OpClass5GHz,
				Channel: 0x2C,
				Pref:    0x01,
				Reason:  0x00,
			},
		}, {
			// Test that no channels are present in the report
		},
	} {
		s.Log("Running test case: ", tc)
		pcapPath, err = wifiutil.CollectPcapForAction(ctx, legacyRouter, fmt.Sprintf("setNonPrefChans%d", tc), channel, freqOpts, setNonPrefChans(chans...))
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
			var header actionHeader
			r := bytes.NewReader(action.Contents)
			if err := binary.Read(r, binary.LittleEndian, &header); err != nil {
				s.Fatal("Unable to read packet header: ", err)
			}
			if header.Category != WNMCategoryCode {
				continue
			}
			foundWNM = true
			actualNonPrefChans := make(map[uint8]wpacli.NonPrefChan)
			for r.Len() > 0 {
				var tagHeader elemHeader
				var tagPreData nonPrefChanPreData
				var tagPostData nonPrefChanPostData
				if err := binary.Read(r, binary.LittleEndian, &tagHeader); err != nil {
					s.Fatal("Unable to read tag header: ", err)
				}
				if int(tagHeader.Len) <= nonPrefChanMinTagSz && tagHeader.Len > 0 {
					// No channels found in this report
					if _, err := r.Seek(int64(tagHeader.Len), io.SeekCurrent); err != nil {
						s.Fatal("Unable to seek: ", err)
					}
					continue
				}
				if err := binary.Read(r, binary.LittleEndian, &tagPreData); err != nil {
					s.Fatal("Unable to read tag pre-data: ", err)
				}
				// Check for the vendor-specific tag number, the WFA OUI, and the correct OUI type
				if tagHeader.ID != TagNumVendor || !bytes.Equal(tagPreData.OUI[:], ieee80211.WFAOUI) || tagPreData.OUIType != OUITypeNonPrefChanReport {
					s.Fatal("Unexpected action packet contents")
				}
				chans := make([]byte, int(tagHeader.Len)-nonPrefChanMinTagSz)
				if _, err := io.ReadFull(r, chans); err != nil {
					s.Fatal("Unable to read non pref chans: ", err)
				}
				if err := binary.Read(r, binary.LittleEndian, &tagPostData); err != nil {
					s.Fatal("Unable to read tag post-data: ", err)
				}
				// There are 7 fixed bytes in the tag. All additional
				// bytes are taken up by a list of channels. Iterate
				// through this list and insert the channels into a map.
				for _, ch := range chans {
					if _, chanExists := actualNonPrefChans[ch]; chanExists {
						s.Fatalf("Malformed non-preferred channel report. Channel %d reported multiple times", ch)
					}
					actualNonPrefChans[ch] = wpacli.NonPrefChan{
						OpClass: tagPreData.OpClass,
						Channel: ch,
						Pref:    tagPostData.Pref,
						Reason:  tagPostData.Reason,
					}
				}
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

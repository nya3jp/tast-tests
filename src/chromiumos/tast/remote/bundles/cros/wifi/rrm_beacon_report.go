// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wifi

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"io"
	"net"
	"strconv"
	"time"

	"github.com/golang/protobuf/ptypes/empty"
	"github.com/google/gopacket/layers"

	"chromiumos/tast/common/wifi/ieee80211"
	"chromiumos/tast/common/wifi/security/wpa"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/remote/bundles/cros/wifi/wifiutil"
	"chromiumos/tast/remote/wificell"
	"chromiumos/tast/remote/wificell/hostapd"
	"chromiumos/tast/remote/wificell/pcap"
	"chromiumos/tast/services/cros/wifi"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: RRMBeaconReport,
		Desc: "Verifies that the DUT responds properly to beacon report requests",
		Contacts: []string{
			"matthewmwang@google.com",
			"chromeos-wifi-champs@google.com", // WiFi oncall rotation; or http://b/new?component=893827
		},
		Attr:         []string{"group:wificell", "wificell_func", "wificell_unstable"},
		ServiceDeps:  []string{wificell.TFServiceName},
		Fixture:      "wificellFixtRouters",
		SoftwareDeps: []string{"rrm_support"},
	})
}

func RRMBeaconReport(ctx context.Context, s *testing.State) {
	/*
	  In this test, we verify that a DUT responds properly to beacon report
	  requests from an AP. Beacon requests are part of the 802.11k Radio
	  Resource Management (RRM) standard and are a way for the AP to
	  ascertain information about the DUT's environment. In particular, a
	  beacon request will ask a DUT about APs that it sees on specified
	  SSIDs and/or channels and a DUT will either scan or use cached scan
	  results to fulfill that request.

	  This test sets up 3 distinct SSIDs on the 2.4GHz phy of the router AP,
	  3 duplicate SSIDs on the 2.4GHz phy of the pcap AP (i.e. there are 3
	  separate networks, and each network has 2 APs). It then connects to
	  the first AP on the first SSID and sends a series of beacon requests.
	  We collect a pcap for each of the responses and compare it to the
	  expected response.
	*/
	type reportBSS struct {
		BSSID   net.HardwareAddr
		Channel uint8
		IEs     []layers.Dot11InformationElementID
	}
	type actionHeader struct {
		Category uint8
		_        [2]uint8
	}
	type beaconRepHeader struct {
		_   uint8
		Len uint8
	}
	type beaconRepInfo struct {
		_       [4]uint8
		Channel uint8
		_       [13]uint8
		BSSID   [6]uint8
		_       [5]uint8
	}
	type elemHeader struct {
		ID  uint8
		Len uint8
	}
	const (
		RRMCategoryCode       = 0x05
		extraFirstReportBytes = 12
	)
	var (
		bcastAddr   = []byte{0xff, 0xff, 0xff, 0xff, 0xff, 0xff}
		testIEsNoMD = []layers.Dot11InformationElementID{
			layers.Dot11InformationElementIDSSID,
			layers.Dot11InformationElementIDVendor,
		}
		testIEs         = append(testIEsNoMD, layers.Dot11InformationElementIDMobilityDomain)
		beaconRepInfoSz = binary.Size(beaconRepInfo{})
	)
	tf := s.FixtValue().(*wificell.TestFixture)
	ctx, restoreBg, err := tf.WifiClient().TurnOffBgscan(ctx)
	if err != nil {
		s.Fatal("Failed to turn off the background scan: ", err)
	}
	defer func() {
		if err := restoreBg(); err != nil {
			s.Error("Failed to restore the background scan config: ", err)
		}
	}()

	// The MBO certification test plan specified that we set up the AP with
	// FT. Note that we don't actually test the FT feature here, other than
	// the initial connection.
	ftResp, err := tf.WifiClient().GetGlobalFTProperty(ctx, &empty.Empty{})
	if err != nil {
		s.Fatal("Failed to get the global FT property: ", err)
	}
	defer func(ctx context.Context) {
		if _, err := tf.WifiClient().SetGlobalFTProperty(ctx, &wifi.SetGlobalFTPropertyRequest{Enabled: ftResp.Enabled}); err != nil {
			s.Errorf("Failed to set global FT property back to %v: %v", ftResp.Enabled, err)
		}
	}(ctx)
	ctx, cancel := ctxutil.Shorten(ctx, time.Second)
	defer cancel()

	if _, err := tf.WifiClient().SetGlobalFTProperty(ctx, &wifi.SetGlobalFTPropertyRequest{Enabled: true}); err != nil {
		s.Fatal("Failed to turn on the global FT property: ", err)
	}

	// Generate 6 different MACs for the 6 different APs
	var macs [6]net.HardwareAddr
	for i := 0; i < len(macs); i++ {
		mac, err := hostapd.RandomMAC()
		if err != nil {
			s.Fatal("Failed to get a random mac address: ", err)
		}
		macs[i] = mac
	}
	var (
		id0 = hex.EncodeToString(macs[0])
		id1 = hex.EncodeToString(macs[3])
	)
	const (
		key0 = "1f1e1d1c1b1a191817161514131211100f0e0d0c0b0a09080706050403020100"
		key1 = "000102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f"
		mdID = "a1b2"
	)

	secConfFac := wpa.NewConfigFactory("chromeos", wpa.Mode(wpa.ModePureWPA2), wpa.Ciphers2(wpa.CipherCCMP), wpa.FTMode(wpa.FTModeMixed))
	var ssids [3]string
	for i := 0; i < len(ssids); i++ {
		ssids[i] = hostapd.RandomSSID("TAST_BEACON_REP_")
	}

	// Note that we choose 2.4GHz channels here because several of our
	// devices are "no IR" for all 5GHz channels
	ap0Ops := []hostapd.Option{
		hostapd.Channel(6), hostapd.Mode(hostapd.Mode80211acMixed), hostapd.BSSID(macs[0].String()),
		hostapd.MobilityDomain(mdID), hostapd.NASIdentifier(id0), hostapd.R1KeyHolder(id0),
		hostapd.R0KHs(fmt.Sprintf("%s %s %s", macs[3], id1, key0)),
		hostapd.R1KHs(fmt.Sprintf("%s %s %s", macs[3], macs[3], key1)),
		hostapd.SSID(ssids[0]), hostapd.HTCaps(hostapd.HTCapHT40), hostapd.VHTChWidth(hostapd.VHTChWidth20Or40),
		hostapd.RRMBeaconReport(),
		hostapd.AdditionalBSSs(
			hostapd.AdditionalBSS{IfaceName: "beaconRepDev1", SSID: ssids[1], BSSID: macs[1].String()},
			hostapd.AdditionalBSS{IfaceName: "beaconRepDev2", SSID: ssids[2], BSSID: macs[2].String()},
		),
	}
	ap1Ops := []hostapd.Option{
		hostapd.Channel(1), hostapd.Mode(hostapd.Mode80211acMixed), hostapd.BSSID(macs[3].String()),
		hostapd.MobilityDomain(mdID), hostapd.NASIdentifier(id1), hostapd.R1KeyHolder(id1),
		hostapd.R0KHs(fmt.Sprintf("%s %s %s", macs[0], id0, key1)),
		hostapd.R1KHs(fmt.Sprintf("%s %s %s", macs[0], macs[0], key0)),
		hostapd.SSID(ssids[0]), hostapd.HTCaps(hostapd.HTCapHT40), hostapd.VHTChWidth(hostapd.VHTChWidth20Or40),
		hostapd.RRMBeaconReport(),
		hostapd.AdditionalBSSs(
			hostapd.AdditionalBSS{IfaceName: "beaconRepDev3", SSID: ssids[1], BSSID: macs[4].String()},
			hostapd.AdditionalBSS{IfaceName: "beaconRepDev4", SSID: ssids[2], BSSID: macs[5].String()},
		),
	}

	s.Log("Starting the first AP")
	ap0, _, deconfig0 := wifiutil.ConfigureAP(ctx, s, ap0Ops, 0, secConfFac)
	defer func(ctx context.Context) {
		if ap0 != nil {
			deconfig0(ctx, ap0)
			ap0 = nil
		}
	}(ctx)
	ctx, cancel = tf.ReserveForDeconfigAP(ctx, ap0)
	defer cancel()
	ap0Chan := uint8(ap0.Config().Channel)

	disconnect := wifiutil.ConnectAP(ctx, s, ap0, 0)
	defer disconnect(ctx)
	ctx, cancel = tf.ReserveForDisconnect(ctx)
	defer cancel()

	if err := tf.VerifyConnection(ctx, ap0); err != nil {
		s.Fatal("Failed to verify connection: ", err)
	}

	s.Log("Starting the second AP")
	ap1, _, deconfig1 := wifiutil.ConfigureAP(ctx, s, ap1Ops, 1, secConfFac)
	defer func(ctx context.Context) {
		if ap1 != nil {
			deconfig1(ctx, ap1)
			ap1 = nil
		}
	}(ctx)
	ctx, cancel = tf.ReserveForDeconfigAP(ctx, ap1)
	defer cancel()
	ap1Chan := uint8(ap1.Config().Channel)

	legacyRouter, err := tf.LegacyRouter()
	if err != nil {
		s.Fatal("Unable to get legacy router: ", err)
	}
	freqOpts, err := ap0.Config().PcapFreqOptions()
	if err != nil {
		s.Fatal("Failed to get pcap frequency options: ", err)
	}

	clientMAC, err := tf.ClientHardwareAddr(ctx)
	if err != nil {
		s.Fatal("Unable to get DUT MAC address: ", err)
	}
	clientMACBytes, err := net.ParseMAC(clientMAC)
	if err != nil {
		s.Fatal("Unable to parse MAC address: ", err)
	}
	runOnce := func(ctx context.Context, params hostapd.BeaconReqParams, expected []reportBSS, name string) error {
		SendBeaconRequest := func(ctx context.Context) error {
			return ap0.SendBeaconRequest(ctx, clientMAC, params)
		}
		pcapPath, err := wifiutil.CollectPcapForAction(ctx, legacyRouter, name, int(ap0Chan), freqOpts, SendBeaconRequest)
		if err != nil {
			return err
		}

		filters := []pcap.Filter{
			pcap.Dot11FCSValid(),
			pcap.TransmitterAddress(clientMACBytes),
			pcap.TypeFilter(layers.LayerTypeDot11MgmtAction, nil),
		}
		packets, err := pcap.ReadPackets(pcapPath, filters...)
		if err != nil {
			return err
		}
		foundRRM := false
		for _, p := range packets {
			layer := p.Layer(layers.LayerTypeDot11MgmtAction)
			if layer == nil {
				return errors.New("found packet without Action layer")
			}
			actual := make(map[string]reportBSS)
			action := layer.(*layers.Dot11MgmtAction)
			var header actionHeader
			r := bytes.NewReader(action.Contents)
			if err := binary.Read(r, binary.LittleEndian, &header); err != nil {
				return err
			}
			if header.Category != RRMCategoryCode {
				continue
			}
			foundRRM = true
			lastReport := false
			// The packet should contain a list of reports, each of
			// which corresponds to a specific BSSID. Each report
			// will contain a list of subelements. The information
			// subelement will contain a list of IEs. Do a triply-
			// nested loop here to parse out the IEs for each AP.
			for r.Len() > 0 {
				if lastReport {
					return errors.New("last report indication true for non-last report")
				}
				var beaconHdr beaconRepHeader
				var beaconInfo beaconRepInfo
				if err := binary.Read(r, binary.LittleEndian, &beaconHdr); err != nil {
					return err
				}
				if err := binary.Read(r, binary.LittleEndian, &beaconInfo); err != nil {
					return err
				}
				//reportLen := action.Contents[i+1]
				bssid := net.HardwareAddr(beaconInfo.BSSID[:])

				// A report for a BSSID can be fragmented. Check
				// the map first in case we've already seen the
				// BSSID
				bss, reportExists := actual[bssid.String()]
				if !reportExists {
					bss.Channel = beaconInfo.Channel
					bss.BSSID = bssid
				}
				subelems := make([]byte, int(beaconHdr.Len)-beaconRepInfoSz)
				if _, err := io.ReadFull(r, subelems); err != nil {
					return err
				}
				subelemR := bytes.NewReader(subelems)
				for subelemR.Len() > 0 {
					var subelemHeader elemHeader
					if err := binary.Read(subelemR, binary.LittleEndian, &subelemHeader); err != nil {
						return err
					}
					if subelemHeader.ID == uint8(hostapd.SubelemInfo) {
						iesLen := int(subelemHeader.Len)
						if !reportExists {
							// The first report of each BSSID includes 12 extra bytes of information before the list of IEs. Skip them.
							if _, err := subelemR.Seek(extraFirstReportBytes, io.SeekCurrent); err != nil {
								return err
							}
							iesLen -= extraFirstReportBytes
						}
						ies := make([]byte, iesLen)
						if _, err := io.ReadFull(subelemR, ies); err != nil {
							return err
						}
						ieR := bytes.NewReader(ies)
						// Append the IEs.
						for ieR.Len() > 0 {
							var ieHeader elemHeader
							if err := binary.Read(ieR, binary.LittleEndian, &ieHeader); err != nil {
								return err
							}
							bss.IEs = append(bss.IEs, layers.Dot11InformationElementID(ieHeader.ID))
							if _, err := ieR.Seek(int64(ieHeader.Len), io.SeekCurrent); err != nil {
								return err
							}
						}
					} else if subelemHeader.ID == uint8(hostapd.SubelemLastIndication) {
						// Only the last report should have this subelement if we've requested it.
						if !params.LastFrame {
							return errors.New("last indication reported but not requested")
						}
						var isLast uint8
						if err := binary.Read(subelemR, binary.LittleEndian, &isLast); err != nil {
							return err
						}
						lastReport = isLast == 1
					} else {
						if _, err := subelemR.Seek(int64(subelemHeader.Len), io.SeekCurrent); err != nil {
							return err
						}
					}
				}
				actual[bss.BSSID.String()] = bss
			}
			// Expect an exact match in BSSID entries except when we request all IEs in the reporting detail.
			if len(expected) != len(actual) {
				return errors.Errorf("Number of BSS entries doesn't match. Got %v, want %v", len(actual), len(expected))
			}
			for _, expectedBSS := range expected {
				actualBSS, ok := actual[expectedBSS.BSSID.String()]
				if !ok {
					return errors.Errorf("BSSID %s not found in report", expectedBSS.BSSID)
				}
				if !bytes.Equal(actualBSS.BSSID, expectedBSS.BSSID) {
					return errors.Errorf("BSSIDs did not match. Got %s, want %s", actualBSS.BSSID, expectedBSS.BSSID)
				}
				if actualBSS.Channel != expectedBSS.Channel {
					return errors.Errorf("Channel for BSSID %s did not match. Got %v, want %v", actualBSS.BSSID, actualBSS.Channel, expectedBSS.Channel)
				}
				actualIEs := make(map[layers.Dot11InformationElementID]struct{})
				for _, ie := range actualBSS.IEs {
					actualIEs[ie] = struct{}{}
				}
				if params.ReportingDetail == hostapd.DetailAllFields && len(actualIEs) == 0 {
					return errors.New("Requested all IEs but got none")
				}
				if params.ReportingDetail != hostapd.DetailAllFields && len(expectedBSS.IEs) != len(actualIEs) {
					return errors.Errorf("Number of IEs doesn't match. Got %v, want %v", len(actualIEs), len(expectedBSS.IEs))
				}
				for _, ie := range expectedBSS.IEs {
					if _, ok := actualIEs[ie]; !ok {
						return errors.Errorf("IEs for BSSID %s did not include all expected IEs. Got %v, want %v", actualBSS.BSSID, actualBSS.IEs, expectedBSS.IEs)
					}
				}
			}
		}
		if !foundRRM {
			return errors.New("no RRM found")
		}
		return nil
	}
	testcases := []struct {
		Request hostapd.BeaconReqParams
		Report  []reportBSS
	}{{
		// Scan for all channels with SSID 0, and include specified IEs.
		// Expect two entries corresponding to the two APs on SSID 0
		// with the IEs requested.
		hostapd.BeaconReqParams{
			OpClass:         ieee80211.OpClass2GHz,
			Channel:         0,
			Duration:        20,
			Mode:            hostapd.ModeActive,
			SSID:            ssids[0],
			BSSID:           bcastAddr,
			ReportingDetail: hostapd.DetailRequestedOnly,
			Request:         testIEs,
		},
		[]reportBSS{{
			BSSID:   macs[0],
			Channel: ap0Chan,
			IEs:     testIEs,
		}, {
			BSSID:   macs[3],
			Channel: ap1Chan,
			IEs:     testIEs,
		},
		},
	}, {
		// Scan for both channels in the report channels element.
		// Expect all 6 APs to show up with all requested IEs for the
		// two APs that support FT, and all requested IEs less the
		// Mobility Domain IE for the APs that don't.
		hostapd.BeaconReqParams{
			OpClass:         ieee80211.OpClass2GHz,
			Channel:         255,
			Duration:        50,
			Mode:            hostapd.ModeActive,
			BSSID:           bcastAddr,
			ReportingDetail: hostapd.DetailRequestedOnly,
			ReportChannels:  []byte{ap0Chan, ap1Chan},
			Request:         testIEs,
			LastFrame:       true,
		},
		[]reportBSS{{
			BSSID:   macs[0],
			Channel: ap0Chan,
			IEs:     testIEs,
		}, {
			BSSID:   macs[1],
			Channel: ap0Chan,
			IEs:     testIEsNoMD,
		}, {
			BSSID:   macs[2],
			Channel: ap0Chan,
			IEs:     testIEsNoMD,
		}, {
			BSSID:   macs[3],
			Channel: ap1Chan,
			IEs:     testIEs,
		}, {
			BSSID:   macs[4],
			Channel: ap1Chan,
			IEs:     testIEsNoMD,
		}, {
			BSSID:   macs[5],
			Channel: ap1Chan,
			IEs:     testIEsNoMD,
		},
		},
	}, {
		// Use cached scan results to get APs on SSID 0 without any IEs.
		// Expect the two APs on SSID 0 with no IEs included.
		hostapd.BeaconReqParams{
			OpClass:         ieee80211.OpClass2GHz,
			Channel:         0,
			Duration:        20,
			Mode:            hostapd.ModeTable,
			SSID:            ssids[0],
			BSSID:           bcastAddr,
			ReportingDetail: hostapd.DetailNone,
			LastFrame:       true,
		},
		[]reportBSS{{
			BSSID:   macs[0],
			Channel: ap0Chan,
		}, {
			BSSID:   macs[3],
			Channel: ap1Chan,
		},
		},
	}, {
		// Passive scan on the second router's channel and include all IEs.
		// Expect the three APs on the second router with all IEs.
		hostapd.BeaconReqParams{
			OpClass:         ieee80211.OpClass2GHz,
			Channel:         ap1Chan,
			Duration:        112,
			Mode:            hostapd.ModePassive,
			BSSID:           bcastAddr,
			ReportingDetail: hostapd.DetailAllFields,
			LastFrame:       true,
		},
		[]reportBSS{{
			BSSID:   macs[3],
			Channel: ap1Chan,
		}, {
			BSSID:   macs[4],
			Channel: ap1Chan,
		}, {
			BSSID:   macs[5],
			Channel: ap1Chan,
		},
		},
	}, {
		// Scan on the second router's channel for a specific BSSID with specific IEs.
		// Expect the AP with that BSSID with the specified IEs.
		hostapd.BeaconReqParams{
			OpClass:         ieee80211.OpClass2GHz,
			Channel:         ap1Chan,
			Duration:        20,
			Mode:            hostapd.ModeActive,
			BSSID:           macs[3],
			ReportingDetail: hostapd.DetailRequestedOnly,
			Request:         testIEs,
			LastFrame:       true,
		},
		[]reportBSS{{
			BSSID:   macs[3],
			Channel: ap1Chan,
			IEs:     testIEs,
		},
		},
	}, {
		// Scan on the two specified channels for SSID 0 with the specified IEs.
		// Expect the two APs on SSID 0 with the specified IEs.
		hostapd.BeaconReqParams{
			OpClass:         ieee80211.OpClass2GHz,
			Channel:         255,
			Duration:        50,
			Mode:            hostapd.ModeActive,
			SSID:            ssids[0],
			BSSID:           bcastAddr,
			ReportingDetail: hostapd.DetailRequestedOnly,
			ReportChannels:  []byte{ap0Chan, ap1Chan},
			Request:         testIEs,
			LastFrame:       true,
		},
		[]reportBSS{{
			BSSID:   macs[0],
			Channel: ap0Chan,
			IEs:     testIEs,
		}, {
			BSSID:   macs[3],
			Channel: ap1Chan,
			IEs:     testIEs,
		},
		},
	}}
	for i, tc := range testcases {
		s.Log("Running test case: ", i)
		if err = runOnce(ctx, tc.Request, tc.Report, strconv.Itoa(i)); err != nil {
			s.Fatalf("Run %d failed: %v", i, err)
		}
	}
}

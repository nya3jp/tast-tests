// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wifi

import (
	"bytes"
	"context"
	"encoding/hex"
	"fmt"
	"net"
	"strconv"
	"time"

	"github.com/golang/protobuf/ptypes/empty"
	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"

	"chromiumos/tast/common/wifi/security/wpa"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/remote/bundles/cros/wifi/wifiutil"
	"chromiumos/tast/remote/wificell"
	"chromiumos/tast/remote/wificell/hostapd"
	"chromiumos/tast/remote/wificell/pcap"
	"chromiumos/tast/services/cros/network"
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
		Attr:         []string{"group:wificell"},
		ServiceDeps:  []string{wificell.TFServiceName},
		Pre:          wificell.TestFixturePreWithFeatures(wificell.TFFeaturesRouters),
		Vars:         []string{"routers", "pcap"},
		SoftwareDeps: []string{"rrm_support"},
	})
}

type reportBSS struct {
	BSSID   net.HardwareAddr
	Channel uint8
	IEs     []layers.Dot11InformationElementID
}

var bcastAddr = []byte{0xff, 0xff, 0xff, 0xff, 0xff, 0xff}
var testIEsNoMD = []layers.Dot11InformationElementID{
	layers.Dot11InformationElementIDSSID,
	layers.Dot11InformationElementIDVendor,
}
var testIEs = append(testIEsNoMD, layers.Dot11InformationElementIDMobilityDomain)

func RRMBeaconReport(ctx context.Context, s *testing.State) {
	tf := s.PreValue().(*wificell.TestFixture)
	defer func(ctx context.Context) {
		if err := tf.CollectLogs(ctx); err != nil {
			s.Log("Error collecting logs, err: ", err)
		}
	}(ctx)
	ctx, cancel := tf.ReserveForCollectLogs(ctx)
	defer cancel()

	ctx, restoreBg, err := tf.TurnOffBgscan(ctx)
	if err != nil {
		s.Fatal("Failed to turn off the background scan: ", err)
	}
	defer func() {
		if err := restoreBg(); err != nil {
			s.Error("Failed to restore the background scan config: ", err)
		}
	}()

	ftResp, err := tf.WifiClient().GetGlobalFTProperty(ctx, &empty.Empty{})
	if err != nil {
		s.Fatal("Failed to get the global FT property: ", err)
	}
	defer func(ctx context.Context) {
		if _, err := tf.WifiClient().SetGlobalFTProperty(ctx, &network.SetGlobalFTPropertyRequest{Enabled: ftResp.Enabled}); err != nil {
			s.Errorf("Failed to set global FT property back to %v: %v", ftResp.Enabled, err)
		}
	}(ctx)
	ctx, cancel = ctxutil.Shorten(ctx, time.Second)
	defer cancel()

	if _, err := tf.WifiClient().SetGlobalFTProperty(ctx, &network.SetGlobalFTPropertyRequest{Enabled: true}); err != nil {
		s.Fatal("Failed to turn on the global FT property: ", err)
	}

	var macs [6]net.HardwareAddr
	for i := 0; i < len(macs); i++ {
		mac, err := hostapd.RandomMAC()
		if err != nil {
			s.Fatal("Failed to get a random mac address: ", err)
		}
		macs[i] = mac
	}
	mac0, err := hostapd.RandomMAC()
	if err != nil {
		s.Fatal("Failed to get a random mac address: ", err)
	}
	mac1, err := hostapd.RandomMAC()
	if err != nil {
		s.Fatal("Failed to get a random mac address: ", err)
	}
	var (
		id0 = hex.EncodeToString(mac0)
		id1 = hex.EncodeToString(mac1)
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
		hostapd.R0KHs(fmt.Sprintf("%s %s %s", mac1, id1, key0)),
		hostapd.R1KHs(fmt.Sprintf("%s %s %s", mac1, mac1, key1)),
		hostapd.SSID(ssids[0]), hostapd.HTCaps(hostapd.HTCapHT40), hostapd.VHTChWidth(hostapd.VHTChWidth20Or40),
		hostapd.RRMBeaconReport(),
		hostapd.AdditionalBSSs(
			hostapd.AdditionalBSS{"beaconRepDev1", ssids[1], macs[1].String()},
			hostapd.AdditionalBSS{"beaconRepDev2", ssids[2], macs[2].String()},
		),
	}
	ap1Ops := []hostapd.Option{
		hostapd.Channel(1), hostapd.Mode(hostapd.Mode80211acMixed), hostapd.BSSID(macs[3].String()),
		hostapd.MobilityDomain(mdID), hostapd.NASIdentifier(id1), hostapd.R1KeyHolder(id1),
		hostapd.R0KHs(fmt.Sprintf("%s %s %s", mac0, id0, key1)),
		hostapd.R1KHs(fmt.Sprintf("%s %s %s", mac0, mac0, key0)),
		hostapd.SSID(ssids[0]), hostapd.HTCaps(hostapd.HTCapHT40), hostapd.VHTChWidth(hostapd.VHTChWidth20Or40),
		hostapd.RRMBeaconReport(),
		hostapd.AdditionalBSSs(
			hostapd.AdditionalBSS{"beaconRepDev3", ssids[1], macs[4].String()},
			hostapd.AdditionalBSS{"beaconRepDev4", ssids[2], macs[5].String()},
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
		pcapPath, err := wifiutil.CollectPcapForAction(ctx, tf.Router(), name, int(ap0Chan), SendBeaconRequest)
		if err != nil {
			s.Fatal("Failed to collect pcap for beacon request: ", err)
		}

		filters := []pcap.Filter{
			pcap.Dot11FCSValid(),
			pcap.TypeFilter(
				layers.LayerTypeDot11,
				func(layer gopacket.Layer) bool {
					dot11 := layer.(*layers.Dot11)
					// Filter sender == MAC of DUT
					return bytes.Equal(dot11.Address2, clientMACBytes)
				},
			),
			pcap.TypeFilter(layers.LayerTypeDot11MgmtAction, nil),
		}
		packets, err := pcap.ReadPackets(pcapPath, filters...)
		if err != nil {
			s.Fatal("Failed to read action packets: ", err)
		}
		for _, p := range packets {
			layer := p.Layer(layers.LayerTypeDot11MgmtAction)
			if layer == nil {
				s.Fatal("Found packet without Action layer")
			}
			actual := make(map[string]reportBSS)
			action := layer.(*layers.Dot11MgmtAction)
			if len(action.Contents) < 3 || action.Contents[0] != 5 { // Radio Measurement
				continue
			}
			lastReport := false
			for i := 3; i < len(action.Contents); {
				if lastReport {
					return errors.New("Last report indication true for non-last report")
				}
				reportLen := action.Contents[i+1]
				i += 2
				bssid := net.HardwareAddr(action.Contents[i+18 : i+24])

				// A report for a BSSID can be fragmented. Check
				// the map first in case we've already seen the
				// BSSID
				bss, reportExists := actual[bssid.String()]
				if !reportExists {
					bss.Channel = action.Contents[i+4]
					bss.BSSID = bssid
				}
				j := i + 29 // offset to beginning of subelements
				if j < len(action.Contents) {
					for j < i+int(reportLen) {
						elemID := action.Contents[j]
						elemLen := action.Contents[j+1]
						j += 2
						if elemID == byte(hostapd.SubelemInfo) {
							// Fragmented reports don't include certain elements.
							tagOffset := 0
							if !reportExists {
								tagOffset = 12
							}
							for k := j + tagOffset; k < j+int(elemLen); {
								bss.IEs = append(bss.IEs, layers.Dot11InformationElementID(action.Contents[k]))
								tagLen := action.Contents[k+1]
								k += int(tagLen) + 2
							}
						}
						if elemID == byte(hostapd.SubelemLastIndication) {
							if !params.LastFrame {
								return errors.New("Last indication reported but not requested")
							}
							if action.Contents[j] == 1 {
								lastReport = true
							}
						}
						j += int(elemLen)
					}
				}
				actual[bss.BSSID.String()] = bss
				i += int(reportLen)
			}
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
		return nil
	}
	testcases := []struct {
		Request hostapd.BeaconReqParams
		Report  []reportBSS
	}{{
		hostapd.BeaconReqParams{
			OpClass:         81,
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
		hostapd.BeaconReqParams{
			OpClass:         81,
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
		hostapd.BeaconReqParams{
			OpClass:         81,
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
		hostapd.BeaconReqParams{
			OpClass:         81,
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
		hostapd.BeaconReqParams{
			OpClass:         81,
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
		hostapd.BeaconReqParams{
			OpClass:         81,
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

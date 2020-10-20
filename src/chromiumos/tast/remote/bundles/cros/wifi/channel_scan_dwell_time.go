// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wifi

import (
	"context"
	"errors"
	"math/rand"
	"sort"
	"strconv"
	"strings"
	"time"

	"chromiumos/tast/common/network/iw"
	remoteiw "chromiumos/tast/remote/network/iw"
	"chromiumos/tast/remote/wificell"
	"chromiumos/tast/remote/wificell/framesender"
	"chromiumos/tast/remote/wificell/hostapd"
	"chromiumos/tast/remote/wificell/pcap"
	"chromiumos/tast/testing"

	"github.com/google/gopacket/layers"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:        ChannelScanDwellTime,
		Desc:        "Tests that channel dwell time for single-channel scan is within acceptable range",
		Contacts:    []string{"wgd@google.com", "chromeos-platform-connectivity@google.com"},
		Attr:        []string{"group:wificell", "wificell_func", "wificell_unstable"},
		ServiceDeps: []string{wificell.TFServiceName},
		Pre:         wificell.TestFixturePre(),
		Vars:        []string{"router", "pcap"},
	})
}

const (
	captureName               = "channel_scan_dwell_time"
	testChannel               = 1
	numBSS                    = 1024
	delayIntervalMilliseconds = 1
	knownTestPrefix           = "wifi_CSDT"
	suffixLetters             = "abcdefghijklmnopqrstuvwxyz0123456789"
	missingBeaconThreshold    = 2
	scanStartDelay            = 200 * time.Millisecond
)

func ChannelScanDwellTime(ctx context.Context, s *testing.State) {
	tf := s.PreValue().(*wificell.TestFixture)
	defer func(ctx context.Context) {
		if err := tf.CollectLogs(ctx); err != nil {
			s.Log("Error collecting logs, err: ", err)
		}
	}(ctx)
	ctx, cancel := tf.ReserveForCollectLogs(ctx)
	defer cancel()

	iwr := remoteiw.NewRemoteRunner(s.DUT().Conn())

	s.Log("Starting packet capture")
	capturer, err := tf.Pcap().StartCapture(ctx, captureName, testChannel, nil)
	if err != nil {
		s.Fatal("Failed to start packet capture: ", err)
	}

	ssidPrefix := knownTestPrefix + "_" + uniqueString(5) + "_"
	s.Log("SSID Prefix: ", ssidPrefix)

	apCfg, err := hostapd.NewConfig(hostapd.Mode(hostapd.Mode80211b), hostapd.Channel(1))
	if err != nil {
		s.Fatal("Failed to create hostapd config: ", err)
	}
	ap, err := tf.Router().StartAPIface(ctx, "ap0", apCfg)
	if err != nil {
		s.Fatal("Failed to create APIface: ", err)
	}
	iface := ap.Interface()

	// s.Log("Connecting to AP (to force passive scanning)")
	// if _, err := tf.ConnectWifiAP(ctx, ap); err != nil {
	// 	s.Fatal("Failed to connect to WiFi: ", err)
	// }
	// defer func(ctx context.Context) {
	// 	if err := tf.CleanDisconnectWifi(ctx); err != nil {
	// 		s.Error("Failed to disconnect WiFi: ", err)
	// 	}
	// }(ctx)
	// ctx, cancel = tf.ReserveForDisconnect(ctx)
	// defer cancel()

	s.Log("Starting frame sender on ", iface)
	sender, err := tf.Router().NewFrameSender(ctx, iface)
	if err != nil {
		s.Fatal("Failed to create frame sender: ", err)
	}
	defer func(ctx context.Context) {
		if err := tf.Router().CloseFrameSender(ctx, sender); err != nil {
			s.Error("Failed to close frame sender: ", err)
		}
	}(ctx)
	ctx, cancel = tf.Router().ReserveForCloseFrameSender(ctx)
	defer cancel()
	opts := []framesender.Option{
		framesender.SSIDPrefix(ssidPrefix),
		framesender.NumBSS(numBSS),
		// Differs from original Autotest, but more correct because it prevents
		// the same SSID being broadcast more than once and causing ambiguity.
		framesender.Count(numBSS),
		framesender.Delay(delayIntervalMilliseconds),
	}
	go func(ctx context.Context) {
		err := sender.Send(ctx, framesender.TypeBeacon, testChannel, opts...)
		if err != nil && !errors.Is(err, context.Canceled) {
			s.Fatal("Failed to send beacon frames: ", err)
		}
	}(ctx)
	// Wait a little while for beacons to start actually being transmitted
	time.Sleep(scanStartDelay)

	s.Log("Performing scan")
	bssList, err := pollScan(ctx, iwr, "wlan0", []int{2412}, 10*time.Second)
	if err != nil {
		s.Fatal("Failed to scan: ", err)
	}
	s.Logf("Found %d APs", len(bssList))

	s.Log("Finishing packet capture")
	if err := tf.Pcap().StopCapture(ctx, capturer); err != nil {
		s.Fatal("Failed to stop capturer: ", err)
	}
	pcapPath, err := capturer.PacketPath(ctx)
	if err != nil {
		s.Fatal("Failed to get packet capture: ", err)
	}
	s.Log("Packet Capture at: ", pcapPath)

	s.Log("Calculating dwell time")
	var ssids []string
	for _, bss := range bssList {
		if strings.HasPrefix(bss.SSID, ssidPrefix) {
			ssids = append(ssids, bss.SSID)
		}
	}
	sort.Strings(ssids)
	if len(ssids) == 0 {
		s.Fatal("No Beacons Found")
	}

	// Analyze scan results
	beaconCount := len(ssids)
	beaconFirst := ssids[0]
	beaconFinal := ssids[len(ssids)-1]
	beaconRange := ssidIndex(beaconFinal) - ssidIndex(beaconFirst) + 1
	if beaconRange-beaconCount > missingBeaconThreshold {
		s.Fatalf("Missed %d beacons: %v", beaconRange-beaconCount, ssids)
	}

	s.Log("First Beacon: ", beaconFirst)
	s.Log("Final Beacon: ", beaconFinal)
	s.Log("Beacon Range: ", beaconRange)
	s.Log("Beacon Count: ", beaconCount)

	// Open Packet Capture and read beacons
	beaconFilter := pcap.TypeFilter(layers.LayerTypeDot11MgmtBeacon, nil)
	packets, err := pcap.ReadPackets(pcapPath, beaconFilter)
	if err != nil {
		s.Fatal("Failed to read beacons from packet capture: ", err)
	}

	// Construct a mapping from SSIDs to broadcast time
	ssidTimestamps := make(map[string]time.Time)
	for _, packet := range packets {
		for _, layer := range packet.Layers() {
			if elem, ok := layer.(*layers.Dot11InformationElement); ok {
				if elem.ID == layers.Dot11InformationElementIDSSID {
					ssid := string(elem.Info)
					ts := packet.Metadata().Timestamp
					if ssid != "" && !ts.IsZero() {
						ssidTimestamps[ssid] = ts
						s.Logf("Beacon %s at %v", ssid, ts)
					}
				}
			}
		}
	}

	timeFirst := ssidTimestamps[beaconFirst]
	timeFinal := ssidTimestamps[beaconFinal]
	dwellTime := timeFinal.Sub(timeFirst)
	s.Log("First Beacon Time: ", timeFirst)
	s.Log("Final Beacon Time: ", timeFinal)
	s.Log("Dwell Time: ", dwellTime)
}

func ssidIndex(ssid string) int {
	idxStr := ssid[strings.LastIndex(ssid, "_")+1:]
	idx, err := strconv.ParseUint(idxStr, 16, 64)
	if err != nil {
		return -1
	}
	return int(idx)
}

// pollScan repeatedly requests a scan until it gets a valid result
func pollScan(ctx context.Context, iwr *iw.Runner, iface string, freqs []int, scanTimeout time.Duration) ([]*iw.BSSData, error) {
	var scanResult *iw.TimedScanData
	err := testing.Poll(ctx, func(ctx context.Context) error {
		ctx, cancel := context.WithTimeout(ctx, scanTimeout)
		defer cancel()

		var err error
		scanResult, err = iwr.TimedScan(ctx, iface, freqs, nil)
		if err != nil {
			testing.ContextLogf(ctx, "Scan Failure (%v), Retrying", err)
		}
		return err
	}, &testing.PollOptions{Interval: 500 * time.Millisecond})
	if err != nil {
		return nil, err
	}
	return scanResult.BSSList, nil
}

func uniqueString(n int) string {
	buf := make([]byte, n)
	for i := range buf {
		buf[i] = suffixLetters[rand.Intn(len(suffixLetters))]
	}
	return string(buf)
}

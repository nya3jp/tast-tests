// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wifi

import (
	"context"
	"math/rand"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/google/gopacket/layers"

	"chromiumos/tast/common/network/iw"
	"chromiumos/tast/common/perf"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/dut"
	"chromiumos/tast/errors"
	remoteip "chromiumos/tast/remote/network/ip"
	remoteiw "chromiumos/tast/remote/network/iw"
	"chromiumos/tast/remote/wificell"
	"chromiumos/tast/remote/wificell/framesender"
	"chromiumos/tast/remote/wificell/hostapd"
	"chromiumos/tast/remote/wificell/pcap"
	"chromiumos/tast/testing"
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
	knownTestPrefix        = "wifi_CSDT"
	suffixLetters          = "abcdefghijklmnopqrstuvwxyz0123456789"
	captureName            = "channel_scan_dwell_time"
	testChannel            = 1
	numBSS                 = 1024
	delayInterval          = 1 * time.Millisecond
	scanStartDelay         = 500 * time.Millisecond
	scanRetryTimeout       = 10 * time.Second
	missingBeaconThreshold = 2
	maxDwellTime           = 250 * time.Millisecond
	minDwellTime           = 5 * time.Millisecond
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

	s.Log("Claiming WiFi Interface")
	if err := claimInterface(ctx, s.DUT(), tf, "wlan0"); err != nil {
		s.Fatal("Unable to claim WiFi interface: ", err)
	}
	defer func(ctx context.Context) {
		if err := releaseInterface(ctx, tf, "wlan0"); err != nil {
			s.Error("Failed to release WiFi interface: ", err)
		}
	}(ctx)
	ctx, cancel = ctxutil.Shorten(ctx, time.Second)
	defer cancel()

	s.Log("Starting packet capture")
	capturer, err := tf.Pcap().StartCapture(ctx, captureName, testChannel, nil)
	if err != nil {
		s.Fatal("Failed to start packet capture: ", err)
	}

	s.Log("Configuring AP on router")
	apCfg, err := hostapd.NewConfig(hostapd.Mode(hostapd.Mode80211b), hostapd.Channel(1))
	if err != nil {
		s.Fatal("Failed to create hostapd config: ", err)
	}
	ap, err := tf.Router().StartAPIface(ctx, "ap0", apCfg)
	if err != nil {
		s.Fatal("Failed to create APIface: ", err)
	}
	iface := ap.Interface()

	s.Log("Starting frame sender on ", iface)
	ssidPrefix := knownTestPrefix + "_" + uniqueString(5) + "_"
	s.Log("SSID Prefix: ", ssidPrefix)
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
		framesender.Count(numBSS),
		framesender.Delay(int(delayInterval.Milliseconds())),
	}
	go func(ctx context.Context) {
		err := sender.Send(ctx, framesender.TypeBeacon, testChannel, opts...)
		if err != nil && !errors.Is(err, context.Canceled) {
			s.Fatal("Failed to send beacon frames: ", err)
		}
	}(ctx)
	// Wait a little while for beacons to start actually being transmitted
	testing.Sleep(ctx, scanStartDelay)

	s.Log("Performing scan")
	freq, err := hostapd.ChannelToFrequency(testChannel)
	if err != nil {
		s.Fatal("Failed to select scan frequency: ", err)
	}
	bssList, err := pollScan(ctx, s.DUT(), "wlan0", []int{freq}, scanRetryTimeout)
	if err != nil {
		s.Fatal("Failed to scan: ", err)
	}
	s.Logf("Scan found %d APs", len(bssList))

	s.Log("Finishing packet capture")
	if err := tf.Pcap().StopCapture(ctx, capturer); err != nil {
		s.Fatal("Failed to stop capturer: ", err)
	}
	pcapPath, err := capturer.PacketPath(ctx)
	if err != nil {
		s.Fatal("Failed to get packet capture: ", err)
	}

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
	s.Logf("Found %d test beacons between %q and %q", beaconCount, beaconFirst, beaconFinal)
	if beaconRange-beaconCount > missingBeaconThreshold {
		s.Fatalf("Missed %d beacons: %v", beaconRange-beaconCount, ssids)
	}

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
					}
				}
			}
		}
	}

	// Use that mapping to figure out when the first and last scanned beacon were
	// transmitted. The difference in timestamps was the dwell time of the scan.
	timeFirst := ssidTimestamps[beaconFirst]
	timeFinal := ssidTimestamps[beaconFinal]
	dwellTime := timeFinal.Sub(timeFirst)
	s.Log("First Beacon Time: ", timeFirst)
	s.Log("Final Beacon Time: ", timeFinal)
	s.Log("Dwell Time: ", dwellTime)
	if (dwellTime < minDwellTime) || (dwellTime > maxDwellTime) {
		s.Fatalf("Dwell time %v is not within range [%v, %v]", dwellTime, minDwellTime, maxDwellTime)
	}

	pv := perf.NewValues()
	pv.Set(perf.Metric{
		Name:      "dwell_time_single_channel_scan",
		Unit:      "seconds",
		Direction: perf.SmallerIsBetter,
	}, dwellTime.Seconds())
	if err := pv.Save(s.OutDir()); err != nil {
		s.Error("Failed to save perf data: ", err)
	}
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
func pollScan(ctx context.Context, dut *dut.DUT, iface string, freqs []int, pollTimeout time.Duration) ([]*iw.BSSData, error) {
	iwr := remoteiw.NewRemoteRunner(dut.Conn())
	var scanResult *iw.TimedScanData
	err := testing.Poll(ctx, func(ctx context.Context) error {
		var err error
		scanResult, err = iwr.TimedScan(ctx, iface, freqs, nil)
		if err != nil {
			testing.ContextLogf(ctx, "Scan Failure (%v), Retrying", err)
		}
		return err
	}, &testing.PollOptions{Timeout: pollTimeout, Interval: 500 * time.Millisecond})
	if err != nil {
		return nil, err
	}
	return scanResult.BSSList, nil
}

// claimInterface disables the interface in Shill and then brings it back up manually.
// CR: Should this be a method on the test fixture? The only Autotest test using
//   the claim_wifi_if() method was ChannelScanDwellTime so it wouldn't be used
//   by any other tests at the moment.
func claimInterface(ctx context.Context, dut *dut.DUT, tf *wificell.TestFixture, iface string) error {
	ipr := remoteip.NewRemoteRunner(dut.Conn())

	// Tell Shill not to touch the interface
	if err := tf.SetWifiEnabled(ctx, false); err != nil {
		return err
	}

	// Wait for the interface to be down
	err := testing.Poll(ctx, func(ctx context.Context) error {
		up, err := ipr.IsLinkUp(ctx, iface)
		if err != nil {
			return err
		}
		if up {
			return errors.New("Waiting for interface to be down")
		}
		return nil
	}, &testing.PollOptions{Interval: 500 * time.Millisecond})
	if err != nil {
		return err
	}

	// Manually bring the interface up for our own use
	if err := ipr.SetLinkUp(ctx, iface); err != nil {
		return err
	}
	return nil
}

// releaseInterface tells Shill that it can manage the interface again.
func releaseInterface(ctx context.Context, tf *wificell.TestFixture, iface string) error {
	return tf.SetWifiEnabled(ctx, true)
}

func uniqueString(n int) string {
	buf := make([]byte, n)
	for i := range buf {
		buf[i] = suffixLetters[rand.Intn(len(suffixLetters))]
	}
	return string(buf)
}

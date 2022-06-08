// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wifi

import (
	"context"
	"fmt"
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

type csdtTestcase struct {
	apChannel    int
	apOpts       []hostapd.Option
	minDwellTime time.Duration
	maxDwellTime time.Duration
}

func init() {
	testing.AddTest(&testing.Test{
		Func: ChannelScanDwellTime,
		Desc: "Tests that channel dwell time for single-channel scan is within acceptable range",
		Contacts: []string{
			"chromeos-wifi-champs@google.com", // WiFi oncall rotation; or http://b/new?component=893827
		},
		Attr:        []string{"group:wificell", "wificell_perf"},
		ServiceDeps: []string{wificell.TFServiceName},
		Fixture:     "wificellFixtWithCapture",
	})
}

/*
Definitions from 802.11 standards:
ProbeDelay: Delay to be used prior to transmitting a Probe frame during active
scanning.
MinChannelTime: The minimum time to spend on each channel when scanning.
MaxChannelTime: The maximum time to spend on each channel when scanning.
These values are integers, and MinChannelTime<=MaxChannelTime. They are set in
the driver, and different chips may have different values.

In an active scan, the station waits for either an indication of an incoming
frame or for the ProbeDelay timer to expire. Then it sends a probe request and
waits for MinChannelTime to elapse.
    a. If the medium is never busy, move to the next channel;
    b. If the medium is busy during the MinChannelTime interval, wait until
    MaxChannelTime and process any probe response.

In this test, the DUT performs an active scan on each channel. dwellTime is
the time the DUT spends on each channel collecting beacon frames. The AP sends
beacon frames every |delayInterval|, |numBSS| beacon frames in total. Since the
medium is busy, the DUT will stay on each channel until MaxChannelTime has
elapsed. dwellTime is calculated based on the timestamps of the first test
beacon frame after the probe request, and the last test beacon frame received
during the active scan. The purpose of this test is to verify that
|minDwellTime| <= dwellTime (MaxChannelTime) <= |maxDwellTime|.
*/

func ChannelScanDwellTime(ctx context.Context, s *testing.State) {
	const (
		knownTestPrefix        = "wifi_CSDT"
		suffixLetters          = "abcdefghijklmnopqrstuvwxyz0123456789"
		captureName            = "channel_scan_dwell_time"
		numBSS                 = 1024
		delayInterval          = 1 * time.Millisecond
		scanStartDelay         = 500 * time.Millisecond
		scanRetryTimeout       = 10 * time.Second
		missingBeaconThreshold = 2
	)

	// TODO(b/182308669): Tighten up min/max bounds on various channel dwell times
	testcases := []csdtTestcase{
		{
			apChannel:    1,
			apOpts:       []hostapd.Option{hostapd.Mode(hostapd.Mode80211nMixed), hostapd.HTCaps(hostapd.HTCapHT40)},
			minDwellTime: 5 * time.Millisecond,
			maxDwellTime: 250 * time.Millisecond,
		},
		{
			apChannel:    9,
			apOpts:       []hostapd.Option{hostapd.Mode(hostapd.Mode80211nMixed), hostapd.HTCaps(hostapd.HTCapHT40)},
			minDwellTime: 5 * time.Millisecond,
			maxDwellTime: 250 * time.Millisecond,
		},
		{
			apChannel:    36,
			apOpts:       []hostapd.Option{hostapd.Mode(hostapd.Mode80211nMixed), hostapd.HTCaps(hostapd.HTCapHT40)},
			minDwellTime: 5 * time.Millisecond,
			maxDwellTime: 250 * time.Millisecond,
		},
	}

	tf := s.FixtValue().(*wificell.TestFixture)

	router, err := tf.StandardRouterWithFrameSenderSupport()
	if err != nil {
		s.Fatal("Failed to get legacy router: ", err)
	}

	pv := perf.NewValues()
	defer func() {
		if err := pv.Save(s.OutDir()); err != nil {
			s.Log("Failed to save perf data, err: ", err)
		}
	}()

	s.Log("Claiming WiFi Interface")
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, time.Second)
	defer cancel()
	clientIface, err := tf.ClientInterface(ctx)
	if err != nil {
		s.Fatal("Unable to get client interface name: ", err)
	}
	if err := claimInterface(ctx, s.DUT(), tf, clientIface); err != nil {
		s.Fatal("Unable to claim WiFi interface: ", err)
	}
	defer func(ctx context.Context) {
		if err := releaseInterface(ctx, tf, clientIface); err != nil {
			s.Error("Failed to release WiFi interface: ", err)
		}
	}(cleanupCtx)

	ipr := remoteip.NewRemoteRunner(s.DUT().Conn())
	dutMAC, err := ipr.MAC(ctx, clientIface)
	if err != nil {
		s.Fatal("Failed to get MAC of WiFi interface: ", err)
	}

	testOnce := func(ctx context.Context, s *testing.State, tc csdtTestcase) bool {
		ssidPrefix := knownTestPrefix + "_" + uniqueString(5, suffixLetters) + "_"

		bssList, capturer, err := func(ctx context.Context) ([]*iw.BSSData, *pcap.Capturer, error) {
			s.Log("Configuring AP on router")
			apOpts := append([]hostapd.Option{hostapd.Channel(tc.apChannel)}, tc.apOpts...)
			ap, err := tf.ConfigureAP(ctx, apOpts, nil)
			if err != nil {
				return nil, nil, errors.Wrap(err, "failed to configure ap")
			}
			defer func(ctx context.Context) {
				if err := tf.DeconfigAP(ctx, ap); err != nil {
					s.Error("Failed to deconfig ap: ", err)
				}
			}(ctx)
			ctx, cancel = tf.ReserveForDeconfigAP(ctx, ap)
			defer cancel()
			capturer, ok := tf.Capturer(ap)
			if !ok {
				return nil, nil, errors.New("failed to get capturer for AP")
			}

			s.Log("Starting frame sender on ", ap.Interface())
			s.Log("SSID Prefix: ", ssidPrefix)
			cleanupCtx := ctx
			ctx, cancel = router.ReserveForCloseFrameSender(ctx)
			defer cancel()
			sender, err := router.NewFrameSender(ctx, ap.Interface())
			if err != nil {
				return nil, nil, errors.Wrap(err, "failed to create frame sender")
			}
			senderDone := make(chan error)
			go func(ctx context.Context) {
				senderDone <- sender.Send(ctx, framesender.TypeBeacon, tc.apChannel,
					framesender.SSIDPrefix(ssidPrefix),
					framesender.NumBSS(numBSS),
					framesender.Count(numBSS),
					framesender.Delay(int(delayInterval.Milliseconds())),
				)
			}(ctx)
			defer func(ctx context.Context) {
				if err := router.CloseFrameSender(ctx, sender); err != nil {
					s.Error("Failed to close frame sender: ", err)
				}
				select {
				case err := <-senderDone:
					if err != nil && !errors.Is(err, context.Canceled) {
						s.Error("Failed to send beacon frames: ", err)
					}
				case <-ctx.Done():
					s.Error("Timed out waiting for frame sender to finish")
				}
			}(cleanupCtx)
			// Wait a little while for beacons to start actually being transmitted
			if err := testing.Sleep(ctx, scanStartDelay); err != nil {
				return nil, nil, errors.Wrap(err, "interrupted while sleeping for frame sender startup")
			}

			s.Log("Performing scan")
			freq, err := hostapd.ChannelToFrequency(tc.apChannel)
			if err != nil {
				return nil, nil, errors.Wrap(err, "failed to select scan frequency")
			}
			bssList, err := pollScan(ctx, s.DUT(), clientIface, []int{freq}, scanRetryTimeout)
			if err != nil {
				return nil, nil, errors.Wrap(err, "failed to scan")
			}
			s.Logf("Scan found %d APs", len(bssList))
			return bssList, capturer, nil
		}(ctx)
		if err != nil {
			s.Fatal("Failed to perform test: ", err)
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

		// Find the first probe request from the DUT.
		// If there are no probe requests, fail.
		probeReqFilter := []pcap.Filter{
			pcap.TypeFilter(layers.LayerTypeDot11MgmtProbeReq, nil),
			pcap.TransmitterAddress(dutMAC),
		}
		probeReqPackets, err := pcap.ReadPackets(pcapPath, probeReqFilter...)
		if err != nil {
			s.Fatal("Failed to read probe requests from packet capture: ", err)
		}
		s.Logf("Received %d probe requests", len(probeReqPackets))
		if len(probeReqPackets) == 0 {
			return false
		}
		probeReqTimestamp := probeReqPackets[0].Metadata().Timestamp
		s.Log("Probe Request Time: ", probeReqTimestamp)

		// Find the first test beacon after the first probe request.
		// Note that beacons arriving *before* the probe request should not be counted.
		beaconFilter := pcap.TypeFilter(layers.LayerTypeDot11MgmtBeacon, nil)
		beaconPackets, err := pcap.ReadPackets(pcapPath, beaconFilter)
		if err != nil {
			s.Fatal("Failed to read beacons from packet capture: ", err)
		}
		beaconFirst := ""
		for _, packet := range beaconPackets {

			// If we've already found the first beacon, we're done
			if beaconFirst != "" {
				break
			}

			// Check to see if the beacon is before the probe.
			// If the beacon follows the probe and matches the prefix, we're done.
			if packet.Metadata().Timestamp.Before(probeReqTimestamp) {
				continue
			}
			for _, layer := range packet.Layers() {
				if elem, ok := layer.(*layers.Dot11InformationElement); ok {
					if elem.ID == layers.Dot11InformationElementIDSSID {
						ssid := string(elem.Info)
						if strings.HasPrefix(ssid, ssidPrefix) {
							beaconFirst = ssid
							s.Log("First beacon found: ", ssid)
							break
						}
					}
				}
			}
		}
		if beaconFirst == "" {
			s.Fatalf("Could not find beacon after probe request (ssid prefix %s)", ssidPrefix)
		}

		// Analyze scan results.
		beaconCount := len(ssids)
		beaconFinal := ssids[len(ssids)-1]
		beaconFirstIdx, err := ssidIndex(beaconFirst)
		if err != nil {
			s.Fatal("Failed to parse beacon SSID: ", err)
		}
		beaconFinalIdx, err := ssidIndex(beaconFinal)
		if err != nil {
			s.Fatal("Failed to parse beacon SSID: ", err)
		}
		beaconRange := beaconFinalIdx - beaconFirstIdx + 1
		s.Logf("Found %d test beacons (initial %q), considering %q through %q",
			beaconCount, ssids[0], beaconFirst, beaconFinal)
		if beaconRange-beaconCount > missingBeaconThreshold {
			s.Fatalf("Missed %d beacons: %v", beaconRange-beaconCount, ssids)
		}

		// Construct a mapping from SSIDs to broadcast time
		ssidTimestamps := make(map[string]time.Time)
		for _, packet := range beaconPackets {
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
		timeFirst, ok := ssidTimestamps[beaconFirst]
		if !ok {
			s.Fatal("Failed to find timestamp of the first beacon ", beaconFirst)
		}
		timeFinal, ok := ssidTimestamps[beaconFinal]
		if !ok {
			s.Fatal("Failed to find the timestamp of the final beacon ", beaconFinal)
		}

		dwellTime := timeFinal.Sub(timeFirst)
		s.Log("First Beacon Time: ", timeFirst)
		s.Log("Final Beacon Time: ", timeFinal)
		s.Log("Dwell Time: ", dwellTime)
		if (dwellTime < tc.minDwellTime) || (dwellTime > tc.maxDwellTime) {
			s.Fatalf("Dwell time %v is not within range [%v, %v]", dwellTime, tc.minDwellTime, tc.maxDwellTime)
		}

		s.Logf("dwell_time_ch%d = %.3f", tc.apChannel, dwellTime.Seconds())
		pv.Set(perf.Metric{
			Name:      fmt.Sprintf("dwell_time_ch%d", tc.apChannel),
			Unit:      "seconds",
			Direction: perf.SmallerIsBetter,
		}, dwellTime.Seconds())
		return true
	}

	for i, tc := range testcases {
		subtest := func(ctx context.Context, s *testing.State) {
			if testOnce(ctx, s, tc) {
				return
			}
			// |beaconFirst| is defined as the first test beacon frame after
			// the probe request, but the captures occasionally miss the probe
			// requests either due to collision with the test beacon frames or
			// the DUT failing to send the probe request, and this causes the
			// test to fail. b/225073589 shows that the loss rate of the probe
			// request is about 3%, rerunning the test gives << 1% failure
			// rate, which is acceptable.
			s.Log("No probe requests in packet capture, run the test again")
			if !testOnce(ctx, s, tc) {
				s.Fatal("No probe requests in packet capture in two channel scans")
			}
		}
		if !s.Run(ctx, fmt.Sprintf("Testcase #%d (ch%d)", i, tc.apChannel), subtest) {
			// Stop if one of the subtest's parameter set fails the test.
			return
		}
	}
}

func ssidIndex(ssid string) (int, error) {
	idxStr := ssid[strings.LastIndex(ssid, "_")+1:]
	idx, err := strconv.ParseUint(idxStr, 16, 64)
	if err != nil {
		return 0, errors.Wrap(err, "failed to parse SSID index")
	}
	return int(idx), nil
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
func claimInterface(ctx context.Context, dut *dut.DUT, tf *wificell.TestFixture, iface string) error {
	ipr := remoteip.NewRemoteRunner(dut.Conn())

	// Tell Shill not to touch the interface
	if err := tf.WifiClient().SetWifiEnabled(ctx, false); err != nil {
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
	return tf.WifiClient().SetWifiEnabled(ctx, true)
}

func uniqueString(n int, chars string) string {
	buf := make([]byte, n)
	for i := range buf {
		buf[i] = chars[rand.Intn(len(chars))]
	}
	return string(buf)
}

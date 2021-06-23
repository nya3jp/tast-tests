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

	legacyRouter, err := tf.LegacyRouter()
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

	testOnce := func(ctx context.Context, s *testing.State, tc csdtTestcase) {
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
			ctx, cancel = legacyRouter.ReserveForCloseFrameSender(ctx)
			defer cancel()
			sender, err := legacyRouter.NewFrameSender(ctx, ap.Interface())
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
				if err := legacyRouter.CloseFrameSender(ctx, sender); err != nil {
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

		// Analyze scan results
		beaconCount := len(ssids)
		beaconFirst := ssids[0]
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
	}

	for i, tc := range testcases {
		subtest := func(ctx context.Context, s *testing.State) {
			testOnce(ctx, s, tc)
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

func uniqueString(n int, chars string) string {
	buf := make([]byte, n)
	for i := range buf {
		buf[i] = chars[rand.Intn(len(chars))]
	}
	return string(buf)
}

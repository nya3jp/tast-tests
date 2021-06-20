// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wifi

import (
	"context"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"chromiumos/tast/common/perf"
	"chromiumos/tast/common/wifi/security"
	"chromiumos/tast/remote/bundles/cros/wifi/wifiutil"
	"chromiumos/tast/remote/wificell"
	"chromiumos/tast/remote/wificell/attenuator"
	"chromiumos/tast/remote/wificell/hostapd"
	"chromiumos/tast/testing"
)

const rnDebugEnabled = false

type roamNaturalTestcase struct {
	apOpts1    []hostapd.Option
	apOpts2    []hostapd.Option
	secConfFac security.ConfigFactory
}

const (
	roamNaturalMaxCenter      = 100
	roamNaturalMinCenter      = 84
	roamNaturalMaxAtten       = 106
	roamNaturalAttenStep      = 2
	roamNaturalRoundPassCount = 2

	roamNaturalRoamTimeout = 5 * time.Second

	roamNaturalLogFilePerm  os.FileMode = 0644
	roamNaturalLogFileFlags             = os.O_WRONLY | os.O_CREATE | os.O_TRUNC
)

type roamNaturalStatsMap map[[2]int]int

type rangeDef struct {
	start int // inclusive
	end   int // exclusive
	step  int
}

var roamNaturalSSID = hostapd.RandomSSID("TAST_ROAM_NAT_")

var roamNaturalAP1Opts = []hostapd.Option{
	hostapd.Mode(hostapd.Mode80211nPure),
	hostapd.Channel(1),
	hostapd.HTCaps(hostapd.HTCapHT20),
	hostapd.BSSID("00:11:22:33:44:55"),
	hostapd.SSID(roamNaturalSSID),
}
var roamNaturalAP2Opts = []hostapd.Option{
	hostapd.Mode(hostapd.Mode80211nPure),
	hostapd.Channel(2),
	hostapd.HTCaps(hostapd.HTCapHT20),
	hostapd.BSSID("00:11:22:33:44:56"),
	hostapd.SSID(roamNaturalSSID),
}
var roamNaturalAP36Opts = []hostapd.Option{
	hostapd.Mode(hostapd.Mode80211nPure),
	hostapd.Channel(36),
	hostapd.HTCaps(hostapd.HTCapHT20),
	hostapd.BSSID("00:11:22:33:44:57"),
	hostapd.SSID(roamNaturalSSID),
}

func init() {
	testing.AddTest(&testing.Test{
		Func:        RoamNatural,
		Desc:        "This test is used to validity check that 'normal' roaming behavior is not broken by any roaming algorithm changes",
		Contacts:    []string{"jakobczyk@google.com"},
		Attr:        []string{"group:wificell_roam", "wificell_roam_perf"},
		ServiceDeps: []string{wificell.TFServiceName},
		Pre:         wificell.TestFixturePreWithFeatures(wificell.TFFeaturesRouters | wificell.TFFeaturesAttenuator),
		Timeout:     time.Minute * 60,
		Vars:        []string{"routers", "pcap", "attenuator"},
		Params: []testing.Param{
			{
				Val: []roamNaturalTestcase{
					{apOpts1: roamNaturalAP1Opts, apOpts2: roamNaturalAP2Opts, secConfFac: nil},
					{apOpts1: roamNaturalAP1Opts, apOpts2: roamNaturalAP36Opts, secConfFac: nil},
				},
			},
		},
	})
}

func rnDebug(s *testing.State, args ...interface{}) {
	if rnDebugEnabled {
		s.Log(args...)
	}
}

// simulateDUTMove modifies attenuation of both APs to simulate DUT moving between them.
// The simulation start closer to AP0, moves so that it is closer to AP1 and then moves back toward AP0.
// offsetRange defines range and stepping of attenuation changes - how "close" to APs DUT gets.
func simulateDUTMove(ctx context.Context, s *testing.State, offsetRange rangeDef, center int,
	attenuator *attenuator.Attenuator, freq0, freq1 int) {
	offset := offsetRange.start
	step := offsetRange.step

	setAttenuation := func(channel int, atten float64, freq int) {
		minAtten, err := attenuator.MinTotalAttenuation(channel)
		if err != nil {
			s.Fatal("Failed to get minimal attenuation")
		}
		if err := attenuator.SetTotalAttenuation(ctx, channel, math.Max(atten, minAtten), freq); err != nil {
			s.Fatal("Failed to set attenuation: ", err)
		}
	}

	for {
		if (step > 0 && offset >= offsetRange.end) || (step < 0 && offset <= offsetRange.end) {
			break
		}

		atten0 := float64(center + offset)
		atten1 := float64(center - offset)
		setAttenuation(0, atten0, freq0)
		setAttenuation(1, atten0, freq0)
		setAttenuation(2, atten1, freq1)
		setAttenuation(3, atten1, freq1)

		if err := testing.Sleep(ctx, 2*time.Second); err != nil {
			s.Fatal("Failed to sleep: ", err)
		}

		offset += step
	}
}

// collectWPAEvents collects all Roam and Disconnected events from wpa_supplicant since last call to this
// or to wpaMonitor.ClearEvents.
func collectWPAEvents(ctx context.Context, s *testing.State, wpaMonitor *wificell.WPAMonitor) (
	skipRoamEvents []*wificell.RoamEvent, disconnectedEvents []*wificell.DisconnectedEvent) {

	timeoutCtx, cancel := context.WithTimeout(ctx, roamNaturalRoamTimeout)
	defer cancel()
	skipRoamEvents = []*wificell.RoamEvent{}
	disconnectedEvents = []*wificell.DisconnectedEvent{}
	for {
		event, err := wpaMonitor.WaitForEvent(timeoutCtx)
		if err != nil {
			s.Fatal("Failed to wait for roam event: ", err)
		}
		if event == nil { // timeout
			break
		}
		rnDebug(s, event)
		switch e := event.(type) {
		case *wificell.RoamEvent:
			if e.Skip {
				skipRoamEvents = append(skipRoamEvents, e)
			}
		case *wificell.DisconnectedEvent:
			disconnectedEvents = append(disconnectedEvents, e)
		}
	}

	return skipRoamEvents, disconnectedEvents
}

func executeRoamNaturalTest(ctx context.Context, s *testing.State, apAllParams [][]hostapd.Option, runIdx int,
	secConfFac security.ConfigFactory, roamStats *roamNaturalStatsMap, failureStats *int) {

	tf := s.PreValue().(*wificell.TestFixture)

	attenuator := tf.Attenuator()
	for i := 0; i < 4; i++ {
		if err := attenuator.SetAttenuation(ctx, i, 0); err != nil {
			s.Fatal("Failed to set attenutation: ", err)
		}
	}

	iface, err := tf.ClientInterface(ctx)
	if err != nil {
		s.Fatal("Failed to get client interface: ", err)
	}

	ap0, freq0, deconfig0 := wifiutil.ConfigureAP(ctx, s, apAllParams[0], 0, secConfFac)
	defer func(ctx context.Context) {
		if ap0 != nil { // ap0 evaluated during execution of the func, so it's last AP created
			deconfig0(ctx, ap0)
			ap0 = nil
		}
	}(ctx)
	ctx, cancel := tf.ReserveForDeconfigAP(ctx, ap0)
	defer cancel()

	disconnect := wifiutil.ConnectAP(ctx, s, ap0, 0)
	defer disconnect(ctx)
	ctx, cancel = tf.ReserveForDisconnect(ctx)
	defer cancel()

	if err := tf.VerifyConnection(ctx, ap0); err != nil {
		s.Fatal("Failed to verify connection: ", err)
	}

	ap1, freq1, deconfig1 := wifiutil.ConfigureAP(ctx, s, apAllParams[1], 1, secConfFac)
	defer func(ctx context.Context) {
		if ap1 != nil {
			deconfig1(ctx, ap1)
			ap1 = nil
		}
	}(ctx)
	ctx, cancel = tf.ReserveForDeconfigAP(ctx, ap1)
	defer cancel()

	wpaMonitor, stop, ctx, err := tf.StartWPAMonitor(ctx)
	if err != nil {
		s.Fatal("Faled to start wpa monitor")
	}
	defer stop()

	skipRoamLog, err := os.OpenFile(filepath.Join(s.OutDir(), strconv.Itoa(runIdx)+"_skip_roam.txt"),
		roamNaturalLogFileFlags, roamNaturalLogFilePerm)
	if err != nil {
		s.Fatal("Failed to create skipped roams log file: ", err)
	}
	defer skipRoamLog.Close()

	assocFailLog, err := os.OpenFile(filepath.Join(s.OutDir(), strconv.Itoa(runIdx)+"_failure.txt"),
		roamNaturalLogFileFlags, roamNaturalLogFilePerm)
	if err != nil {
		s.Fatal("Failed to create association failures log file: ", err)
	}
	defer assocFailLog.Close()

	for center := roamNaturalMinCenter; center < roamNaturalMaxCenter; center += 2 * roamNaturalAttenStep {
		// The attenuation should [con,di]verge around center. We move
		// the attenuation out 2dBm at a time until roamNaturalMaxAtten is hit
		// on one AP, at which point we tear that AP down to simulate it
		// disappearing from the DUT's view. This should trigger a deauth
		// if the DUT is still associated.
		maxOffset := roamNaturalMaxAtten - center

		for roundPass := 0; roundPass < roamNaturalRoundPassCount; roundPass++ {
			offsetRanges := []rangeDef{
				{0, maxOffset, roamNaturalAttenStep},
				{maxOffset, 0, -roamNaturalAttenStep},
				{0, -maxOffset, -roamNaturalAttenStep},
				{-maxOffset, 0, roamNaturalAttenStep},
			}

			for offsetRangeIdx, offsetRange := range offsetRanges {
				err = tf.ClearBSSIDIgnoreDUT(ctx)
				if err != nil {
					s.Fatal("Failed to clear wpa BSSID_IGNORE: ", err)
				}
				rnDebug(s, "Cleared wpa BSSID_IGNORE")

				wpaMonitor.ClearEvents(ctx)

				rnDebug(s, "Varying attenuation in range: ", offsetRange)

				simulateDUTMove(ctx, s, offsetRange, center, attenuator, freq0, freq1)

				if offsetRangeIdx%2 == 1 {
					// The APs' RSSIs should have converged. No reason to
					// check for disconnects/roams here.
					continue
				}

				if offsetRangeIdx == 0 {
					// First AP is no longer in view
					rnDebug(s, "deconfig ap0")
					if err := deconfig0(ctx, ap0); err != nil {
						s.Fatal("Failed to deconfig ap: ", err)
					}
					ap0 = nil
				} else if offsetRangeIdx == 2 {
					// Second AP is no longer in view
					rnDebug(s, "deconfig ap1")
					if err := deconfig1(ctx, ap1); err != nil {
						s.Fatal("Failed to deconfig ap: ", err)
					}
					ap1 = nil
				}

				rnDebug(s, "checking skipped roams and disconnects")
				skipRoamEvents, disconnectedEvents := collectWPAEvents(ctx, s, wpaMonitor)

				if len(disconnectedEvents) > 0 {
					// Association failure happened, check if this
					// was because a roam was skipped.
					if len(skipRoamEvents) > 0 {
						// Skipped roam caused association failure, log this
						// so we can re-examine the roam decision.
						for _, roam := range skipRoamEvents {
							str := roam.ToLogString()
							s.Log(str)
							skipRoamLog.WriteString(str)
							freqPair := [2]int{int(roam.CurFreq / 1000), int(roam.SelFreq / 1000)}
							(*roamStats)[freqPair] = (*roamStats)[freqPair] + 1
						}
					} else {
						// Association failure happened for some other reason
						// (likely because AP disappeared before scan
						// results returned). Log the failure for the
						// timestamp in case we'd like to take a closer look.
						for _, disconnect := range disconnectedEvents {
							str := disconnect.ToLogString()
							s.Log(str)
							assocFailLog.WriteString(str)
							(*failureStats)++
						}
					}
				}

				// Reset the attenuation here. In some groamer cells, the
				// attenuation for 5GHz channels is miscalibrated such that
				// the RSSI is lower than expected. If we bring the AP back
				// up while it's still maximally attenuated, it may not be
				// visible to the DUT (the test was written deliberately so
				// that it wouldn't happen even at full attenuation for
				// properly calibrated cells, but this is apparently not
				// always a good assumption).
				if err := attenuator.SetAttenuation(ctx, 0, 0); err != nil {
					s.Fatal("Failed to set attenutation: ", err)
				}

				// bring back the AP that was stopped earlier
				var ap *wificell.APIface
				if offsetRangeIdx == 0 {
					rnDebug(s, "bringing back AP 0")
					ap0, freq0, deconfig0 = wifiutil.ConfigureAP(ctx, s, apAllParams[0], 0, secConfFac)
					ap = ap0
				} else if offsetRangeIdx == 2 {
					rnDebug(s, "bringing back AP 1")
					ap1, freq1, deconfig1 = wifiutil.ConfigureAP(ctx, s, apAllParams[1], 1, secConfFac)
					ap = ap1
				}

				rnDebug(s, "discovering AP")
				if err := tf.DiscoverBSSID(ctx, ap.Config().BSSID, iface, []byte(ap.Config().SSID)); err != nil {
					s.Fatal("Failed to discover AP: ", err)
				}
			}
		}
	}
}

func dumpRoamNaturalStats(ctx context.Context, s *testing.State, roamStats *roamNaturalStatsMap, failureStats int) {
	pv := perf.NewValues()

	for freqs, skips := range *roamStats {
		s.Logf("%d association failures caused by skipped roams from %d GHz to %d GHz",
			skips, freqs[0], freqs[1])

		metric := perf.Metric{
			Name:      fmt.Sprintf("roam_natural_%d_%d", freqs[0], freqs[1]),
			Unit:      "roams_skipped",
			Direction: perf.SmallerIsBetter,
			Multiple:  false,
		}

		pv.Set(metric, float64(skips))
	}

	metric := perf.Metric{
		Name:      "roam_natural_assoc_failures",
		Unit:      "assocation_failures",
		Direction: perf.SmallerIsBetter,
		Multiple:  false,
	}

	pv.Set(metric, float64(failureStats))

	if err := pv.Save(s.OutDir()); err != nil {
		s.Error("Failed saving perf data: ", err)
	}
}

// RoamNatural executes the test case
//
// Bring up two APs, connect, vary attenuation as if the device is moving
// between the two APs (i.e. the signal gets weaker on one and stronger on the
// other until the first one cannot be seen anymore). At some point before the
// first AP is torn down, the device should have roamed to the second AP. If it
// doesn't there will be an association failure, which we can then log and
// write to a file. Ideally, there would be no association failures and a roam
// every time we expected one. Realistically, RSSI can vary quite widely, and
// we can't expect to see a good roam signal on every scan even where there
// should be one.
// This test is used to validity check that "normal" roaming behavior is not
// broken by any roaming algorithm changes. A couple failed associations is
// acceptable, but any more than that is a good indication that roaming has
// become too sticky.
func RoamNatural(ctx context.Context, s *testing.State) {
	tf := s.PreValue().(*wificell.TestFixture)
	defer func(ctx context.Context) {
		if err := tf.CollectLogs(ctx); err != nil {
			s.Error("Error collecting logs, err: ", err)
		}
	}(ctx)
	ctx, cancel := tf.ReserveForCollectLogs(ctx)
	defer cancel()

	roamStats := roamNaturalStatsMap{
		{2, 2}: 0,
		{2, 5}: 0,
		{5, 2}: 0,
	}
	failureStats := 0

	testCases := s.Param().([]roamNaturalTestcase)
	for i, tc := range testCases {
		executeRoamNaturalTest(ctx, s, [][]hostapd.Option{tc.apOpts1, tc.apOpts2}, i, tc.secConfFac, &roamStats, &failureStats)
	}

	dumpRoamNaturalStats(ctx, s, &roamStats, failureStats)
}

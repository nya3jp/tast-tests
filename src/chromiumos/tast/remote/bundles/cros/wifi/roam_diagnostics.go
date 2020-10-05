// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Bring an AP up, connect to it, set the attenuation, and vary a second AP
// around the same RSSI as the first AP. Perform a scan after every change in
// attenuation and observe when the device roams between APs. Record all roam
// events in a file for analysis.
// The purpose of this diagnostic is to capture the stickiness of the device's
// roam algorithm. For example, the stickier the algorithm, the more skewed
// toward higher RSSI differentials (between current and target AP) the
// distribution of roams in the output files will be. This is not necessarily
// a good thing as it's important for a device to be able to move between APs
// when it needs to. Therefore, we use network_WiFi_RoamNatural in conjunction
// with this test to ensure that normal roam behavior is not broken."""

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
	"chromiumos/tast/remote/wificell/hostapd"
	"chromiumos/tast/services/cros/network"
	"chromiumos/tast/testing"
)

type roamDiagnosticsTestcase struct {
	apOpts1    []hostapd.Option
	apOpts2    []hostapd.Option
	secConfFac security.ConfigFactory
}

const (
	roamDiagnosticsMaxAttenuation   float64 = 96
	roamDiagnosticsMinAttenuation   float64 = 56
	roamDiagnosticsAttenuationStep  float64 = 4
	roamDiagnosticsAttenuationRange float64 = 12
	roamDiagnosticsRoamBuckets              = 7
	roamDiagnosticsRoundPassCount           = 2
	roamDiagnosticsScanCount                = 2

	roamDiagnosticsRoamTimeout  = 5 * time.Second
	roamDiagnosticsScansTimeout = 10 * time.Second
)

type roamDiagnosticsStatsMap map[[2]int](*[roamDiagnosticsRoamBuckets]int)

var roamDiagnosticsSSID = hostapd.RandomSSID("TAST_TEST_")

var roamDiagnosticsAP1Opts = []hostapd.Option{
	hostapd.Mode(hostapd.Mode80211nPure),
	hostapd.Channel(1),
	hostapd.HTCaps(hostapd.HTCapHT20),
	hostapd.BSSID("00:11:22:33:44:55"),
	hostapd.SSID(roamNaturalSSID),
}
var roamDiagnosticsAP2Opts = []hostapd.Option{
	hostapd.Mode(hostapd.Mode80211nPure),
	hostapd.Channel(2),
	hostapd.HTCaps(hostapd.HTCapHT20),
	hostapd.BSSID("00:11:22:33:44:56"),
	hostapd.SSID(roamNaturalSSID),
}
var roamDiagnosticsAP36Opts = []hostapd.Option{
	hostapd.Mode(hostapd.Mode80211nPure),
	hostapd.Channel(36),
	hostapd.HTCaps(hostapd.HTCapHT20),
	hostapd.BSSID("00:11:22:33:44:57"),
	hostapd.SSID(roamNaturalSSID),
}

func init() {
	testing.AddTest(&testing.Test{
		Func:        RoamDiagnostics,
		Desc:        "Bring up two APs and attenuate them around several values to observe and assess roam stickiness",
		Contacts:    []string{"jakobczyk@google.com"},
		Attr:        []string{"group:wificell", "wificell_func", "wificell_unstable"},
		ServiceDeps: []string{wificell.TFServiceName},
		Pre:         wificell.TestFixturePreWithFeatures(wificell.TFFeaturesRouters | wificell.TFFeaturesAttenuator),
		Timeout:     time.Minute * 90,
		Vars:        []string{"routers", "pcap", "attenuator"},
		Params: []testing.Param{
			{
				Val: []roamDiagnosticsTestcase{
					{apOpts1: roamDiagnosticsAP1Opts, apOpts2: roamDiagnosticsAP2Opts, secConfFac: nil},
					{apOpts1: roamDiagnosticsAP1Opts, apOpts2: roamDiagnosticsAP36Opts, secConfFac: nil},
				},
			},
		},
	})
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func clamp(x, xmin, xmax int) int {
	return min(xmax, max(x, xmin))
}

func requestScans(ctx context.Context, s *testing.State, wpaMonitor *wificell.WPAMonitor) {
	tf := s.PreValue().(*wificell.TestFixture)

	scanSuccess := false
	for scan := 0; scan < roamDiagnosticsScanCount; scan++ {
		timeoutCtx, cancel := context.WithTimeout(ctx, roamDiagnosticsScansTimeout)
		defer cancel()
		req := &network.RequestScansRequest{Count: 0}
		if _, err := tf.WifiClient().RequestScans(timeoutCtx, req); err != nil {
			s.Fatal("Failed to request scan: ", err)
		}
		timeoutCtx, cancel = context.WithTimeout(ctx, roamDiagnosticsScansTimeout)
		defer cancel()
		received := false
		for {
			event, err := wpaMonitor.WaitForEvent(timeoutCtx)
			if err != nil {
				s.Fatal("Failed to wait for scan event: ", err)
			}
			if event == nil { // timeout
				break
			}
			_, received = event.(*wificell.ScanResultsEvent)
			if received {
				break
			}
		}

		if received {
			scanSuccess = true
			break
		}
		s.Logf("Scan failed %d time(s)", scan+1)
	}
	if !scanSuccess {
		s.Fatal("Unable to get scan results")
	}
}

func updateRoamStats(ctx context.Context, s *testing.State, wpaMonitor *wificell.WPAMonitor, roamLog *os.File,
	stats *roamDiagnosticsStatsMap) {

	timeoutCtx, cancel := context.WithTimeout(ctx, roamDiagnosticsRoamTimeout)
	defer cancel()
	var roam *wificell.RoamEvent
	received := false
	for {
		event, err := wpaMonitor.WaitForEvent(timeoutCtx)
		if err != nil {
			s.Fatal("Failed to wait for roam event: ", err)
		}
		if event == nil { // timeout
			break
		}
		roam, received = event.(*wificell.RoamEvent)
		if received && !roam.Skip {
			break
		}
		received = false
		roam = nil
	}

	if roam != nil {
		str := fmt.Sprintf("%+v\n", roam)
		s.Log("Roam event: ", str)
		roamLog.WriteString(str)

		freqPair := [2]int{int(roam.CurFreq / 1000), int(roam.SelFreq / 1000)}
		bucketIdx := clamp((roam.SelLevel-roam.CurLevel)/2, 0, roamDiagnosticsRoamBuckets-1)
		freqStats := (*stats)[freqPair]
		freqStats[bucketIdx]++
	}
}

func executeRoamDiagnosticsTest(ctx context.Context, s *testing.State, apAllParams [][]hostapd.Option, runIdx int,
	secConfFac security.ConfigFactory, stats *roamDiagnosticsStatsMap) {

	tf := s.PreValue().(*wificell.TestFixture)

	attenuator := tf.Attenuator()

	ap0, freq0, deconfig := wifiutil.ConfigureAP(ctx, s, apAllParams[0], 0, secConfFac)
	defer deconfig(ctx, ap0)
	ctx, cancel := tf.ReserveForDeconfigAP(ctx, ap0)
	defer cancel()

	disconnect := wifiutil.ConnectAP(ctx, s, ap0, 0)
	defer disconnect(ctx)
	ctx, cancel = tf.ReserveForDisconnect(ctx)
	defer cancel()

	if err := tf.VerifyConnection(ctx, ap0); err != nil {
		s.Fatal("Failed to verify connection: ", err)
	}

	ap1, freq1, deconfig := wifiutil.ConfigureAP(ctx, s, apAllParams[1], 1, secConfFac)
	defer deconfig(ctx, ap1)
	ctx, cancel = tf.ReserveForDeconfigAP(ctx, ap1)
	defer cancel()

	wpaMonitor, stop, ctx, err := tf.StartWPAMonitor(ctx)
	if err != nil {
		s.Fatal("Faled to start wpa monitor")
	}
	defer stop()

	roamLog, err := os.OpenFile(filepath.Join(s.OutDir(), strconv.Itoa(runIdx)+"_roam.txt"),
		os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		s.Fatal("Failed to create roam log file: ", err)
	}
	defer roamLog.Close()

	for atten0 := roamDiagnosticsMinAttenuation; atten0 <= roamDiagnosticsMaxAttenuation; atten0 += roamDiagnosticsAttenuationStep {
		attenuator.SetTotalAttenuation(ctx, 0, atten0, freq0)
		attenuator.SetTotalAttenuation(ctx, 1, atten0, freq0)
		s.Logf("Set attenuation ap%d, f=%d a=%f", 0, freq0, atten0)

		// Vary the RSSI of the second AP around that of the first AP.
		minAtten1, err := attenuator.MinTotalAttenuation(0)
		if err != nil {
			s.Fatal("Failed to get minimal attenuation")
		}
		minAtten1 = math.Max(atten0-roamDiagnosticsAttenuationRange, minAtten1)
		maxAtten1 := atten0 + roamDiagnosticsAttenuationRange

		for roundPass := 0; roundPass < roamDiagnosticsRoundPassCount; roundPass++ {
			// for atten1 in (maxAtten1 ... minAtten1 ... maxAtten1)
			step := -roamDiagnosticsAttenuationStep
			for atten1 := maxAtten1; step < 0 || atten1 < maxAtten1; {
				attenuator.SetTotalAttenuation(ctx, 2, atten1, freq1)
				attenuator.SetTotalAttenuation(ctx, 3, atten1, freq1)
				s.Logf("Set attenuation ap%d, f=%d a=%f", 1, freq1, atten1)

				wpaMonitor.ClearEvents(ctx)

				// Explicitly ask shill to perform a scan. This
				// should induce a roam if the RSSI difference is large enough.
				requestScans(ctx, s, wpaMonitor)

				// Check if roaming happened and update stats
				updateRoamStats(ctx, s, wpaMonitor, roamLog, stats)

				atten1 += step
				if atten1 <= minAtten1 {
					step = -step
					atten1 = minAtten1
				}
			}
		}
	}
}

func dumpRoamDiagnosticsStats(ctx context.Context, s *testing.State, stats *roamDiagnosticsStatsMap) {
	pv := perf.NewValues()

	for freqs, freqStats := range *stats {
		s.Logf("Roams from %d GHz to %d GHz", freqs[0], freqs[1])
		totalRoams := 0
		for _, roams := range freqStats {
			totalRoams += roams
		}

		metric := perf.Metric{
			Name:      fmt.Sprintf("roam_diagnostics_%d_%d", freqs[0], freqs[1]),
			Unit:      "roams",
			Direction: perf.SmallerIsBetter,
			Multiple:  true,
		}

		totalRoams = max(1, totalRoams)
		for bucketIdx, roams := range freqStats {
			s.Logf("%d roams %d%% with diff >= %d", roams, 100*roams/totalRoams, bucketIdx*2)
			pv.Append(metric, float64(roams))
		}
	}

	if err := pv.Save(s.OutDir()); err != nil {
		s.Error("Failed saving perf data: ", err)
	}
}

// RoamDiagnostics executes the test case
func RoamDiagnostics(ctx context.Context, s *testing.State) {
	tf := s.PreValue().(*wificell.TestFixture)
	defer func(ctx context.Context) {
		if err := tf.CollectLogs(ctx); err != nil {
			s.Log("Error collecting logs, err: ", err)
		}
	}(ctx)
	ctx, cancel := tf.ReserveForCollectLogs(ctx)
	defer cancel()

	stats := roamDiagnosticsStatsMap{
		{2, 2}: {0},
		{2, 5}: {0},
		{5, 2}: {0},
	}

	testCases := s.Param().([]roamDiagnosticsTestcase)
	for i, tc := range testCases {
		executeRoamDiagnosticsTest(ctx, s, [][]hostapd.Option{tc.apOpts1, tc.apOpts2}, i, tc.secConfFac, &stats)
	}

	dumpRoamDiagnosticsStats(ctx, s, &stats)
}

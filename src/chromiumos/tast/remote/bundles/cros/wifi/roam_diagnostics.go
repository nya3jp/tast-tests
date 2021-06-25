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
	"time"

	"chromiumos/tast/common/perf"
	"chromiumos/tast/common/wifi/security"
	"chromiumos/tast/remote/bundles/cros/wifi/wifiutil"
	"chromiumos/tast/remote/wificell"
	"chromiumos/tast/remote/wificell/attenuator"
	"chromiumos/tast/remote/wificell/hostapd"
	"chromiumos/tast/services/cros/wifi"
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

	roamDiagnosticsLogFilePerm  os.FileMode = 0644
	roamDiagnosticsLogFileFlags             = os.O_WRONLY | os.O_CREATE | os.O_TRUNC
)

type roamDiagnosticsFreqPair [2]int
type roamDiagnosticsStatsMap map[roamDiagnosticsFreqPair](*[roamDiagnosticsRoamBuckets]int)

var roamDiagnosticsSSID = hostapd.RandomSSID("TAST_ROAM_DIAG_")

var roamDiagnosticsAP1Opts = []hostapd.Option{
	hostapd.Mode(hostapd.Mode80211nPure),
	hostapd.Channel(1),
	hostapd.HTCaps(hostapd.HTCapHT20),
	hostapd.BSSID("00:11:22:33:44:55"),
	hostapd.SSID(roamDiagnosticsSSID),
}
var roamDiagnosticsAP2Opts = []hostapd.Option{
	hostapd.Mode(hostapd.Mode80211nPure),
	hostapd.Channel(2),
	hostapd.HTCaps(hostapd.HTCapHT20),
	hostapd.BSSID("00:11:22:33:44:56"),
	hostapd.SSID(roamDiagnosticsSSID),
}
var roamDiagnosticsAP36Opts = []hostapd.Option{
	hostapd.Mode(hostapd.Mode80211nPure),
	hostapd.Channel(36),
	hostapd.HTCaps(hostapd.HTCapHT20),
	hostapd.BSSID("00:11:22:33:44:57"),
	hostapd.SSID(roamDiagnosticsSSID),
}

func init() {
	testing.AddTest(&testing.Test{
		Func:        RoamDiagnostics,
		Desc:        "Bring up two APs and attenuate them around several values to observe and assess roam stickiness",
		Contacts:    []string{"jakobczyk@google.com"},
		Attr:        []string{"group:wificell_roam", "wificell_roam_perf"},
		ServiceDeps: []string{wificell.TFServiceName},
		Fixture:     "wificellFixtRoaming",
		Timeout:     time.Minute * 90,
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

// requestScans requests network scan and waits for first scan event from wpa_supplicant.
// Tries roamDiagnosticsScanCount times. Calls s.Fatal if not successful.
func requestScans(ctx context.Context, s *testing.State, wpaMonitor *wificell.WPAMonitor) {
	tf := s.FixtValue().(*wificell.TestFixture)

	scanSuccess := false
retryLoop:
	for scan := 0; scan < roamDiagnosticsScanCount; scan++ {
		timeoutCtx, cancel := context.WithTimeout(ctx, roamDiagnosticsScansTimeout)
		defer cancel()
		req := &wifi.RequestScansRequest{Count: 1}
		if _, err := tf.WifiClient().RequestScans(timeoutCtx, req); err != nil {
			s.Fatal("Failed to request scan: ", err)
		}
		for {
			event, err := wpaMonitor.WaitForEvent(timeoutCtx)
			if err != nil {
				s.Fatal("Failed to wait for scan event: ", err)
			}
			if event == nil { // timeout
				break
			}
			_, scanSuccess = event.(*wificell.ScanResultsEvent)
			if scanSuccess {
				break retryLoop
			}
		}
		s.Logf("Scan failed %d time(s)", scan+1)
	}
	if !scanSuccess {
		s.Fatal("Unable to get scan results")
	}
}

// updateRoamStats checks if roaming happened in roamDiagnosticsRoamTimeout time
// and updades statistics accordingly.
func updateRoamStats(ctx context.Context, s *testing.State, wpaMonitor *wificell.WPAMonitor, roamLog *os.File,
	stats *roamDiagnosticsStatsMap) {

	timeoutCtx, cancel := context.WithTimeout(ctx, roamDiagnosticsRoamTimeout)
	defer cancel()
	var roam *wificell.RoamEvent
	for {
		var received bool
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
		roam = nil
	}

	if roam != nil {
		str := fmt.Sprintf("%+v\n", roam)
		s.Log("Roam event: ", str)
		roamLog.WriteString(str)

		freqPair := roamDiagnosticsFreqPair{int(roam.CurFreq / 1000), int(roam.SelFreq / 1000)}
		bucketIdx := wifiutil.Clamp((roam.SelLevel-roam.CurLevel)/2, 0, roamDiagnosticsRoamBuckets-1)
		freqStats := (*stats)[freqPair]
		freqStats[bucketIdx]++
	}
}

func resetAttenuation(ctx context.Context, s *testing.State, attenuator *attenuator.Attenuator) {
	for i := 0; i < 4; i++ {
		if err := attenuator.SetAttenuation(ctx, i, 0); err != nil {
			s.Fatal("Failed to set attenutation: ", err)
		}
	}
}

func setTotalAttenuation(ctx context.Context, s *testing.State, attenuator *attenuator.Attenuator,
	apIdx int, atten float64, freq int) {

	if err := attenuator.SetTotalAttenuation(ctx, apIdx*2, atten, freq); err != nil {
		s.Fatal("Failed to set attenutation: ", err)
	}
	if err := attenuator.SetTotalAttenuation(ctx, apIdx*2+1, atten, freq); err != nil {
		s.Fatal("Failed to set attenutation: ", err)
	}
	s.Logf("Set attenuation ap%d, f=%d a=%f", apIdx, freq, atten)
}

// executeRoamDiagnosticsTest executes the test once with given parameters.
// Updates statistics.
func executeRoamDiagnosticsTest(ctx context.Context, s *testing.State, ap0Params, ap1Params []hostapd.Option, runIdx int,
	secConfFac security.ConfigFactory, stats *roamDiagnosticsStatsMap) {

	tf := s.FixtValue().(*wificell.TestFixture)

	attenuator := tf.Attenuator()
	resetAttenuation(ctx, s, attenuator)

	ap0, freq0, deconfig := wifiutil.ConfigureAP(ctx, s, ap0Params, 0, secConfFac)
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

	ap1, freq1, deconfig := wifiutil.ConfigureAP(ctx, s, ap1Params, 1, secConfFac)
	defer deconfig(ctx, ap1)
	ctx, cancel = tf.ReserveForDeconfigAP(ctx, ap1)
	defer cancel()

	wpaMonitor, stop, ctx, err := tf.StartWPAMonitor(ctx)
	if err != nil {
		s.Fatal("Failed to start wpa monitor")
	}
	defer stop()

	roamLog, err := os.OpenFile(filepath.Join(s.OutDir(), fmt.Sprintf("%d_roam.txt", runIdx)),
		roamDiagnosticsLogFileFlags, roamDiagnosticsLogFilePerm)
	if err != nil {
		s.Fatal("Failed to create roam log file: ", err)
	}
	defer roamLog.Close()

	for atten0 := roamDiagnosticsMinAttenuation; atten0 <= roamDiagnosticsMaxAttenuation; atten0 += roamDiagnosticsAttenuationStep {
		setTotalAttenuation(ctx, s, attenuator, 0, atten0, freq0)

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
				setTotalAttenuation(ctx, s, attenuator, 1, atten1, freq1)

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

// dumpRoamDiagnosticsStats prints statistics to log and to performance metrics system.
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

		totalRoams = wifiutil.Max(1, totalRoams)
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
//
// Bring an AP up, connect to it, set the attenuation, and vary a second AP
// around the same RSSI as the first AP. Perform a scan after every change in
// attenuation and observe when the device roams between APs. Record all roam
// events in a file for analysis.
// The purpose of this diagnostic is to capture the stickiness of the device's
// roam algorithm. For example, the stickier the algorithm, the more skewed
// toward higher RSSI differentials (between current and target AP) the
// distribution of roams in the output files will be. This is not necessarily
// a good thing as it's important for a device to be able to move between APs
// when it needs to. Therefore, we use RoamNatural in conjunction
// with this test to ensure that normal roam behavior is not broken.
func RoamDiagnostics(ctx context.Context, s *testing.State) {
	stats := roamDiagnosticsStatsMap{
		{2, 2}: {0},
		{2, 5}: {0},
		{5, 2}: {0},
	}

	testCases := s.Param().([]roamDiagnosticsTestcase)
	for i, tc := range testCases {
		executeRoamDiagnosticsTest(ctx, s, tc.apOpts1, tc.apOpts2, i, tc.secConfFac, &stats)
	}

	dumpRoamDiagnosticsStats(ctx, s, &stats)
}

// Copyright 2020 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wifi

import (
	"context"
	"time"

	"chromiumos/tast/common/network/iw"
	"chromiumos/tast/common/perf"
	"chromiumos/tast/errors"
	remoteiw "chromiumos/tast/remote/network/iw"
	"chromiumos/tast/remote/wificell"
	ap "chromiumos/tast/remote/wificell/hostapd"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: ScanPerf,
		Desc: "Measure BSS scan performance in various setup",
		Contacts: []string{
			"chromeos-wifi-champs@google.com", // WiFi oncall rotation; or http://b/new?component=893827
		},
		Attr:        []string{"group:wificell", "wificell_perf"},
		ServiceDeps: []string{wificell.TFServiceName},
		Fixture:     "wificellFixt",
	})
}

func ScanPerf(ctx context.Context, s *testing.State) {
	/*
		This test measures WiFi scan time with established network connection (background scan)
		or without (foreground scan) and compares with thresholds to indicate pass or not.
		Full (all channels) scan times are obtained as avg from tests with `scanTimes` times.
		Here are the steps:
		1- Configures the AP (e.g. specifies DTIM value).
		2- Performs single channel foreground scan.
		3- Performs full foreground scan multiple times.
		4- Full background scan:
		4-1- Connect DUT to AP
		4-2- Performs multiple scans.
		5- Deconfigures from defer() stack.
	*/

	const (
		// Repeated scan times to obtain averages.
		scanTimes = 5

		// Upper bounds for different scan methods.
		fgSingleChannelScanTimeout = time.Second
		fgFullScanTimeout          = 10 * time.Second
		bgFullScanTimeout          = 15 * time.Second
		pollTimeout                = 15 * time.Second
	)

	tf := s.FixtValue().(*wificell.TestFixture)

	options := wificell.DefaultOpenNetworkAPOptions()

	apIface, err := tf.ConfigureAP(ctx, options, nil)
	if err != nil {
		s.Fatal("Failed to configure the AP: ", err)
	}
	defer func(ctx context.Context) {
		if err := tf.DeconfigAP(ctx, apIface); err != nil {
			s.Error("Failed to deconfig the AP: ", err)
		}
	}(ctx)
	ctx, cancel := tf.ReserveForDeconfigAP(ctx, apIface)
	defer cancel()
	s.Log("AP setup done")

	ssid := apIface.Config().SSID
	freq, err := ap.ChannelToFrequency(apIface.Config().Channel)
	if err != nil {
		s.Fatalf("Failed to convert channel %d to frequency: %v", apIface.Config().Channel, err)
	}
	iface, err := tf.ClientInterface(ctx)
	if err != nil {
		s.Fatal("Failed to get DUT's interface: ", err)
	}
	iwr := remoteiw.NewRemoteRunner(s.DUT().Conn())

	pv := perf.NewValues()
	defer func() {
		if err := pv.Save(s.OutDir()); err != nil {
			s.Error("Failed to save perf data: ", err)
		}
	}()

	logDuration := func(label string, duration time.Duration) {
		pv.Set(perf.Metric{
			Name:      label,
			Variant:   "Average",
			Unit:      "seconds",
			Direction: perf.SmallerIsBetter,
		}, duration.Seconds())
		s.Logf("%s: %s", label, duration)
	}

	// pollTimedScan polls "iw scan" with specific SSID and returns scan duration.
	// Each scan takes at most scanTimeout, and the polling takes at most pollTimeout.
	pollTimedScan := func(ctx context.Context, freqs []int, scanTimeout, pollTimeout time.Duration, ssid, iface string, iwr *iw.Runner) (time.Duration, error) {
		var scanResult *iw.TimedScanData
		if pollTimeout < scanTimeout {
			pollTimeout = scanTimeout
		}
		err := testing.Poll(ctx, func(ctx context.Context) error {
			ctx, cancel := context.WithTimeout(ctx, scanTimeout)
			defer cancel()

			// Declare err to avoid multivariable short redeclaration as we don't want scanResult being shadowed.
			// We need to access scanResult after testing.Poll().
			var err error
			scanResult, err = iwr.TimedScan(ctx, iface, freqs, []string{ssid})
			if err != nil {
				return err
			}
			for _, bssList := range scanResult.BSSList {
				if bssList.SSID == ssid {
					return nil
				}
			}
			return errors.Errorf("iw scan found no SSID %s", ssid)
		}, &testing.PollOptions{Timeout: pollTimeout, Interval: 500 * time.Millisecond})
		if err != nil {
			return 0, err
		}
		return scanResult.Time, nil
	}

	// Foreground single channel scan.
	// Foreground scan means the scan is performed without any established connection.
	if duration, err := pollTimedScan(ctx, []int{freq}, fgSingleChannelScanTimeout, pollTimeout, ssid, iface, iwr); err != nil {
		s.Errorf("Failed to perform single channel scan at frequency %d: %v", freq, err)
	} else {
		logDuration("scan_time_foreground_single_scan", duration)
	}

	// Foreground full scan.
	count := 0
	var sum time.Duration
	for i := 1; i <= scanTimes; i++ {
		if duration, err := pollTimedScan(ctx, nil, fgFullScanTimeout, pollTimeout, ssid, iface, iwr); err != nil {
			s.Errorf("Failed to perform full channel scan at frequency %d: %v", freq, err)
		} else {
			s.Logf("Foreground scan #(%d/%d) duration: %s", i, scanTimes, duration)
			sum += duration
			count++
		}
	}
	if count == 0 {
		s.Errorf("Failed to perform all full channel scans at frequency %d in foreground scan test", freq)
	} else {
		avg := time.Duration(int64(sum) / int64(count))
		s.Logf("Foreground scan average duration: %s", avg)
		logDuration("scan_time_foreground_full", avg)
	}

	// Background full scan.
	ctx, restoreBg, err := tf.WifiClient().TurnOffBgscan(ctx)
	if err != nil {
		s.Fatal("Failed to turn off the background scan: ", err)
	}
	defer func() {
		if err := restoreBg(); err != nil {
			s.Error("Failed to restore the background scan config: ", err)
		}
	}()

	// DUT connecting to the AP.
	if _, err := tf.ConnectWifiAP(ctx, apIface); err != nil {
		s.Fatal("DUT: failed to connect to WiFi: ", err)
	}
	defer func(ctx context.Context) {
		if err := tf.CleanDisconnectWifi(ctx); err != nil {
			s.Error("Failed to disconnect WiFi, err: ", err)
		}
	}(ctx)
	ctx, cancel = tf.ReserveForDisconnect(ctx)
	defer cancel()
	s.Log("Connected")

	count = 0
	sum = 0
	for i := 1; i <= scanTimes; i++ {
		if duration, err := pollTimedScan(ctx, nil, bgFullScanTimeout, pollTimeout, ssid, iface, iwr); err != nil {
			s.Errorf("Failed to perform full channel scan at frequency %d: %v", freq, err)
		} else {
			s.Logf("Background scan #(%d/%d) duration: %s", i, scanTimes, duration)
			sum += duration
			count++
		}
	}
	if count == 0 {
		s.Errorf("Failed to perform all full channel scans at frequency %d in background scan test", freq)
	} else {
		avg := time.Duration(int64(sum) / int64(count))
		s.Logf("Background scan average duration: %s", avg)
		logDuration("scan_time_background_full", avg)
	}
}

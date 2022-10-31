// Copyright 2020 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wifi

import (
	"context"
	"fmt"
	"time"

	"github.com/golang/protobuf/ptypes/empty"

	"chromiumos/tast/common/network/iw"
	"chromiumos/tast/common/perf"
	"chromiumos/tast/errors"
	remoteiw "chromiumos/tast/remote/network/iw"
	"chromiumos/tast/remote/wificell"
	ap "chromiumos/tast/remote/wificell/hostapd"
	"chromiumos/tast/rpc"
	"chromiumos/tast/services/cros/wifi"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/wlan"
)

type scanPerfTestcase struct {
	apOpts []ap.Option
}

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
		Params: []testing.Param{
			{
				Name: "dtim1",
				Val: []scanPerfTestcase{{
					apOpts: []ap.Option{ap.DTIMPeriod(1)},
				}},
			},
			{
				Name: "dtim2",
				Val: []scanPerfTestcase{{
					apOpts: []ap.Option{ap.DTIMPeriod(2)},
				}},
			},
		},
	})
}

func ScanPerf(ctx context.Context, s *testing.State) {
	/*
		This test measures WiFi scan time with established network connection (background scan)
		or without (foreground scan) and compared with thresholds to indicate pass or not.
		Full (all channels) scan times are obtained as avg from tests with `scanTimes` times.
		Thresholds are applied to each single full scan test.
		Here are the steps:
		1- Configures the AP (e.g. specifies DTIM value).
		2- Performs single foreground scan.
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

		// Thresholds for scan tests.
		fgFullScanThreshold        = 4 * time.Second
		bgFullScanThreshold        = 7 * time.Second
		fgFullScanThresholdRelaxed = 5 * time.Second
		bgFullScanThresholdRelaxed = 8 * time.Second
	)

	// TODO(b/253099273): The following chipsets are known to have slower fg scan times.
	// Use relaxed threshold until the bug has been solved. Same for the bg scan.
	fgRelaxedChipSet := map[string]struct{}{
		wlan.DeviceNames[wlan.QualcommAtherosQCA6174]:     {},
		wlan.DeviceNames[wlan.QualcommWCN3990]:            {},
		wlan.DeviceNames[wlan.QualcommAtherosQCA6174SDIO]: {},
	}

	// TODO(b/253096914): The following chipsets are known to have slower bg scan times.
	bgRelaxedChipSet := map[string]struct{}{
		wlan.DeviceNames[wlan.MediaTekMT7921PCIE]: {},
		wlan.DeviceNames[wlan.MediaTekMT7921SDIO]: {},
	}

	// WiFi chips in this list will not be compared with a threshold and always pass.
	var exemptList []string
	// TODO(b/256486257): We lack data for WiFi6E models so threshold is not determined yet.
	wifi6e := []string{
		wlan.DeviceNames[wlan.IntelAX211],
		wlan.DeviceNames[wlan.QualcommWCN6855],
	}
	exemptList = append(exemptList, wifi6e...)

	tf := s.FixtValue().(*wificell.TestFixture)

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
	fgSingleScan := func(ctx context.Context, freq int, ssid, iface string, iwr *iw.Runner) {
		if duration, err := pollTimedScan(ctx, []int{freq}, fgSingleChannelScanTimeout, pollTimeout, ssid, iface, iwr); err != nil {
			s.Errorf("Failed to perform single channel scan at frequency %d: %v", freq, err)
		} else {
			logDuration("scan_time_foreground_single_scan", duration)
		}
	}

	fgFullScan := func(ctx context.Context, freq int, ssid, iface string, iwr *iw.Runner, devName string, exempt bool) {
		var fgFullScanSum time.Duration
		threshold := fgFullScanThreshold
		if _, ok := fgRelaxedChipSet[devName]; ok {
			threshold = fgFullScanThresholdRelaxed
			s.Logf("There is a known issue (b/253099273) for this WiFi chip (%s), use a relaxed threshold: %s", devName, threshold)
		}
		for i := 0; i < scanTimes; i++ {
			if duration, err := pollTimedScan(ctx, nil, fgFullScanTimeout, pollTimeout, ssid, iface, iwr); err != nil {
				s.Errorf("Failed to perform full channel scan at frequency %d: %v", freq, err)
			} else {
				if !exempt && duration > threshold {
					s.Errorf("Foreground scan #(%d/%d) duration: %s. Exceed threshold: %s", i+1, scanTimes, duration, threshold)
				} else {
					s.Logf("Foreground scan #(%d/%d) duration: %s", i+1, scanTimes, duration)
				}
				fgFullScanSum += duration
			}
		}
		fgFullScanAvg := fgFullScanSum / scanTimes
		s.Logf("Foreground scan average duration: %s", fgFullScanAvg)
		logDuration("scan_time_foreground_full", fgFullScanAvg)
	}

	bgFullScan := func(ctx context.Context, freq int, ssid, iface string, iwr *iw.Runner, devName string, exempt bool, apIface *wificell.APIface) {
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
		ctx, cancel := tf.ReserveForDisconnect(ctx)
		defer cancel()
		s.Log("Connected")

		var bgFullScanSum time.Duration
		threshold := bgFullScanThreshold
		if _, ok := bgRelaxedChipSet[devName]; ok {
			threshold = bgFullScanThresholdRelaxed
			s.Logf("There is a known issue (b/253096914) for this WiFi chip (%s), use a relaxed threshold: %s", devName, threshold)
		}
		for i := 0; i < scanTimes; i++ {
			if duration, err := pollTimedScan(ctx, nil, bgFullScanTimeout, pollTimeout, ssid, iface, iwr); err != nil {
				s.Errorf("Failed to perform full channel scan at frequency %d: %v", freq, err)
			} else {
				if !exempt && duration > threshold {
					s.Errorf("Background scan #(%d/%d) duration: %s. Exceed threshold: %s", i+1, scanTimes, duration, threshold)
				} else {
					s.Logf("Background scan #(%d/%d) duration: %s", i+1, scanTimes, duration)
				}
				bgFullScanSum += duration
			}
		}
		bgFullScanAvg := bgFullScanSum / scanTimes
		s.Logf("Background scan average duration: %s", bgFullScanAvg)
		logDuration("scan_time_background_full", bgFullScanAvg)
	}

	testOnce := func(ctx context.Context, s *testing.State, options []ap.Option, devName string, exempt bool) {
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

		s.Logf("DTIM is set as %d", apIface.Config().DTIMPeriod)

		fgSingleScan(ctx, freq, ssid, iface, iwr)
		fgFullScan(ctx, freq, ssid, iface, iwr, devName, exempt)
		bgFullScan(ctx, freq, ssid, iface, iwr, devName, exempt, apIface)
	}

	r, err := rpc.Dial(ctx, s.DUT(), s.RPCHint())
	if err != nil {
		s.Fatal("Failed to connect rpc: ", err)
	}
	defer r.Close(ctx)

	client := wifi.NewShillServiceClient(r.Conn)

	// Get the information of the WLAN device.
	devInfo, err := client.GetDeviceInfo(ctx, &empty.Empty{})
	if err != nil {
		s.Fatal("Failed obtaining WLAN device information through rpc: ", err)
	}

	exempt := false
	for _, n := range exemptList {
		if n == devInfo.Name {
			exempt = true
			s.Log("This device has no threshold for ScanPerf tests and will always pass")
			break
		}
	}

	testcases := s.Param().([]scanPerfTestcase)
	for i, tc := range testcases {
		subtest := func(ctx context.Context, s *testing.State) {
			options := append(wificell.DefaultOpenNetworkAPOptions(), tc.apOpts...)
			testOnce(ctx, s, options, devInfo.Name, exempt)
		}
		if !s.Run(ctx, fmt.Sprintf("Testcase #%d", i), subtest) {
			return
		}
	}

	s.Log("Tearing down")
}

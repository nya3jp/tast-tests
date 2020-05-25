// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wifi

import (
	"context"
	"time"

	"github.com/golang/protobuf/ptypes/empty"

	"chromiumos/tast/common/network/iw"
	"chromiumos/tast/common/perf"
	"chromiumos/tast/errors"
	remote_iw "chromiumos/tast/remote/network/iw"
	"chromiumos/tast/remote/wificell"
	"chromiumos/tast/remote/wificell/hostapd"
	"chromiumos/tast/services/cros/network"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:        ScanPerf,
		Desc:        "Measure BSS scan performance in various setup",
		Contacts:    []string{"deanliao@google.com", "chromeos-platform-connectivity@google.com"},
		Attr:        []string{"group:wificell", "wificell_func", "wificell_unstable"},
		ServiceDeps: []string{"tast.cros.network.WifiService"},
		Vars:        []string{"router"},
	})
}

func ScanPerf(fullCtx context.Context, s *testing.State) {
	var tfOps []wificell.TFOption
	if router, _ := s.Var("router"); router != "" {
		tfOps = append(tfOps, wificell.TFRouter(router))
	}

	tf, err := wificell.NewTestFixture(fullCtx, fullCtx, s.DUT(), s.RPCHint(), tfOps...)
	if err != nil {
		s.Fatal("Failed to set up test fixture: ", err)
	}
	defer func() {
		if err := tf.Close(fullCtx); err != nil {
			s.Log("Failed to tear down test fixture: ", err)
		}
	}()

	tfCtx, cancel := tf.ReserveForClose(fullCtx)
	defer cancel()

	ap, err := tf.DefaultOpenNetworkAP(tfCtx)
	if err != nil {
		s.Fatal("Failed to configure the AP: ", err)
	}
	defer func() {
		if err := tf.DeconfigAP(tfCtx, ap); err != nil {
			s.Error("Failed to deconfig the AP: ", err)
		}
	}()
	ctx, cancel := tf.ReserveForDeconfigAP(tfCtx, ap)
	defer cancel()
	s.Log("AP setup done")

	ssid := ap.Config().Ssid
	freq, err := hostapd.ChannelToFrequency(ap.Config().Channel)
	if err != nil {
		s.Logf("Failed to convert channel %d to frequency: %s", ap.Config().Channel, err)
	}
	iface, err := tf.ClientInterface(ctx)
	if err != nil {
		s.Log("Failed to get DUT's interface: ", err)
	}
	iwr := remote_iw.NewRunner(s.DUT().Conn())

	pv := perf.NewValues()
	defer func() {
		if err := pv.Save(s.OutDir()); err != nil {
			s.Log("Failed to save perf data: ", err)
		}
	}()

	logDuration := func(label string, duration time.Duration) {
		pv.Set(perf.Metric{
			Name:      label,
			Unit:      "seconds",
			Direction: perf.SmallerIsBetter,
		}, duration.Seconds())
		s.Logf("%s: %s", label, duration)
	}

	// pollTimedScan polls "iw scan" with specific SSID and returns scan duration.
	// Each scan takes at most scanTimeout, and the polling takes at most pollTimeout.
	pollTimedScan := func(ctx context.Context, freqs []int, scanTimeout, pollTimeout time.Duration) (time.Duration, error) {
		var scanResult *iw.TimedScanData
		if pollTimeout < scanTimeout {
			pollTimeout = scanTimeout
		}
		err := testing.Poll(ctx, func(ctx context.Context) error {
			ctx, cancel := context.WithTimeout(ctx, scanTimeout)
			defer cancel()

			// Declare err here is because we want to avoid redeclare scanResult in the
			// iwr.TimedScan() statement as we need to access it after testing.Poll().
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

	pollTimeout := 15 * time.Second
	// Single channel scan.
	if duration, err := pollTimedScan(ctx, []int{freq}, 10*time.Second, pollTimeout); err != nil {
		s.Errorf("Failed to perform single channel scan at frequency %d: %s", freq, err)
	} else {
		logDuration("scan_time_foreground_single_scan", duration)
	}

	// Foreground full scan.
	if duration, err := pollTimedScan(ctx, nil, 10*time.Second, pollTimeout); err != nil {
		s.Errorf("Failed to perform full channel scan at frequency %d: %s", freq, err)
	} else {
		logDuration("scan_time_foreground_full", duration)
	}

	// Background full scan.
	// DUT connecting to the AP.
	if _, err := tf.ConnectWifiAP(ctx, ap); err != nil {
		s.Fatal("DUT: failed to connect to WiFi: ", err)
	}
	defer func() {
		if _, err := tf.WifiClient().DeleteEntriesForSSID(ctx, &network.DeleteEntriesForSSIDRequest{Ssid: []byte(ssid)}); err != nil {
			s.Errorf("Failed to remove entries for ssid=%s, err: %v", ap.Config().Ssid, err)
		}
	}()
	s.Log("Connected")

	// Disable background scan.
	s.Log("Disable the DUT's WiFi background scan")
	method, err := tf.WifiClient().GetBgscanMethod(ctx, &empty.Empty{})
	if err != nil {
		s.Fatal("Unable to get the DUT's WiFi bgscan method: ", err)
	}
	if _, err := tf.WifiClient().SetBgscanMethod(ctx, &network.SetBgscanMethodRequest{Method: "none"}); err != nil {
		s.Fatal("Unable to stop the DUT's WiFi bgscan: ", err)
	}
	defer func() {
		s.Log("Restore the DUT's WiFi background scan")
		if _, err := tf.WifiClient().SetBgscanMethod(ctx, &network.SetBgscanMethodRequest{Method: method.Method}); err != nil {
			s.Errorf("Failed to restore the DUT's bgscan method to %s: %s", method.Method, err)
		}
	}()

	if duration, err := pollTimedScan(ctx, nil, 15*time.Second, pollTimeout); err != nil {
		s.Errorf("Failed to perform full channel scan at frequency %d: %s", freq, err)
	} else {
		logDuration("scan_time_foreground_full", duration)
	}
}

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
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	remoteiw "chromiumos/tast/remote/network/iw"
	"chromiumos/tast/remote/wificell"
	"chromiumos/tast/remote/wificell/hostapd"
	"chromiumos/tast/services/cros/network"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:     ScanPerf,
		Desc:     "Measure BSS scan performance in various setup",
		Contacts: []string{"deanliao@google.com", "chromeos-platform-connectivity@google.com"},
		// TODO(b/158433447): Run in a group for wificell-dependent perf tests.
		Attr:        []string{"group:wificell", "wificell_perf"},
		ServiceDeps: []string{"tast.cros.network.WifiService"},
		Vars:        []string{"router"},
	})
}

// Upper bounds for different scan methods.
const (
	fgSingleChannelScanTimeout = time.Second
	fgFullScanTimeout          = 10 * time.Second
	bgFullScanTimeout          = 15 * time.Second
)

func ScanPerf(ctx context.Context, s *testing.State) {
	var tfOps []wificell.TFOption
	if router, _ := s.Var("router"); router != "" {
		tfOps = append(tfOps, wificell.TFRouter(router))
	}

	tf, err := wificell.NewTestFixture(ctx, ctx, s.DUT(), s.RPCHint(), tfOps...)
	if err != nil {
		s.Fatal("Failed to set up test fixture: ", err)
	}
	defer func(ctx context.Context) {
		if err := tf.Close(ctx); err != nil {
			s.Error("Failed to tear down test fixture: ", err)
		}
	}(ctx)
	ctx, cancel := tf.ReserveForClose(ctx)
	defer cancel()

	ap, err := tf.DefaultOpenNetworkAP(ctx)
	if err != nil {
		s.Fatal("Failed to configure the AP: ", err)
	}
	defer func(ctx context.Context) {
		if err := tf.DeconfigAP(ctx, ap); err != nil {
			s.Error("Failed to deconfig the AP: ", err)
		}
	}(ctx)
	ctx, cancel = tf.ReserveForDeconfigAP(ctx, ap)
	defer cancel()
	s.Log("AP setup done")

	ssid := ap.Config().SSID
	freq, err := hostapd.ChannelToFrequency(ap.Config().Channel)
	if err != nil {
		s.Fatalf("Failed to convert channel %d to frequency: %v", ap.Config().Channel, err)
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

	pollTimeout := 15 * time.Second
	// Foreground single channel scan.
	// Foreground scan means the scan is performed without any established connection.
	if duration, err := pollTimedScan(ctx, []int{freq}, fgSingleChannelScanTimeout, pollTimeout); err != nil {
		s.Errorf("Failed to perform single channel scan at frequency %d: %v", freq, err)
	} else {
		logDuration("scan_time_foreground_single_scan", duration)
	}

	// Foreground full scan.
	if duration, err := pollTimedScan(ctx, nil, fgFullScanTimeout, pollTimeout); err != nil {
		s.Errorf("Failed to perform full channel scan at frequency %d: %v", freq, err)
	} else {
		logDuration("scan_time_foreground_full", duration)
	}

	// Background full scan, which means the scan is performed with a established connection.
	// Disable background scan mode first.
	s.Log("Disable the DUT's WiFi background scan")
	method, err := tf.WifiClient().GetBgscanMethod(ctx, &empty.Empty{})
	if err != nil {
		s.Fatal("Unable to get the DUT's WiFi bgscan method: ", err)
	}
	if _, err := tf.WifiClient().SetBgscanMethod(ctx, &network.SetBgscanMethodRequest{Method: "none"}); err != nil {
		s.Fatal("Unable to stop the DUT's WiFi bgscan: ", err)
	}
	defer func(ctx context.Context) {
		s.Log("Restore the DUT's WiFi background scan to ", method.Method)
		if _, err := tf.WifiClient().SetBgscanMethod(ctx, &network.SetBgscanMethodRequest{Method: method.Method}); err != nil {
			s.Errorf("Failed to restore the DUT's bgscan method to %s: %v", method.Method, err)
		}
	}(ctx)
	ctx, cancel = ctxutil.Shorten(ctx, 2*time.Second)
	defer cancel()

	// DUT connecting to the AP.
	if _, err := tf.ConnectWifiAP(ctx, ap, nil); err != nil {
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

	if duration, err := pollTimedScan(ctx, nil, bgFullScanTimeout, pollTimeout); err != nil {
		s.Errorf("Failed to perform full channel scan at frequency %d: %v", freq, err)
	} else {
		logDuration("scan_time_background_full", duration)
	}
}

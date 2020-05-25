// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wifi

import (
	"context"
	"time"

	"chromiumos/tast/common/perf"
	"chromiumos/tast/remote/network/iw"
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
	iwr := iw.NewRunner(s.DUT().Conn())

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
		}, float64(duration)/1e9)
		s.Logf("%s: %s", label, duration)
	}

	// Single channel scan.
	sCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	result, err := iwr.TimedScan(sCtx, iface, []int{freq}, []string{ssid})
	logDuration("scan_time_single_channel_scan", result.Time)

	// Foreground full scan.
	sCtx, cancel = context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	result, err = iwr.TimedScan(sCtx, iface, []int{}, []string{ssid})
	logDuration("scan_time_foreground_full_scan", result.Time)

	// Background full scan.
	// DUT connecting to the AP.
	if err := tf.ConnectWifi(ctx, ap); err != nil {
		s.Fatal("DUT: failed to connect to WiFi: ", err)
	}
	defer func() {
		if _, err := tf.WifiClient().DeleteEntriesForSSID(ctx, &network.DeleteEntriesForSSIDRequest{Ssid: ssid}); err != nil {
			s.Errorf("Failed to remove entries for ssid=%s, err: %v", ap.Config().Ssid, err)
		}
	}()
	s.Log("Connected")
	result, err = iwr.TimedScan(sCtx, iface, []int{}, []string{ssid})
	logDuration("scan_time_background_full_scan", result.Time)
}

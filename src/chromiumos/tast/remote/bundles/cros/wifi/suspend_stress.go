// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wifi

import (
	"context"
	"fmt"
	"math"
	"time"

	"chromiumos/tast/common/perf"
	"chromiumos/tast/common/wifi/security"
	"chromiumos/tast/common/wifi/security/wpa"
	"chromiumos/tast/remote/wificell"
	"chromiumos/tast/remote/wificell/hostapd"
	"chromiumos/tast/testing"
)

type suspendStressParam struct {
	suspendCount int
	apOps        []hostapd.Option
	// If unassigned, use default security config: open network.
	secConfFac security.ConfigFactory
}

func init() {
	testing.AddTest(&testing.Test{
		Func: SuspendStress,
		Desc: "Asserts WiFi connectivity after suspend-resume cycle using powerd_dbus_suspend command",
		Contacts: []string{
			"chharry@google.com",              // Test author
			"chromeos-wifi-champs@google.com", // WiFi oncall rotation; or http://b/new?component=893827
		},
		Attr:        []string{"group:wificell"},
		ServiceDeps: []string{wificell.TFServiceName},
		Fixture:     "wificellFixt",
		Params: []testing.Param{
			{
				Name:      "80211g",
				ExtraAttr: []string{"wificell_suspend", "wificell_unstable"},
				Val: []suspendStressParam{
					{suspendCount: 5, apOps: []hostapd.Option{hostapd.Channel(1), hostapd.Mode(hostapd.Mode80211g)}},
					{suspendCount: 5, apOps: []hostapd.Option{hostapd.Channel(6), hostapd.Mode(hostapd.Mode80211g)}},
					{suspendCount: 5, apOps: []hostapd.Option{hostapd.Channel(11), hostapd.Mode(hostapd.Mode80211g)}},
				},
			},
			{
				Name:      "80211n24ht40",
				ExtraAttr: []string{"wificell_suspend", "wificell_unstable"},
				Val: []suspendStressParam{
					{
						suspendCount: 5,
						apOps:        []hostapd.Option{hostapd.Channel(6), hostapd.Mode(hostapd.Mode80211nPure), hostapd.HTCaps(hostapd.HTCapHT40)},
					},
				},
			},
			{
				Name:      "80211n5ht40",
				ExtraAttr: []string{"wificell_suspend", "wificell_unstable"},
				Val: []suspendStressParam{
					{
						suspendCount: 5,
						apOps:        []hostapd.Option{hostapd.Channel(48), hostapd.Mode(hostapd.Mode80211nPure), hostapd.HTCaps(hostapd.HTCapHT40Minus)},
					},
				},
			},
			{
				Name:      "80211acvht80",
				ExtraAttr: []string{"wificell_suspend", "wificell_unstable"},
				Val: []suspendStressParam{
					{
						suspendCount: 5,
						apOps: []hostapd.Option{
							hostapd.Channel(157), hostapd.Mode(hostapd.Mode80211acPure), hostapd.HTCaps(hostapd.HTCapHT40Plus),
							hostapd.VHTChWidth(hostapd.VHTChWidth80), hostapd.VHTCenterChannel(155), hostapd.VHTCaps(hostapd.VHTCapSGI80),
						},
					},
				},
			},
			{
				Name:      "hidden",
				ExtraAttr: []string{"wificell_suspend", "wificell_unstable"},
				Val: []suspendStressParam{
					{suspendCount: 5, apOps: []hostapd.Option{hostapd.Channel(6), hostapd.Mode(hostapd.Mode80211g), hostapd.Hidden()}},
					{suspendCount: 5, apOps: []hostapd.Option{hostapd.Channel(36), hostapd.Mode(hostapd.Mode80211nPure), hostapd.Hidden(), hostapd.HTCaps(hostapd.HTCapHT20)}},
					{suspendCount: 5, apOps: []hostapd.Option{hostapd.Channel(48), hostapd.Mode(hostapd.Mode80211nPure), hostapd.Hidden(), hostapd.HTCaps(hostapd.HTCapHT20)}},
				},
			},
			{
				Name:      "wpa2",
				ExtraAttr: []string{"wificell_suspend", "wificell_unstable"},
				Val: []suspendStressParam{
					{
						suspendCount: 5,
						apOps:        []hostapd.Option{hostapd.Channel(1), hostapd.Mode(hostapd.Mode80211nPure), hostapd.HTCaps(hostapd.HTCapHT20)},
						secConfFac:   wpa.NewConfigFactory("chromeos", wpa.Mode(wpa.ModePureWPA2), wpa.Ciphers2(wpa.CipherCCMP)),
					},
				},
			},
			{
				Name:      "stress_80211n24ht40",
				ExtraAttr: []string{"wificell_stress", "wificell_unstable"},
				Timeout:   time.Hour * 3,
				Val: []suspendStressParam{
					{
						suspendCount: 690,
						apOps:        []hostapd.Option{hostapd.Channel(6), hostapd.Mode(hostapd.Mode80211nPure), hostapd.HTCaps(hostapd.HTCapHT40)},
					},
				},
			},
			{
				Name:      "stress_wpa2",
				ExtraAttr: []string{"wificell_stress", "wificell_unstable"},
				Timeout:   time.Hour * 3,
				Val: []suspendStressParam{
					{
						suspendCount: 690,
						apOps:        []hostapd.Option{hostapd.Channel(1), hostapd.Mode(hostapd.Mode80211nPure), hostapd.HTCaps(hostapd.HTCapHT20)},
						secConfFac:   wpa.NewConfigFactory("chromeos", wpa.Mode(wpa.ModePureWPA2), wpa.Ciphers2(wpa.CipherCCMP)),
					},
				},
			},
		},
	})
}

func SuspendStress(ctx context.Context, s *testing.State) {
	const (
		suspendTime = 10 * time.Second
	)

	tf := s.FixtValue().(*wificell.TestFixture)

	pv := perf.NewValues()
	defer func() {
		if err := pv.Save(s.OutDir()); err != nil {
			s.Log("Failed to save perf data, err: ", err)
		}
	}()

	testOnce := func(ctx context.Context, s *testing.State, suspendCount int, apOps []hostapd.Option, secConfFac security.ConfigFactory) {
		ap, err := tf.ConfigureAP(ctx, apOps, secConfFac)
		if err != nil {
			s.Fatal("Failed to configure ap: ", err)
		}
		defer func(ctx context.Context) {
			if err := tf.DeconfigAP(ctx, ap); err != nil {
				s.Error("Failed to deconfig ap: ", err)
			}
		}(ctx)
		ctx, cancel := tf.ReserveForDeconfigAP(ctx, ap)
		defer cancel()

		s.Log("AP setup done; connecting")

		if _, err := tf.ConnectWifiAP(ctx, ap); err != nil {
			s.Fatal("Failed to connect to WiFi: ", err)
		}
		defer func(ctx context.Context) {
			if err := tf.CleanDisconnectWifi(ctx); err != nil {
				s.Error("Failed to disconnect WiFi: ", err)
			}
		}(ctx)
		ctx, cancel = tf.ReserveForDisconnect(ctx)
		defer cancel()

		if err := tf.PingFromDUT(ctx, ap.ServerIP().String()); err != nil {
			s.Error("Failed to ping from the DUT: ", err)
		}

		s.Logf("Start suspend-resume for %d times", suspendCount)
		var connectTimes []float64
		for i := 0; i < suspendCount; i++ {
			connectTime, err := tf.SuspendAssertConnect(ctx, suspendTime)
			if err != nil {
				s.Error("Failed to assert WiFi connection after suspend-resume: ", err)
			} else {
				connectTimes = append(connectTimes, connectTime.Seconds())
			}
		}

		if len(connectTimes) == 0 {
			s.Error("Suspend stress finished; no successful suspend")
			return
		}

		// Calculate the fastest, slowest, and average connect time.
		fastest := math.Inf(1)
		slowest := math.Inf(-1)
		var total float64
		for _, t := range connectTimes {
			fastest = math.Min(fastest, t)
			slowest = math.Max(slowest, t)
			total += t
		}
		average := total / float64(len(connectTimes))
		s.Logf("Suspend stress finished; success rate=%d/%d; reconnect time (seconds): fastest=%f, slowest=%f, average=%f", len(connectTimes), suspendCount, fastest, slowest, average)

		name := "suspend_stress_reconnect_time_" + ap.Config().PerfDesc()
		pv.Set(perf.Metric{
			Name:      name,
			Variant:   "Fastest",
			Unit:      "seconds",
			Direction: perf.SmallerIsBetter,
		}, fastest)
		pv.Set(perf.Metric{
			Name:      name,
			Variant:   "Slowest",
			Unit:      "seconds",
			Direction: perf.SmallerIsBetter,
		}, slowest)
		pv.Set(perf.Metric{
			Name:      name,
			Variant:   "Average",
			Unit:      "seconds",
			Direction: perf.SmallerIsBetter,
		}, average)
	}

	testcases := s.Param().([]suspendStressParam)
	for i, tc := range testcases {
		subtest := func(ctx context.Context, s *testing.State) {
			testOnce(ctx, s, tc.suspendCount, tc.apOps, tc.secConfFac)
		}
		if !s.Run(ctx, fmt.Sprintf("Testcase #%d", i), subtest) {
			// Stop if one of the subtest's parameter set fails the test.
			return
		}
	}
}

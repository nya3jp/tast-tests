// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wifi

import (
	"context"

	"chromiumos/tast/common/network/ping"
	"chromiumos/tast/common/perf"
	"chromiumos/tast/common/wifi/security/wpa"
	remoteping "chromiumos/tast/remote/network/ping"
	"chromiumos/tast/remote/wificell"
	"chromiumos/tast/remote/wificell/hostapd"
	"chromiumos/tast/testing"
)

type ptkParam struct {
	rekeyPeriod      int
	pingCount        int
	pingInterval     float64
	allowedLossCount int
}

func init() {
	testing.AddTest(&testing.Test{
		Func:        PTK,
		Desc:        "Verifies that pairwise temporal key rotations works as expected",
		Contacts:    []string{"chharry@google.com", "chromeos-platform-connectivity@google.com"},
		Attr:        []string{"group:wificell", "wificell_func"},
		ServiceDeps: []string{wificell.TFServiceName},
		Pre:         wificell.TestFixturePreWithCapture(),
		Vars:        []string{"router", "pcap"},
		Params: []testing.Param{
			{
				// Default case.
				ExtraAttr: []string{"wificell_unstable"},
				// The ping configuration gives us around 75 seconds to ping,
				// which covers around 15 rekeys with 5 seconds period.
				Val: ptkParam{
					rekeyPeriod:      5,
					pingCount:        150,
					pingInterval:     0.5,
					allowedLossCount: 30, // Allow 20% ping loss.
				},
			},
			{
				// A stricter case for b/167149633.
				Name:      "low_ping_loss",
				ExtraAttr: []string{"wificell_unstable"},
				Val: ptkParam{
					rekeyPeriod:  5,
					pingCount:    150,
					pingInterval: 0.5,
					// One rekey contains ~10 pings. Let the threshold=8
					// so that the test will fail if we have problem in
					// any rekey.
					allowedLossCount: 8,
				},
			},
		},
	})
}

func PTK(ctx context.Context, s *testing.State) {
	param := s.Param().(ptkParam)

	tf := s.PreValue().(*wificell.TestFixture)
	defer func(ctx context.Context) {
		if err := tf.CollectLogs(ctx); err != nil {
			s.Log("Error collecting logs, err: ", err)
		}
	}(ctx)
	ctx, cancel := tf.ReserveForCollectLogs(ctx)
	defer cancel()

	apOps := []hostapd.Option{
		hostapd.Mode(hostapd.Mode80211nPure),
		hostapd.Channel(1), hostapd.HTCaps(hostapd.HTCapHT20),
	}
	secConfFac := wpa.NewConfigFactory(
		"chromeos", wpa.Mode(wpa.ModeMixed),
		wpa.Ciphers(wpa.CipherTKIP, wpa.CipherCCMP),
		wpa.Ciphers2(wpa.CipherCCMP),
		wpa.PTKRekeyPeriod(param.rekeyPeriod),
	)
	ap, err := tf.ConfigureAP(ctx, apOps, secConfFac)
	if err != nil {
		s.Fatal("Failed to configure ap: ", err)
	}
	defer func(ctx context.Context) {
		if err := tf.DeconfigAP(ctx, ap); err != nil {
			s.Error("Failed to deconfig ap: ", err)
		}
	}(ctx)
	ctx, cancel = tf.ReserveForDeconfigAP(ctx, ap)
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

	s.Logf("Pinging with count=%d interval=%g second(s)", param.pingCount, param.pingInterval)
	// As we need to record ping loss, we cannot use tf.PingFromDUT() here.
	pr := remoteping.NewRemoteRunner(s.DUT().Conn())
	res, err := pr.Ping(ctx, ap.ServerIP().String(), ping.Count(param.pingCount), ping.Interval(param.pingInterval))
	if err != nil {
		s.Fatal("Failed to ping from DUT: ", err)
	}
	s.Logf("Ping result=%+v", res)

	lossCount := res.Sent - res.Received
	if lossCount > param.allowedLossCount {
		s.Errorf("Unexpected packet loss: got %d, want <= %d", lossCount, param.allowedLossCount)
	}

	pv := perf.NewValues()
	pv.Set(perf.Metric{
		Name:      "ptk_ping_loss",
		Unit:      "percent",
		Direction: perf.SmallerIsBetter,
	}, res.Loss)
	if err := pv.Save(s.OutDir()); err != nil {
		s.Error("Failed to save perf data: ", err)
	}
}

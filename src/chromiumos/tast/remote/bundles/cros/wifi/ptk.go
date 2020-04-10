// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wifi

import (
	"context"
	"time"

	"chromiumos/tast/common/network/ping"
	"chromiumos/tast/common/perf"
	"chromiumos/tast/common/wifi/security/wpa"
	"chromiumos/tast/ctxutil"
	remoteping "chromiumos/tast/remote/network/ping"
	"chromiumos/tast/remote/wificell"
	"chromiumos/tast/remote/wificell/hostapd"
	"chromiumos/tast/services/cros/network"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:        PTK,
		Desc:        "Verifies that pairwise temporal key rotations works as expected",
		Contacts:    []string{"chharry@google.com", "chromeos-platform-connectivity@google.com"},
		Attr:        []string{"group:wificell", "wificell_func", "wificell_unstable"},
		ServiceDeps: []string{wificell.TFServiceName},
		Pre:         wificell.TestFixturePreWithCapture(),
		Vars:        []string{"router", "pcap"},
	})
}

func PTK(ctx context.Context, s *testing.State) {
	// The ping configuration gives us around 75 seconds to ping,
	// which covers around 15 rekeys with 5 seconds period.
	const (
		rekeyPeriod       = 5
		pingCount         = 150
		pingLossThreshold = 20.0 // Allow 20% ping loss.
		pingInterval      = 0.5
	)

	tf := s.PreValue().(*wificell.TestFixture)
	defer func() {
		if err := tf.CollectLogs(ctx); err != nil {
			s.Log("Error collecting logs, err: ", err)
		}
	}()
	ctx, cancel := ctxutil.Shorten(ctx, time.Second)
	defer cancel()

	apOps := []hostapd.Option{
		hostapd.Mode(hostapd.Mode80211nPure),
		hostapd.Channel(1), hostapd.HTCaps(hostapd.HTCapHT20),
	}
	secConfFac := wpa.NewConfigFactory(
		"chromeos", wpa.Mode(wpa.ModeMixed),
		wpa.Ciphers(wpa.CipherTKIP, wpa.CipherCCMP),
		wpa.Ciphers2(wpa.CipherCCMP),
		wpa.PTKRekeyPeriod(rekeyPeriod),
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

	ctx, _ = tf.ReserveForDeconfigAP(ctx, ap)

	s.Log("AP setup done; connecting")

	if _, err := tf.ConnectWifiAP(ctx, ap); err != nil {
		s.Fatal("Failed to connect to WiFi: ", err)
	}
	defer func() {
		if err := tf.DisconnectWifi(ctx); err != nil {
			s.Error("Failed to disconnect WiFi: ", err)
		}
		req := &network.DeleteEntriesForSSIDRequest{Ssid: []byte(ap.Config().SSID)}
		if _, err := tf.WifiClient().DeleteEntriesForSSID(ctx, req); err != nil {
			s.Errorf("Failed to remove entries for ssid=%s: %v", ap.Config().SSID, err)
		}
	}()

	s.Logf("Pinging with count=%d interval=%g second(s)", pingCount, pingInterval)
	// As we need to record ping loss, we cannot use tf.PingFromDUT() here.
	pr := remoteping.NewRemoteRunner(s.DUT().Conn())
	res, err := pr.Ping(ctx, ap.ServerIP().String(), ping.Count(pingCount), ping.Interval(pingInterval))
	if err != nil {
		s.Fatal("Failed to ping from DUT: ", err)
	}
	s.Logf("Ping result=%+v", res)

	if res.Loss > pingLossThreshold {
		s.Errorf("Unexpected packet loss percentage: got %g%%, want <= %g%%", res.Loss, pingLossThreshold)
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

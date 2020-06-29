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
	"chromiumos/tast/services/cros/network"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:        PTK,
		Desc:        "Verifies that pairwise temporal key rotations work as expected",
		Contacts:    []string{"chharry@google.com", "chromeos-platform-connectivity@google.com"},
		Attr:        []string{"group:wificell", "wificell_func", "wificell_unstable"},
		ServiceDeps: []string{"tast.cros.network.WifiService"},
		Vars:        []string{"router"},
	})
}

func PTK(ctx context.Context, s *testing.State) {
	// These settings combine to give us around 75 (150 * 0.5) seconds of ping time,
	// which should be around 15 rekeys with 5 seconds period.
	const (
		rekeyPeriod       = 5
		pingCount         = 150
		pingLossThreshold = 20
		pingInterval      = 0.5
	)

	router, _ := s.Var("router")
	tf, err := wificell.NewTestFixture(ctx, ctx, s.DUT(), s.RPCHint(), wificell.TFRouter(router))
	if err != nil {
		s.Fatal("Failed to set up test fixture: ", err)
	}
	defer func(ctx context.Context) {
		if err := tf.Close(ctx); err != nil {
			s.Error("Failed to tear down test fixture: ", err)
		}
	}(ctx)

	var cancel context.CancelFunc
	ctx, cancel = tf.ReserveForClose(ctx)
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

	ctx, cancel = tf.ReserveForDeconfigAP(ctx, ap)
	defer cancel()

	s.Log("AP setup done")

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
	s.Log("Connected")

	s.Logf("Pinging with count=%d interval=%g second(s)", pingCount, pingInterval)
	pr := remoteping.NewRemoteRunner(s.DUT().Conn())
	res, err := pr.Ping(ctx, ap.ServerIP().String(), ping.Count(pingCount), ping.Interval(pingInterval))
	if err != nil {
		s.Fatal("Failed to ping from DUT: ", err)
	}
	s.Logf("Ping result=%+v", res)

	pv := perf.NewValues()
	pv.Set(perf.Metric{
		Name:      "ptk_ping_loss",
		Unit:      "percent",
		Direction: perf.SmallerIsBetter,
	}, res.Loss)
	defer func() {
		if err := pv.Save(s.OutDir()); err != nil {
			s.Error("Failed to save perf data: ", err)
		}
	}()

	if res.Loss > pingLossThreshold {
		s.Errorf("Unexpected packet loss percentage: got %g%%, want <= %g%%", res.Loss, pingLossThreshold)
	}

	s.Log("Deconfiguring")
}

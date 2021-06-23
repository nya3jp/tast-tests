// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wifi

import (
	"context"
	"time"

	"chromiumos/tast/common/network/ping"
	"chromiumos/tast/common/perf"
	"chromiumos/tast/common/shillconst"
	"chromiumos/tast/common/wifi/security/wpa"
	"chromiumos/tast/ctxutil"
	remoteping "chromiumos/tast/remote/network/ping"
	"chromiumos/tast/remote/wificell"
	"chromiumos/tast/remote/wificell/hostapd"
	"chromiumos/tast/services/cros/wifi"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

type ptkParam struct {
	rekeyPeriod      int
	pingCount        int
	pingInterval     float64
	allowedLossCount int
}

// The ping configuration gives us around 75 seconds to ping,
// which covers around 15 rekeys with 5 seconds period.
var defaultPTKParam = ptkParam{
	rekeyPeriod:      5,
	pingCount:        150,
	pingInterval:     0.5,
	allowedLossCount: 30, // Allow 20% ping loss.
}

// TODO(b/183463918): remove the restriction once the bug is solved.
var ptkBuggyPlatform = []string{"kukui", "jacuzzi"}

func init() {
	testing.AddTest(&testing.Test{
		Func: PTK,
		Desc: "Verifies that pairwise temporal key rotations works as expected",
		Contacts: []string{
			"chharry@google.com",              // Test author
			"chromeos-wifi-champs@google.com", // WiFi oncall rotation; or http://b/new?component=893827
		},
		Attr:        []string{"group:wificell", "wificell_func"},
		ServiceDeps: []string{wificell.TFServiceName},
		Fixture:     "wificellFixtWithCapture",
		Params: []testing.Param{
			{
				// Default case.
				ExtraHardwareDeps: hwdep.D(hwdep.SkipOnPlatform(ptkBuggyPlatform...)),
				Val:               defaultPTKParam,
			},
			{
				// Qualcomm QCA6174A-3 case.
				// TODO(b/183463918): remove this once the issue is fixed.
				Name:              "qca6174a3",
				ExtraHardwareDeps: hwdep.D(hwdep.Platform(ptkBuggyPlatform...)),
				ExtraAttr:         []string{"wificell_unstable"},
				Val:               defaultPTKParam,
			},
		},
	})
}

func PTK(ctx context.Context, s *testing.State) {
	param := s.Param().(ptkParam)

	tf := s.FixtValue().(*wificell.TestFixture)

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
	ctx, cancel := tf.ReserveForDeconfigAP(ctx, ap)
	defer cancel()

	s.Log("AP setup done; connecting")

	connectResp, err := tf.ConnectWifiAP(ctx, ap)
	if err != nil {
		s.Fatal("Failed to connect to WiFi: ", err)
	}
	servicePath := connectResp.ServicePath
	defer func(ctx context.Context) {
		if err := tf.CleanDisconnectWifi(ctx); err != nil {
			s.Error("Failed to disconnect WiFi: ", err)
		}
	}(ctx)
	ctx, cancel = tf.ReserveForDisconnect(ctx)
	defer cancel()

	// Total rekey count less 2 for a buffer. We expect 2 transitions (false -> true, true -> false) for each rekey
	rekeyCount := int(float64(param.pingCount)*param.pingInterval/float64(param.rekeyPeriod)) - 2
	if rekeyCount <= 0 {
		s.Fatal("Ping duration is too short")
	}
	props := make([]*wificell.ShillProperty, rekeyCount*2)
	for i := range props {
		props[i] = &wificell.ShillProperty{
			Property:       shillconst.ServicePropertyWiFiRekeyInProgress,
			Method:         wifi.ExpectShillPropertyRequest_ON_CHANGE,
			ExpectedValues: []interface{}{i%2 == 0},
		}
	}
	monitorProps := []string{shillconst.ServicePropertyIsConnected}
	pingBuffer := 5 * time.Second
	waitBuffer := 5 * time.Second
	waitCtx, cancel := context.WithTimeout(ctx, time.Duration(float64(param.pingCount)*param.pingInterval)*time.Second+pingBuffer+waitBuffer)
	defer cancel()
	waitForProps, err := tf.ExpectShillProperty(waitCtx, servicePath, props, monitorProps)

	s.Logf("Pinging with count=%d interval=%g second(s)", param.pingCount, param.pingInterval)
	// As we need to record ping loss, we cannot use tf.PingFromDUT() here.
	pingCtx, cancel := ctxutil.Shorten(waitCtx, waitBuffer)
	defer cancel()
	pr := remoteping.NewRemoteRunner(s.DUT().Conn())
	res, err := pr.Ping(pingCtx, ap.ServerIP().String(), ping.Count(param.pingCount),
		ping.Interval(param.pingInterval), ping.SaveOutput("ptk_ping.log"))
	if err != nil {
		s.Fatal("Failed to ping from DUT: ", err)
	}
	s.Logf("Ping result=%+v", res)

	lossCount := res.Sent - res.Received
	if lossCount > param.allowedLossCount {
		s.Errorf("Unexpected packet loss: got %d, want <= %d", lossCount, param.allowedLossCount)
	}

	monitorResult, err := waitForProps()
	if err != nil {
		s.Error("Failed to wait for rekey events: ", err)
	}

	for _, ph := range monitorResult {
		if ph.Name == shillconst.ServicePropertyIsConnected {
			if !ph.Value.(bool) {
				s.Error("Failed to stay connected during rekey process")
			}
		}
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

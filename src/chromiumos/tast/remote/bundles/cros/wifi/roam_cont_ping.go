// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wifi

import (
	"context"
	"strconv"
	"time"

	"chromiumos/tast/common/network/ping"
	"chromiumos/tast/common/wifi/security/wpa"
	"chromiumos/tast/common/wifi/security/wpaeap"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/remote/bundles/cros/wifi/wifiutil"
	remoteping "chromiumos/tast/remote/network/ping"
	"chromiumos/tast/remote/wificell"
	"chromiumos/tast/remote/wificell/hostapd"
	"chromiumos/tast/remote/wificell/verifier"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

type pingParam struct {
	opts []ping.Option
	// Max packets lost per roaming round.
	maxLoss int
}

func init() {
	testing.AddTest(&testing.Test{
		Func: RoamContPing,
		Desc: "Send ping every 10ms and check how many packets are lost on average during roaming",
		Contacts: []string{
			"jck@semihalf.com",                // Test author
			"chromeos-wifi-champs@google.com", // WiFi oncall rotation; or http://b/new?component=893827
		},
		Attr:         []string{"group:wificell", "wificell_perf", "wificell_unstable"},
		ServiceDeps:  []string{wificell.TFServiceName},
		HardwareDeps: hwdep.D(hwdep.Wifi80211ac()),
		Fixture:      "wificellFixt",
		Timeout:      time.Minute * 5, // The average test time doubled.
		Vars:         []string{"wifi.RoamContPing.rounds"},
		Params: []testing.Param{{
			Name: "none",
			Val: wifiutil.ContParam{
				Rounds: 50,
				ApOpts: [2][]hostapd.Option{{hostapd.Channel(1), hostapd.Mode(hostapd.Mode80211g)},
					{hostapd.Channel(157), hostapd.Mode(hostapd.Mode80211acPure), hostapd.HTCaps(hostapd.HTCapHT40Plus), hostapd.VHTCaps(hostapd.VHTCapSGI80),
						hostapd.VHTChWidth(hostapd.VHTChWidth80), hostapd.VHTCenterChannel(155)}},
				Param: pingParam{
					opts: []ping.Option{ping.Count(1000), ping.Interval(0.01)},
					// With 10ms ping interval, abnormal time without service would be 25 packets = 250ms.
					maxLoss: 25,
				},
			},
		}, {
			Name: "psk",
			Val: wifiutil.ContParam{
				Rounds: 50,
				ApOpts: [2][]hostapd.Option{{hostapd.Channel(1), hostapd.Mode(hostapd.Mode80211g)},
					{hostapd.Channel(157), hostapd.Mode(hostapd.Mode80211acPure), hostapd.HTCaps(hostapd.HTCapHT40Plus), hostapd.VHTCaps(hostapd.VHTCapSGI80),
						hostapd.VHTChWidth(hostapd.VHTChWidth80), hostapd.VHTCenterChannel(155)}},
				SecConfFac: wpa.NewConfigFactory("chromeos", wpa.Mode(wpa.ModePureWPA2), wpa.Ciphers2(wpa.CipherCCMP)),
				Param: pingParam{
					opts:    []ping.Option{ping.Count(1000), ping.Interval(0.01)},
					maxLoss: 25,
				},
			},
		}, {
			Name: "ft_psk",
			Val: wifiutil.ContParam{
				Rounds: 50,
				ApOpts: [2][]hostapd.Option{{hostapd.Channel(1), hostapd.Mode(hostapd.Mode80211g)},
					{hostapd.Channel(157), hostapd.Mode(hostapd.Mode80211acPure), hostapd.HTCaps(hostapd.HTCapHT40Plus), hostapd.VHTCaps(hostapd.VHTCapSGI80),
						hostapd.VHTChWidth(hostapd.VHTChWidth80), hostapd.VHTCenterChannel(155)}},
				SecConfFac: wpa.NewConfigFactory("chromeos", wpa.Mode(wpa.ModePureWPA2), wpa.Ciphers2(wpa.CipherCCMP), wpa.FTMode(wpa.FTModePure)),
				EnableFT:   true,
				Param: pingParam{
					opts:    []ping.Option{ping.Count(1000), ping.Interval(0.01)},
					maxLoss: 25,
				},
			},
		}, {
			Name: "eap",
			Val: wifiutil.ContParam{
				Rounds: 50,
				ApOpts: [2][]hostapd.Option{{hostapd.Channel(1), hostapd.Mode(hostapd.Mode80211g)},
					{hostapd.Channel(157), hostapd.Mode(hostapd.Mode80211acPure), hostapd.HTCaps(hostapd.HTCapHT40Plus), hostapd.VHTCaps(hostapd.VHTCapSGI80),
						hostapd.VHTChWidth(hostapd.VHTChWidth80), hostapd.VHTCenterChannel(155)}},
				SecConfFac: wpaeap.NewConfigFactory(
					wifiutil.Cert1.CACred.Cert, wifiutil.Cert1.ServerCred,
					wpaeap.ClientCACert(wifiutil.Cert1.CACred.Cert), wpaeap.ClientCred(wifiutil.Cert1.ClientCred),
					wpaeap.Mode(wpa.ModePureWPA2),
				),
				Param: pingParam{
					opts:    []ping.Option{ping.Count(1000), ping.Interval(0.01)},
					maxLoss: 25,
				},
			},
		}, {
			Name: "ft_eap",
			Val: wifiutil.ContParam{
				Rounds: 50,
				ApOpts: [2][]hostapd.Option{{hostapd.Channel(1), hostapd.Mode(hostapd.Mode80211g)},
					{hostapd.Channel(157), hostapd.Mode(hostapd.Mode80211acPure), hostapd.HTCaps(hostapd.HTCapHT40Plus), hostapd.VHTCaps(hostapd.VHTCapSGI80),
						hostapd.VHTChWidth(hostapd.VHTChWidth80), hostapd.VHTCenterChannel(155)}},
				SecConfFac: wpaeap.NewConfigFactory(
					wifiutil.Cert1.CACred.Cert, wifiutil.Cert1.ServerCred,
					wpaeap.ClientCACert(wifiutil.Cert1.CACred.Cert), wpaeap.ClientCred(wifiutil.Cert1.ClientCred),
					wpaeap.Mode(wpa.ModePureWPA2), wpaeap.FTMode(wpa.FTModePure),
				),
				EnableFT: true,
				Param: pingParam{
					opts:    []ping.Option{ping.Count(1000), ping.Interval(0.01)},
					maxLoss: 25,
				},
			},
		}},
	})
}

func RoamContPing(ctx context.Context, s *testing.State) {
	tf := s.FixtValue().(*wificell.TestFixture)

	param := s.Param().(wifiutil.ContParam)

	// Allow override
	var rounds int
	roundsStr, ok := s.Var("wifi.RoamContPing.rounds")
	if !ok {
		rounds = param.Rounds
	} else {
		var err error
		rounds, err = strconv.Atoi(roundsStr)
		if err != nil {
			s.Fatal("Failed to convert value, err: ", err)
		}
	}

	ctx, ct, finish := wifiutil.ContinuityTestInitialSetup(ctx, s, tf)
	defer finish()
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	var vf *verifier.Verifier
	var resultAssertF func(context.Context, []verifier.ResultType)
	pingF := func(ctx context.Context) (verifier.ResultType, error) {
		// We need more result data than a simple tf.PingFromDUT(), so we use a separate runner.
		pr := remoteping.NewRemoteRunner(s.DUT().Conn())
		res, err := pr.Ping(ctx, wifiutil.ServerIP(), param.Param.(pingParam).opts...)
		if err != nil {
			testing.ContextLog(ctx, "ping error: ", err)
			return verifier.ResultType{}, err
		}
		testing.ContextLogf(ctx, "Continuity: ping statistics=%+v", res)
		return verifier.ResultType{Data: res, Timestamp: time.Now()}, nil
	}
	resultAssertF = func(ctx context.Context, results []verifier.ResultType) {
		var sent, received int
		for i, ret := range results {
			pingData := ret.Data.(*ping.Result)
			testing.ContextLogf(ctx, "Iteration %d: End Time=%s, Packets lost=%d",
				i+1, ret.Timestamp.Format("15:04:05.000"), pingData.Sent-pingData.Received)
			sent += pingData.Sent
			received += pingData.Received
		}
		loss := sent - received
		testing.ContextLogf(ctx, "Total packets lost=%d/%d (%d per round)",
			loss, sent, loss/rounds)

		if loss > rounds*param.Param.(pingParam).maxLoss {
			s.Fatal("Loss threshold exceeded")
		}
	}
	vf = verifier.NewVerifier(ctx, pingF)
	defer vf.Finish()
	ctx, cancel = ctxutil.Shorten(ctx, time.Second)
	defer cancel()

	ctx, destroy := ct.ContinuityTestSetupFinalize(ctx, s)
	defer destroy()
	ctx, cancel = ctxutil.Shorten(ctx, time.Second)
	defer cancel()

	vf.StartJob()

	for i := 0; i < rounds; i++ {
		ct.ContinuityRound(ctx, s, i)
	}
	results, err := vf.StopJob()
	if err != nil {
		s.Fatal("Error while receiving verification results, err: ", err)
		return
	}
	resultAssertF(ctx, results)
}

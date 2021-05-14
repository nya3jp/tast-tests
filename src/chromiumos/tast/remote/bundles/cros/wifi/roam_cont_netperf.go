// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wifi

import (
	"context"
	"strconv"
	"time"

	"chromiumos/tast/common/wifi/security/wpa"
	"chromiumos/tast/common/wifi/security/wpaeap"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/remote/bundles/cros/wifi/wifiutil"
	"chromiumos/tast/remote/network/netperf"
	"chromiumos/tast/remote/wificell"
	"chromiumos/tast/remote/wificell/hostapd"
	ap "chromiumos/tast/remote/wificell/hostapd"
	"chromiumos/tast/remote/wificell/verifier"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

type netperfParam struct {
	testType  netperf.TestType
	threshold float64
}

var netperfApOpts [2][]hostapd.Option = [2][]hostapd.Option{
	{hostapd.Channel(48), hostapd.Mode(ap.Mode80211nPure), hostapd.HTCaps(hostapd.HTCapHT40)},
	{hostapd.Channel(6), hostapd.Mode(ap.Mode80211nPure), hostapd.HTCaps(hostapd.HTCapHT40)}}

func init() {
	testing.AddTest(&testing.Test{
		Func: RoamContNetperf,
		Desc: "See how much performance drops during roaming",
		Contacts: []string{
			"jck@semihalf.com",                // Test author
			"chromeos-wifi-champs@google.com", // WiFi oncall rotation; or http://b/new?component=893827
		},
		Attr:         []string{"group:wificell", "wificell_perf", "wificell_unstable"},
		ServiceDeps:  []string{wificell.TFServiceName},
		HardwareDeps: hwdep.D(hwdep.Wifi80211ac()),
		Fixture:      "wificellFixtWithCapture",
		Timeout:      time.Minute * 10, // The average test time doubled.
		Vars:         []string{"router", "pcap", "wifi.RoamContNetperf.rounds"},
		Params: []testing.Param{{
			Name: "stream_none", // From DUT's PoV, Stream are upload tests.
			Val: wifiutil.ContParam{
				Rounds:   50,
				ApOpts:   netperfApOpts,
				EnableFT: false,
				Param:    netperfParam{testType: netperf.TestTypeTCPStream, threshold: 0.8},
			},
		}, {
			Name: "maerts_none", // From DUT's PoV, Maerts are download tests.
			Val: wifiutil.ContParam{
				Rounds:   50,
				ApOpts:   netperfApOpts,
				EnableFT: false,
				Param:    netperfParam{testType: netperf.TestTypeTCPMaerts, threshold: 0.8},
			},
		}, {
			Name: "stream_psk",
			Val: wifiutil.ContParam{
				Rounds:     50,
				ApOpts:     netperfApOpts,
				SecConfFac: wpa.NewConfigFactory("chromeos", wpa.Mode(wpa.ModePureWPA2), wpa.Ciphers2(wpa.CipherCCMP)),
				EnableFT:   false,
				Param:      netperfParam{testType: netperf.TestTypeTCPStream, threshold: 0.8},
			},
		}, {
			Name: "stream_ft_psk",
			Val: wifiutil.ContParam{
				Rounds:     50,
				ApOpts:     netperfApOpts,
				SecConfFac: wpa.NewConfigFactory("chromeos", wpa.Mode(wpa.ModePureWPA2), wpa.Ciphers2(wpa.CipherCCMP)),
				EnableFT:   true,
				Param:      netperfParam{testType: netperf.TestTypeTCPStream, threshold: 0.8},
			},
		}, {
			Name: "stream_eap",
			Val: wifiutil.ContParam{
				Rounds: 50,
				ApOpts: netperfApOpts,
				SecConfFac: wpaeap.NewConfigFactory(
					wifiutil.Cert1.CACred.Cert, wifiutil.Cert1.ServerCred,
					wpaeap.ClientCACert(wifiutil.Cert1.CACred.Cert), wpaeap.ClientCred(wifiutil.Cert1.ClientCred),
					wpaeap.Mode(wpa.ModePureWPA2),
				),
				EnableFT: false,
				Param:    netperfParam{testType: netperf.TestTypeTCPStream, threshold: 0.8},
			},
		}, {
			Name: "stream_ft_eap",
			Val: wifiutil.ContParam{
				Rounds: 50,
				ApOpts: netperfApOpts,
				SecConfFac: wpaeap.NewConfigFactory(
					wifiutil.Cert1.CACred.Cert, wifiutil.Cert1.ServerCred,
					wpaeap.ClientCACert(wifiutil.Cert1.CACred.Cert), wpaeap.ClientCred(wifiutil.Cert1.ClientCred),
					wpaeap.Mode(wpa.ModePureWPA2),
				),
				EnableFT: true,
				Param:    netperfParam{testType: netperf.TestTypeTCPStream, threshold: 0.8},
			},
		}, {
			Name: "maerts_psk",
			Val: wifiutil.ContParam{
				Rounds:     50,
				ApOpts:     netperfApOpts,
				SecConfFac: wpa.NewConfigFactory("chromeos", wpa.Mode(wpa.ModePureWPA2), wpa.Ciphers2(wpa.CipherCCMP)),
				EnableFT:   false,
				Param:      netperfParam{testType: netperf.TestTypeTCPMaerts, threshold: 0.8},
			},
		}, {
			Name: "maerts_ft_psk",
			Val: wifiutil.ContParam{
				Rounds:     50,
				ApOpts:     netperfApOpts,
				SecConfFac: wpa.NewConfigFactory("chromeos", wpa.Mode(wpa.ModePureWPA2), wpa.Ciphers2(wpa.CipherCCMP)),
				EnableFT:   true,
				Param:      netperfParam{testType: netperf.TestTypeTCPMaerts, threshold: 0.8},
			},
		}, {
			Name: "maerts_eap",
			Val: wifiutil.ContParam{
				Rounds: 50,
				ApOpts: netperfApOpts,
				SecConfFac: wpaeap.NewConfigFactory(
					wifiutil.Cert1.CACred.Cert, wifiutil.Cert1.ServerCred,
					wpaeap.ClientCACert(wifiutil.Cert1.CACred.Cert), wpaeap.ClientCred(wifiutil.Cert1.ClientCred),
					wpaeap.Mode(wpa.ModePureWPA2),
				),
				EnableFT: false,
				Param:    netperfParam{testType: netperf.TestTypeTCPMaerts, threshold: 0.8},
			},
		}, {
			Name: "maerts_ft_eap",
			Val: wifiutil.ContParam{
				Rounds: 50,
				ApOpts: netperfApOpts,
				SecConfFac: wpaeap.NewConfigFactory(
					wifiutil.Cert1.CACred.Cert, wifiutil.Cert1.ServerCred,
					wpaeap.ClientCACert(wifiutil.Cert1.CACred.Cert), wpaeap.ClientCred(wifiutil.Cert1.ClientCred),
					wpaeap.Mode(wpa.ModePureWPA2),
				),
				EnableFT: true,
				Param:    netperfParam{testType: netperf.TestTypeTCPMaerts, threshold: 0.8},
			},
		}},
	})
}

func RoamContNetperf(ctx context.Context, s *testing.State) {
	tf := s.FixtValue().(*wificell.TestFixture)
	defer func(ctx context.Context) {
		if err := tf.CollectLogs(ctx); err != nil {
			s.Log("Error collecting logs, err: ", err)
		}
	}(ctx)
	ctx, cancel := tf.ReserveForCollectLogs(ctx)
	defer cancel()

	param := s.Param().(wifiutil.ContParam)

	// Allow override
	var rounds int
	roundsStr, ok := s.Var("wifi.RoamContNetperf.rounds")
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
	ctx, cancel = ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	var vf *verifier.Verifier
	addrs, err := tf.ClientIPv4Addrs(ctx)
	if err != nil || len(addrs) == 0 {
		s.Fatal("Failed to get the IP address, err: ", err)
	}
	pcap, _ := tf.LegacyPcap()
	serverConn := pcap.Conn()

	// If we actually have pcap available, we can attempt at moving packet generation there.
	if serverConn != ct.Router().Conn() {
		s.Log("Separate pcap router detected, setting up test in two-router setup")

		ctx, pcapFinish := ct.SetupPcapRouterConnection(ctx, s, ct.Router().Conn(), serverConn)
		defer pcapFinish()
		ctx, cancel = ctxutil.Shorten(ctx, 5*time.Second)
		defer cancel()

	} else {
		s.Fatal("pcap router not detected")
	}

	session, err := netperf.NewContinuousSession(ctx,
		s.DUT().Conn(), addrs[0].String(),
		serverConn, wifiutil.PerfServerIP.String(),
		netperf.Config{
			TestType: param.Param.(netperfParam).testType,
			TestTime: 5 * time.Second})
	if err != nil {
		s.Fatal("Failed to create session, err: ", err)
	}
	defer session.Close(ctx)

	err = session.WarmupStations(ctx)
	if err != nil {
		s.Fatal("Failed to warmup stations, err: ", err)
	}
	res, err := session.Run(ctx)
	if err != nil {
		s.Fatal("netperf error: ", err)
	}
	s.Logf("Achieved starting throughput: %f Mbps", res[0].Measurements[netperf.CategoryThroughput])
	var targetThroughput float64
	if serverConn == ct.Router().Conn() {
		// In degraded, one-router test we must be more tolerant for results.
		targetThroughput = res[0].Measurements[netperf.CategoryThroughput] * 0.3
	} else {
		// In two-router setup, let's target the real throughput threshold.
		targetThroughput = res[0].Measurements[netperf.CategoryThroughput] * param.Param.(netperfParam).threshold
	}
	netperfF := func(ctx context.Context) (verifier.ResultType, error) {
		res, err := session.Run(ctx)
		if err != nil {
			testing.ContextLog(ctx, "netperf error: ", err)
			return verifier.ResultType{}, err
		}
		if len(res) == 0 {
			testing.ContextLog(ctx, "netperf returned empty result")
			return verifier.ResultType{}, nil
		}
		testing.ContextLogf(ctx, "Continuity: netperf statistics=%+v", res)
		return verifier.ResultType{Data: res[0], Timestamp: time.Now()}, nil
	}

	vf = verifier.NewVerifier(ctx, netperfF)
	defer vf.Finish()
	ctx, cancel = ctxutil.Shorten(ctx, time.Second)
	defer cancel()

	ctx, destroy := ct.ContinuityTestSetupFinalize(ctx, s)
	defer destroy()
	ctx, cancel = ctxutil.Shorten(ctx, time.Second)
	defer cancel()

	var roundResults []verifier.ResultType
	for i := 0; i < rounds; i++ {
		vf.StartJob()
		ct.ContinuityRound(ctx, s, i)
		results, err := vf.StopJob()
		if err != nil {
			s.Fatal("Error while receiving verification results, err: ", err)
			return
		}
		for _, ret := range results {
			result := ret.Data.(*netperf.Result)
			if result == nil {
				testing.ContextLog(ctx, "Skipping empty result")
				continue
			}
			roundResults = append(roundResults, ret)
		}
	}

	var history netperf.History
	for i, ret := range roundResults {
		result := ret.Data.(*netperf.Result)
		testing.ContextLogf(ctx, "Iteration %d: End Time=%s, Throughput=%f",
			i+1,
			ret.Timestamp.Format("15:04:05.000"),
			result.Measurements[netperf.CategoryThroughput])
		history = append(history, result)
	}

	aggregateResult, err := netperf.AggregateSamples(ctx, history)
	if err != nil {
		s.Fatal("Samples aggregation error: ", err)
	}
	testing.ContextLogf(ctx, "Average throughput %f, threshold %f",
		aggregateResult.Measurements[netperf.CategoryThroughput], targetThroughput)
	if aggregateResult.Measurements[netperf.CategoryThroughput] < targetThroughput {
		s.Fatal("Throughput too low")
	}
}

// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package network

import (
	"context"

	"github.com/golang/protobuf/ptypes/empty"

	"chromiumos/tast/common/network/diag"
	"chromiumos/tast/remote/wificell"
	"chromiumos/tast/remote/wificell/hostapd"
	"chromiumos/tast/rpc"
	"chromiumos/tast/services/cros/network"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: DiagFailSignalStrength,
		Desc: "Tests that the WiFi signal strength network diagnostic routine fails when the signal strength is below a threshold",
		Contacts: []string{
			"tbegin@chromium.org",            // test author
			"khegde@chromium.org",            // network diagnostics author
			"cros-network-health@google.com", // network-health team
		},
		ServiceDeps:  []string{wificell.TFServiceName, "tast.cros.network.NetDiagService"},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:wificell", "wificell_cq", "wificell_unstable"},
		Pre:          wificell.TestFixturePreWithFeatures(wificell.TFFeaturesRouters | wificell.TFFeaturesAttenuator),
		Vars:         []string{"routers", "pcap", "attenuator"},
	})
}

// DiagFailSignalStrength tests that when the WiFi signal is attenuated, the WiFi
// signal strength network diagnostics routine fails.
func DiagFailSignalStrength(ctx context.Context, s *testing.State) {
	const channel = 1
	var apOpts = []hostapd.Option{
		hostapd.Mode(hostapd.Mode80211nPure),
		hostapd.Channel(channel),
		hostapd.HTCaps(hostapd.HTCapHT20),
		hostapd.SSID(hostapd.RandomSSID("TAST_SIGNAL_STRENGTH_")),
	}

	tf := s.PreValue().(*wificell.TestFixture)
	ap, err := tf.ConfigureAP(ctx, apOpts, nil)
	if err != nil {
		s.Fatal("Failed to configure AP: ", err)
	}
	defer tf.DeconfigAP(ctx, ap)
	ctx, cancel := tf.ReserveForDeconfigAP(ctx, ap)
	defer cancel()

	attenuator := tf.Attenuator()
	minAtten, err := attenuator.MinTotalAttenuation(channel)
	if err != nil {
		s.Fatal("Failed to get minimal attenuation")
	}
	freq, err := hostapd.ChannelToFrequency(ap.Config().Channel)
	if err != nil {
		s.Fatal("Failed to get AP frequency: ", err)
	}
	if err := attenuator.SetTotalAttenuation(ctx, channel, minAtten, freq); err != nil {
		s.Fatal("Failed to set attenuation: ", err)
	}

	if _, err := tf.ConnectWifiAP(ctx, ap); err != nil {
		s.Fatal("Failed to connectt WiFi AP: ", err)
	}
	defer tf.CleanDisconnectWifi(ctx)
	ctx, cancel = tf.ReserveForDisconnect(ctx)
	defer cancel()

	if err := tf.VerifyConnection(ctx, ap); err != nil {
		s.Fatal("Failed to verify connection: ", err)
	}

	cl, err := rpc.Dial(ctx, s.DUT(), s.RPCHint(), "cros")
	if err != nil {
		s.Fatal("Failed to connect to the RPC service on the DUT: ", err)
	}

	di := network.NewNetDiagServiceClient(cl.Conn)

	_, err = di.SetupDiagAPI(ctx, &empty.Empty{})
	if err != nil {
		s.Fatal("Failed to setup diag API: ", err)
	}

	req := &network.RunRoutineRequest{
		Routine: diag.RoutineSignalStrength,
	}
	res, err := di.RunRoutine(ctx, req)
	if err != nil {
		s.Fatal("Failed to run diag routine: ", err)
	}

	result := &diag.RoutineResult{
		Verdict:  diag.RoutineVerdict(res.Verdict),
		Problems: res.Problems,
	}
	const problemWeakSignal = 0
	expectedResult := &diag.RoutineResult{
		Verdict:  diag.VerdictProblem,
		Problems: []uint32{problemWeakSignal},
	}
	if err := diag.CheckRoutineResult(result, expectedResult); err != nil {
		s.Fatal("Routine result did not match: ", err)
	}
}

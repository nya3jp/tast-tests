// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package network

import (
	"context"
	"time"

	"github.com/golang/protobuf/ptypes/empty"

	"chromiumos/tast/common/network/diag"
	"chromiumos/tast/errors"
	"chromiumos/tast/remote/wificell"
	"chromiumos/tast/remote/wificell/hostapd"
	"chromiumos/tast/rpc"
	"chromiumos/tast/services/cros/network"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: DiagSignalStrength,
		Desc: "Tests that the WiFi signal strength network diagnostic routine reports the correct verdict if the signal strength is both attenuated and unattenuated",
		Contacts: []string{
			"tbegin@chromium.org",            // test author
			"khegde@chromium.org",            // network diagnostics author
			"cros-network-health@google.com", // network-health team
		},
		ServiceDeps:  []string{wificell.TFServiceName, "tast.cros.network.NetDiagService"},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:wificell_roam", "wificell_roam_perf"},
		Pre:          wificell.TestFixturePreWithFeatures(wificell.TFFeaturesRouters | wificell.TFFeaturesAttenuator),
		Vars:         []string{"routers", "pcap", "attenuator"},
		Timeout:      time.Minute * 2,
	})
}

// DiagSignalStrength tests that when the WiFi signal is unattenuated, the WiFi
// signal strength network diagnostics routine passes, and when the signal is
// attenuated, the routine fails.
func DiagSignalStrength(ctx context.Context, s *testing.State) {
	var apOpts = []hostapd.Option{
		hostapd.Mode(hostapd.Mode80211nPure),
		hostapd.Channel(1),
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
	setAttenuation := func(attenDb float64) error {
		for c := 0; c <= 3; c++ {
			if err := attenuator.SetAttenuation(ctx, c, attenDb); err != nil {
				return err
			}
		}
		return nil
	}

	// Set attenuator to minimum value and test that the signal strength is strong.
	if err := setAttenuation(0); err != nil {
		s.Fatal("Failed to set minimum attenuation: ", err)
	}

	if _, err := tf.ConnectWifiAP(ctx, ap); err != nil {
		s.Fatal("Failed to connect to the WiFi AP: ", err)
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

	pollAndValidateRoutine := func(expectedResult *diag.RoutineResult) error {
		req := &network.RunRoutineRequest{
			Routine: diag.RoutineSignalStrength,
		}
		if err := testing.Poll(ctx, func(ctx context.Context) error {
			res, err := di.RunRoutine(ctx, req)
			if err != nil {
				return testing.PollBreak(errors.Wrap(err, "failed to run diag routine"))
			}

			result := &diag.RoutineResult{
				Verdict:  diag.RoutineVerdict(res.Verdict),
				Problems: res.Problems,
			}

			if err := diag.CheckRoutineResult(result, expectedResult); err != nil {
				return errors.Wrap(err, "routine result did not match")
			}

			return nil

		}, &testing.PollOptions{Timeout: 30 * time.Second}); err != nil {
			return errors.Wrap(err, "timeout waiting for routine to have expected results")
		}

		return nil
	}

	expectedResult := &diag.RoutineResult{
		Verdict:  diag.VerdictNoProblem,
		Problems: []uint32{},
	}
	if err := pollAndValidateRoutine(expectedResult); err != nil {
		s.Fatal("Error running routine with default signal strength: ", err)
	}

	// Set the attenuation to a higher value and check that the
	// signal strength is weak. This should still leave the network connected,
	// but with a weak signal.
	weakConnectionAttenuationDb := 65.0
	if err := setAttenuation(weakConnectionAttenuationDb); err != nil {
		s.Fatal("Failed to set weak connection attenuation: ", err)
	}

	const problemWeakSignal = 0
	expectedResult = &diag.RoutineResult{
		Verdict:  diag.VerdictProblem,
		Problems: []uint32{problemWeakSignal},
	}
	if err := pollAndValidateRoutine(expectedResult); err != nil {
		s.Fatal("Error running routine with attenuated signal strength: ", err)
	}
}

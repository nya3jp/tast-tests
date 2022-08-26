// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package cellular

import (
	"context"
	"fmt"
	"time"

	cbiperf "chromiumos/tast/remote/cellular/callbox/iperf"
	"chromiumos/tast/remote/cellular/callbox/manager"
	"chromiumos/tast/remote/network/iperf"
	"chromiumos/tast/testing"
)

type iperfTestCaseConfiguration struct {
	testType          cbiperf.TestType
	additionalOptions []iperf.ConfigOption
}

type iperfTestCase struct {
	callboxOpts         *manager.ConfigureCallboxRequestBody
	iperfConfigurations []iperfTestCaseConfiguration
}

// cellularInterface is the name of the cellular interface to use.
// TODO(b/241964523): Dynamically get cellular interface from shill at runtime.
const cellularInterface = "rmnet_data0"

func init() {
	testing.AddTest(&testing.Test{
		Func:         Iperf,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Conducts cellular performance tests between DUT and callbox using Iperf to compare actual throughput results with expected for a given network configuration",
		Contacts: []string{
			"jstanko@google.com",
			"cros-connectivity@google.com",
		},
		Attr:         []string{"group:cellular", "cellular_callbox"},
		SoftwareDeps: []string{"chrome"},
		Fixture:      "callboxManagedFixture",
		Timeout:      15 * time.Minute,
		Params: []testing.Param{
			{
				// Establishes a mimo2x2 connection with the callbox and runs multiple iperf sessions with various configurations
				Name: "mimo2x2",
				Val: iperfTestCase{
					callboxOpts: &manager.ConfigureCallboxRequestBody{
						Hardware:     "CMW",
						CellularType: "LTE",
						ParameterList: []string{
							"band", "2",
							"bw", "20",
							"mimo", "2x2",
							"tm", "3",
							"pul", "0",
							"pdl", "high",
						},
					},
					iperfConfigurations: []iperfTestCaseConfiguration{
						{
							testType: cbiperf.TestTypeUDPTx,
							// use lower bitrate for udp upload otherwise results can be very inconsistent
							additionalOptions: []iperf.ConfigOption{iperf.MaxBandwidthOption(100 * iperf.Mbps)},
						},
						{
							testType: cbiperf.TestTypeTCPTx,
						},
						{
							testType: cbiperf.TestTypeUDPRx,
						},
						{
							testType: cbiperf.TestTypeTCPRx,
						},
					},
				},
			},
		},
	})
}

func Iperf(ctx context.Context, s *testing.State) {
	tc := s.Param().(iperfTestCase)
	tf := s.FixtValue().(*manager.TestFixture)
	dutConn := s.DUT().Conn()

	if err := tf.ConnectToCallbox(ctx, dutConn, tc.callboxOpts, cellularInterface); err != nil {
		s.Fatal("Failed to initialize cellular connection: ", err)
	}

	testManager := cbiperf.NewTestManager(tf.Vars.Callbox, dutConn, tf.CallboxManagerClient)

	for _, config := range tc.iperfConfigurations {
		subTest := func(ctx context.Context, s *testing.State) {
			history, err := testManager.RunOnce(ctx, config.testType, cellularInterface, config.additionalOptions)
			if err != nil {
				s.Fatal("Failed to run iperf session: ", err)
			}

			result, err := iperf.NewResultFromHistory(*history)
			if err != nil {
				s.Fatal("Failed to calculate iperf result statistics: ", err)
			}

			min, target, err := testManager.CalculateExpectedThroughput(ctx, config.testType)
			if err != nil {
				s.Fatal("Failed to calculate expected throughput: ", err)
			}

			if result.Throughput < min {
				s.Fatalf("Throughput below required, got: %v want: %v Mbit/s", result.Throughput/iperf.Mbps, min/iperf.Mbps)
			}

			if result.Throughput < target {
				s.Logf("WARNING: Throughput below target, got: %v want: %v Mbit/s", result.Throughput/iperf.Mbps, target/iperf.Mbps)
			}

			s.Logf("Finished Iperf test %v, minimum: %v target: %v actual: %v +/- %v Mbit/s",
				config.testType, min/iperf.Mbps, target/iperf.Mbps, result.Throughput/iperf.Mbps, result.StdDeviation/iperf.Mbps)
		}

		s.Run(ctx, fmt.Sprintf("Testcase %s", config.testType), subTest)
	}
}

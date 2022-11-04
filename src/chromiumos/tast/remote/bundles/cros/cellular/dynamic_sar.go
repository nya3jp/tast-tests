// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package cellular

import (
	"context"
	"fmt"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/dut"
	"chromiumos/tast/errors"
	"chromiumos/tast/remote/cellular/callbox/manager"
	"chromiumos/tast/remote/cellular/callbox/power"
	"chromiumos/tast/remote/firmware/reporters"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

const (
	sarSampleCount = 500
)

type dSARTestCase struct {
	testPower       float64
	startingOptions *manager.ConfigureCallboxRequestBody
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         DynamicSar,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Verifies that Tx power received at the callbox is within acceptable limits for a given SAR level/band combination",
		Contacts: []string{
			"jstanko@google.com",
			"chromeos-cellular-team@google.com",
		},
		Attr:         []string{"group:cellular", "cellular_callbox"},
		ServiceDeps:  []string{"tast.cros.cellular.RemoteCellularService"},
		SoftwareDeps: []string{"chrome"},
		// restrict tests to models that we have SAR tables for, TODO: revisit with (b/257515425)
		HardwareDeps: hwdep.D(hwdep.Model("lazor")),
		Fixture:      "callboxManagedFixture",
		Timeout:      10 * time.Minute,
		Params: []testing.Param{
			{
				Name: "lte",
				Val: dSARTestCase{
					// perform test at max LTE power
					testPower: 24.5,
					startingOptions: &manager.ConfigureCallboxRequestBody{
						Hardware:     "CMW",
						CellularType: "LTE",
						ParameterList: []string{
							"band", "3",
							"bw", "20",
							"mimo", "2x2",
							"tm", "3",
							"pul", "0",
							"pdl", "excellent",
						},
					},
				},
			},
		},
	})
}

func DynamicSar(ctx context.Context, s *testing.State) {
	tc := s.Param().(dSARTestCase)
	tf := s.FixtValue().(*manager.TestFixture)
	dutConn := s.DUT().Conn()

	if err := tf.ConnectToCallbox(ctx, dutConn, tc.startingOptions); err != nil {
		s.Fatal("Failed to initialize cellular connection: ", err)
	}

	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 30*time.Second)
	defer cancel()

	session := power.NewTxMeasurementSession(tf.CallboxManagerClient, tf.RemoteCellularClient)
	defer session.Close(cleanupCtx)

	sarConfigs, err := supportedConfigurations(ctx, s.DUT())
	if err != nil {
		s.Fatal("Failed to get supported SAR power levels: ", err)
	}

	// start a subtest for each supported SAR configuration
	for _, config := range sarConfigs {
		subTest := func(ctx context.Context, s *testing.State) {
			measurementConfig := &power.TxMeasurementConfiguration{
				TestPower: tc.testPower,
				// calibrate just below the target power as we may be clipped if we're close to the device's max
				CalibrationPower: config.expectedPower - 2*config.threshold,
				SarLevel:         config.level,
				SampleCount:      sarSampleCount,
			}

			result, err := session.Run(ctx, measurementConfig)
			if err != nil {
				s.Fatal("Failed to run dynamic SAR session: ", err)
			}

			if result.Average < config.expectedPower-config.threshold || result.Average > config.expectedPower+config.threshold {
				s.Fatalf("Tx power outside of limits, want %f +/- %f dBm got: %f", config.expectedPower, config.threshold, result.Average)
			}

			s.Logf("Completed SAR measurement, Min: %f, Max: %f, Average: %f +/- %f dBm", result.Min, result.Max, result.Average, result.StandardDeviation)
		}

		s.Run(ctx, fmt.Sprintf("SAR level: %s", config.name), subTest)
	}
}

type sarConfig struct {
	name          string
	level         int
	expectedPower float64
	threshold     float64
}

var powers = map[string][]sarConfig{
	"trogdor": []sarConfig{
		sarConfig{
			name:          "HIGH",
			level:         1,
			expectedPower: 21.5,
			threshold:     1.2,
		},
		sarConfig{
			name:          "MEDIUM",
			level:         2,
			expectedPower: 18.8,
			threshold:     1.2,
		},
		sarConfig{
			name:          "LOW",
			level:         3,
			expectedPower: 13.3,
			threshold:     1.2,
		},
	},
}

// supportedConfigurations fetches the supported SAR levels and expected Tx power for the DUT.
// TODO(b/257515425): Move to a more scalable solution.
func supportedConfigurations(ctx context.Context, dut *dut.DUT) ([]sarConfig, error) {
	board, err := reporters.New(dut).Board(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get board name")
	}

	configurations, ok := powers[board]
	if !ok {
		return nil, errors.Errorf("failed to get configurations, unknown board name %q", board)
	}

	return configurations, nil
}

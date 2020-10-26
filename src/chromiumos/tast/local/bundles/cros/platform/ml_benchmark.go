// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package platform

import (
	"context"

	"chromiumos/tast/local/bundles/cros/platform/mlbenchmark"
	"chromiumos/tast/testing"
)

type benchmarkParams struct {
	driver     string // name of the driver (.so file)
	configFile string // config file for the driver
	scenario   string // name of the scenario
}

func init() {
	testing.AddTest(&testing.Test{
		Func: MLBenchmark,
		Desc: "Verifies that the ML Benchmarks work end to end",
		Contacts: []string{
			"jmpollock@google.com",
			"chromeos-platform-ml@google.com",
		},
		Attr: []string{
			"group:mainline",
			"informational",
			"group:crosbolt",
			"crosbolt_nightly",
		},
		SoftwareDeps: []string{"ml_benchmark"},
		Params: []testing.Param{
			{
				Name: "soda_no_nnapi",
				Val: benchmarkParams{
					driver:     "libsoda_benchmark_driver.so",
					configFile: "soda-scenario-1.config",
					scenario:   "soda_no_nnapi",
				},
			},
			{
				Name: "handwriting_no_nnapi",
				Val: benchmarkParams{
					driver:     "libhandwriting_benchmark.so",
					configFile: "handwriting-scenario-1.config",
					scenario:   "handwriting_no_nnapi",
				},
			},
		},
	})
}

func MLBenchmark(ctx context.Context, s *testing.State) {
	const workspacePath = "/usr/local/ml_benchmark"

	p, ok := s.Param().(benchmarkParams)
	if !ok {
		s.Fatal("Failed to convert test params to benchmarkParams")
	}

	if err := mlbenchmark.ExecuteScenario(ctx, s.OutDir(), workspacePath, p.driver, p.configFile, p.scenario); err != nil {
		s.Fatalf("Error occurred running the benchmark %+v", err)
	}
}

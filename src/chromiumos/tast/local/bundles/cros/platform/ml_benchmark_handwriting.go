// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package platform

import (
	"context"

	"chromiumos/tast/local/bundles/cros/platform/mlbenchmark"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: MLBenchmarkHandwriting,
		Desc: "Executes benchmarking of handwriting recognition",
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
		SoftwareDeps: []string{"amd64", "ml_benchmark"},
	})
}

// MLBenchmarkHandwriting benchmarks the latency for handwriting recognition
func MLBenchmarkHandwriting(ctx context.Context, s *testing.State) {
	const (
		workspacePath = "/usr/local/ml_benchmark"
		driver        = "libhandwriting_benchmark.so"
		configFile    = "handwriting-scenario-1.config"
		scenario      = "handwriting_no_nnapi"
	)

	mlbenchmark.ExecuteScenario(ctx, s, workspacePath, driver, configFile, scenario)
}

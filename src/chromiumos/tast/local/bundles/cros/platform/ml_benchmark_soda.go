// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package platform

import (
	"context"
	"encoding/json"
	"io/ioutil"
	"os"
	"path/filepath"

	"chromiumos/tast/common/perf"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: MLBenchmarkSoDA,
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
	})
}

// MLBenchmarkSoDA Benchmarks the 90th percentile latency for SoDA
// (Speech on Device API)
func MLBenchmarkSoDA(ctx context.Context, s *testing.State) {
	const (
		workspacePath = "/usr/local/ml_benchmark"
		logFilename   = "ml_benchmark_logs.txt"
	)

	outputFile, err := ioutil.TempFile("", "soda_benchmark_results")
	if err != nil {
		s.Fatal("Cannot create output JSON file")
	}
	// We have proven the filename is fine, and we don't need the descriptor as
	// the cmd will write to it later and we'll read with ioutil.
	outputFile.Close()
	defer os.Remove(outputFile.Name())

	cmd := testexec.CommandContext(ctx,
		"ml_benchmark",
		"--workspace_path="+workspacePath,
		"--output_path="+outputFile.Name())

	logFile, err := os.OpenFile(filepath.Join(s.OutDir(), logFilename),
		os.O_WRONLY|os.O_CREATE|os.O_APPEND,
		0644)
	if err != nil {
		s.Fatal("Cannot open a logfile for the ml benchmark to write to")
	}
	defer logFile.Close()
	cmd.Stderr = logFile
	cmd.Stdout = logFile

	if err := cmd.Run(); err != nil {
		s.Fatalf("ML Benchmark failed, see %s for more details", logFilename)
	}

	outputJSON, err := ioutil.ReadFile(outputFile.Name())
	if err != nil {
		s.Fatal("Unable to open the results file from the ML Benchmark: ", err)
	}

	// Types from BenchmarkResults proto in ml_benchmark package
	// platform2/ml_benchmark/proto/benchmark_config.proto?l=48
	type LatencyPercentiles struct {
		Percentile90 int `json:"90"`
	}

	type BenchmarkResults struct {
		LatenciesUs    LatencyPercentiles `json:"percentile_latencies_in_us"`
		ResultsMessage string             `json:"results_message"`
		Status         int                `json:"status"`
		TotalAccuracy  float64            `json:"total_accuracy"`
	}

	var results BenchmarkResults
	if err := json.Unmarshal(outputJSON, &results); err != nil {
		s.Fatal("Failed to parse the results from the ML benchmark: ", err)
	}

	if results.Status != 0 {
		s.Fatalf("The ML Benchmark returned an error, status message: %s",
			results.ResultsMessage)
	}

	if results.LatenciesUs.Percentile90 == 0 {
		s.Fatalf("No 90th percentile found in the results: %s", outputJSON)
	}

	soda90PercentileLatencyMetric := perf.Metric{
		Name:      "soda_90th_percentile_latency",
		Unit:      "ms",
		Direction: perf.SmallerIsBetter,
	}
	p := perf.NewValues()
	p.Set(soda90PercentileLatencyMetric, float64(results.LatenciesUs.Percentile90)/1000)

	if err := p.Save(s.OutDir()); err != nil {
		s.Fatal("Failed saving perf data: ", err)
	}
}

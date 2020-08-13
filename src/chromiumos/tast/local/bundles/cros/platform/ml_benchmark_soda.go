// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package platform

import (
	"context"
	"io/ioutil"
	"os"
	"path/filepath"

	proto "github.com/golang/protobuf/proto"

	"chromiumos/tast/common/perf"
	"chromiumos/tast/local/bundles/cros/platform/ml"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: MLBenchmarkSoDA,
		Desc: "Verifies that the ML Benchmarks work end to end",
		Contacts: []string{
			"franklinh@google.com",
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
		workspacePath  = "/usr/local/ml_benchmark"
		outputFilename = "soda_benchmark_results.pb"
		logFilename    = "ml_benchmark_logs.txt"
	)

	cmd := testexec.CommandContext(ctx,
		"ml_benchmark",
		"--workspace_path="+workspacePath,
		"--output_path="+outputFilename)

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

	var resultsPath = filepath.Join(workspacePath, outputFilename)
	if _, err := os.Stat(resultsPath); os.IsNotExist(err) {
		s.Fatalf("Results file at %s was not found", resultsPath)
	}

	resultsFile, err := ioutil.ReadFile(resultsPath)
	if err != nil {
		s.Fatal("Unable to open the results file from the ML Benchmark: ", err)
	}

	results := ml.BenchmarkResults{}
	if err := proto.Unmarshal(resultsFile, &results); err != nil {
		s.Fatal("Failed to parse the protobuf results from the ML benchmark: ", err)
	}

	if results.Status != ml.BenchmarkReturnStatus_OK {
		s.Fatalf("The ML Benchmark returned an error, status message: %s",
			results.ResultsMessage)
	}

	if len(results.PercentileLatenciesInUs) == 0 {
		s.Fatal("No percentile latencies included in the results, " +
			"check if the results are produced with the latest ML driver")
	}

	latency90Percentile, latencyExists := results.PercentileLatenciesInUs[90]
	if !latencyExists {
		s.Fatal("No 90th percentile found in the results")
	}

	soda90PercentileLatencyMetric := perf.Metric{
		Name:      "soda_90th_percentile_latency",
		Unit:      "ms",
		Direction: perf.SmallerIsBetter,
	}
	p := perf.NewValues()
	p.Set(soda90PercentileLatencyMetric, float64(latency90Percentile)/1000)

	if err := p.Save(s.OutDir()); err != nil {
		s.Fatal("Failed saving perf data: ", err)
	}
}

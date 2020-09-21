// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package mlbenchmark contains functionality shared by tests that execute the
// ml_benchmark tool with various scenarios.
package mlbenchmark

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

// Types from BenchmarkResults proto in ml_benchmark package
// platform2/ml_benchmark/proto/benchmark_config.proto?l=48
type latencyPercentiles struct {
	Percentile50 int `json:"50"`
	Percentile90 int `json:"90"`
	Percentile95 int `json:"95"`
	Percentile99 int `json:"99"`
}

type benchmarkResults struct {
	LatenciesUs    latencyPercentiles `json:"percentile_latencies_in_us"`
	ResultsMessage string             `json:"results_message"`
	Status         int                `json:"status"`
	TotalAccuracy  float64            `json:"total_accuracy"`
}

func addLatencyMetric(p *perf.Values, name string, latencyMS float64) {
	m := perf.Metric{
		Name:      name,
		Unit:      "ms",
		Direction: perf.SmallerIsBetter,
	}
	p.Set(m, latencyMS)
}

func processOutputFile(s *testing.State, scenario, outputFilename string) {
	outputJSON, err := ioutil.ReadFile(outputFilename)
	if err != nil {
		s.Fatal("Unable to open the results file from the ML Benchmark: ", err)
	}

	var results benchmarkResults
	if err := json.Unmarshal(outputJSON, &results); err != nil {
		s.Fatal("Failed to parse the results from the ML benchmark: ", err)
	}

	if results.Status != 0 {
		s.Fatalf("The ML Benchmark returned an error, status message: %s",
			results.ResultsMessage)
	}

	p := perf.NewValues()

	if results.LatenciesUs.Percentile50 != 0 {
		addLatencyMetric(p, scenario+"_50th_perc_latency", float64(results.LatenciesUs.Percentile50/1000))
	}
	if results.LatenciesUs.Percentile90 != 0 {
		addLatencyMetric(p, scenario+"_90th_perc_latency", float64(results.LatenciesUs.Percentile90/1000))
	}
	if results.LatenciesUs.Percentile95 != 0 {
		addLatencyMetric(p, scenario+"_95th_perc_latency", float64(results.LatenciesUs.Percentile95/1000))
	}
	if results.LatenciesUs.Percentile99 != 0 {
		addLatencyMetric(p, scenario+"_99th_perc_latency", float64(results.LatenciesUs.Percentile99/1000))
	}

	if err := p.Save(s.OutDir()); err != nil {
		s.Fatal("Failed saving perf data: ", err)
	}
}

// ExecuteScenario invokes ml_benchmark, parses the output from the driver and
// creates a set of Perf values to be collected and uploaded into crosbolt.
func ExecuteScenario(ctx context.Context, s *testing.State, workspacePath, driver, configFile, scenario string) {
	outputFile, err := ioutil.TempFile("", scenario+"_results*.json")
	if err != nil {
		s.Fatal("Cannot create output JSON file")
	}
	// We have proven the filename is fine, and we don't need the descriptor as
	// the cmd will write to it later and we'll read with ioutil.
	err = outputFile.Close()
	if err != nil {
		s.Fatal("Cannot close output JSON file")
	}
	defer os.Remove(outputFile.Name())

	cmd := testexec.CommandContext(ctx,
		"ml_benchmark",
		"--config_file_name="+configFile,
		"--driver_library_path="+driver,
		"--workspace_path="+workspacePath,
		"--output_path="+outputFile.Name())

	logFilename := scenario + "_logs.txt"
	logFile, err := os.OpenFile(filepath.Join(s.OutDir(), logFilename),
		os.O_WRONLY|os.O_CREATE|os.O_APPEND,
		0644)
	if err != nil {
		s.Fatal("Cannot open a logfile for the ml_benchmark to write to")
	}
	defer logFile.Close()
	cmd.Stderr = logFile
	cmd.Stdout = logFile

	if err := cmd.Run(); err != nil {
		s.Fatalf("ML Benchmark failed, see %s for more details", logFilename)
	}

	processOutputFile(s, scenario, outputFile.Name())
}

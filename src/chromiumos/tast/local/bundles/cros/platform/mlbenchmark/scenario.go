// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package mlbenchmark contains functionality used by the ml_benchmark tast
// test. This is all implementation that developers don't need to get confused
// by when writing additional scenarios, so keeping it out of the way here.
package mlbenchmark

import (
	"context"
	"encoding/json"
	"io/ioutil"
	"os"
	"path/filepath"

	"chromiumos/tast/common/perf"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/testexec"
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
	LatenciesUS    latencyPercentiles `json:"percentile_latencies_in_us"`
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

func processOutputFile(scenario, outDir, outputFilename string) error {
	outputJSON, err := ioutil.ReadFile(outputFilename)
	if err != nil {
		errors.Wrap(err, "unable to open the results file from the ML Benchmark")
	}

	var results benchmarkResults
	if err := json.Unmarshal(outputJSON, &results); err != nil {
		errors.Wrap(err, "failed to parse the results from the ML benchmark")
	}

	if results.Status != 0 {
		errors.Wrapf(err, "the ML Benchmark returned an error, status message: %s, status code: %d",
			results.ResultsMessage, results.Status)
	}

	p := perf.NewValues()

	if results.LatenciesUS.Percentile50 != 0 {
		addLatencyMetric(p, scenario+"_50th_perc_latency", float64(results.LatenciesUS.Percentile50/1000))
	}
	if results.LatenciesUS.Percentile90 != 0 {
		addLatencyMetric(p, scenario+"_90th_perc_latency", float64(results.LatenciesUS.Percentile90/1000))
	}
	if results.LatenciesUS.Percentile95 != 0 {
		addLatencyMetric(p, scenario+"_95th_perc_latency", float64(results.LatenciesUS.Percentile95/1000))
	}
	if results.LatenciesUS.Percentile99 != 0 {
		addLatencyMetric(p, scenario+"_99th_perc_latency", float64(results.LatenciesUS.Percentile99/1000))
	}

	if err := p.Save(outDir); err != nil {
		errors.Wrap(err, "failed saving perf data")
	}
	return nil
}

// ExecuteScenario invokes ml_benchmark, parses the output from the driver and
// creates a set of Perf values to be collected and uploaded into crosbolt.
func ExecuteScenario(ctx context.Context, outDir, workspacePath, driver, configFile, scenario string) error {
	tempFilePattern := scenario + "_results*.json"
	outputFile, err := ioutil.TempFile("", tempFilePattern)
	defer os.Remove(outputFile.Name())
	if err != nil {
		return errors.Wrapf(err, "cannot create output JSON file from pattern: %s", tempFilePattern)
	}

	// We have proven the filename is fine, and we don't need the descriptor as
	// the cmd will write to it later and we'll read with ioutil.
	if err = outputFile.Close(); err != nil {
		return errors.Wrapf(err, "cannot close output JSON file: %s", outputFile.Name())
	}

	cmd := testexec.CommandContext(ctx,
		"ml_benchmark",
		"--config_file_name="+configFile,
		"--driver_library_path="+driver,
		"--workspace_path="+workspacePath,
		"--output_path="+outputFile.Name())

	logFilename := scenario + "_logs.txt"
	logFile, err := os.OpenFile(filepath.Join(outDir, logFilename),
		os.O_WRONLY|os.O_CREATE|os.O_APPEND,
		0644)
	if err != nil {
		return errors.Wrapf(err, "cannot open logfile %s for the ml_benchmark to write to", logFilename)
	}
	defer logFile.Close()
	cmd.Stderr = logFile
	cmd.Stdout = logFile

	if err := cmd.Run(); err != nil {
		return errors.Wrapf(err, "benchmark failed, see %s for more details", logFilename)
	}

	return processOutputFile(scenario, outDir, outputFile.Name())
}

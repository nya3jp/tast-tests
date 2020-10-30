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
	"time"

	"chromiumos/tast/common/perf"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/power"
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

type metric struct {
	Name                 string    `json:"name"`
	Units                string    `json:"units"`
	ImprovementDirection string    `json:"improvement_direction"`
	Cardinality          string    `json:"cardinality"`
	Values               []float64 `json:"values"`
}

type benchmarkResults struct {
	LatenciesUS    latencyPercentiles `json:"percentile_latencies_in_us"`
	ResultsMessage string             `json:"results_message"`
	Status         int                `json:"status"`
	TotalAccuracy  float64            `json:"total_accuracy"`
	Metrics        []metric           `json:"metrics"`
}

func addLatencyMetric(p *perf.Values, name string, latencyMS float64) {
	m := perf.Metric{
		Name:      name,
		Unit:      "ms",
		Direction: perf.SmallerIsBetter,
	}
	p.Set(m, latencyMS)
}

func addMetric(p *perf.Values, m metric, scenario string) error {
	var direction perf.Direction
	switch m.ImprovementDirection {
	case "smaller_is_better":
		direction = perf.SmallerIsBetter
	case "bigger_is_better":
		direction = perf.BiggerIsBetter
	default:
		return errors.Errorf("unhandled ImprovementDirection %s", m.ImprovementDirection)
	}

	p.Set(perf.Metric{
		Name:      scenario + "_" + m.Name,
		Unit:      m.Units,
		Direction: direction,
		Multiple:  m.Cardinality == "multiple"}, m.Values...)

	return nil
}

func processOutputFile(scenario, outDir, outputFilename string, additionalMetrics []metric) error {
	outputJSON, err := ioutil.ReadFile(outputFilename)
	if err != nil {
		return errors.Wrap(err, "unable to open the results file from the benchmark")
	}

	var results benchmarkResults
	if err := json.Unmarshal(outputJSON, &results); err != nil {
		return errors.Wrap(err, "failed to parse the results from the benchmark")
	}

	if results.Status != 0 {
		return errors.Wrapf(err, "benchmark returned an error, status message %s, status code %d",
			results.ResultsMessage, results.Status)
	}

	p := perf.NewValues()

	for _, m := range append(results.Metrics, additionalMetrics...) {
		if err := addMetric(p, m, scenario); err != nil {
			return err
		}
	}

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
		return errors.Wrap(err, "failed saving perf data")
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
		return errors.Wrapf(err, "cannot create output JSON file from pattern %s", tempFilePattern)
	}

	// We have proven the filename is fine, and we don't need the descriptor as
	// the cmd will write to it later and we'll read with ioutil.
	if err = outputFile.Close(); err != nil {
		return errors.Wrapf(err, "cannot close output JSON file %s", outputFile.Name())
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

	quitSampling := make(chan struct{}, 1)
	samplingResult := make(chan float64)
	samplingInterval := 1 * time.Second
	samplingFunction, err := GetReadMomentaryPowerW(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to get power sampling lambda")
	}
	go SamplePower(ctx, samplingFunction, samplingInterval, quitSampling, samplingResult)

	raplEnergyBefore, err := power.NewRAPLSnapshot()
	if err != nil {
		testing.ContextLog(ctx, "RAPL Energy status is not available for this board")
	}

	if err := cmd.Run(); err != nil {
		return errors.Wrapf(err, "benchmark failed, see %s for more details", logFilename)
	}

	quitSampling <- struct{}{}
	batteryPower := <-samplingResult
	raplPower := 0.

	if raplEnergyBefore != nil {
		energyDif, err := raplEnergyBefore.DiffWithCurrentRAPL()
		if err != nil {
			return errors.Wrap(err, "failed to get RAPL power usage")
		}
		raplPower = energyDif.Total()
	}

	additionalMetrics := []metric{{
		Name:                 "total_power_from_battery",
		Units:                "J",
		ImprovementDirection: "smaller_is_better",
		Cardinality:          "single",
		Values:               []float64{batteryPower},
	}, {
		Name:                 "total_power_from_rapl",
		Units:                "J",
		ImprovementDirection: "smaller_is_better",
		Cardinality:          "single",
		Values:               []float64{raplPower},
	}}

	return processOutputFile(scenario, outDir, outputFile.Name(), additionalMetrics)
}

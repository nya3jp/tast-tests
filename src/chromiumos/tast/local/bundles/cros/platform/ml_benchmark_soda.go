// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package platform

import (
	"bytes"
	"context"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	proto "github.com/golang/protobuf/proto"

	"chromiumos/tast/common/perf"
	"chromiumos/tast/local/bundles/cros/platform/ml"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/shutil"
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
			"group:crosbolt",
			"crosbolt_nightly",
		},
		SoftwareDeps: []string{"ml_benchmark"},
	})
}

// MLBenchmarkSoDA Benchmarks the 90th percentile latency for SoDA
// (Speech on Device API)
func MLBenchmarkSoDA(ctx context.Context, s *testing.State) {
	var workspacePath = "/usr/local/ml_benchmark"
	var outputFilename = "soda_benchmark_results.pb"
	cmd := testexec.CommandContext(ctx,
		"ml_benchmark",
		"--workspace_path="+workspacePath,
		"--output_path="+outputFilename)
	var outputBytes bytes.Buffer
	cmd.Stderr = &outputBytes
	cmd.Stdout = &outputBytes

	if err := cmd.Run(); err != nil {
		s.Errorf("%s failed: %v", shutil.EscapeSlice(cmd.Args), err)
	}

	var outputText = outputBytes.String()
	logFilename := "ml_benchmark_logs.txt"
	if err := ioutil.WriteFile(
		filepath.Join(s.OutDir(), logFilename),
		outputBytes.Bytes(),
		0644); err != nil {
		s.Error("Count not sucessfully write out the ml log file: ", err)
		return
	}

	if strings.Contains(outputText, "ERROR") ||
		strings.Contains(outputText, "FATAL") {
		s.Error("Encountered an error within the logs, see " + logFilename)
		return
	}

	var resultsPath = filepath.Join(workspacePath, outputFilename)
	if _, err := os.Stat(resultsPath); os.IsNotExist(err) {
		s.Errorf("Results file at %s was not found", resultsPath)
		return
	}

	resultsFile, err := ioutil.ReadFile(resultsPath)
	if err != nil {
		s.Error("Unable to open the results file from the ML Benchmark: ", err)
		return
	}

	var benchmarkResults = &ml.BenchmarkResults{}

	if err := proto.Unmarshal(resultsFile, benchmarkResults); err != nil {
		s.Error("Failed to parse the protobuf results from the ML benchmark: ", err)
		return
	}

	if benchmarkResults.Status != ml.BenchmarkReturnStatus_OK {
		s.Errorf("The ML Benchmark returned an error, status message: %s",
			benchmarkResults.ResultsMessage)
		return
	}

	if len(benchmarkResults.PercentileLatenciesInUs) == 0 {
		s.Error("No percentile latencies included in the results, " +
			"check if the results are produced with the latest ML driver")
		return
	}

	var latencyMap = &(benchmarkResults.PercentileLatenciesInUs)
	if latency90Percentile, latencyExists := (*latencyMap)[90]; latencyExists {
		var soda90PercentileLatencyMetric = perf.Metric{
			Name:      "soda_90th_percentile_latency",
			Unit:      "ms",
			Direction: perf.SmallerIsBetter,
		}
		p := perf.NewValues()
		p.Set(soda90PercentileLatencyMetric, float64(latency90Percentile)/1000)

		if err := p.Save(s.OutDir()); err != nil {
			s.Error("Failed saving perf data: ", err)
		}
	} else {
		s.Error("No 90th percentile found in the results")
	}
}

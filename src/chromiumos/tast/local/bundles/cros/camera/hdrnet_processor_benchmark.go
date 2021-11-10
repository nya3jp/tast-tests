// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package camera

import (
	"context"
	"path/filepath"
	"strings"
	"time"

	"chromiumos/tast/common/perf"
	"chromiumos/tast/local/benchmark"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         HDRnetProcessorBenchmark,
		Desc:         "Runs the HDRnet processor benchmark and reports the measurements",
		Contacts:     []string{"jcliang@chromium.org", "chromeos-camera-eng@google.com"},
		Attr:         []string{"group:crosbolt", "crosbolt_perbuild"},
		SoftwareDeps: []string{"camera_feature_hdrnet"},
		Timeout:      5 * time.Minute,
	})
}

// HDRnetProcessorBenchmark runs the HDRnet processor benchmark.
func HDRnetProcessorBenchmark(ctx context.Context, s *testing.State) {
	const exec = "hdrnet_processor_benchmark"
	const consoleOutput = "console_output.txt"
	result, err := benchmark.New(
		exec,
		// Also produce a human-readable output file for ease of debugging.
		benchmark.OutputFile(filepath.Join(s.OutDir(), consoleOutput)),
		benchmark.OutputResultFormat(benchmark.Console),
	).Run(ctx)

	if err != nil {
		s.Fatal("Failed to run HDRnet processor benchmark: ", err)
	}

	perfValues := perf.NewValues()
	for _, r := range result.Benchmarks {
		// Converts raw benchmark name like: "BM_HdrNetProcessorFullProcessing/1280/720" to more
		// readable perf metric name: "HdrNetProcessorFullProcessing_1280x720_{CpuTime,RealTime}".
		args := strings.Split(r.RunName, "/")
		nameSlice := []string{strings.TrimPrefix(args[0], "BM_"), strings.Join(args[1:], "x")}
		cpuTime := perf.Metric{
			Name:      strings.Join(append(nameSlice, "CpuTime"), "_"),
			Unit:      r.TimeUnit,
			Direction: perf.SmallerIsBetter,
			Multiple:  false,
		}
		perfValues.Set(cpuTime, r.CPUTime)
		realTime := perf.Metric{
			Name:      strings.Join(append(nameSlice, "RealTime"), "_"),
			Unit:      r.TimeUnit,
			Direction: perf.SmallerIsBetter,
			Multiple:  false,
		}
		perfValues.Set(realTime, r.RealTime)
	}

	if err := perfValues.Save(s.OutDir()); err != nil {
		s.Fatal("Failed to save perf metrics: ", err)
	}
}

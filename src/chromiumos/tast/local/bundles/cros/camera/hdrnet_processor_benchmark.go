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
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:     HdrnetProcessorBenchmark,
		Desc:     "Runs the HDRnet processor benchmark and reports the measurements",
		Contacts: []string{"jcliang@chromium.org", "chromeos-camera-eng@google.com"},
		// HDRnet is currently only available on Intel TGL and ADL platforms.
		HardwareDeps: hwdep.D(hwdep.Platform("volteer", "brya")),
		Attr:         []string{"group:crosbolt", "crosbolt_perbuild"},
		Timeout:      5 * time.Minute,
	})
}

// HdrnetProcessorBenchmark runs the HDRnet processor benchmark.
func HdrnetProcessorBenchmark(ctx context.Context, s *testing.State) {
	const exec = "hdrnet_processor_benchmark"
	const consoleOutput = "console_output.txt"
	jsonOutput, err := benchmark.New(
		exec,
		benchmark.ResultFormat(benchmark.JSON),
		benchmark.OutputFile(filepath.Join(s.OutDir(), consoleOutput)),
		benchmark.OutputResultFormat(benchmark.Console),
	).Run(ctx)

	if err != nil {
		s.Fatal("Failed to run HDRnet processor benchmar: ", err)
	}

	result := benchmark.UnmarshalJSONBytes(jsonOutput)
	perfValues := perf.NewValues()
	for _, r := range result.Benchmarks {
		// Converts raw benchmark name like: "BM_HdrNetProcessorFullProcessing/1280/720" to more
		// readable perf metric name: "HdrNetProcessorFullProcessing_1280x720_{CpuTime,RealTime}".
		args := strings.Split(r.RunName, "/")
		nameSlice := []string{strings.Replace(args[0], "BM_", "", 1), strings.Join(args[1:], "x")}
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

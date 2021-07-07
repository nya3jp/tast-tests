// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package audio

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"chromiumos/tast/common/perf"
	"chromiumos/tast/common/testexec"
	"chromiumos/tast/testing"
)

// crasBenchParameters contains all the data needed to run a single test iteration.
type crasBenchParameters struct {
	BenchmarkFilter string // cras_bench filter for this test case.
	MetricFps       bool   // Result contains metric fps.
}

func init() {
	testing.AddTest(&testing.Test{
		Func:     CrasBench,
		Desc:     "Micro-benchmarks for the ChromeOS audio server",
		Contacts: []string{"paulhsia@chromium.org", "tast-owners@chromium.org"},
		Attr:     []string{"group:crosbolt", "crosbolt_perbuild"},
		Timeout:  2 * time.Minute,
		Params: []testing.Param{
			{
				Name: "dsp",
				Val: crasBenchParameters{
					BenchmarkFilter: "BM_Dsp",
					MetricFps:       true,
				},
			},
			{
				Name: "cras_mixer_ops",
				Val: crasBenchParameters{
					BenchmarkFilter: "BM_CrasMixerOps",
					MetricFps:       false,
				},
			},
		},
	})
}

func CrasBench(ctx context.Context, s *testing.State) {
	param := s.Param().(crasBenchParameters)
	out, err := testexec.CommandContext(ctx, "cras_bench", "--benchmark_format=json", "--benchmark_filter="+param.BenchmarkFilter).Output(testexec.DumpLogOnError)
	if err != nil {
		s.Fatal("Failed to execute cras_bench: ", err)
	}

	result := struct {
		Benchmarks []struct {
			Name    string  `json:"name"`
			CPUTime float64 `json:"cpu_time"`
			FPS     float64 `json:"frames_per_second"`
		} `json:"benchmarks"`
	}{}
	if err := json.Unmarshal(out, &result); err != nil {
		s.Fatal("Failed to unmarshal test results: ", err)
	}

	p := perf.NewValues()
	for _, res := range result.Benchmarks {
		name := strings.ReplaceAll(res.Name, "/", "_")
		cpuTime := perf.Metric{Name: name, Variant: "cpu_time", Unit: "ns", Direction: perf.SmallerIsBetter}
		p.Set(cpuTime, res.CPUTime)
		if param.MetricFps {
			fps := perf.Metric{Name: name, Variant: "fps", Unit: "fps", Direction: perf.BiggerIsBetter}
			p.Set(fps, res.FPS)
		}
	}
	if err := p.Save(s.OutDir()); err != nil {
		s.Error("Failed saving perf data: ", err)
	}
}

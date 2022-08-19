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
	"chromiumos/tast/testing/hwdep"
)

// crasBenchParameters contains all the data needed to run a single test iteration.
type crasBenchParameters struct {
	BenchmarkFilter  string // cras_bench filter for this test case.
	MetricFps        bool   // If the result contains metric FPS.
	Time4096Frame    bool   // If the result contains metric "time_per_4096_frames".
	MaxTime4096Frame bool   // If the result contains metric "max_time_per_4096_frames".
}

func init() {
	testing.AddTest(&testing.Test{
		Func:     CrasBench,
		Desc:     "Micro-benchmarks for the ChromeOS audio server",
		Contacts: []string{"paulhsia@chromium.org", "cychiang@chromium.org"},
		Attr:     []string{"group:crosbolt", "crosbolt_perbuild"},
		Timeout:  2 * time.Minute,
		Params: []testing.Param{
			{
				Name: "apm",
				Val: crasBenchParameters{
					BenchmarkFilter:  "BM_Apm",
					MetricFps:        true,
					Time4096Frame:    false,
					MaxTime4096Frame: false,
				},
			},
			{
				Name: "dsp",
				Val: crasBenchParameters{
					BenchmarkFilter:  "BM_Dsp",
					MetricFps:        true,
					Time4096Frame:    false,
					MaxTime4096Frame: false,
				},
			},
			{
				Name: "cras_mixer_ops",
				Val: crasBenchParameters{
					BenchmarkFilter:  "BM_CrasMixerOps",
					MetricFps:        false,
					Time4096Frame:    false,
					MaxTime4096Frame: false,
				},
			},
			{
				Name:              "alsa",
				ExtraHardwareDeps: hwdep.D(hwdep.Speaker()),
				Val: crasBenchParameters{
					BenchmarkFilter:  "BM_Alsa",
					MetricFps:        true,
					Time4096Frame:    true,
					MaxTime4096Frame: true,
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

	// An output example (in JSON format) of `cras_bench`.
	// {
	//   ...,
	//   "benchmarks": [
	//     {
	//       "name": "BM_Dsp/Eq2/256",
	//       "family_index": 0,
	//       "per_family_instance_index": 0,
	//       "run_name": "BM_Dsp/Eq2/256",
	//       "run_type": "iteration",
	//       "repetitions": 1,
	//       "repetition_index": 0,
	//       "threads": 1,
	//       "iterations": 323328,
	//       "real_time": 2.1650514616736227e+03,
	//       "cpu_time": 2.1631811411322246e+03,
	//       "time_unit": "ns",
	//       "frames_per_second": 1.1834422699617644e+08,
	//       "time_per_48k_frames": 4.0569433410672857e-04
	//     },
	//     ...
	//   ]
	// }
	result := struct {
		Benchmarks []struct {
			Name             string  `json:"name"`
			CPUTime          float64 `json:"cpu_time"`
			FPS              float64 `json:"frames_per_second"`
			Time4096Frame    float64 `json:"time_per_4096_frames"`
			MaxTime4096Frame float64 `json:"max_time_per_4096_frames"`
		} `json:"benchmarks"`
	}{}
	if err := json.Unmarshal(out, &result); err != nil {
		s.Fatal("Failed to unmarshal test results: ", err)
	}

	p := perf.NewValues()
	for _, res := range result.Benchmarks {
		// Name field in perf.Metric accepts only "_", "." and "-".
		name := strings.ReplaceAll(res.Name, "/", "_")
		cpuTime := perf.Metric{Name: name, Variant: "cpu_time", Unit: "ns", Direction: perf.SmallerIsBetter}
		p.Set(cpuTime, res.CPUTime)
		if param.MetricFps {
			fps := perf.Metric{Name: name, Variant: "fps", Unit: "fps", Direction: perf.BiggerIsBetter}
			p.Set(fps, res.FPS)
		}
		if param.Time4096Frame {
			time4096 := perf.Metric{Name: name, Variant: "time_per_4096_frames", Unit: "ns", Direction: perf.SmallerIsBetter}
			p.Set(time4096, res.Time4096Frame)
		}
		if param.MaxTime4096Frame {
			maxTime4096 := perf.Metric{Name: name, Variant: "max_time_per_4096_frames", Unit: "ns", Direction: perf.SmallerIsBetter}
			p.Set(maxTime4096, res.MaxTime4096Frame)
		}
	}
	if err := p.Save(s.OutDir()); err != nil {
		s.Error("Failed saving perf data: ", err)
	}
}

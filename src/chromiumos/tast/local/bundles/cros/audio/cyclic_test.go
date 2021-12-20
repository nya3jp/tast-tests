// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package audio

import (
	"context"
	"strconv"
	"time"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/testing"
)

// cyclicTestParameters contains all the data needed to run a single test iteration.
type cyclicTestParameters struct {
	Priority int // Priority of the process
	Threads  int // Number of threads
	Interval int // Interval time in us
	Loops    int // Number of times
}

func init() {
	testing.AddTest(&testing.Test{
		Func:     CyclicTest,
		Desc:     "benchmarks of cyclic_test",
		Contacts: []string{"eddyhsu@chromium.org"},

		// TODO(eddyhsu): update Attr
		Attr:    []string{"group:crosbolt", "crosbolt_perbuild"},
		Timeout: 2 * time.Minute,
		Params: []testing.Param{
			{
				Name: "basic",
				Val: cyclicTestParameters{
					Priority: 80,
					Threads:  1,
					Interval: 1000,
					Loops:    1000,
				},
			},
			{
				Name: "mulit_thread",
				Val: cyclicTestParameters{
					Priority: 80,
					Threads:  8,
					Interval: 1000,
					Loops:    1000,
				},
			},
			{
				Name: "low_priority",
				Val: cyclicTestParameters{
					Priority: 20,
					Threads:  1,
					Interval: 1000,
					Loops:    1000,
				},
			},
		},
	})
}

func CyclicTest(ctx context.Context, s *testing.State) {
	param := s.Param().(cyclicTestParameters)
	out, err := testexec.CommandContext(ctx, "cyclictest",
		"--priority"+strconv.Itoa(param.Priority),
		"--interval"+strconv.Itoa(param.Interval),
		"--threads"+strconv.Itoa(param.Thread),
		"--loops"+strconv.Itoa(param.Loops)).Output(testexec.DumpLogOnError)
	if err != nil {
		s.Fatal("Failed to execute cyclictest: ", err)
	}

	// Parse the results
	s.Log(out)

	// TODO(eddyhsu): use perf
	/*
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
		}
		if err := p.Save(s.OutDir()); err != nil {
			s.Error("Failed saving perf data: ", err)
		}*/
}

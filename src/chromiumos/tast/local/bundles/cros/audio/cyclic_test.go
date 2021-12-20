// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package audio

import (
	"context"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"chromiumos/tast/common/perf"
	"chromiumos/tast/common/testexec"
	"chromiumos/tast/testing"
)

// cyclicTestParameters contains all the data needed to run a single test iteration.
type cyclicTestParameters struct {
	Priority       int // Priority of the process
	Threads        int // Number of threads
	IntervalUs     int // Interval time
	Loops          int // Number of times
	P99ThresholdUs int // P99 latency threshold
}

const CrasPriority = 12
const CrasClientPriority = 10
const DefaultIntervalUs = 1000
const DefaultLoops = 1000
const DefaultP99ThresholdUs = 40

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
				Name: "cras_pr",
				Val: cyclicTestParameters{
					Priority:       CrasPriority,
					Threads:        1,
					IntervalUs:     DefaultIntervalUs,
					Loops:          DefaultLoops,
					P99ThresholdUs: DefaultP99ThresholdUs,
				},
			},
			{
				Name: "cras_client_pr",
				Val: cyclicTestParameters{
					Priority:       CrasClientPriority,
					Threads:        1,
					IntervalUs:     DefaultIntervalUs,
					Loops:          DefaultLoops,
					P99ThresholdUs: DefaultP99ThresholdUs,
				},
			},
			{
				Name: "cras_pr_multi_thread",
				Val: cyclicTestParameters{
					Priority:       CrasPriority,
					Threads:        4,
					IntervalUs:     DefaultIntervalUs,
					Loops:          DefaultLoops,
					P99ThresholdUs: DefaultP99ThresholdUs,
				},
			},
			{
				Name: "cras_client_pr_multi_thread",
				Val: cyclicTestParameters{
					Priority:       CrasClientPriority,
					Threads:        4,
					IntervalUs:     DefaultIntervalUs,
					Loops:          DefaultLoops,
					P99ThresholdUs: DefaultP99ThresholdUs,
				},
			},
		},
	})
}

type cyclicTestStats struct {
	latencies []int
	median    int
	p99       int
}

func CalculateStats(log string, threads int, s *testing.State) []cyclicTestStats {
	stats := make([]cyclicTestStats, threads)
	dataRe := regexp.MustCompile(`^[ \t]+\d+:[ \t]+\d+:[ \t]+\d+$`)
	integerRe := regexp.MustCompile(`\d+`)
	for _, line := range strings.Split(log, "\n") {
		if !dataRe.MatchString(line) {
			continue
		}
		datas := integerRe.FindAllString(line, 3)

		tid, err := strconv.Atoi(datas[0])
		if err != nil {
			s.Fatal("Failed to parse data: ", err)
		}
		latency, err := strconv.Atoi(datas[2])
		if err != nil {
			s.Fatal("Failed to parse data: ", err)
		}

		stats[tid].latencies = append(stats[tid].latencies, latency)
	}
	for idx := range stats {
		sort.Ints(stats[idx].latencies)
		count := len(stats[idx].latencies)
		if count == 0 {
			continue
		}
		stats[idx].median = stats[idx].latencies[count/2]
		stats[idx].p99 = stats[idx].latencies[count*99/100]
	}
	return stats
}

func CyclicTest(ctx context.Context, s *testing.State) {
	param := s.Param().(cyclicTestParameters)
	out, err := testexec.CommandContext(ctx, "cyclictest",
		"--priority="+strconv.Itoa(param.Priority),
		"--interval="+strconv.Itoa(param.IntervalUs),
		"--threads="+strconv.Itoa(param.Threads),
		"--loops="+strconv.Itoa(param.Loops),
		"--distance=0",
		"--verbose").Output(testexec.DumpLogOnError)
	if err != nil {
		s.Fatal("Failed to execute cyclictest: ", err)
	}

	// Parse the results
	stats := CalculateStats(string(out), param.Threads, s)

	p := perf.NewValues()
	for index, stat := range stats {
		name := "Thread_" + strconv.Itoa(index)
		p99Latency := perf.Metric{
			Name:      name,
			Variant:   "p99_latency",
			Unit:      "us",
			Direction: perf.SmallerIsBetter}
		p.Set(p99Latency, float64(stat.p99))
		medianLatency := perf.Metric{
			Name:      name,
			Variant:   "p50_latency",
			Unit:      "us",
			Direction: perf.SmallerIsBetter}
		p.Set(medianLatency, float64(stat.median))

		if stat.p99 > param.P99ThresholdUs {
			s.Error("p99 latency exceeds threshold: ", stat.p99,
				" > ", param.P99ThresholdUs)
		}
	}
	if err := p.Save(s.OutDir()); err != nil {
		s.Error("Failed saving perf data: ", err)
	}
}

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
	Priority       int  // Priority of the process
	Threads        int  // Number of threads
	IntervalUs     int  // Interval time
	Loops          int  // Number of times
	P99ThresholdUs int  // P99 latency threshold
	Stress         bool // stress test?
}

const CrasPriority = 12
const CrasClientPriority = 10
const StressPriority = 20
const DefaultIntervalUs = 10000
const DefaultLoops = 1000
const DefaultP99ThresholdUs = 100
const DefaultStressWorker = 10000

func init() {
	testing.AddTest(&testing.Test{
		Func:     CyclicTest,
		Desc:     "benchmarks of cyclic_test",
		Contacts: []string{"eddyhsu@chromium.org"},
		Attr:     []string{"group:crosbolt", "crosbolt_perbuild"},
		Timeout:  2 * time.Minute,
		Params: []testing.Param{
			{
				Name: "cras_pr",
				Val: cyclicTestParameters{
					Priority:       CrasPriority,
					Threads:        1,
					IntervalUs:     DefaultIntervalUs,
					Loops:          DefaultLoops,
					P99ThresholdUs: DefaultP99ThresholdUs,
					Stress:         false,
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
					Stress:         false,
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
					Stress:         false,
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
					Stress:         false,
				},
			},
			{
				Name: "cras_pr_with_stress",
				Val: cyclicTestParameters{
					Priority:       CrasPriority,
					Threads:        1,
					IntervalUs:     DefaultIntervalUs,
					Loops:          DefaultLoops,
					P99ThresholdUs: DefaultP99ThresholdUs,
					Stress:         true,
				},
			},
		},
	})
}

// cyclicTestStat contains the statistics of a thread.
type cyclicTestStat struct {
	median int
	p99    int
}

// ParseLatency parses `log`, collects the latencies of each thread
// and returns the results.
// The order of latencies for each thread is the same as the log.
func ParseLatency(log string, threads int, s *testing.State) [][]int {
	latencies := make([][]int, threads)
	dataRe := regexp.MustCompile(`^[ \t]+\d+:[ \t]+\d+:[ \t]+\d+$`)
	integerRe := regexp.MustCompile(`\d+`)
	for _, line := range strings.Split(log, "\n") {
		if !dataRe.MatchString(line) {
			continue
		}
		ints := integerRe.FindAllString(line, 3)

		tid, err := strconv.Atoi(ints[0])
		if err != nil {
			s.Fatal("Failed to parse data: ", err)
		}
		latency, err := strconv.Atoi(ints[2])
		if err != nil {
			s.Fatal("Failed to parse data: ", err)
		}

		latencies[tid] = append(latencies[tid], latency)
	}
	return latencies
}

// CalculateStats calcultes the statistics results of `latencies` and
// returns as `cyclicTestStat`.
func CalculateStats(latencies [][]int) []cyclicTestStat {
	stats := make([]cyclicTestStat, len(latencies))
	for idx := range stats {
		sort.Ints(latencies[idx])
		count := len(latencies[idx])
		if count == 0 {
			continue
		}
		stats[idx].median = latencies[idx][count/2]
		stats[idx].p99 = latencies[idx][count*99/100]
	}
	return stats
}

func CyclicTest(ctx context.Context, s *testing.State) {
	param := s.Param().(cyclicTestParameters)

	stress := testexec.CommandContext(ctx, "stress-ng",
		"--cpu="+strconv.Itoa(DefaultStressWorker),
		"--sched=rr",
		"--sched-prio="+strconv.Itoa(StressPriority))
	if param.Stress {
		err := stress.Start()
		if err != nil {
			s.Fatal("Failed to start stress-ng: ", err)
		}
	}

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

	if param.Stress {
		err := stress.Kill()
		if err != nil {
			s.Fatal("Failed to kill stress-ng: ", err)
		}
	}

	// The log will look like(task_number:count:latency_us):
	// Max CPUs = 8
	// Online CPUs = 8
	// # /dev/cpu_dma_latency set to 0us
	// Thread 0 Interval: 1000
	//        0:       0:       9
	//        0:       1:      18
	//        0:       2:      15
	//        0:       3:      14
	//        0:       4:      14
	//        0:       5:      14
	//        0:       6:      24
	//        0:       7:      16
	//        0:       8:      15
	//        0:       9:      14
	// ...
	latencies := ParseLatency(string(out), param.Threads, s)
	stats := CalculateStats(latencies)

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

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

type schedPolicy int

const (
	// none indicates no threads will be invoked.
	none schedPolicy = iota
	// rrSched uses rr as the scheduler.
	rrSched
	// otherSched uses other(normal) as the scheduler.
	otherSched
)

// cyclicTestParameters contains all the data needed to run a single test iteration.
type cyclicTestParameters struct {
	Priority       int         // Priority of the process
	Threads        int         // Number of threads
	IntervalUs     int         // Interval time
	Loops          int         // Number of times
	P99ThresholdUs int         // P99 latency threshold
	StressSched    schedPolicy // the schedule policy of the stress workload. none implies no stress.
}

const (
	// crasPrioriy indicates the rt-priority of cras.
	crasPriority = 12
	// crasClientPriority indicates the rt-priority of cras client.
	crasClientPriority = 10
	// stressPriority indicates the rt-priority of stress threads.
	stressPriority = 20
	// defaultIntervalUs is the default interval used in cyclictest.
	defaultIntervalUs = 10000
	// defaultLoops is the default number of loops tested in cyclictest.
	defaultLoops = 6000
	// defaultP99ThresholdUs is the default p99 latency threshold allowed in cyclictest.
	defaultP99ThresholdUs = 100
	// defaultStressWorker is the number of workers spawned in the stress test.
	defaultStressWorker = 20
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         CyclicBench,
		Desc:         "Benchmarks for scheduling latency with cyclictest binary",
		Contacts:     []string{"eddyhsu@chromium.org", "paulhsia@chromium.org", "cychiang@chromium.org"},
		Attr:         []string{"group:crosbolt", "crosbolt_perbuild"},
		SoftwareDeps: []string{"cras"},
		Timeout:      3 * time.Minute,
		Params: []testing.Param{
			{
				Name: "rr12_1thread_10ms",
				Val: cyclicTestParameters{
					Priority:       crasPriority,
					Threads:        1,
					IntervalUs:     defaultIntervalUs,
					Loops:          defaultLoops,
					P99ThresholdUs: defaultP99ThresholdUs,
					StressSched:    none,
				},
			},
			{
				Name: "rr10_1thread_10ms",
				Val: cyclicTestParameters{
					Priority:       crasClientPriority,
					Threads:        1,
					IntervalUs:     defaultIntervalUs,
					Loops:          defaultLoops,
					P99ThresholdUs: defaultP99ThresholdUs,
					StressSched:    none,
				},
			},
			{
				Name: "rr12_4thread_10ms",
				Val: cyclicTestParameters{
					Priority:       crasPriority,
					Threads:        4,
					IntervalUs:     defaultIntervalUs,
					Loops:          defaultLoops,
					P99ThresholdUs: defaultP99ThresholdUs,
					StressSched:    none,
				},
			},
			{
				Name: "rr10_4thread_10ms",
				Val: cyclicTestParameters{
					Priority:       crasClientPriority,
					Threads:        4,
					IntervalUs:     defaultIntervalUs,
					Loops:          defaultLoops,
					P99ThresholdUs: defaultP99ThresholdUs,
					StressSched:    none,
				},
			},
			{
				Name: "rr12_1thread_10ms_stress_rr20_20workers",
				Val: cyclicTestParameters{
					Priority:       crasPriority,
					Threads:        1,
					IntervalUs:     defaultIntervalUs,
					Loops:          defaultLoops,
					P99ThresholdUs: defaultP99ThresholdUs,
					StressSched:    rrSched,
				},
			},
			{
				Name: "rr12_1thread_10ms_stress_normal_20workers",
				Val: cyclicTestParameters{
					Priority:       crasPriority,
					Threads:        1,
					IntervalUs:     defaultIntervalUs,
					Loops:          defaultLoops,
					P99ThresholdUs: defaultP99ThresholdUs,
					StressSched:    otherSched,
				},
			},
		},
	})
}

// cyclicTestStat contains the statistics of a thread.
type cyclicTestStat struct {
	min    int
	median int
	p99    int
	max    int
}

// parseLatency parses `log`, collects the latencies of each thread
// and returns the results.
// The order of latencies for each thread is the same as the log.
func parseLatency(log string, threads int, s *testing.State) [][]int {
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

// calculateStats calcultes the statistics results of `latencies` and
// returns as `cyclicTestStat`.
func calculateStats(latencies [][]int) []cyclicTestStat {
	stats := make([]cyclicTestStat, len(latencies))
	for idx := range stats {
		sort.Ints(latencies[idx])
		count := len(latencies[idx])
		if count == 0 {
			continue
		}
		stats[idx].min = latencies[idx][0]
		stats[idx].median = latencies[idx][count/2]
		stats[idx].p99 = latencies[idx][count*99/100]
		stats[idx].max = latencies[idx][count-1]
	}
	return stats
}

func (s schedPolicy) String() string {
	return []string{"none", "rr", "other"}[s]
}

func CyclicBench(ctx context.Context, s *testing.State) {
	param := s.Param().(cyclicTestParameters)

	// Set the timeout of stress to be 10% more of the expected time
	// of cyclic test in case the stress-ng failed to be killed.
	timeout := param.Loops * param.IntervalUs / 1000000 * 11 / 10

	// TODO(eddyhsu): let stress priority configurable.
	stress := testexec.CommandContext(ctx, "stress-ng",
		"--cpu="+strconv.Itoa(defaultStressWorker),
		"--sched="+param.StressSched.String(),
		"--sched-prio="+strconv.Itoa(stressPriority),
		"--timeout="+strconv.Itoa(timeout)+"s")
	// Working directory of `stress-ng` must be readable and writeable
	stress.Dir = "/tmp"

	if param.StressSched != none {
		if err := stress.Start(); err != nil {
			s.Fatal("Failed to start stress-ng: ", err)
		}
	}

	out, err := testexec.CommandContext(ctx, "cyclictest",
		// TODO(eddyhsu): supports other types of policy.
		"--policy=rr",
		"--priority="+strconv.Itoa(param.Priority),
		"--interval="+strconv.Itoa(param.IntervalUs),
		"--threads="+strconv.Itoa(param.Threads),
		"--loops="+strconv.Itoa(param.Loops),
		// When there are multi-threads, the interval of the i-th
		// thread will be (`interval` + i * `distance`).
		// Set distance to 0 to make all the intervals equal.
		"--distance=0",
		"--verbose").Output(testexec.DumpLogOnError)
	if err != nil {
		s.Fatal("Failed to execute cyclictest: ", err)
	}

	if param.StressSched != none {
		if err := stress.Wait(); err != nil {
			s.Error("stress-ng failed to finish: ", err)
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
	latencies := parseLatency(string(out), param.Threads, s)
	stats := calculateStats(latencies)

	p := perf.NewValues()
	for index, stat := range stats {
		name := "Thread_" + strconv.Itoa(index)
		minLatency := perf.Metric{
			Name:      name,
			Variant:   "min_latency",
			Unit:      "us",
			Direction: perf.SmallerIsBetter}
		p.Set(minLatency, float64(stat.min))
		medianLatency := perf.Metric{
			Name:      name,
			Variant:   "p50_latency",
			Unit:      "us",
			Direction: perf.SmallerIsBetter}
		p.Set(medianLatency, float64(stat.median))
		p99Latency := perf.Metric{
			Name:      name,
			Variant:   "p99_latency",
			Unit:      "us",
			Direction: perf.SmallerIsBetter}
		p.Set(p99Latency, float64(stat.p99))
		maxLatency := perf.Metric{
			Name:      name,
			Variant:   "max_latency",
			Unit:      "us",
			Direction: perf.SmallerIsBetter}
		p.Set(maxLatency, float64(stat.max))

		if stat.p99 > param.P99ThresholdUs {
			s.Error("p99 latency exceeds threshold: ", stat.p99,
				" > ", param.P99ThresholdUs)
		}
	}
	if err := p.Save(s.OutDir()); err != nil {
		s.Error("Failed saving perf data: ", err)
	}
}

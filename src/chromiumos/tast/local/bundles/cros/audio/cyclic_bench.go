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
	"chromiumos/tast/errors"
	"chromiumos/tast/testing"
)

type schedPolicy int

const (
	// rrSched uses rr as the scheduler.
	rrSched schedPolicy = iota
	// otherSched uses other(normal) as the scheduler.
	otherSched
)

type schedConfig struct {
	Policy   schedPolicy // the schedule policy.
	Priority int         // Priority of the process. If `Policy` is real time, `Priority` is real time priority. If `Policy` is CFS, `Priority` specify the nice value.
}

// cyclicTestParameters contains all the data needed to run a single test iteration.
type cyclicTestParameters struct {
	Config         schedConfig  // The schedule config of the cyclictest.
	Threads        int          // Number of threads.
	IntervalUs     int          // Interval time.
	Loops          int          // Number of times.
	P99ThresholdUs int          // P99 latency threshold.
	StressConfig   *schedConfig // The schedule config of the stress process. if `StressConfig` is nil, no stress process will be run.
}

const (
	// crasPrioriy indicates the rt-priority of cras.
	crasPriority = 12
	// crasClientPriority indicates the rt-priority of cras client.
	crasClientPriority = 10
	// defaultStressPriority indicates the default rt-priority of stress threads.
	defaultStressPriority = 20
	// defaultIntervalUs is the default interval used in cyclictest.
	defaultIntervalUs = 10000
	// defaultLoops is the default number of loops tested in cyclictest.
	defaultLoops = 6000
	// defaultP99ThresholdUs is the default p99 latency threshold allowed in cyclictest.
	defaultP99ThresholdUs = 100
	// defaultStressWorker is the number of workers spawned in the stress test per cpu thread.
	defaultStressWorker = 2
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
					Config: schedConfig{
						Policy:   rrSched,
						Priority: crasPriority,
					},
					Threads:        1,
					IntervalUs:     defaultIntervalUs,
					Loops:          defaultLoops,
					P99ThresholdUs: defaultP99ThresholdUs,
					StressConfig:   nil,
				},
			},
			{
				Name: "rr10_1thread_10ms",
				Val: cyclicTestParameters{
					Config: schedConfig{
						Policy:   rrSched,
						Priority: crasClientPriority,
					},
					Threads:        1,
					IntervalUs:     defaultIntervalUs,
					Loops:          defaultLoops,
					P99ThresholdUs: defaultP99ThresholdUs,
					StressConfig:   nil,
				},
			},
			{
				Name: "rr12_4thread_10ms",
				Val: cyclicTestParameters{
					Config: schedConfig{
						Policy:   rrSched,
						Priority: crasPriority,
					},
					Threads:        4,
					IntervalUs:     defaultIntervalUs,
					Loops:          defaultLoops,
					P99ThresholdUs: defaultP99ThresholdUs,
					StressConfig:   nil,
				},
			},
			{
				Name: "rr10_4thread_10ms",
				Val: cyclicTestParameters{
					Config: schedConfig{
						Policy:   rrSched,
						Priority: crasClientPriority,
					},
					Threads:        4,
					IntervalUs:     defaultIntervalUs,
					Loops:          defaultLoops,
					P99ThresholdUs: defaultP99ThresholdUs,
					StressConfig:   nil,
				},
			},
			{
				Name: "rr12_1thread_10ms_stress_rr20_2workers_per_cpu",
				Val: cyclicTestParameters{
					Config: schedConfig{
						Policy:   rrSched,
						Priority: crasPriority,
					},
					Threads:        1,
					IntervalUs:     defaultIntervalUs,
					Loops:          defaultLoops,
					P99ThresholdUs: defaultP99ThresholdUs,
					StressConfig: &schedConfig{
						Policy:   rrSched,
						Priority: defaultStressPriority,
					},
				},
			},
			{
				Name: "rr12_1thread_10ms_stress_nice_p0_2workers_per_cpu",
				Val: cyclicTestParameters{
					Config: schedConfig{
						Policy:   rrSched,
						Priority: crasPriority,
					},
					Threads:        1,
					IntervalUs:     defaultIntervalUs,
					Loops:          defaultLoops,
					P99ThresholdUs: defaultP99ThresholdUs,
					StressConfig: &schedConfig{
						Policy:   otherSched,
						Priority: 0,
					},
				},
			},
			{
				Name: "nice_p0_1thread_10ms",
				Val: cyclicTestParameters{
					Config: schedConfig{
						Policy:   otherSched,
						Priority: 0,
					},
					Threads:        1,
					IntervalUs:     defaultIntervalUs,
					Loops:          defaultLoops,
					P99ThresholdUs: defaultP99ThresholdUs,
					StressConfig:   nil,
				},
			},
			{
				Name: "nice_n20_1thread_10ms",
				Val: cyclicTestParameters{
					Config: schedConfig{
						Policy:   otherSched,
						Priority: -20,
					},
					Threads:        1,
					IntervalUs:     defaultIntervalUs,
					Loops:          defaultLoops,
					P99ThresholdUs: defaultP99ThresholdUs,
					StressConfig:   nil,
				},
			},
			{
				Name: "nice_p19_1thread_10ms",
				Val: cyclicTestParameters{
					Config: schedConfig{
						Policy:   otherSched,
						Priority: 19,
					},
					Threads:        1,
					IntervalUs:     defaultIntervalUs,
					Loops:          defaultLoops,
					P99ThresholdUs: 5000,
					StressConfig:   nil,
				},
			},
			{
				Name: "nice_p0_1thread_10ms_stress_nice_p0_2workers_per_cpu",
				Val: cyclicTestParameters{
					Config: schedConfig{
						Policy:   otherSched,
						Priority: 0,
					},
					Threads:        1,
					IntervalUs:     defaultIntervalUs,
					Loops:          defaultLoops,
					P99ThresholdUs: 30000,
					StressConfig: &schedConfig{
						Policy:   otherSched,
						Priority: 0,
					},
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
	return []string{"rr", "other"}[s]
}

func getNumberOfCPU(ctx context.Context) (int, error) {
	lscpu := testexec.CommandContext(ctx, "lscpu")
	out, err := lscpu.Output()
	if err != nil {
		return -1, errors.Wrap(err, "lscpu failed")
	}
	cpuRe := regexp.MustCompile(`^CPU\(s\):\s*(.*)$`)
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if !cpuRe.MatchString(line) {
			continue
		}
		cpus := cpuRe.FindStringSubmatch(line)
		ret, err := strconv.Atoi(cpus[1])
		if err != nil {
			return -1, errors.Wrap(err, "parsing number of cpus failed")
		}
		return ret, nil
	}
	return -1, errors.New("can't find CPU(s) info in lscpu")
}

func getCommandContext(ctx context.Context, param cyclicTestParameters) (*testexec.Cmd, error) {
	switch param.Config.Policy {
	case rrSched:
		return testexec.CommandContext(ctx, "cyclictest",
			"--policy="+param.Config.Policy.String(),
			"--priority="+strconv.Itoa(param.Config.Priority),
			"--interval="+strconv.Itoa(param.IntervalUs),
			"--threads="+strconv.Itoa(param.Threads),
			"--loops="+strconv.Itoa(param.Loops),
			// When there are multi-threads, the interval of the i-th
			// thread will be (`interval` + i * `distance`).
			// Set distance to 0 to make all the intervals equal.
			"--distance=0",
			"--verbose"), nil
	case otherSched:
		return testexec.CommandContext(ctx, "nice",
			"-n", strconv.Itoa(param.Config.Priority),
			"cyclictest",
			"--policy=other",
			"--interval="+strconv.Itoa(param.IntervalUs),
			"--threads="+strconv.Itoa(param.Threads),
			"--loops="+strconv.Itoa(param.Loops),
			// When there are multi-threads, the interval of the i-th
			// thread will be (`interval` + i * `distance`).
			// Set distance to 0 to make all the intervals equal.
			"--distance=0",
			"--verbose"), nil
	}
	return nil, errors.New("unsupported scheduling policy")
}

func getStressContext(ctx context.Context, param cyclicTestParameters) (*testexec.Cmd, error) {
	if param.StressConfig == nil {
		return nil, nil
	}

	// Set the timeout of stress to be 10% more of the expected time
	// of cyclic test in case the stress-ng failed to be killed.
	timeout := param.Loops * param.IntervalUs / 1000000 * 11 / 10

	cpus, err := getNumberOfCPU(ctx)
	if err != nil {
		return nil, err
	}
	totalWorkers := defaultStressWorker * cpus

	switch param.StressConfig.Policy {
	case rrSched:
		return testexec.CommandContext(ctx, "stress-ng",
			"--cpu="+strconv.Itoa(totalWorkers),
			"--sched="+param.StressConfig.Policy.String(),
			"--sched-prio="+strconv.Itoa(param.StressConfig.Priority),
			"--timeout="+strconv.Itoa(timeout)+"s"), nil
	case otherSched:
		return testexec.CommandContext(ctx, "nice",
			"-n", strconv.Itoa(param.StressConfig.Priority),
			"stress-ng",
			"--cpu="+strconv.Itoa(totalWorkers),
			"--sched=other",
			"--timeout="+strconv.Itoa(timeout)+"s"), nil
	}
	return nil, errors.New("unsupported stress scheduling policy")
}

func CyclicBench(ctx context.Context, s *testing.State) {
	param := s.Param().(cyclicTestParameters)

	stress, err := getStressContext(ctx, param)
	if err != nil {
		s.Error("Failed to get stress command context: ", err)
	}

	if stress != nil {
		// Working directory of `stress-ng` must be readable and writeable
		stress.Dir = "/tmp"
		if err := stress.Start(); err != nil {
			s.Fatal("Failed to start stress-ng: ", err)
		}
	}

	cmd, err := getCommandContext(ctx, param)
	if err != nil {
		s.Fatal("Failed to get command context of cyclictest: ", err)
	}
	out, err := cmd.Output(testexec.DumpLogOnError)
	if err != nil {
		s.Fatal("Failed to execute cyclictest: ", err)
	}

	if stress != nil {
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

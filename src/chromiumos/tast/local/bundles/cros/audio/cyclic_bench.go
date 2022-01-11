// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package audio

import (
	"context"
	"regexp"
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

type affinity int

const (
	// defaultAff will use all the processors in round-robin order.
	defaultAff affinity = iota
	// smallCore will run all the threads on a single small core.
	smallCore
	// bigCore will run all the threads on a single big core.
	bigCore
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
	Affinity       affinity     // Run cyclictest threads on which sets of processors.
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
					Affinity:       defaultAff,
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
					Affinity:       defaultAff,
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
					Affinity:       defaultAff,
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
					Affinity:       defaultAff,
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
					Affinity:       defaultAff,
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
					Affinity:       defaultAff,
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
					Affinity:       defaultAff,
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
					Affinity:       defaultAff,
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
					Affinity:       defaultAff,
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
					Affinity:       defaultAff,
					P99ThresholdUs: 30000,
					StressConfig: &schedConfig{
						Policy:   otherSched,
						Priority: 0,
					},
				},
			},
			{
				Name: "rr12_1thread_10ms_small_core",
				Val: cyclicTestParameters{
					Config: schedConfig{
						Policy:   rrSched,
						Priority: crasPriority,
					},
					Threads:        1,
					IntervalUs:     defaultIntervalUs,
					Loops:          defaultLoops,
					Affinity:       smallCore,
					P99ThresholdUs: defaultP99ThresholdUs,
					StressConfig:   nil,
				},
				ExtraSoftwareDeps: []string{"arm"}, // arm has heterogeneous cores.
			},
			{
				Name: "rr12_1thread_10ms_big_core",
				Val: cyclicTestParameters{
					Config: schedConfig{
						Policy:   rrSched,
						Priority: crasPriority,
					},
					Threads:        1,
					IntervalUs:     defaultIntervalUs,
					Loops:          defaultLoops,
					Affinity:       bigCore,
					P99ThresholdUs: defaultP99ThresholdUs,
					StressConfig:   nil,
				},
				ExtraSoftwareDeps: []string{"arm"}, // arm has heterogeneous cores.
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

func parseCyclicBenchResult(log string) ([]cyclicTestStat, error) {
	statRe := regexp.MustCompile(`^([a-z0-9]+):\s*(\d+)$`)

	// the log should look like
	// Thread #<num>:
	// min: <min>
	// median: <median>
	// p99: <p99>
	// max: <max>
	var results []cyclicTestStat
	var curStat cyclicTestStat
	for _, line := range strings.Split(log, "\n") {
		if !statRe.MatchString(line) {
			continue
		}
		stat := statRe.FindStringSubmatch(line)
		statType := stat[1]
		value, err := strconv.Atoi(stat[2])
		if err != nil {
			return results, errors.Wrap(err, "unrecognized stat type: "+line)
		}
		if statType == "min" {
			curStat.min = value
		} else if statType == "median" {
			curStat.median = value
		} else if statType == "p99" {
			curStat.p99 = value
		} else if statType == "max" {
			curStat.max = value
			results = append(results, curStat)
		} else {
			return results, errors.New("unrecognized stat type: " + line)
		}
	}
	return results, nil
}

func (s schedPolicy) String() string {
	return []string{"rr", "other"}[s]
}

func (a affinity) String() string {
	return []string{"default", "small_core", "big_core"}[a]
}

func CyclicBench(ctx context.Context, s *testing.State) {
	param := s.Param().(cyclicTestParameters)

	cmdStr := []string{"cyclic_bench.py",
		"--policy=" + param.Config.Policy.String(),
		"--priority=" + strconv.Itoa(param.Config.Priority),
		"--interval=" + strconv.Itoa(param.IntervalUs),
		"--threads=" + strconv.Itoa(param.Threads),
		"--loops=" + strconv.Itoa(param.Loops),
		"--affinity=" + param.Affinity.String(),
	}
	if param.StressConfig != nil {
		cmdStr = append(cmdStr,
			"--stress_policy="+param.StressConfig.Policy.String(),
			"--stress_priority="+strconv.Itoa(param.StressConfig.Priority),
			"--workers_per_cpu="+strconv.Itoa(defaultStressWorker))
	}
	out, err := testexec.CommandContext(ctx, cmdStr[0], cmdStr[1:]...).Output(testexec.DumpLogOnError)
	if err != nil {
		s.Fatal("Failed to execute cyclic_bench.py: ", err)
	}

	stats, err := parseCyclicBenchResult(string(out))
	if err != nil {
		s.Error("Failed to parse result file: ", err)
	}

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

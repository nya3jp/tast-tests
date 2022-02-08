// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package vm

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"chromiumos/tast/common/perf"
	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/vm/audioutils"
	"chromiumos/tast/local/bundles/cros/vm/dlc"
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
	Config              schedConfig   // The schedule config of the cyclictest.
	Threads             int           // Number of threads.
	Interval            time.Duration // Interval time.
	Loops               int           // Number of times.
	Affinity            affinity      // Run cyclictest threads on which sets of processors.
	P99Threshold        time.Duration // P99 latency threshold.
	StressConfig        *schedConfig  // The schedule config of the stress process. if `StressConfig` is nil, no stress process will be run.
	StressOutOfVMConfig *schedConfig  // The schedule config of the stress process out of VM. If `StressOutOfVMConfig` is nil, no stress out of VM will be run.
}

const (
	// crasPrioriy indicates the rt-priority of cras.
	crasPriority = 12
	// crasClientPriority indicates the rt-priority of cras client.
	crasClientPriority = 10
	// defaultStressPriority indicates the default rt-priority of stress threads.
	defaultStressPriority = 20
	// defaultInterval is the default interval used in cyclictest.
	defaultInterval = 10000 * time.Microsecond
	// defaultLoops is the default number of loops tested in cyclictest.
	defaultLoops = 6000
	// defaultP99Threshold is the default p99 latency threshold allowed in cyclictest.
	defaultP99Threshold = 1200 * time.Microsecond
	// defaultStressWorker is the number of workers spawned in the stress test per cpu thread.
	defaultStressWorker = 2
)

const runCyclicTest string = "run-cyclic-test.sh"

func init() {
	testing.AddTest(&testing.Test{
		Func:         AudioCyclicBench,
		Desc:         "Benchmarks for scheduling latency with cyclictest binary",
		Contacts:     []string{"eddyhsu@chromium.org", "paulhsia@chromium.org", "cychiang@chromium.org"},
		Attr:         []string{"group:crosbolt", "crosbolt_perbuild"},
		Data:         []string{runCyclicTest},
		SoftwareDeps: []string{"cras", "vm_host", "dlc"},
		Timeout:      6 * time.Minute,
		Fixture:      "vmDLC",
		Params: []testing.Param{
			{
				Name: "rr12_1thread_10ms",
				Val: cyclicTestParameters{
					Config: schedConfig{
						Policy:   rrSched,
						Priority: crasPriority,
					},
					Threads:             1,
					Interval:            defaultInterval,
					Loops:               defaultLoops,
					Affinity:            defaultAff,
					P99Threshold:        defaultP99Threshold,
					StressConfig:        nil,
					StressOutOfVMConfig: nil,
				},
			},
			{
				Name: "rr10_1thread_10ms",
				Val: cyclicTestParameters{
					Config: schedConfig{
						Policy:   rrSched,
						Priority: crasClientPriority,
					},
					Threads:             1,
					Interval:            defaultInterval,
					Loops:               defaultLoops,
					Affinity:            defaultAff,
					P99Threshold:        defaultP99Threshold,
					StressConfig:        nil,
					StressOutOfVMConfig: nil,
				},
			},
			{
				Name: "rr12_1thread_10ms_stress_nice_p0_2workers_per_cpu",
				Val: cyclicTestParameters{
					Config: schedConfig{
						Policy:   rrSched,
						Priority: crasPriority,
					},
					Threads:      1,
					Interval:     defaultInterval,
					Loops:        defaultLoops,
					Affinity:     defaultAff,
					P99Threshold: defaultP99Threshold,
					StressConfig: &schedConfig{
						Policy:   otherSched,
						Priority: 0,
					},
					StressOutOfVMConfig: nil,
				},
			},
			{
				Name: "nice_p0_1thread_10ms",
				Val: cyclicTestParameters{
					Config: schedConfig{
						Policy:   otherSched,
						Priority: 0,
					},
					Threads:             1,
					Interval:            defaultInterval,
					Loops:               defaultLoops,
					Affinity:            defaultAff,
					P99Threshold:        1800 * time.Microsecond,
					StressConfig:        nil,
					StressOutOfVMConfig: nil,
				},
			},
			{
				Name: "nice_n20_1thread_10ms",
				Val: cyclicTestParameters{
					Config: schedConfig{
						Policy:   otherSched,
						Priority: -20,
					},
					Threads:             1,
					Interval:            defaultInterval,
					Loops:               defaultLoops,
					Affinity:            defaultAff,
					P99Threshold:        1800 * time.Microsecond,
					StressConfig:        nil,
					StressOutOfVMConfig: nil,
				},
			},
			{
				Name: "nice_p19_1thread_10ms",
				Val: cyclicTestParameters{
					Config: schedConfig{
						Policy:   otherSched,
						Priority: 19,
					},
					Threads:             1,
					Interval:            defaultInterval,
					Loops:               defaultLoops,
					Affinity:            defaultAff,
					P99Threshold:        5000 * time.Microsecond,
					StressConfig:        nil,
					StressOutOfVMConfig: nil,
				},
			},
			{
				Name: "nice_p0_1thread_10ms_stress_nice_p0_2workers_per_cpu",
				Val: cyclicTestParameters{
					Config: schedConfig{
						Policy:   otherSched,
						Priority: 0,
					},
					Threads:      1,
					Interval:     defaultInterval,
					Loops:        defaultLoops,
					Affinity:     defaultAff,
					P99Threshold: 30000 * time.Microsecond,
					StressConfig: &schedConfig{
						Policy:   otherSched,
						Priority: 0,
					},
					StressOutOfVMConfig: nil,
				},
			},
			{
				Name: "rr12_1thread_10ms_stress_out_of_vm_nice_p0_2workers_per_cpu",
				Val: cyclicTestParameters{
					Config: schedConfig{
						Policy:   rrSched,
						Priority: crasPriority,
					},
					Threads:      1,
					Interval:     defaultInterval,
					Loops:        defaultLoops,
					Affinity:     defaultAff,
					P99Threshold: defaultP99Threshold,
					StressConfig: nil,
					StressOutOfVMConfig: &schedConfig{
						Policy:   otherSched,
						Priority: 0,
					},
				},
			},
		},
	})
}

func (s schedPolicy) String() string {
	return []string{"rr", "other"}[s]
}

func (a affinity) String() string {
	return []string{"default", "small_core", "big_core"}[a]
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

func getStressCommandContext(ctx context.Context, param cyclicTestParameters) (*testexec.Cmd, error) {
	if param.StressOutOfVMConfig == nil {
		return nil, nil
	}

	config := param.StressOutOfVMConfig

	// Set the timeout of stress to be 10% more of the expected time
	// of cyclic test in case the stress-ng failed to be killed.
	timeout := param.Loops * (int(param.Interval / time.Microsecond)) / 1000000 * 11 / 10
	if param.StressConfig != nil {
		// Set the timeout twice if there's stress inside the vm
		// to avoid stress out of vm finishing too early.
		timeout = timeout * 2
	}

	cpus, err := getNumberOfCPU(ctx)
	if err != nil {
		return nil, err
	}
	totalWorkers := defaultStressWorker * cpus

	switch config.Policy {
	case rrSched:
		return testexec.CommandContext(ctx, "stress-ng",
			"--cpu="+strconv.Itoa(totalWorkers),
			"--sched="+config.Policy.String(),
			"--sched-prio="+strconv.Itoa(config.Priority),
			"--timeout="+strconv.Itoa(timeout)+"s"), nil
	case otherSched:
		return testexec.CommandContext(ctx, "nice",
			"-n", strconv.Itoa(config.Priority),
			"stress-ng",
			"--cpu="+strconv.Itoa(totalWorkers),
			"--sched=other",
			"--timeout="+strconv.Itoa(timeout)+"s"), nil
	}
	return nil, errors.New("unsupported stress scheduling policy")
}

func AudioCyclicBench(ctx context.Context, s *testing.State) {
	param := s.Param().(cyclicTestParameters)
	data := s.FixtValue().(dlc.FixtData)

	kernelLogPath := filepath.Join(s.OutDir(), "kernel.log")
	outputFilePath := filepath.Join(s.OutDir(), "output.log")

	testArgs := []string{
		"--policy=" + param.Config.Policy.String(),
		"--priority=" + strconv.Itoa(param.Config.Priority),
		"--interval=" + strconv.Itoa(int(param.Interval/time.Microsecond)),
		"--threads=" + strconv.Itoa(param.Threads),
		"--loops=" + strconv.Itoa(param.Loops),
		"--affinity=" + param.Affinity.String(),
		"--output_file=" + outputFilePath,
		"--json"}
	if param.StressConfig != nil {
		testArgs = append(testArgs,
			"--stress_policy="+param.StressConfig.Policy.String(),
			"--stress_priority="+strconv.Itoa(param.StressConfig.Priority),
			"--workers_per_cpu="+strconv.Itoa(defaultStressWorker))
	}

	kernelArgs := []string{
		fmt.Sprintf("init=%s", s.DataPath(runCyclicTest)),
		"--",
	}
	kernelArgs = append(kernelArgs, testArgs...)

	stressOutOfVM, err := getStressCommandContext(ctx, param)
	if err != nil {
		s.Error("Failed to get stress(out of vm) command context: ", err)
	}

	if stressOutOfVM != nil {
		// Working directory of `stress-ng` must be readable and writeable
		stressOutOfVM.Dir = "/tmp"
		if err := stressOutOfVM.Start(); err != nil {
			s.Fatal("Failed to start stress workload out of vm: ", err)
		}
	}

	s.Log("Running Cyclic test")
	if err := audioutils.RunCrosvm(ctx, data.Kernel, kernelLogPath, kernelArgs, audioutils.Config{}); err != nil {
		s.Fatal("Failed to run crosvm: ", err)
	}

	buf, err := ioutil.ReadFile(outputFilePath)
	if err != nil {
		s.Error("Failed to read output file: ", err)
	}

	stats := struct {
		CyclicTestStat []struct {
			ThreadID float64 `json:"thread_id"`
			Min      float64 `json:"min"`
			Median   float64 `json:"median"`
			P99      float64 `json:"p99"`
			Max      float64 `json:"max"`
		} `json:"stats"`
	}{}
	err = json.Unmarshal(buf, &stats)
	if err != nil {
		s.Error("Failed to parse result file: ", err)
	}

	if stressOutOfVM != nil {
		if err := stressOutOfVM.Wait(); err != nil {
			s.Error("stress-ng failed to finish: ", err)
		}
	}

	p := perf.NewValues()
	for _, stat := range stats.CyclicTestStat {
		threadID := int(stat.ThreadID)
		name := "Thread_" + strconv.Itoa(threadID)
		minLatency := perf.Metric{
			Name:      name,
			Variant:   "min_latency",
			Unit:      "us",
			Direction: perf.SmallerIsBetter}
		p.Set(minLatency, stat.Min)
		medianLatency := perf.Metric{
			Name:      name,
			Variant:   "p50_latency",
			Unit:      "us",
			Direction: perf.SmallerIsBetter}
		p.Set(medianLatency, stat.Median)
		p99Latency := perf.Metric{
			Name:      name,
			Variant:   "p99_latency",
			Unit:      "us",
			Direction: perf.SmallerIsBetter}
		p.Set(p99Latency, stat.P99)
		maxLatency := perf.Metric{
			Name:      name,
			Variant:   "max_latency",
			Unit:      "us",
			Direction: perf.SmallerIsBetter}
		p.Set(maxLatency, stat.Max)

		if stat.P99 > float64(param.P99Threshold/time.Microsecond) {
			s.Error("p99 latency exceeds threshold: ", stat.P99,
				" > ", param.P99Threshold)
		}
	}
	if err := p.Save(s.OutDir()); err != nil {
		s.Error("Failed saving perf data: ", err)
	}

}

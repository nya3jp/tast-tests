// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package crostini

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/crostini"
	"chromiumos/tast/local/crostini/perfutil"
	"chromiumos/tast/local/perf"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/local/vm"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         CPUPerf,
		Desc:         "Tests Crostini CPU performance",
		Contacts:     []string{"cylee@chromium.org", "cros-containers-dev@google.com"},
		Attr:         []string{"group:crosbolt", "crosbolt_perbuild"},
		Timeout:      12 * time.Minute,
		Data:         []string{crostini.ImageArtifact},
		Pre:          crostini.StartedByArtifact(),
		SoftwareDeps: []string{"chrome", "vm_host"},
	})
}

func CPUPerf(ctx context.Context, s *testing.State) {
	cont := s.PreValue().(crostini.PreData).Container

	perfValues := perf.NewValues()
	defer perfValues.Save(s.OutDir())

	// Prepare error log file.
	errFile, err := os.Create(filepath.Join(s.OutDir(), "error_log.txt"))
	if err != nil {
		s.Fatal("Failed to create error log: ", err)
	}
	defer errFile.Close()

	testSysbench(ctx, s, errFile, cont, perfValues)
	testLmbench(ctx, s, errFile, cont, perfValues)
}

func testSysbench(ctx context.Context, s *testing.State, errFile *os.File, cont *vm.Container, perfValues *perf.Values) {
	// Find sysbench binary location.
	out, err := perfutil.RunCmd(ctx, testexec.CommandContext(ctx, "which", "sysbench"), errFile)
	if err != nil {
		// TODO(dverkamp): sysbench is not currently built on arm platforms,
		// so instead of failing the whole test, just don't report sysbench
		// results if the binary isn't available.
		// See https://crbug.com/922178 for additional details.
		s.Log("Failed to find sysbench binary location: ", err)
		return
	}

	sysbenchBinaryFile := strings.TrimSpace(string(out))
	s.Log("Found sysbench binary location: ", sysbenchBinaryFile)

	// Util object to run sysbench in container.
	sysbenchRunner, err := perfutil.NewHostBinaryRunner(ctx, sysbenchBinaryFile, cont, errFile)
	if err != nil {
		s.Fatal("Failed to setup sysbench to run in container: ", err)
	}

	// Parse sysbench result for "sysbench cpu run". We care about "total number of events" only so far.
	// Sample output:
	// Running the test with following options:
	// Number of threads: 1
	// Initializing random number generator from current time
	//
	//
	// Prime numbers limit: 10000
	//
	// Initializing worker threads...
	//
	// Threads started!
	//
	// CPU speed:
	//     events per second:  1170.61
	//
	// General statistics:
	//     total time:                          10.0004s
	//     total number of events:              11708
	//
	// Latency (ms):
	//          min:                                  0.84
	//          avg:                                  0.85
	//          max:                                  3.47
	//          95th percentile:                      0.89
	//          sum:                               9997.74
	//
	// Threads fairness:
	//     events (avg/stddev):           11708.0000/0.00
	//     execution time (avg/stddev):   9.9977/0.00
	parseSysbenchOutput := func(out string) (numEvents int, err error) {
		samplePattern := regexp.MustCompile(`(?m)^\s*total number of events:\s+(\d+)`)
		matched := samplePattern.FindStringSubmatch(out)
		if matched == nil {
			return 0, errors.New("failed to parse sysbench result")
		}
		numEvents, err = strconv.Atoi(matched[1])
		if err != nil {
			return 0, errors.Wrapf(err, "could not parse int from %q", matched[1])
		}
		return numEvents, nil
	}

	measureSysbench := func(numThread int) error {
		args := []string{
			"cpu",
			"run",
			fmt.Sprintf("--num-threads=%d", numThread),
		}
		hostCmd := testexec.CommandContext(ctx, "sysbench", args...)
		out, err := perfutil.RunCmd(ctx, hostCmd, errFile)
		if err != nil {
			return errors.Wrap(err, "failed to run sysbench on host")
		}
		hostNumEvents, err := parseSysbenchOutput(string(out))
		if err != nil {
			perfutil.WriteError(ctx, errFile, strings.Join(hostCmd.Args, " "), out)
			return errors.Wrap(err, "failed to parse sysbench output on host")
		}

		guestCmd := sysbenchRunner.Command(ctx, args...)
		out, err = perfutil.RunCmd(ctx, guestCmd, errFile)
		if err != nil {
			return errors.Wrap(err, "failed to run sysbench on guest")
		}
		guestNumEvents, err := parseSysbenchOutput(string(out))
		if err != nil {
			perfutil.WriteError(ctx, errFile, strings.Join(guestCmd.Args, " "), out)
			return errors.Wrap(err, "failed to parse sysbench output on guest")
		}

		ratio := float64(guestNumEvents) / float64(hostNumEvents)
		s.Logf("sysbench num threads: %v, host events: %v, guest events %v, guest/host ratio %.3f",
			numThread, hostNumEvents, guestNumEvents, ratio)

		metricName := func(subName string) string {
			return fmt.Sprintf("sysbench_%v_threads_%v", numThread, subName)
		}
		perfValues.Append(
			perf.Metric{
				Name:      "crostini_cpu",
				Variant:   metricName("host"),
				Unit:      "events",
				Direction: perf.BiggerIsBetter,
				Multiple:  true,
			},
			float64(hostNumEvents))
		perfValues.Append(
			perf.Metric{
				Name:      "crostini_cpu",
				Variant:   metricName("guest"),
				Unit:      "events",
				Direction: perf.BiggerIsBetter,
				Multiple:  true,
			},
			float64(guestNumEvents))
		perfValues.Append(
			perf.Metric{
				Name:      "crostini_cpu",
				Variant:   metricName("ratio"),
				Unit:      "percentage",
				Direction: perf.BiggerIsBetter,
				Multiple:  true,
			},
			ratio)
		return nil
	}

	numCPU := runtime.NumCPU()
	const repeatNum = 3
	for numThreads := 1; numThreads <= numCPU; numThreads++ {
		for numTry := 1; numTry <= repeatNum; numTry++ {
			s.Logf("Measuring sysbench for %v thread(s) (%v/%v)", numThreads, numTry, repeatNum)
			if err := measureSysbench(numThreads); err != nil {
				s.Errorf("sysbench for %d thread(s) failed: %v", numThreads, err)
			}
		}
	}
}

func testLmbench(ctx context.Context, s *testing.State, errFile *os.File, cont *vm.Container, perfValues *perf.Values) {
	// Latest lmbench defaults to install individual microbenchamrks in /usr/lib/lmbench/bin/<arch dependent folder>
	// (e.g., /usr/lib/lmbench/bin/x86_64-linux-gnu). So needs to find the exact path.
	out, err := perfutil.RunCmd(ctx, cont.Command(ctx, "find", "/usr/lib/lmbench", "-name", "lat_syscall"), errFile)
	if err != nil {
		s.Fatal("Failed to find syscall benchmark binary in container: ", err)
	}
	guestSyscallBenchBinary := strings.TrimSpace(string(out))
	s.Log("Found syscall benchmark installed in container: ", guestSyscallBenchBinary)

	// Output parser. Sample output: "Simple write: 0.2412 microseconds".
	// It's always in microseconds for lat_syscall.
	parseSyscallBenchOutput := func(out string) (time.Duration, error) {
		samplePattern := regexp.MustCompile(`.*: (\d*\.?\d+) microseconds`)
		matched := samplePattern.FindStringSubmatch(strings.TrimSpace(out))
		if matched == nil {
			return 0.0, errors.Errorf("unable to match time from %q", out)
		}
		t, err := strconv.ParseFloat(matched[1], 64)
		if err != nil {
			return 0.0, errors.Wrapf(err, "failed to parse time %q in lat_syscall output", matched[1])
		}
		return time.Duration(t * float64(time.Microsecond)), nil
	}

	// Measure syscall time.
	measureSyscallTime := func(args ...string) error {
		options := []string{
			"-N", "10", // repetition times.
		}
		allArgs := append(options, args...)

		// Current version of lmbench on CrOS installs individual benchmarks in /usr/local/bin so
		// can be called directly.
		out, err := perfutil.RunCmd(ctx, testexec.CommandContext(ctx, "lat_syscall", allArgs...), errFile)
		if err != nil {
			return errors.Wrap(err, "failed to run lat_syscall on host")
		}
		hostTime, err := parseSyscallBenchOutput(string(out))
		if err != nil {
			return errors.Wrap(err, "failed to parse lat_syscall output on host")
		}

		// Guest binary is in /usr/lib/lmbench/...
		guestCommandArgs := append([]string{guestSyscallBenchBinary}, allArgs...)
		out, err = perfutil.RunCmd(ctx, cont.Command(ctx, guestCommandArgs...), errFile)
		if err != nil {
			return errors.Wrap(err, "failed to run lat_syscall on guest")
		}
		guestTime, err := parseSyscallBenchOutput(string(out))
		if err != nil {
			return errors.Wrap(err, "failed to parse lat_syscall output on guest")
		}

		// Output.
		ratio := float64(guestTime) / float64(hostTime)
		s.Logf("syscall %v: host %v, guest %v, guest/host ratio %.2f", args[0], hostTime, guestTime, ratio)

		metricName := func(subName string) string {
			sysCallName := args[0]
			// The name "null" actually runs getpid() underneath.
			if sysCallName == "null" {
				sysCallName = "getpid"
			}
			return fmt.Sprintf("syscall_%s_%s", sysCallName, subName)
		}

		perfValues.Set(
			perf.Metric{
				Name:      "crostini_cpu",
				Variant:   metricName("host"),
				Unit:      "microseconds",
				Direction: perf.SmallerIsBetter,
				Multiple:  false,
			},
			perfutil.ToTimeUnit(time.Microsecond, hostTime)...)
		perfValues.Set(
			perf.Metric{
				Name:      "crostini_cpu",
				Variant:   metricName("guest"),
				Unit:      "microseconds",
				Direction: perf.SmallerIsBetter,
				Multiple:  false,
			},
			perfutil.ToTimeUnit(time.Microsecond, guestTime)...)
		perfValues.Set(
			perf.Metric{
				Name:      "crostini_cpu",
				Variant:   metricName("ratio"),
				Unit:      "percentage",
				Direction: perf.SmallerIsBetter,
				Multiple:  false,
			},
			ratio)
		return nil
	}

	// lat_syscall reads /dev/zero and writes to /dev/null. "null" calls getpid().
	for _, syscall := range []string{"null", "read", "write"} {
		if err := measureSyscallTime(syscall); err != nil {
			s.Errorf("Failed to measure syscall time for command %v: %v", syscall, err)
		}
	}
	// The three commands operate on a file.
	for _, syscall := range []string{"stat", "fstat", "open"} {
		if err := measureSyscallTime(syscall, "/bin/ls"); err != nil {
			s.Errorf("Failed to measure syscall time for command %v: %v", syscall, err)
		}
	}
}

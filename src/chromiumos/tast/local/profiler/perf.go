// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package profiler

import (
	"bufio"
	"context"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"syscall"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
	"chromiumos/tast/fsutil"
	"chromiumos/tast/shutil"
)

// perf represents the perf profiler.
//
// perf supports gathering profiler data using the
// command "perf" with the perfType ("record", "stat record", or "stat") specified.
type perf struct {
	cmd    *testexec.Cmd
	opts   *PerfOpts
	outDir string
}

// perfType represents the type of perf that the users
// want to use.
type perfType int

// Type of perf
const (
	// perfRecord runs "perf record -e cycles -g" on the DUT.
	perfRecord perfType = iota
	// perfStatRecord runs "perf stat record -a" on the DUT.
	perfStatRecord
	// perfStat runs "perf stat -a" on the DUT.
	perfStat
	// perfSched runs "perf sched record" on the DUT
	perfSched

	perfRecordFileName     = "perf_record.data"
	perfStatRecordFileName = "perf_stat_record.data"
	perfStatFileName       = "perf_stat.data"
	perfSchedFileName      = "perf_sched.data"

	// Used in perfStat to get CPU cycle count on all processes.
	PerfAllProcs = 0
)

var (
	noCyclesRegexp = regexp.MustCompile(`(?s)\s+\<not counted\>\s+cycles`)
	cyclesRegexp   = regexp.MustCompile(`(?s)\s+(\d+)\s+cycles`)
	secondsRegexp  = regexp.MustCompile(`(?s)\s+(\d+\.?[\d+]*)\s+seconds time elapsed`)
)

// PerfStatOutput holds output of perf stat.
type PerfStatOutput struct {
	CyclesPerSecond float64
}

// PerfSchedOutput holds output metrics of perf sched
type PerfSchedOutput struct {
	// Maximum latency from wake up to switch
	MaxLatencyMs float64
}

// PerfOpts represents options for running perf.
type PerfOpts struct {
	// t indicates the type of profiler running ("record", "stat record", or "stat").
	t perfType

	// Used in perfStat.
	// Indicate the target process.
	pid int

	// Used in perfStat.
	// A pointer to the output of perfStat.
	perfStatOutput *PerfStatOutput

	// Used in perf sched to get stats of process
	procName string

	// Used in perf sched to provide output
	perfSchedOutput *PerfSchedOutput
}

// PerfStatOpts creates a PerfOpts for running "perf stat -a" on the DUT.
// out is a pointer to PerfStatOutput, which will hold CPU cycle count per second spent
// on pid process after End() is called on RunningProf.
// Set pid to PerfAllProcs to get cycle count for the whole system.
func PerfStatOpts(out *PerfStatOutput, pid int) *PerfOpts {
	return &PerfOpts{t: perfStat, pid: pid, perfStatOutput: out}
}

// PerfRecordOpts creates a PerfOpts for running "perf record -e cycles -g" on the DUT.
func PerfRecordOpts() *PerfOpts {
	return &PerfOpts{t: perfRecord}
}

// PerfStatRecordOpts creates a PerfOpts for running "perf stat record -a" on the DUT.
func PerfStatRecordOpts() *PerfOpts {
	return &PerfOpts{t: perfStatRecord}
}

// PerfSchedOpts creates a PerfOpts for running "perf sched record" on the DUT.
func PerfSchedOpts(out *PerfSchedOutput, procName string) *PerfOpts {
	return &PerfOpts{t: perfSched, procName: procName, perfSchedOutput: out}
}

// Perf creates a Profiler instance that constructs the profiler.
// For opts parameter, nil is treated as the zero value of PerfOpts.
func Perf(opts *PerfOpts) Profiler {
	// Set default options if needed.
	if opts == nil {
		opts = PerfRecordOpts()
	}
	return func(ctx context.Context, outDir string) (instance, error) {
		return newPerf(ctx, outDir, opts)
	}
}

// newPerf creates and runs perf command to start recording perf.data with the options specified.
func newPerf(ctx context.Context, outDir string, opts *PerfOpts) (instance, error) {
	if opts.t == perfStat && opts.pid < 0 {
		return nil, errors.Errorf("invalid pid %d for perfStat", opts.pid)
	}

	cmd, err := getCmd(ctx, outDir, opts)
	if err != nil {
		return nil, err
	}

	if err := cmd.Start(); err != nil {
		cmd.DumpLog(ctx)
		return nil, errors.Wrapf(err, "failed running %s", shutil.EscapeSlice(cmd.Args))
	}

	success := false
	defer func() {
		if !success {
			cmd.Kill()
			cmd.Wait()
		}
	}()

	// KASLR makes looking up the symbols from the binary impossible, save
	// the running symbols from DUT to outDir.
	kallsymsPath := filepath.Join(outDir, "kallsyms")
	if err := fsutil.CopyFile("/proc/kallsyms", kallsymsPath); err != nil {
		return nil, errors.Wrap(err, "failed copying /proc/kallsyms to output directory")
	}

	success = true
	return &perf{
		cmd:    cmd,
		opts:   opts,
		outDir: outDir,
	}, nil
}

func getCmd(ctx context.Context, outDir string, opts *PerfOpts) (*testexec.Cmd, error) {
	switch opts.t {
	case perfRecord:
		outputPath := filepath.Join(outDir, perfRecordFileName)
		return testexec.CommandContext(ctx, "perf", "record", "-e", "cycles", "-g", "--output", outputPath), nil
	case perfStatRecord:
		outputPath := filepath.Join(outDir, perfStatRecordFileName)
		return testexec.CommandContext(ctx, "perf", "stat", "record", "-a", "--output", outputPath), nil
	case perfStat:
		outputPath := filepath.Join(outDir, perfStatFileName)
		if (*opts).pid == PerfAllProcs {
			return testexec.CommandContext(ctx, "perf", "stat", "-a", "-e", "cycles", "--output", outputPath), nil
		}
		return testexec.CommandContext(ctx, "perf", "stat", "-a", "-p", strconv.Itoa(opts.pid), "-e", "cycles", "--output", outputPath), nil
	case perfSched:
		outputPath := filepath.Join(outDir, perfSchedFileName)
		return testexec.CommandContext(ctx, "perf", "sched", "record", "--output", outputPath), nil
	default:
		return nil, errors.Errorf("invalid perf type: %v", opts.t)
	}
}

// getMaxLatencyMs is for perf sched latency and parses Maximum latency from wake up to switch
func getMaxLatencyMs(ctx context.Context, perfSchedFile, procName string) (float64, error) {
	cmd := testexec.CommandContext(ctx, "perf", "sched", "latency", "-i", perfSchedFile)

	output, err := cmd.Output()
	if err != nil {
		return 0, errors.Wrap(err, "failed to get output of perf sched latency")
	}

	PerfSchedLatencyFile := filepath.Join(filepath.Dir(perfSchedFile), "perf_sched_latency.out")
	if err := ioutil.WriteFile(PerfSchedLatencyFile, output, 0644); err != nil {
		return 0, errors.Wrap(err, "failed to write latency file")
	}

	file, err := os.Open(PerfSchedLatencyFile)
	if err != nil {
		return 0, errors.Wrap(err, "failed to open perf sched latency file")
	}
	defer file.Close()

	re, err := regexp.Compile(`max:\s*(.+?)\s*ms`)
	if err != nil {
		return 0, errors.Wrap(err, "failed to compile regex")
	}

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		if strings.Contains(scanner.Text(), procName+":") {
			res := re.FindAllStringSubmatch(scanner.Text(), -1)
			if res == nil {
				return 0, errors.New("failed to parse max latency")
			}
			f, err := strconv.ParseFloat(res[0][1], 64)
			if err != nil {
				return 0, errors.Wrap(err, "failed to parse max latency")
			}
			return f, nil
		}
	}

	return 0, errors.New("failed to read perf sched file")
}

// parseStatFile parses the output file of perf stat command to get CPU cycles per second
// spent in a process. The file should contain cycles and seconds elapsed.
// The return value is a float64 for cycles per second.
func parseStatFile(path string) (float64, error) {
	b, err := ioutil.ReadFile(path)
	if err != nil {
		return 0, errors.Wrapf(err, "failed to read %q", path)
	}

	s := string(b)

	if noCyclesRegexp.FindString(s) != "" {
		return 0, errors.New("got 0 cycle")
	}

	m := cyclesRegexp.FindStringSubmatch(s)
	if m == nil {
		return 0, errors.New("no cycles in perf stat output")
	}
	cycles, err := strconv.ParseInt(m[1], 0, 64)
	if err != nil {
		return 0, errors.Wrap(err, "failed to parse cycles")
	}

	m = secondsRegexp.FindStringSubmatch(s)
	if m == nil {
		return 0, errors.New("no seconds in perf stat output")
	}
	seconds, err := strconv.ParseFloat(m[1], 64)
	if err != nil {
		return 0, errors.Wrap(err, "failed to parse seconds")
	}

	cyclesPerSecond := float64(cycles) / seconds

	return cyclesPerSecond, nil
}

func (p *perf) handleStat() error {
	perfPath := filepath.Join(p.outDir, perfStatFileName)

	cyclesPerSecond, err := parseStatFile(perfPath)
	if err != nil {
		return errors.Wrap(err, "failed to parse stat file")
	}
	p.opts.perfStatOutput.CyclesPerSecond = cyclesPerSecond
	return nil
}

func (p *perf) handleSched(ctx context.Context) error {
	perfPath := filepath.Join(p.outDir, perfSchedFileName)

	maxLatencyMs, err := getMaxLatencyMs(ctx, perfPath, p.opts.procName)
	if err != nil {
		return errors.Wrap(err, "failed to parse sched file")
	}

	p.opts.perfSchedOutput.MaxLatencyMs = maxLatencyMs
	return nil
}

func (p *perf) handleOutput(ctx context.Context) error {
	switch p.opts.t {
	case perfStat:
		if err := p.handleStat(); err != nil {
			return errors.Wrap(err, "failed to handle perf stat result")
		}
	case perfSched:
		if err := p.handleSched(ctx); err != nil {
			return errors.Wrap(err, "failed to handle perf sched result")
		}
	}
	return nil
}

// end interrupts the perf command and ends the recording of perf.data.
func (p *perf) end(ctx context.Context) error {
	// Interrupt the cmd to stop recording perf.
	p.cmd.Signal(syscall.SIGINT)
	err := p.cmd.Wait()
	// The signal is interrupt intentionally, so we check the wait status
	// instead of refusing the error.
	if ws, ok := testexec.GetWaitStatus(err); !ok || !ws.Signaled() || ws.Signal() != syscall.SIGINT {
		return errors.Wrap(err, "failed waiting for the command to exit")
	}
	return p.handleOutput(ctx)
}

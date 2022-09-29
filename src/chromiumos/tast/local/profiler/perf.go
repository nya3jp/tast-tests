// Copyright 2019 The ChromiumOS Authors
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

	"golang.org/x/sys/unix"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
	"chromiumos/tast/fsutil"
	"chromiumos/tast/shutil"
)

// perf represents the perf profiler.
//
// perf supports gathering profiler data using commands:
// "record", "stat record", "stat", or "sched".
type perf struct {
	cmd    *testexec.Cmd
	opts   *PerfOpts
	outDir string
}

const (
	perfRecordFileName     = "perf_record.data"
	perfStatRecordFileName = "perf_stat_record.data"
	perfStatFileName       = "perf_stat.data"
	perfSchedFileName      = "perf_sched.data"

	// PerfAllProcs is used in perf stat to get CPU cycle count on all processes.
	PerfAllProcs = 0
)

// PerfRecordSamplingType optionally adds extra information to samples.
type PerfRecordSamplingType int

// Type of sample.
const (
	PerfRecordDefault PerfRecordSamplingType = iota
	// perf record -g
	PerfRecordCallgraph
	// perf record -b
	PerfRecordBranchStack
)

// PerfRecordSamplingRateType defines the type of sampling rate.
type PerfRecordSamplingRateType int

// Type of sampling rate.
const (
	// perf record -F
	PerfRecordFrequency PerfRecordSamplingRateType = iota
	// perf record -c
	PerfRecordPeriod
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

// PerfSchedOutput holds output metrics of perf sched.
type PerfSchedOutput struct {
	// Maximum latency from wake up to switch
	MaxLatencyMs float64
}

// PerfRecordSamplingRate represents the rate of sampling.
type PerfRecordSamplingRate struct {
	RateType PerfRecordSamplingRateType
	Value    int
}

// PerfOpts represents options for running perf.
type PerfOpts struct {
	// Exactly one of these fields is non-nil.

	// Used to run "perf record -e <even>" on the DUT.
	record *perfRecordOpts
	// Used to run "perf stat record -a" on the DUT.
	statRecord *perfStatRecordOpts
	// Used to run "perf stat -a" on the DUT.
	stat *perfStatOpts
	// Used to run "perf sched record" on the DUT.
	sched *perfSchedOpts
}

type perfRecordOpts struct {
	// Used in perf record to specify event for sampling.
	event string

	// Used in perf record to specify period or frequency of sampling.
	samplingRate *PerfRecordSamplingRate

	// Appends data to samples.
	// Examples can be: branch stack, callgraph, etc.
	samplingType PerfRecordSamplingType
}

type perfStatRecordOpts struct {
}

type perfSchedOpts struct {
	// Used to get stats of process.
	procName string

	// Used to provide output.
	output *PerfSchedOutput
}

type perfStatOpts struct {
	// Indicate the target process.
	pid int

	// A pointer to the output of perf stat.
	output *PerfStatOutput
}

// PerfStatOpts creates a PerfOpts for running "perf stat -a" on the DUT.
// out is a pointer to PerfStatOutput, which will hold CPU cycle count per second spent
// on pid process after End() is called on RunningProf.
// Set pid to PerfAllProcs to get cycle count for the whole system.
func PerfStatOpts(out *PerfStatOutput, pid int) *PerfOpts {
	return &PerfOpts{stat: &perfStatOpts{pid: pid, output: out}}
}

// PerfRecordOpts creates PerfOpts for running "perf record -e <event> [-c <period>|-F <freq>] [-b|-g]" on DUT.
// PerfRecordOpts("", nil, PerfRecordCallgraph) implies default options equivalent to former PerfRecordOpts().
// PerfRecordOpts("instructions", &PerfRecordSamplingRate{RateType: PerfRecordFrequency, Value: 100}, PerfRecordBranchStack)
// is equivalent to "perf record -e instructions -F 100 -b".
func PerfRecordOpts(recordEvent string, samplingRate *PerfRecordSamplingRate, samplingType PerfRecordSamplingType) *PerfOpts {
	return &PerfOpts{record: &perfRecordOpts{event: recordEvent, samplingRate: samplingRate, samplingType: samplingType}}
}

// PerfStatRecordOpts creates a PerfOpts for running "perf stat record -a" on the DUT.
func PerfStatRecordOpts() *PerfOpts {
	return &PerfOpts{statRecord: &perfStatRecordOpts{}}
}

// PerfSchedOpts creates a PerfOpts for running "perf sched record" on the DUT.
func PerfSchedOpts(out *PerfSchedOutput, procName string) *PerfOpts {
	return &PerfOpts{sched: &perfSchedOpts{procName: procName, output: out}}
}

// Perf creates a Profiler instance that constructs the profiler.
// For opts parameter, nil is treated as the zero value of PerfOpts.
func Perf(opts *PerfOpts) Profiler {
	// Set default options if needed.
	if opts == nil {
		opts = PerfRecordOpts("", nil, PerfRecordCallgraph)
	}
	return func(ctx context.Context, outDir string) (instance, error) {
		return newPerf(ctx, outDir, opts)
	}
}

// newPerf creates and runs perf command to start recording perf.data with the options specified.
func newPerf(ctx context.Context, outDir string, opts *PerfOpts) (instance, error) {
	if opts.stat != nil && opts.stat.pid < 0 {
		return nil, errors.Errorf("invalid pid for perf stat: %v", opts.stat.pid)
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
	perfArgs := make([]string, 0)
	if opts.record != nil {
		outputPath := filepath.Join(outDir, perfRecordFileName)
		// "-N" skips writing debug data to ~/.debug on DUT.
		perfArgs = append(perfArgs, "record", "-N", "--output", outputPath)
		var event string
		if opts.record.event != "" {
			event = opts.record.event
		} else {
			event = "cycles"
		}
		perfArgs = append(perfArgs, "-e", event)
		if opts.record.samplingRate != nil {
			value := strconv.Itoa(opts.record.samplingRate.Value)
			switch opts.record.samplingRate.RateType {
			case PerfRecordPeriod:
				perfArgs = append(perfArgs, "-c", value)
			case PerfRecordFrequency:
				perfArgs = append(perfArgs, "-F", value)
			}
		}
		switch opts.record.samplingType {
		case PerfRecordBranchStack:
			perfArgs = append(perfArgs, "-b")
		case PerfRecordCallgraph:
			perfArgs = append(perfArgs, "-g")
		case PerfRecordDefault:
			// Default: no extra arguments in recording.
		}
	}
	if opts.stat != nil {
		if len(perfArgs) != 0 {
			return nil, errors.Errorf("more than one command option was initialized: perf %v and stat", perfArgs[0])
		}
		outputPath := filepath.Join(outDir, perfStatFileName)
		perfArgs = append(perfArgs, "stat", "-a", "-e", "cycles", "--output", outputPath)
		if opts.stat.pid != PerfAllProcs {
			perfArgs = append(perfArgs, "-p", strconv.Itoa(opts.stat.pid))
		}
	}
	if opts.statRecord != nil {
		if len(perfArgs) != 0 {
			return nil, errors.Errorf("more than one command option was initialized: perf %v and stat record", perfArgs[0])
		}
		outputPath := filepath.Join(outDir, perfStatRecordFileName)
		perfArgs = append(perfArgs, "stat", "record", "-a", "--output", outputPath)
	}
	if opts.sched != nil {
		if len(perfArgs) != 0 {
			return nil, errors.Errorf("more than one command option was initialized: perf %v and sched", perfArgs[0])
		}
		outputPath := filepath.Join(outDir, perfSchedFileName)
		perfArgs = append(perfArgs, "sched", "record", "--output", outputPath)
	}
	if len(perfArgs) == 0 {
		return nil, errors.New("none of the known perf options was initialized")
	}
	return testexec.CommandContext(ctx, "perf", perfArgs...), nil
}

// getMaxLatencyMs is for perf sched latency and parses Maximum latency from wake up to switch.
func getMaxLatencyMs(ctx context.Context, perfSchedFile, procName string) (float64, error) {
	cmd := testexec.CommandContext(ctx, "perf", "sched", "latency", "-i", perfSchedFile)

	output, err := cmd.Output()
	if err != nil {
		return 0, errors.Wrap(err, "failed to get output of perf sched latency")
	}

	perfSchedLatencyFile := filepath.Join(filepath.Dir(perfSchedFile), "perf_sched_latency.out")
	if err := ioutil.WriteFile(perfSchedLatencyFile, output, 0644); err != nil {
		return 0, errors.Wrap(err, "failed to write latency file")
	}

	file, err := os.Open(perfSchedLatencyFile)
	if err != nil {
		return 0, errors.Wrap(err, "failed to open perf sched latency file")
	}
	defer file.Close()

	re := regexp.MustCompile(`max:\s*(.+?)\s*ms`)
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
	p.opts.stat.output.CyclesPerSecond = cyclesPerSecond
	return nil
}

func (p *perf) handleSched(ctx context.Context) error {
	perfPath := filepath.Join(p.outDir, perfSchedFileName)

	maxLatencyMs, err := getMaxLatencyMs(ctx, perfPath, p.opts.sched.procName)
	if err != nil {
		return errors.Wrap(err, "failed to parse sched file")
	}

	p.opts.sched.output.MaxLatencyMs = maxLatencyMs
	return nil
}

func (p *perf) handleOutput(ctx context.Context) error {
	if p.opts.stat != nil {
		if err := p.handleStat(); err != nil {
			return errors.Wrap(err, "failed to handle perf stat result")
		}
	} else if p.opts.sched != nil {
		if err := p.handleSched(ctx); err != nil {
			return errors.Wrap(err, "failed to handle perf sched result")
		}
	}
	return nil
}

// end interrupts the perf command and ends the recording of perf.data.
func (p *perf) end(ctx context.Context) error {
	// Interrupt the cmd to stop recording perf.
	p.cmd.Signal(unix.SIGINT)
	err := p.cmd.Wait()
	// The signal is interrupt intentionally, so we check the wait status
	// instead of refusing the error.
	if ws, ok := testexec.GetWaitStatus(err); !ok || !ws.Signaled() || ws.Signal() != unix.SIGINT {
		return errors.Wrap(err, "failed waiting for the command to exit")
	}
	return p.handleOutput(ctx)
}

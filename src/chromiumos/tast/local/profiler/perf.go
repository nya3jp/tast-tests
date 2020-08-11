// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package profiler

import (
	"context"
	"io/ioutil"
	"path/filepath"
	"regexp"
	"strconv"
	"syscall"

	"chromiumos/tast/errors"
	"chromiumos/tast/fsutil"
	"chromiumos/tast/local/sysutil"
	"chromiumos/tast/local/testexec"
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

// PerfType represents the type of perf that the users
// want to use.
type PerfType int

// Type of perf
const (
	// PerfRecord runs "perf record -e cycles -g" on the DUT.
	PerfRecord PerfType = iota
	// PerfStatRecord runs "perf stat record -a" on the DUT.
	PerfStatRecord
	// PerfStat runs "perf stat -a" on the DUT.
	PerfStat

	perfRecordFileName     = "perf_record.data"
	perfStatRecordFileName = "perf_stat_record.data"
	perfStatFileName       = "perf_stat.data"
)

var (
	noCyclesRegexp = regexp.MustCompile(`(?s)\s+\<not counted\>\s+cycles`)
	cyclesRegexp   = regexp.MustCompile(`(?s)\s+(\d+)\s+cycles`)
	secondsRegexp  = regexp.MustCompile(`(?s)\s+(\d+\.?[\d+]*)\s+seconds time elapsed`)
)

// PerfOpts represents options for running perf.
type PerfOpts struct {
	// Type indicates the type of profiler running ("record", "stat record", or "stat").
	// The default is PerfRecord.
	Type PerfType

	// Used in PerfStat.
	// Indicate the target process.
	Pid int
}

// PerfStatOpts creates a PerfOpts for PerfStat.
// Set pid to 0 to get cycle count for the whole system.
func PerfStatOpts(pid int) (*PerfOpts, error) {
	if pid < 0 {
		return nil, errors.Errorf("invalid pid %d for PerfStat", pid)
	}
	return &PerfOpts{Type: PerfStat, Pid: pid}, nil
}

// PerfRecordOpts creates a PerfOpts for PerfRecord.
func PerfRecordOpts() *PerfOpts {
	return &PerfOpts{Type: PerfRecord}
}

// PerfStatRecordOpts creates a PerfOpts for PerfStatRecord.
func PerfStatRecordOpts() *PerfOpts {
	return &PerfOpts{Type: PerfStatRecord}
}

// Perf creates a Profiler instance that constructs the profiler.
// For opts parameter, nil is treated as the zero value of PerfOpts.
func Perf(opts *PerfOpts) Profiler {
	// Set default options if needed.
	if opts == nil {
		opts = &PerfOpts{}
	}
	return func(ctx context.Context, outDir string) (instance, error) {
		return newPerf(ctx, outDir, opts)
	}
}

// newPerf creates and runs perf command to start recording perf.data with the options specified.
func newPerf(ctx context.Context, outDir string, opts *PerfOpts) (instance, error) {
	// TODO(crbug.com/996728): aarch64 is disabled before the kernel crash is fixed.
	u, err := sysutil.Uname()
	if err != nil {
		return nil, errors.Wrap(err, "failed getting system architecture")
	}
	if u.Machine == "aarch64" {
		return nil, errors.Errorf("running perf on %s is disabled (crbug.com/996728)", u.Machine)
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
	switch (*opts).Type {
	case PerfRecord:
		outputPath := filepath.Join(outDir, perfRecordFileName)
		return testexec.CommandContext(ctx, "perf", "record", "-e", "cycles", "-g", "--output", outputPath), nil
	case PerfStatRecord:
		outputPath := filepath.Join(outDir, perfStatRecordFileName)
		return testexec.CommandContext(ctx, "perf", "stat", "record", "-a", "--output", outputPath), nil
	case PerfStat:
		outputPath := filepath.Join(outDir, perfStatFileName)
		if (*opts).Pid == 0 {
			return testexec.CommandContext(ctx, "perf", "stat", "-a", "-e", "cycles", "--output", outputPath), nil
		}
		return testexec.CommandContext(ctx, "perf", "stat", "-a", "-p", strconv.Itoa((*opts).Pid), "-e", "cycles", "--output", outputPath), nil
	default:
		return nil, errors.New("invalid perf type")
	}
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
		return 0, errors.New("error got 0 cycle")
	}

	m := cyclesRegexp.FindStringSubmatch(s)
	if m == nil {
		return 0, errors.New("error finding cycles")
	}
	cycles, cyclesErr := strconv.ParseInt(m[1], 0, 64)
	if cyclesErr != nil {
		return 0, errors.Wrap(cyclesErr, "error parsing cycles")
	}

	m = secondsRegexp.FindStringSubmatch(s)
	if m == nil {
		return 0, errors.New("error finding seconds")
	}
	seconds, secondsErr := strconv.ParseFloat(m[1], 64)
	if secondsErr != nil {
		return 0, errors.Wrap(secondsErr, "error parsing seconds")
	}

	cyclesPerSecond := float64(cycles) / seconds

	return cyclesPerSecond, nil
}

func (p *perf) handleStat() (Output, error) {
	perfPath := filepath.Join(p.outDir, perfStatFileName)

	cyclesPerSecond, err := parseStatFile(perfPath)
	if err != nil {
		return OutputNull(), errors.Wrap(err, "error parsing stat file")
	}

	return Output{Props: map[string]interface{}{"cyclesPerSecond": cyclesPerSecond}}, nil
}

// end interrupts the perf command and ends the recording of perf.data.
func (p *perf) end() (Output, error) {
	// Interrupt the cmd to stop recording perf.
	p.cmd.Signal(syscall.SIGINT)
	err := p.cmd.Wait()
	// The signal is interrupt intentionally, so we check the wait status
	// instead of refusing the error.
	if ws, ok := testexec.GetWaitStatus(err); !ok || !ws.Signaled() || ws.Signal() != syscall.SIGINT {
		return OutputNull(), errors.Wrap(err, "failed waiting for the command to exit")
	}
	var res Output
	if p.opts.Type == PerfStat {
		if res, err = p.handleStat(); err != nil {
			return OutputNull(), errors.Wrap(err, "failed to handle perf stat result")
		}
		return res, nil
	}
	return OutputNull(), nil
}

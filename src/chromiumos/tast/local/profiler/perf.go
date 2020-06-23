// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package profiler

import (
	"bufio"
	"context"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"syscall"

	perfpkg "chromiumos/tast/common/perf"
	"chromiumos/tast/errors"
	"chromiumos/tast/fsutil"
	"chromiumos/tast/local/sysutil"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/shutil"
)

// perf represents the perf profiler.
//
// perf supports gathering profiler data using the
// command "perf" with the perfType ("stat" or "record") specified.
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
	// PerfStat runs "perf stat record -a" on the DUT.
	PerfStat
	// PerfStat only runs "perf stat -a" on the DUT.
	PerfStatOnly
)

const perfRecordFileName = "perf_record.data"
const perfStatFileName = "perf_stat.data"
const perfStatOnlyFileName = "perf_stat_only.data"

// PerfOpts represents options for running perf.
type PerfOpts struct {
	// Type indicates the type of profiler running ("record" or "stat").
	// The default is PerfRecord.
	Type PerfType

	// Indicate the target process.
	Pid int

	// Indicate the name tag for perf result.
	tag string
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

	cmd, err := getCmd(ctx, outDir, opts.Type, opts.Pid)
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

func getCmd(ctx context.Context, outDir string, perfType PerfType, pid int) (*testexec.Cmd, error) {
	switch perfType {
	case PerfRecord:
		outputPath := filepath.Join(outDir, perfRecordFileName)
		return testexec.CommandContext(ctx, "perf", "record", "-a", "-g", "-F", "5000", "--output", outputPath), nil
	case PerfStat:
		outputPath := filepath.Join(outDir, perfStatFileName)
		if pid == -1 {
			return testexec.CommandContext(ctx, "perf", "stat", "record", "-a", "--output", outputPath), nil
		}
		return testexec.CommandContext(ctx, "perf", "stat", "record", "-a", "-p", strconv.Itoa(pid), "--output", outputPath), nil
	case PerfStatOnly:
		outputPath := filepath.Join(outDir, perfStatOnlyFileName)
		if pid == -1 {
			return testexec.CommandContext(ctx, "perf", "stat", "-a", "-e", "cycles", "--output", outputPath), nil
		}
		return testexec.CommandContext(ctx, "perf", "stat", "-a", "-p", strconv.Itoa(pid), "-e", "cycles", "--output", outputPath), nil
	default:
		return nil, errors.New("invalid perf type")
	}
}

// TODO: handle output file.
func handleStatOnly(p *perf) error {
	perfPath := filepath.Join(p.outDir, perfStatOnlyFileName)

	// cycleRegexp extracts cycle number from perf stat output.
	cyclesRegexp := regexp.MustCompile(`^\s+(\d+)\s+cycles`)
	secondsRegexp := regexp.MustCompile(`^\s+(\d+\.?[\d+]*)\s+seconds time elapsed`)

	file, err := os.Open(perfPath)
	defer file.Close()

	if err != nil {
		return err
	}

	var cycles int64 = -1
	var seconds float64 = -1.0

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		l := scanner.Text()

		m := cyclesRegexp.FindStringSubmatch(l)
		if m != nil {
			cycles, err = strconv.ParseInt(m[1], 0, 64)
			if err != nil {
				return errors.Wrap(err, "error parsing cycles")
			}
			continue
		}

		m = secondsRegexp.FindStringSubmatch(l)
		if m != nil {
			seconds, err = strconv.ParseFloat(m[1], 64)
			if err != nil {
				return errors.Wrap(err, "error parsing seconds")
			}
			continue
		}
	}
	if err := scanner.Err(); err != nil {
		return errors.Wrap(err, "error while scanning perf stat output.")
	}

	if cycles == -1 || seconds == -1 {
		return errors.Wrap(err, "Failed to parse perf stat output.")
	}

	cyclePerSecond := float64(cycles) / seconds
	pv := perfpkg.NewValues()

	pv.Set(perfpkg.Metric{
		Name:      "cras_cycles_per_second",
		Unit:      "cycles",
		Direction: perfpkg.SmallerIsBetter,
	}, cyclePerSecond)

	if err := pv.Save(p.outDir); err != nil {
		return errors.Wrap(err, "Cannot save perf data: ")
	}

	return nil
}

// end interrupts the perf command and ends the recording of perf.data.
func (p *perf) end() error {
	// Interrupt the cmd to stop recording perf.
	p.cmd.Signal(syscall.SIGINT)
	err := p.cmd.Wait()
	// The signal is interrupt intentionally, so we check the wait status
	// instead of refusing the error.
	if ws, ok := testexec.GetWaitStatus(err); !ok || !ws.Signaled() || ws.Signal() != syscall.SIGINT {
		return errors.Wrap(err, "failed waiting for the command to exit")
	}
	if p.opts.Type == PerfStatOnly {
		if err := handleStatOnly(p); err != nil {
			return errors.Wrap(err, "failed to handle perf stat only result")
		}
	}
	return nil
}

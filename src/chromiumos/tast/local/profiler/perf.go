// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package profiler

import (
	"context"
	"path/filepath"
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
// command "perf" with the perfType ("stat" or "record") specified.
type perf struct {
	cmd *testexec.Cmd
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
)

// PerfOpts represents options for running perf.
type PerfOpts struct {
	// Type indicates the type of profiler running ("record" or "stat").
	// The default is PerfRecord.
	Type PerfType
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
	// Run perf only on x86_64 devices.
	u, err := sysutil.Uname()
	if err != nil {
		return nil, errors.Wrap(err, "failed getting system architecture")
	}
	if u.Machine == "aarch64" {
		return nil, errors.Wrapf(err, "unsupported architecture for running perf: %s", u.Machine)
	}

	cmd, err := getCmd(ctx, outDir, opts.Type)
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
		cmd: cmd,
	}, nil
}

func getCmd(ctx context.Context, outDir string, perfType PerfType) (*testexec.Cmd, error) {
	outputPath := filepath.Join(outDir, "perf.data")
	switch perfType {
	case PerfRecord:
		return testexec.CommandContext(ctx, "perf", "record", "-e", "cycles", "-g", "--output", outputPath), nil
	case PerfStat:
		return testexec.CommandContext(ctx, "perf", "stat", "record", "-a", "--output", outputPath), nil
	default:
		return nil, errors.New("invalid perf type")
	}
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
	return nil
}

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
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/shutil"
)

// perf represents the perf profiler.
//
// perf supports gathering profiler data using the
// command "perf" with the perfType ("stat" or "record") specified.
type perf struct {
	cmd      *testexec.Cmd
	perfType PerfType
}

// PerfType represents the type of perf that the users
// want to use.
type PerfType int

// Type of perf
const (
	PerfRecord = 0 + iota
	PerfStat
)

// PerfOpts represents options for running perf.
type PerfOpts struct {
	Type PerfType
}

// Perf creates a perf instance that manages running the profiler.
// For default options (Type = PerfRecord), input for opts can be nil or empty PerfOpts struct.
func Perf(opts *PerfOpts) *perf {
	var perfType PerfType
	if opts == nil {
		perfType = PerfRecord
	} else {
		perfType = opts.Type
	}
	return &perf{
		perfType: perfType,
	}
}

// new creates and runs perf command to start recording perf.data with the options specified.
func (p *perf) new(ctx context.Context, outDir string) error {
	var err error
	p.cmd, err = getCmd(ctx, outDir, p.perfType)
	if err != nil {
		return err
	}

	if err := p.cmd.Start(); err != nil {
		p.cmd.DumpLog(ctx)
		return errors.Wrapf(err, "failed running %s", shutil.EscapeSlice(p.cmd.Args))
	}

	success := false
	defer func() {
		if !success {
			p.cmd.Kill()
			p.cmd.Wait()
		}
	}()

	// KASLR makes looking up the symbols from the binary impossible, save
	// the running symbols from DUT to outDir.
	kallsymsPath := filepath.Join(outDir, "kallsyms")
	if err := fsutil.CopyFile("/proc/kallsyms", kallsymsPath); err != nil {
		return errors.Wrap(err, "failed copying /proc/kallsyms to output directory")
	}

	success = true
	return nil
}

func getCmd(ctx context.Context, outDir string, perfType PerfType) (*testexec.Cmd, error) {
	outputPath := filepath.Join(outDir, "perf.data")
	if perfType == PerfRecord {
		return testexec.CommandContext(ctx, "perf", "record", "-e", "cycles", "-g", "--output", outputPath), nil
	} else if perfType == PerfStat {
		return testexec.CommandContext(ctx, "perf", "stat", "record", "-a", "--output", outputPath), nil
	} else {
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

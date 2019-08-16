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
// command "perf record".
type perf struct {
	cmd *testexec.Cmd
}

// PerfType represents the type of perf that the users
// want to use.
type PerfType int

// Type of perf
const (
	PerfRecord = 1 + iota
	PerfStat
)

// PerfOpts represents options for perf
type PerfOpts struct {
	Type PerfType
}

// newPerfOpts runs perf command to start recording perf.data with the options specified.
// options given must be type PerfOpts.
func newPerfOpts(ctx context.Context, outDir string, opts interface{}) (instance, error) {
	perfOpts, ok := opts.(PerfOpts)
	if !ok {
		return nil, errors.New("options for perf profiler must be type PerfOpts")
	}
	cmd, err := getCmd(ctx, outDir, perfOpts.Type)
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
	if perfType == PerfRecord {
		return testexec.CommandContext(ctx, "perf", "record", "-e", "cycles", "-g", "--output", outputPath), nil
	} else if perfType == PerfStat {
		return testexec.CommandContext(ctx, "perf", "stat", "record", "-a", "--output", outputPath), nil
	} else {
		return nil, errors.New("invalid perf type")
	}
}

// newPerf runs perf command to start recording perf.data with
// default options: Type = PerfRecord.
func newPerf(ctx context.Context, outDir string) (instance, error) {
	opts := PerfOpts{
		Type: PerfRecord,
	}
	return newPerfOpts(ctx, outDir, opts)
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

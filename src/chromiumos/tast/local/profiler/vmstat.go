// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package profiler

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"syscall"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/shutil"
)

// vmstat represents the vmstat profiler.
//
// vmstat supports running 'vmstat' command in the DUT during testing.
// vmstat will run every X seconds in interval (default is 1 seconds) and
// stored the result in vmstat.data in the specified output directory.
type vmstat struct {
	cmd *testexec.Cmd
	out *os.File
}

// VMStatOpts represents options for running vmstat.
type VMStatOpts struct {
	// Interval indicates the duration between each vmstat run.
	// Interval must be able to convert to a non-decimal in seconds.
	// For example, vmstat can run 2 seconds but not 2.3 seconds.
	// 2.3 will be rounded to 2 for vmstat interval.
	Interval time.Duration
}

// VMStat creates a Profiler instance that constructs and runs the profiler.
// For default options (Interval = 1 seconds), input for opts can be nil
// or empty VMStatOpts struct.
func VMStat(opts *VMStatOpts) Profiler {
	// Set default options if needed.
	opts = getVMStatOptsDefault(opts)
	return func(ctx context.Context, outDir string) (instance, error) {
		return newVMStat(ctx, outDir, opts)
	}
}

func getVMStatOptsDefault(opts *VMStatOpts) *VMStatOpts {
	if opts == nil {
		opts = &VMStatOpts{}
	}
	// Default option for Interval is 1 seconds.
	if opts.Interval == 0 {
		opts.Interval = 1 * time.Second
	}
	return opts
}

// newVMStat runs vmstat command to start recording vmstat.data with the options specified.
func newVMStat(ctx context.Context, outDir string, opts *VMStatOpts) (instance, error) {
	outputPath := filepath.Join(outDir, "vmstat.data")
	out, err := os.Create(outputPath)
	if err != nil {
		return nil, errors.Wrap(err, "failed creating output file")
	}

	success := false
	defer func() {
		if !success {
			out.Close()
		}
	}()

	// Get the int value of Interval in seconds.
	interval := int(opts.Interval.Seconds())
	cmd := testexec.CommandContext(ctx, "vmstat", strconv.Itoa(interval))
	cmd.Stdout = out
	if err := cmd.Start(); err != nil {
		cmd.DumpLog(ctx)
		return nil, errors.Wrapf(err, "failed running %s", shutil.EscapeSlice(cmd.Args))
	}

	success = true
	return &vmstat{
		cmd: cmd,
		out: out,
	}, nil
}

// end interrupts the vmstat command and ends the recording of vmstat.data.
func (v *vmstat) end() error {
	// Interrupt the cmd to stop recording.
	v.cmd.Signal(syscall.SIGINT)
	err := v.cmd.Wait()
	if errClose := v.out.Close(); errClose != nil {
		return errors.Wrap(errClose, "failed closing output file")
	}
	// The signal is interrupt intentionally, so we check the wait status
	// instead of refusing the error.
	if ws, ok := testexec.GetWaitStatus(err); !ok || !ws.Signaled() || ws.Signal() != syscall.SIGINT {
		return errors.Wrap(err, "failed waiting for the command to exit")
	}
	return nil
}

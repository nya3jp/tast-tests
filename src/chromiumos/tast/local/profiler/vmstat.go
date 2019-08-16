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
// vmstat will run every X seconds (default is 1 seconds) and stored the
// result in vmstat.data in the specified output directory.
type vmstat struct {
	cmd      *testexec.Cmd
	out      *os.File
	interval time.Duration
}

// VMStatOpts represents options for running vmstat.
type VMStatOpts struct {
	// Interval must be able to convert to a non-decimal in seconds.
	// For example, vmstat can run 2 seconds but not 2.3 seconds.
	// 2.3 will be rounded to 2 for vmstat interval.
	Interval time.Duration
}

// VMStat creates a vmstat instance that manages running the profiler.
// For default options (Interval = 1 seconds), input for opts can be nil.
func VMStat(opts *VMStatOpts) *vmstat {
	interval := 1 * time.Second
	if opts != nil {
		interval = opts.Interval
	}
	return &vmstat{
		interval: interval,
	}
}

// new creates and runs vmstat command to start recording vmstat.data with the options specified.
func (v *vmstat) new(ctx context.Context, outDir string) error {
	outputPath := filepath.Join(outDir, "vmstat.data")
	var err error
	v.out, err = os.Create(outputPath)
	if err != nil {
		return errors.Wrap(err, "failed creating output file")
	}

	success := false
	defer func() {
		if !success {
			v.out.Close()
		}
	}()

	interval := int(v.interval.Seconds())
	v.cmd = testexec.CommandContext(ctx, "vmstat", strconv.Itoa(interval))
	v.cmd.Stdout = v.out
	if err := v.cmd.Start(); err != nil {
		v.cmd.DumpLog(ctx)
		return errors.Wrapf(err, "failed running %s", shutil.EscapeSlice(v.cmd.Args))
	}

	success = true
	return nil
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

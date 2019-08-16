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

// top represents the top profiler.
//
// top supports running 'top' command in the DUT during testing.
// top will run and poll every X seconds in interval (default is 5 seconds).
// Any processes with a %cpu of zero will be stripped from the output.
// The output of top will be stored in top.data in the specified
// output directory.
type top struct {
	cmdTop *testexec.Cmd
	cmdAwk *testexec.Cmd
	out    *os.File
}

// TopOpts represents options for running top.
type TopOpts struct {
	// Interval indicates the duration between each poll.
	// Interval must be able to convert to a non-decimal in seconds.
	// For example, vmstat can run 2 seconds but not 2.3 seconds.
	// 2.3 will be rounded to 2 for vmstat interval.
	Interval time.Duration
}

// Top creates a Profiler instance that constructs and runs the profiler.
// For default options (Interval = 5 seconds), input for opts can be nil
// or empty VMStatOpts struct.
func Top(opts *TopOpts) Profiler {
	// Set default options if needed.
	opts = getTopOptsDefault(opts)
	return func(ctx context.Context, outDir string) (instance, error) {
		return newTop(ctx, outDir, opts)
	}
}

func getTopOptsDefault(opts *TopOpts) *TopOpts {
	if opts == nil {
		opts = &TopOpts{}
	}
	// Default option for Interval is 5 seconds.
	if opts.Interval == 0 {
		opts.Interval = 5 * time.Second
	}
	return opts
}

// newTop runs top command to start recording top.data.
func newTop(ctx context.Context, outDir string, opts *TopOpts) (instance, error) {
	outputPath := filepath.Join(outDir, "top.data")
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

	cmdTop := testexec.CommandContext(ctx, "top", "-b", "-c", "-w", "200", "-d", strconv.Itoa(interval), "-H")
	cmdAwk := testexec.CommandContext(ctx, "awk", "$1 ~ /[0-9]+/ && $9 == \"0.0\" {next} {print}")
	cmdAwk.Stdin, err = cmdTop.StdoutPipe()
	if err != nil {
		return nil, errors.Wrap(err, "failed piping command 'top' with 'awk'")
	}
	cmdAwk.Stdout = out

	if err := cmdAwk.Start(); err != nil {
		cmdAwk.DumpLog(ctx)
		return nil, errors.Wrapf(err, "failed running %s", shutil.EscapeSlice(cmdAwk.Args))
	}
	defer func() {
		if !success {
			cmdAwk.Kill()
			cmdAwk.Wait()
		}
	}()

	if err := cmdTop.Start(); err != nil {
		cmdTop.DumpLog(ctx)
		return nil, errors.Wrapf(err, "failed running %s", shutil.EscapeSlice(cmdTop.Args))
	}

	success = true
	return &top{
		cmdTop: cmdTop,
		cmdAwk: cmdAwk,
		out:    out,
	}, nil
}

// end interrupts the top command and ends the recording of top.data.
func (t *top) end() error {
	// Interrupt the cmd to stop recording.
	t.cmdTop.Signal(syscall.SIGINT)
	errTop := t.cmdTop.Wait()
	errAwk := t.cmdAwk.Wait()
	if errClose := t.out.Close(); errClose != nil {
		return errors.Wrap(errClose, "failed closing output file")
	}
	if errTop != nil {
		return errors.Wrap(errTop, "failed waiting for the command 'top' to exit")
	}
	if errAwk != nil {
		return errors.Wrap(errAwk, "failed waiting for the command 'awk' to exit")
	}
	return nil
}

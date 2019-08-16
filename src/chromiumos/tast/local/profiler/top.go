// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package profiler

import (
	"context"
	"os"
	"path/filepath"
	"syscall"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/shutil"
)

// top represents the top profiler.
//
// top supports running 'top' command in the DUT during testing.
// The output of top will be stored in top.data in the specified
// output directory.
type top struct {
	cmdTop *testexec.Cmd
	cmdAwk *testexec.Cmd
	out    *os.File
}

// TopOpts represents options for running top.
type TopOpts struct {
}

// Top creates a top instance that manages running the profiler.
// Input for opts can be nil or empty PerfOpts struct.
func Top(opts *TopOpts) *top {
	return &top{}
}

// new creates and runs top command to start recording top.data with the options specified.
func (t *top) new(ctx context.Context, outDir string) error {
	outputPath := filepath.Join(outDir, "top.data")
	var err error
	t.out, err = os.Create(outputPath)
	if err != nil {
		return errors.Wrap(err, "failed creating output file")
	}

	success := false
	defer func() {
		if !success {
			t.out.Close()
		}
	}()

	// Starts top on the DUT and polls every 5 seconds. Any processes
	// with a %cpu of zero will be stripped from the output.
	t.cmdTop = testexec.CommandContext(ctx, "top", "-b", "-c", "-w", "200", "-d", "5", "-H")
	t.cmdAwk = testexec.CommandContext(ctx, "awk", "$1 ~ /[0-9]+/ && $9 == \"0.0\" {next} {print}")
	t.cmdAwk.Stdin, err = t.cmdTop.StdoutPipe()
	if err != nil {
		return errors.Wrap(err, "failed piping command 'top' with 'awk'")
	}
	t.cmdAwk.Stdout = t.out

	if err := t.cmdAwk.Start(); err != nil {
		t.cmdAwk.DumpLog(ctx)
		return errors.Wrapf(err, "failed running %s", shutil.EscapeSlice(t.cmdAwk.Args))
	}
	defer func() {
		if !success {
			t.cmdAwk.Kill()
			t.cmdAwk.Wait()
		}
	}()

	if err := t.cmdTop.Start(); err != nil {
		t.cmdTop.DumpLog(ctx)
		return errors.Wrapf(err, "failed running %s", shutil.EscapeSlice(t.cmdTop.Args))
	}

	success = true
	return nil
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

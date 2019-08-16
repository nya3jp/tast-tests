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
	cmd *testexec.Cmd
	out *os.File
}

// VMStatOpts represents options for vmstat
type VMStatOpts struct {
	Interval int
}

// newVMStatOpts runs vmstat command to start recording vmstat.data with the options specified.
// options given must be type VMStatOpts.
func newVMStatOpts(ctx context.Context, outDir string, opts interface{}) (instance, error) {
	vmstatOpts, ok := opts.(VMStatOpts)
	if !ok {
		return nil, errors.New("options for vmstat profiler must be type VMStatOpts")
	}

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

	cmd := testexec.CommandContext(ctx, "vmstat", strconv.Itoa(vmstatOpts.Interval))
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

// newVMStat runs vmstat command to start recording vmstat.data.
// with default options: Interval = 1
func newVMStat(ctx context.Context, outDir string) (instance, error) {
	opts := VMStatOpts{
		Interval: 1,
	}
	return newVMStatOpts(ctx, outDir, opts)
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

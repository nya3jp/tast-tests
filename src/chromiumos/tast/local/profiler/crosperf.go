// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package profiler

import (
	"context"
	"path/filepath"
	"syscall"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/shutil"
)

// crosPerf represents the crosperf profiler.
//
// crosperf supports capture system profiler data while running test
// in ChromeOS. It offers the support of gathering profiler data using the
// command "perf record".
type crosPerf struct {
	cmd *testexec.Cmd
}

// newCrosPerf runs perf command to start recording perf.data.
func newCrosPerf(ctx context.Context, outDir string) (instance, error) {
	outputPath := filepath.Join(outDir, "perf.data")
	cmd := testexec.CommandContext(ctx, "perf", "record", "-e", "cycles", "-g", "--output", outputPath)
	if err := cmd.Start(); err != nil {
		cmd.DumpLog(ctx)
		return nil, errors.Wrapf(err, "failed running %s", shutil.EscapeSlice(cmd.Args))
	}
	return &crosPerf{
		cmd: cmd,
	}, nil
}

// end interrupts the perf command and ends the recording of perf.data.
func (p *crosPerf) end() error {
	if p.cmd == nil {
		return errors.New("profiler need to be started to end")
	}
	// Interrupt the cmd to stop recording perf.
	p.cmd.Signal(syscall.SIGINT)
	err := p.cmd.Wait()
	// The signal is interrupt intentionally, so we check the wait status
	// instead of refusing the error.
	ws, ok := testexec.GetWaitStatus(err)
	if !ok || !ws.Signaled() || ws.Signal() != syscall.SIGINT {
		errors.Wrap(err, "failed waiting for the command to exit")
	}
	return nil
}

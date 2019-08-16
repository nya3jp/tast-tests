// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package profiler

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"syscall"

	"chromiumos/tast/errors"
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

// newPerf runs perf command to start recording perf.data.
func newPerf(ctx context.Context, outDir string) (instance, error) {
	outputPath := filepath.Join(outDir, "perf.data")
	cmd := testexec.CommandContext(ctx, "perf", "record", "-e", "cycles", "-g", "--output", outputPath)
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
	if err := getKallsyms(outDir); err != nil {
		return nil, errors.Wrap(err, "failed copying /proc/kallsyms to output directory")
	}

	success = true
	return &perf{
		cmd: cmd,
	}, nil
}

func getKallsyms(outDir string) error {
	kallsyms, err := os.Open("/proc/kallsyms")
	if err != nil {
		return errors.Wrap(err, "failed opening /proc/kallsyms")
	}

	success := false
	defer func() {
		if !success {
			kallsyms.Close()
		}
	}()

	kallsymsPath := filepath.Join(outDir, "kallsyms")
	out, err := os.Create(kallsymsPath)
	if err != nil {
		return errors.Wrap(err, "failed creating kallsyms file")
	}
	defer func() {
		if !success {
			out.Close()
		}
	}()

	if _, err = io.Copy(out, kallsyms); err != nil {
		return errors.Wrap(err, "failed copying")
	}
	success = true
	ksErr := kallsyms.Close()
	outErr := out.Close()
	if ksErr != nil {
		return errors.Wrap(err, "failed closing /proc/kallsyms")
	}
	if outErr != nil {
		return errors.Wrap(err, "failed closing output kallsyms file")
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
	return nil
}

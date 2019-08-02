// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package crosperf supports capture system profiler data while running test
// in ChromeOS. It offers the support of gathering profiler data using the
// command "perf record".
package crosperf

import (
	"context"
	"path/filepath"
	"strings"
	"syscall"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/testexec"
)

// CrosPerf represents the profiler in ChromeOS.
type CrosPerf struct {
	cmd *testexec.Cmd
}

// Start runs perf command to start recording perf.data.
func (p *CrosPerf) Start(ctx context.Context, outDir string) error {
	outputPath := filepath.Join(outDir, "perf.data")
	p.cmd = testexec.CommandContext(ctx, "perf", "record", "-e", "cycles", "-g", "--output", outputPath)
	if err := p.cmd.Start(); err != nil {
		p.cmd.DumpLog(ctx)
		return errors.Wrapf(err, "failed running %q", strings.Join(p.cmd.Args, " "))
	}
	return nil
}

// End interrupts the perf command and ends the recording of perf.data.
func (p *CrosPerf) End() error {
	if p.cmd == nil {
		return errors.New("profiler need to be started to end")
	}
	// Interrupt the cmd to stop recording perf.
	p.cmd.Signal(syscall.SIGINT)
	err := p.cmd.Wait()
	// The signal is interrupt intentionally, so we pass this error.
	if err.Error() == "signal: interrupt" {
		return nil
	}
	return err
}

// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package power

import (
	"context"
	"os"
	"path/filepath"
	"syscall"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/testexec"
)

type perfTrace struct {
	ctx       context.Context
	dir       string
	extraArgs []string
	perf      *testexec.Cmd
	// command or whatever so we can ctrl-c it
}

const perfDataFile = "perf.data"
const scriptDataFile = "perf.data.dump"

// Setup starts a Linux perf trace.
func (a *perfTrace) Setup() error {
	args := append([]string{"record", "-o", filepath.Join(a.dir, perfDataFile)}, a.extraArgs...)
	a.perf = testexec.CommandContext(a.ctx, "perf", args...)
	if err := a.perf.Start(); err != nil {
		return errors.Wrap(err, "failed to start perf trace")
	}
	return nil
}

const sigintError = "signal: interrupt"

// Cleanup stops the Linux perf trace
func (a *perfTrace) Cleanup() error {
	if err := a.perf.Signal(syscall.SIGINT); err != nil {
		return errors.Wrap(err, "failed to interrupt perf trace")
	}
	if err := a.perf.Wait(testexec.DumpLogOnError); err != nil && err.Error() != sigintError {
		return errors.Wrap(err, "failed to wait for perf to stop")
	}
	dumpFile, err := os.Create(filepath.Join(a.dir, scriptDataFile))
	if err != nil {
		return errors.Wrap(err, "failed to create perf script dump file")
	}
	script := testexec.CommandContext(a.ctx, "perf", "script", "-i", filepath.Join(a.dir, perfDataFile))
	script.Stdout = dumpFile
	if err := script.Run(testexec.DumpLogOnError); err != nil {
		return errors.Wrap(err, "failed run perf script")
	}
	return nil
}

// PerfTrace creates an Action to start a Linux perf trace.
func PerfTrace(ctx context.Context, dir string, extraArgs []string) Action {
	return &perfTrace{
		ctx:       ctx,
		dir:       dir,
		extraArgs: extraArgs,
		perf:      nil,
	}
}

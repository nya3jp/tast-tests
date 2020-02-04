// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package setup

import (
	"context"
	"os"
	"path/filepath"
	"syscall"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

// PerfTrace starts a performance trace and returns a callback that stop the
// trace and dumps the results.
func PerfTrace(ctx context.Context, dir string, extraArgs []string) Result {
	const perfDataFileName = "perf.data"
	perfDataFile := filepath.Join(dir, perfDataFileName)
	args := append([]string{"record", "-o", perfDataFile}, extraArgs...)
	perf := testexec.CommandContext(ctx, "perf", args...)
	if err := perf.Start(); err != nil {
		return ResultFailed(errors.Wrap(err, "failed to start perf trace"))
	}
	testing.ContextLog(ctx, "Starting Linux perf trace")

	return ResultSucceeded(func(ctx context.Context) error {
		// Send ctrl-c to stop the perf trace.
		if err := perf.Signal(syscall.SIGINT); err != nil {
			return errors.Wrap(err, "failed to interrupt perf trace")
		}
		const sigintError = "signal: interrupt"
		if err := perf.Wait(testexec.DumpLogOnError); err != nil && err.Error() != sigintError {
			return errors.Wrap(err, "failed to wait for perf to stop")
		}

		// Get symbols for the perf.data file by calling 'perf script'
		const scriptDataFileName = "perf.data.dump"
		scriptDataFile := filepath.Join(dir, scriptDataFileName)
		dumpFile, err := os.Create(scriptDataFile)
		if err != nil {
			return errors.Wrap(err, "failed to create perf script dump file")
		}
		script := testexec.CommandContext(ctx, "perf", "script", "-i", perfDataFile)
		script.Stdout = dumpFile
		if err := script.Run(testexec.DumpLogOnError); err != nil {
			return errors.Wrap(err, "failed run perf script")
		}
		testing.ContextLog(ctx, "Stopped Linux perf trace")
		return nil
	})
}

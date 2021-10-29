// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package faillog provides a way to record logs on test failure.
package faillog

import (
	"context"
	"os"
	"path/filepath"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/fsutil"
	"chromiumos/tast/local/chrome/lacros/launcher"
	"chromiumos/tast/local/faillog"
	"chromiumos/tast/testing"
)

const lacrosFaillogDir = "lacros_faillog"

// SaveIf saves a lacros specific faillog if the hasError closure returns true.
// The intended use for this is to pass testing.State's HasError to this.
func SaveIf(ctx context.Context, f launcher.FixtValue, hasError func() bool) {
	if hasError() {
		Save(ctx, f)
	}
}

// Save saves a lacros specific faillog. If the faillog directory already exists
// then it does nothing. This is to support the use case of adding multiple
// calls to Save over a test, and only getting the faillog for the first
// failure.
func Save(ctx context.Context, f launcher.FixtValue) {
	// Runs the given command with redirecting its stdout to outpath.
	// On error, logging it then ignored.
	run := func(outpath string, cmds ...string) {
		out, err := os.Create(outpath)
		if err != nil {
			testing.ContextLogf(ctx, "Failed to create %q: %v", outpath, err)
			return
		}
		defer out.Close()
		cmd := testexec.CommandContext(ctx, cmds[0], cmds[1:]...)
		cmd.Stdout = out
		if err := cmd.Run(testexec.DumpLogOnError); err != nil {
			testing.ContextLogf(ctx, "Failed to run %q: %v", cmds[0], err)
		}
	}

	// Save a point-of-failure faillog.
	dir, ok := testing.ContextOutDir(ctx)
	if !ok {
		testing.ContextLog(ctx, "Error creating basic_faillog directory")
		return
	}

	dir = filepath.Join(dir, lacrosFaillogDir)

	// Don't overwrite if it has already been made. This lets callers
	// defer this function multiple times and get the result of the first
	// invocation, before any cleanup.
	if _, err := os.Stat(dir); err == nil {
		testing.ContextLog(ctx, "Skipping overwriting ", lacrosFaillogDir)
		return
	}

	if err := os.MkdirAll(dir, 0755); err != nil {
		testing.ContextLog(ctx, "Error creating basic_faillog directory: ", err)
		return
	}
	faillog.SaveToDir(ctx, dir)

	// Dump current mount status. Specifically noexec on /mnt/stateful_partition
	// is interesting.
	run(filepath.Join(dir, "mount.txt"), "mount")

	// Also check the dearchived files.
	run(filepath.Join(dir, "lacros-ls.txt"), "ls", "-l", f.LacrosPath())

	// Copy lacros log at the point of failure.
	if err := fsutil.CopyFile(launcher.LogFile(ctx), filepath.Join(dir, "lacros.log")); err != nil {
		testing.ContextLog(ctx, "Failed to save lacros logs: ", err)
	}
}

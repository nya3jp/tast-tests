// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package lacrosfaillog provides a way to record logs on test failure.
package lacrosfaillog

import (
	"context"
	"os"
	"path/filepath"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/fsutil"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/lacros/lacrosfixt"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/faillog"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

const lacrosFaillogDir = "lacros_faillog"

// SaveIf saves a lacros specific faillog if the hasError closure returns true.
// The intended use for this is to pass testing.State's HasError to this.
func SaveIf(ctx context.Context, lacrosPath string, hasError func() bool) {
	if hasError() {
		Save(ctx, lacrosPath)
	}
}

// Save saves a lacros specific faillog. If the faillog directory already exists
// then it does nothing. This is to support the use case of adding multiple
// calls to Save over a test, and only getting the faillog for the first
// failure.
func Save(ctx context.Context, lacrosPath string) {
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
	run(filepath.Join(dir, "lacros-ls.txt"), "ls", "-l", lacrosPath)

	// Copy lacros log at the point of failure.
	if err := fsutil.CopyFile(lacrosfixt.LacrosLogPath, filepath.Join(dir, "lacros.log")); err != nil {
		testing.ContextLog(ctx, "Failed to save lacros logs: ", err)
	}
}

// StopRecordAndSaveOnError stops the screen record and saves it under lacros faillog dir if the hasError closure returns true and there is a record started.
func StopRecordAndSaveOnError(ctx context.Context, tconn *chrome.TestConn, hasRecordStarted bool, hasError func() bool) {
	if hasRecordStarted {
		out, ok := testing.ContextOutDir(ctx)
		if !ok {
			testing.ContextLog(ctx, "OutDir not found")
			return
		}
		dir := filepath.Join(out, lacrosFaillogDir)
		if _, err := os.Stat(dir); os.IsNotExist(err) {
			os.MkdirAll(dir, 0755)
		}
		uiauto.StopRecordFromKBAndSaveOnError(ctx, tconn, hasError, dir)
	}
}

// StartRecord starts screen record from keyboard.
// It clicks Ctrl+Shift+F5 then select to record the whole desktop.
// The caller should also call StopRecordFromKB to stop the screen recorder,
// and save the record file.
// For more, see https://chromium.googlesource.com/chromiumos/platform/tast-tests/+/refs/heads/main/src/chromiumos/tast/local/chrome/uiauto/screen_recorder.go
func StartRecord(ctx context.Context, tconn *chrome.TestConn) error {
	// Start a screen recording for troubleshooting a failure in launching Lacros.
	kb, err := input.VirtualKeyboard(ctx)
	if err != nil {
		testing.ContextLog(ctx, "Failed to setup keyboard for screen recording: ", err)
		return err
	}
	if err := uiauto.StartRecordFromKB(ctx, tconn, kb); err != nil {
		testing.ContextLog(ctx, "Failed to start screen recording on DUT: ", err)
		return err
	}
	return nil
}

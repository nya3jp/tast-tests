// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package crostini

import (
	"bytes"
	"context"
	"io/ioutil"
	"path/filepath"

	"chromiumos/tast/local/vm"
	"chromiumos/tast/testing"
)

// RunCrostiniPostTest runs hooks that should run after every test but before
// the precondition closes (if it's going to) e.g. collecting logs from the
// container.
func RunCrostiniPostTest(ctx context.Context, cont *vm.Container) {
	if cont == nil {
		testing.ContextLog(ctx, "No active container")
		return
	}
	dir, ok := testing.ContextOutDir(ctx)
	if !ok || dir == "" {
		testing.ContextLog(ctx, "Failed to get name of directory")
		return
	}
	trySaveContainerLogs(ctx, dir, cont)

	if err := cont.Cleanup(cleanupCtx, "."); err != nil {
		testing.ContextLog(ctx, "Failed to remove all files in home directory in the container: ", err)
	}
}

// When we run trySaveContainerLogs we only want to capture logs since we last
// ran i.e. from the test that just finished, not all logs since the start of
// the suite. Sadly, Debian's journalctl in stable is too old to support cursor
// files, so we have to parse a cursor out of the log stream and remember it
// between calls to trySaveContainerLogs.
var cursor string

// trySaveContainerLogs fetches new (i.e. since last time the function
// successfully ran) logs from the container and writes them to
// crostini_journalctl.txt
func trySaveContainerLogs(ctx context.Context, dir string, cont *vm.Container) {
	args := []string{"sudo", "journalctl", "--no-pager", "--show-cursor"}
	if cursor != "" {
		args = append(args, "--cursor")
		args = append(args, cursor)
	}
	cmd := cont.Command(ctx, args...)
	output, err := cmd.Output()
	if err != nil {
		testing.ContextLog(ctx, "Error running journalctl: ", err)
		return
	}

	path := filepath.Join(dir, "crostini_journalctl.txt")
	err = ioutil.WriteFile(path, output, 0644)
	if err != nil {
		testing.ContextLog(ctx, "Error writing journalctl to log: ", err)
		return
	}

	cursorMarker := []byte("-- cursor: ")
	pos := bytes.LastIndex(output, cursorMarker)
	if pos == -1 {
		testing.ContextLog(ctx, "No journalctl cursor found")
		return
	}
	cursor = string(output[pos+len(cursorMarker):])
}

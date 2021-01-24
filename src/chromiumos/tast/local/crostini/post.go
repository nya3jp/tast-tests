// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package crostini

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"time"

	"chromiumos/tast/local/syslog"
	"chromiumos/tast/local/vm"
	"chromiumos/tast/testing"
)

// RunCrostiniPostTest runs hooks that should run after every test but before
// the precondition closes (if it's going to) e.g. collecting logs from the
// container.
func RunCrostiniPostTest(ctx context.Context, p PreData) {
	dir, ok := testing.ContextOutDir(ctx)
	if !ok || dir == "" {
		testing.ContextLog(ctx, "Failed to get name of directory")
		return
	}

	if p.ScreenRecorder != nil {
		if err := p.ScreenRecorder.Stop(ctx); err != nil {
			testing.ContextLogf(ctx, "Failed to stop recording: %s", err)
		} else {
			// TODO (jinrongwu): only save the record if the test case fails when porting to fixture.
			testing.ContextLogf(ctx, "Saving screen record to %s", dir)
			if err := p.ScreenRecorder.SaveInBytes(ctx, filepath.Join(dir, "screenRecord.webm")); err != nil {
				testing.ContextLogf(ctx, "Failed to save screen record in bytes: %s", err)
			}
		}
		p.ScreenRecorder.Release(ctx)
	}

	if p.Container == nil {
		testing.ContextLog(ctx, "No active container")
		return
	}
	if err := p.Container.Cleanup(ctx, "."); err != nil {
		testing.ContextLog(ctx, "Failed to remove all files in home directory in the container: ", err)
	}

	TrySaveVMLogs(ctx, p.Container.VM)
	trySaveContainerLogs(ctx, dir, p.Container)
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
	if cont == nil {
		testing.ContextLog(ctx, "No active container")
		return
	}
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

// Persistent reader for VM logs, keeps track of where it was up to.
// Internally it closes the old file and opens the new as logs get rotated, we
// never explicitly close it.
var logReader *syslog.LineReader

func newLogReader(ctx context.Context, machine *vm.VM) (*syslog.LineReader, error) {
	path := "/run/daemon-store/crosvm/" + machine.Concierge.GetOwnerID() + "/log/" + vm.GetEncodedName(machine.Name()) + ".log"

	// Only wait 1 second for the log file to exist, don't want to hang until
	// timeout if it doesn't exist, instead we continue.
	return syslog.NewLineReader(ctx, path, true,
		&testing.PollOptions{Timeout: 1 * time.Second})
}

// TrySaveVMLogs writes logs since the last call to the
// current test's output folder.
func TrySaveVMLogs(ctx context.Context, machine *vm.VM) {
	if logReader == nil {
		var err error
		logReader, err = newLogReader(ctx, machine)
		if err != nil {
			testing.ContextLog(ctx, "Error creating log reader: ", err)
			return
		}
	}

	dir, ok := testing.ContextOutDir(ctx)
	if !ok || dir == "" {
		testing.ContextLog(ctx, "Failed to get name of directory")
		return
	}

	path := filepath.Join(dir, "termina_logs.txt")
	f, err := os.Create(path)
	if err != nil {
		testing.ContextLog(ctx, "Error creating file: ", err)
		return
	}
	defer f.Close()

	for {
		line, err := logReader.ReadLine()
		if err != nil {
			if err != io.EOF {
				testing.ContextLog(ctx, "Error reading file: ", err)
			}
			break
		}
		_, err = f.WriteString(line)
		if err != nil {
			testing.ContextLog(ctx, fmt.Sprintf("Error writing %s to file: ", line), err)
		}
	}
}

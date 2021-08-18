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

	"chromiumos/tast/local/cryptohome"
	"chromiumos/tast/local/syslog"
	"chromiumos/tast/local/vm"
	"chromiumos/tast/testing"
)

// PostTimeout is the standard time reserved for post-test tasks.
var PostTimeout = 30 * time.Second

// RunCrostiniPostTest runs hooks that should run after every test but before
// the precondition closes (if it's going to) e.g. collecting logs from the
// container.
func RunCrostiniPostTest(ctx context.Context, p PreData) {
	dir, ok := testing.ContextOutDir(ctx)
	if !ok || dir == "" {
		testing.ContextLog(ctx, "Failed to get name of directory")
		return
	}

	// If we haven't connected to chrome successfully, then the
	// test didn't get to do anything with the VM that could
	// possibly have generated logs, and even if it did we
	// couldn't access them, so bail out here.
	if p.Chrome == nil {
		testing.ContextLog(ctx, "Failed before connecting to chrome, no logs generated")
		return
	}

	// Container logs require a running VM and container. If one
	// hasn't been set, we can't fetch them.
	if p.Container != nil {
		trySaveContainerLogs(ctx, dir, p.Container)

		if err := p.Container.Cleanup(ctx, "."); err != nil {
			testing.ContextLog(ctx, "Failed to remove all files in home directory in the container: ", err)
		}
	} else {
		testing.ContextLog(ctx, "No active container, can't get journalctl logs")
	}

	// LXC logs only require a running VM, so even if the
	// container hasn't been set we can try to get a running VM
	// and use that.
	var machine *vm.VM
	if p.Container != nil {
		machine = p.Container.VM
	} else {
		machine2, err := vm.GetRunningVM(ctx, p.Chrome.NormalizedUser())
		machine = machine2
		if err != nil {
			testing.ContextLog(ctx, "Failed to get running VM, won't get LXC logs: ", err)
		}
	}
	if machine != nil {
		writeLXCLogs(ctx, dir, machine)
	}

	// VM logs are stored on the host, so we don't need the VM to
	// be running at all to get them.
	trySaveVMLogs(ctx, dir, p.Chrome.NormalizedUser())
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

func newLogReader(ctx context.Context, user string) (*syslog.LineReader, error) {
	ownerID, err := cryptohome.UserHash(ctx, user)
	if err != nil {
		return nil, err
	}
	path := "/run/daemon-store/crosvm/" + ownerID + "/log/" + vm.GetEncodedName(vm.DefaultVMName) + ".log"

	// Only wait 1 second for the log file to exist, don't want to hang until
	// timeout if it doesn't exist, instead we continue.
	return syslog.NewLineReader(ctx, path, true,
		&testing.PollOptions{Timeout: 1 * time.Second})
}

// trySaveVMLogs writes logs since the last call to the
// current test's output folder.
func trySaveVMLogs(ctx context.Context, dir, user string) {
	if logReader == nil {
		var err error
		logReader, err = newLogReader(ctx, user)
		if err != nil {
			testing.ContextLog(ctx, "Error creating log reader: ", err)
			return
		}
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

func writeLXCLogs(ctx context.Context, dir string, machine *vm.VM) {
	testing.ContextLog(ctx, "Creating file")
	path := filepath.Join(dir, "crostini_logs.txt")
	f, err := os.Create(path)
	if err != nil {
		testing.ContextLog(ctx, "Error creating file: ", err)
		return
	}
	defer f.Close()

	f.WriteString("lxc info and lxc.log:\n")
	cmd := machine.Command(ctx, "sh", "-c", "LXD_DIR=/mnt/stateful/lxd LXD_CONF=/mnt/stateful/lxd_conf lxc info penguin --show-log")
	cmd.Stdout = f
	cmd.Stderr = f
	err = cmd.Run()
	if err != nil {
		testing.ContextLog(ctx, "Error getting lxc logs: ", err)
	}

	f.WriteString("\n\nconsole.log:\n")
	cmd = machine.Command(ctx, "sh", "-c", "LXD_DIR=/mnt/stateful/lxd  LXD_CONF=/mnt/stateful/lxd_conf lxc console penguin --show-log")
	cmd.Stdout = f
	cmd.Stderr = f
	err = cmd.Run()
	if err != nil {
		testing.ContextLog(ctx, "Error getting boot logs: ", err)
	}
}

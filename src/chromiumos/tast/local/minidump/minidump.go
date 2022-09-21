// Copyright 2018 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package minidump saves minidumps without making processes crash.
// This is useful for investigating hanging processes.
package minidump

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/shirou/gopsutil/v3/process"
	"golang.org/x/sys/unix"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/testing"
)

// Matcher specifies processes to save minidumps of.
type Matcher func(*process.Process) bool

// MatchByName specifies processes by process names.
func MatchByName(names ...string) Matcher {
	return func(proc *process.Process) bool {
		pname, err := proc.Name()
		if err != nil {
			return false
		}
		for _, name := range names {
			if pname == name {
				return true
			}
		}
		return false
	}
}

// MatchByPID specifies processes by PIDs.
func MatchByPID(pids ...int32) Matcher {
	return func(proc *process.Process) bool {
		for _, pid := range pids {
			if proc.Pid == pid {
				return true
			}
		}
		return false
	}
}

// SaveWithoutCrash saves minidumps of processes matched by matchers.
// Minidumps are saved in dir as "<process name>.<PID>.dmp".
func SaveWithoutCrash(ctx context.Context, dir string, matchers ...Matcher) {
	procs := freeze(ctx, matchers...)
	defer unfreeze(ctx, procs)

	if len(procs) == 0 {
		testing.ContextLog(ctx, "minidump.SaveWithoutCrash called but no process matched")
		return
	}

	for _, p := range procs {
		name, err := p.Name()
		if err != nil {
			name = "unknown"
		}
		path := filepath.Join(dir, fmt.Sprintf("%s.%d.dmp", name, p.Pid))
		testing.ContextLog(ctx, "Saving minidump: ", path)
		saveWithoutCrash(ctx, p.Pid, path)
	}
}

// freeze finds processes matched by matchers and sends SIGSTOP to them.
func freeze(ctx context.Context, matchers ...Matcher) []*process.Process {
	all, err := process.Processes()
	if err != nil {
		testing.ContextLog(ctx, "Failed enumerating processes: ", err)
		return nil
	}

	var matched []*process.Process
	for _, p := range all {
		for _, m := range matchers {
			if m(p) {
				if err := p.SendSignal(unix.SIGSTOP); err != nil {
					continue
				}
				matched = append(matched, p)
				break
			}
		}
	}

	return matched
}

// unfreeze sends SIGCONT to procs.
func unfreeze(ctx context.Context, procs []*process.Process) {
	for _, p := range procs {
		if err := p.SendSignal(unix.SIGCONT); err != nil {
			testing.ContextLog(ctx, "Failed sending SIGCONT to ", p.Pid)
		}
	}
}

// saveWithoutCrash saves minidump of a process without making it crash.
func saveWithoutCrash(ctx context.Context, pid int32, path string) {
	tmp, err := ioutil.TempDir("", "gcore.")
	if err != nil {
		testing.ContextLog(ctx, "Failed creating a temporary directory: ", err)
		return
	}
	defer os.RemoveAll(tmp)

	cmd := testexec.CommandContext(ctx, "gcore", "-o", filepath.Join(tmp, "core"), fmt.Sprint(pid))
	if err := cmd.Run(); err != nil {
		testing.ContextLog(ctx, "Failed to save core: ", err)
		cmd.DumpLog(ctx)
		return
	}

	cmd = testexec.CommandContext(ctx, "core2md", filepath.Join(tmp, fmt.Sprintf("core.%d", pid)), fmt.Sprintf("/proc/%d", pid), path)
	if err := cmd.Run(); err != nil {
		testing.ContextLog(ctx, "Failed to convert to minidump: ", err)
		cmd.DumpLog(ctx)
	}
}

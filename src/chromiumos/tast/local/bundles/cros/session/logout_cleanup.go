// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package session

import (
	"context"
	"time"

	"github.com/shirou/gopsutil/v3/process"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/crash"
	"chromiumos/tast/local/session"
	"chromiumos/tast/local/sysutil"
	"chromiumos/tast/local/upstart"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         LogoutCleanup,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Verifies all processes owned by chronos are destroyed on logout",
		Contacts: []string{
			"mnissler@chromium.org", // session_manager owner
			"hidehiko@chromium.org", // Tast port author
		},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:mainline"},
	})
}

// isActiveChronosProcess returns true if the process with the given pid is owned
// by the chronos user and not in the zombie state (indicating that it's exited
// but hasn't yet been waited on by its parent).
func isActiveChronosProcess(pid int32) bool {
	p, err := process.NewProcess(pid)
	if err != nil {
		// The process may be gone already.
		return false
	}

	if status, err := p.Status(); err != nil || status[0] == "Z" || status[0] == "X" {
		// The process exited already, or is a zombie that hasn't yet been reaped
		// by its parent (https://crbug.com/963144), or is in the mysterious "dead" state
		// that "should never be seen" but sometimes is (https://crrev.com/c/1303781).
		return false
	}

	uids, err := p.Uids()
	if err != nil {
		// The process is gone between NewProcess() and Uids() call.
		return false
	}
	// euid is stored at [1].
	return uint32(uids[1]) == sysutil.ChronosUID
}

// findChronosProcesses returns a list of PIDs owned by chronos user.
func findChronosProcesses() ([]int32, error) {
	all, err := process.Pids()
	if err != nil {
		return nil, err
	}

	var pids []int32
	for _, pid := range all {
		if isActiveChronosProcess(pid) {
			pids = append(pids, pid)
		}
	}
	return pids, nil
}

func LogoutCleanup(ctx context.Context, s *testing.State) {
	var cmds []*testexec.Cmd
	defer func() {
		for _, cmd := range cmds {
			cmd.Kill()
		}
	}()

	// Tell crash_reporter to ignore the intentional crashes we create in bash
	crash.EnableCrashBlocking(ctx, "bash")
	defer crash.DisableCrashBlocking()

	func() {
		cr, err := chrome.New(ctx)
		if err != nil {
			s.Fatal("Failed to log in with Chrome: ", err)
		}
		defer cr.Close(ctx)

		// Starts background jobs which start infinite loop processes
		// owned by chronos.
		cmds = append(cmds,
			testexec.CommandContext(
				ctx, "su", "chronos", "-c", "while :; do sleep 30 ; done"),
			// Create a test process that ignores SIGTEREM (15).
			testexec.CommandContext(
				ctx, "su", "chronos", "-c", "trap 15; while :; do sleep 30 ; done"))
		for _, cmd := range cmds {
			if err := cmd.Start(); err != nil {
				s.Fatal("Failed to start command: ", err)
			}
		}

		testing.ContextLog(ctx, "Waiting for processes owned by chronos to start")
		for _, cmd := range cmds {
			p, err := process.NewProcess(int32(cmd.Process.Pid))
			if err != nil {
				s.Fatalf("Job %d not found: %v", cmd.Process.Pid, err)
			}
			if err = testing.Poll(ctx, func(context.Context) error {
				children, err := p.Children()
				if err != nil {
					return err
				}
				for _, child := range children {
					if !isActiveChronosProcess(child.Pid) {
						// There may be a small chance that the fork succeeded but UID is not yet set.
						// So, this can be transient error.
						return errors.New("child job isn't running as chronos user")
					}
				}
				return nil
			}, &testing.PollOptions{Timeout: 10 * time.Second}); err != nil {
				s.Fatal("Job owned by chronos wasn't launched: ", err)
			}
		}
	}()

	chronosPIDs, err := findChronosProcesses()
	if err != nil {
		s.Fatal("Failed to list processes owned by chronos: ", err)
	}

	oldPID, err := session.GetSessionManagerPID()
	if err != nil {
		s.Fatal("session_manager not found: ", err)
	}

	// Emulate logout. chrome.Chrome.Close() does not log out. So, here,
	// manually restart "ui" job for the emulation.
	if err := upstart.RestartJob(ctx, "ui"); err != nil {
		s.Fatal("Failed to log out: ", err)
	}

	testing.ContextLog(ctx, "Waiting for new session_manager process")
	if err := testing.Poll(ctx, func(context.Context) error {
		pid, err := session.GetSessionManagerPID()
		if err != nil || pid == oldPID {
			return errors.New("waiting for new session_manager")
		}
		return nil
	}, &testing.PollOptions{}); err != nil {
		s.Fatal("session_manager was not launched: ", err)
	}

	// The process may be running uninterruptable operations. In that case
	// even if SIGKILL is delivered, the process may not be yet collected
	// immediately. Thus, wait until they are, actually.
	if err := testing.Poll(ctx, func(context.Context) error {
		for _, pid := range chronosPIDs {
			if isActiveChronosProcess(pid) {
				return errors.Errorf("process %d owned by chronos is still alive after logout", pid)
			}
		}
		return nil
	}, &testing.PollOptions{Timeout: 10 * time.Second}); err != nil {
		s.Fatal("Chronos processes are not terminated: ", err)
	}
}

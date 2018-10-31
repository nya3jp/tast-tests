// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package session

import (
	"context"
	"time"

	"github.com/shirou/gopsutil/process"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/session"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/local/upstart"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         LogoutCleanup,
		Desc:         "Verifies all processes owned by chronos are destroyed on logout",
		Attr:         []string{"informational"},
		SoftwareDeps: []string{"chrome_login"},
	})
}

// isChronosProcess returns true if process with given pid is owned by chronos
// user.
func isChronosProcess(pid int32) bool {
	const chronosUID = 1000

	p, err := process.NewProcess(pid)
	if err != nil {
		// The process may be gone already.
		return false
	}
	uids, err := p.Uids()
	if err != nil {
		// The process is gone between NewProcess() and Uids() call.
		return false
	}

	// euid is stored at [1].
	return uids[1] == chronosUID
}

// findChronosProcesses returns a list of PIDs owned by chronos user.
func findChronosProcesses() ([]int32, error) {
	all, err := process.Pids()
	if err != nil {
		return nil, err
	}

	var pids []int32
	for _, pid := range all {
		if isChronosProcess(pid) {
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

	func() {
		cr, err := chrome.New(ctx)
		if err != nil {
			s.Fatal("Failed to log in by Chrome: ", err)
		}
		defer cr.Close(ctx)

		// Starts background jobs which start infinite loop processes
		// owned by chronos.
		cmds = append(cmds,
			testexec.CommandContext(
				ctx, "su", "chronos", "-c", "while :; do :; done"),
			// Create a test process that ignores SIGTEREM (15).
			testexec.CommandContext(
				ctx, "su", "chronos", "-c", "trap 15; while :; do :; done"))
		for _, cmd := range cmds {
			if err := cmd.Start(); err != nil {
				s.Fatal("Failed to start command: ", err)
			}
		}

		testing.ContextLogf(ctx, "Waiting for processes owned by chronos start")
		for _, cmd := range cmds {
			p, err := process.NewProcess(int32(cmd.Process.Pid))
			if err != nil {
				s.Fatalf("Job %d is not found: %v", cmd.Process.Pid, err)
			}
			if err = testing.Poll(ctx, func(context.Context) error {
				children, err := p.Children()
				if err != nil {
					return err
				}
				for _, child := range children {
					if !isChronosProcess(child.Pid) {
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
		s.Fatal("session_manager is not found: ", err)
	}

	// Emulate logout. chrome.Chrome.Close() does not log out. So, here,
	// manually restart "ui" job for the emulation.
	if err := upstart.RestartJob(ctx, "ui"); err != nil {
		s.Fatal("Failed to log out: ", err)
	}

	testing.ContextLogf(ctx, "Waiting for the new session_manager launch")
	if err := testing.Poll(ctx, func(context.Context) error {
		pid, err := session.GetSessionManagerPID()
		if err != nil || pid == oldPID {
			return errors.New("waiting for new session_manager")
		}
		return nil
	}, &testing.PollOptions{}); err != nil {
		s.Fatal("session_manager was not launched: ", err)
	}

	for _, pid := range chronosPIDs {
		if isChronosProcess(pid) {
			s.Errorf("A process (pid: %d) owned by chronos is kept alive after logout", pid)
		}
	}
}

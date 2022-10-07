// Copyright 2021 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package meta

import (
	"context"
	"os"
	"strings"
	"syscall"

	"github.com/shirou/gopsutil/v3/process"

	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         DisruptSSH,
		Desc:         "Terminates the current SSH connection",
		Contacts:     []string{"tast-owners@google.com"},
		BugComponent: "b:1034625",
		// This test always fails.
	})
}

func DisruptSSH(ctx context.Context, s *testing.State) {
	// Walk up the process tree to find the sshd process corresponding to
	// the current session.
	proc, err := process.NewProcess(int32(os.Getpid()))
	if err != nil {
		s.Fatal("Failed to find self process: ", err)
	}
	for {
		exe, err := proc.Exe()
		if err != nil {
			s.Fatalf("Failed to get executable path of process %d: %v", proc.Pid, err)
		}
		if strings.HasSuffix(exe, "/sshd") {
			break
		}

		ppid, err := proc.Ppid()
		if err != nil {
			s.Fatalf("Failed to get parent PID of process %d: %v", proc.Pid, err)
		}
		if ppid == 0 {
			s.Fatal("Could not find sshd; see ps.txt")
		}

		proc, err = process.NewProcess(ppid)
		if err != nil {
			s.Fatalf("Process %d missing: %v", ppid, err)
		}
	}

	s.Logf("Killing sshd(%d); expect SSH connection drop", proc.Pid)

	// Terminate the sshd.
	if err := proc.SendSignal(syscall.SIGKILL); err != nil {
		s.Fatalf("Failed to send SIGKILL to sshd(%d): %v", proc.Pid, err)
	}

	// Wait for the current process to be terminated.
	<-ctx.Done()

	s.Fatalf("Failed to terminate sshd(%d) for unknown reason", proc.Pid)
}

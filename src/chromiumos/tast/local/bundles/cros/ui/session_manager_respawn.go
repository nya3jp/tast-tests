// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"
	"syscall"
	"time"

	"github.com/shirou/gopsutil/process"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/ui/respawn"
	"chromiumos/tast/local/upstart"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: SessionManagerRespawn,
		Desc: "Checks that session_manager respawns after exit",
		Contacts: []string{
			"mnissler@chromium.org", // session_manager owner
			"chromeos-ui@google.com",
		},
		SoftwareDeps: []string{"chrome"},
	})
}

func SessionManagerRespawn(ctx context.Context, s *testing.State) {
	const sessionManagerPath = "/sbin/session_manager"
	getPID := func() (int, error) {
		all, err := process.Pids()
		if err != nil {
			return -1, err
		}

		for _, pid := range all {
			if proc, err := process.NewProcess(pid); err != nil {
				// Assume that the process exited.
				continue
			} else if exe, err := proc.Exe(); err == nil && exe == sessionManagerPath {
				return int(pid), nil
			}
		}
		return -1, errors.Errorf("%v process not found", sessionManagerPath)
	}

	if err := upstart.EnsureJobRunning(ctx, "ui"); err != nil {
		s.Fatal("Failed to ensure ui job is running: ", err)
	}
	pid := respawn.TestRespawn(ctx, s, "session_manager", getPID)

	respawnStopped := false
	const (
		maxRespawns    = 30 // very high upper bound; see ui-respawn script for actual logic
		respawnTimeout = 5 * time.Second
	)
	s.Log("Repeatedly killing session_manager to check that ui-respawn stops restarting it eventually")
	for i := 0; i < maxRespawns; i++ {
		s.Logf("Killing %d and watching for respawn", pid)
		if err := syscall.Kill(pid, syscall.SIGKILL); err != nil {
			s.Fatalf("Failed to kill %d: %v", pid, err)
		}
		var err error
		if pid, err = respawn.WaitForProc(ctx, getPID, respawnTimeout, pid); err != nil {
			s.Log("session_manager (correctly) not respawned")
			respawnStopped = true
			break
		}
	}
	if !respawnStopped {
		s.Errorf("session_manager was still respawned after being killed %d times", maxRespawns)
	}

	if err := upstart.EnsureJobRunning(ctx, "ui"); err != nil {
		s.Fatal("Failed to ensure ui job is running: ", err)
	}
}

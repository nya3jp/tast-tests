// Copyright 2018 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"
	"time"

	"golang.org/x/sys/unix"

	ups "chromiumos/tast/common/upstart"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/upstart"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         SessionManagerRespawn,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Checks that session_manager respawns after exit",
		Contacts: []string{
			"chromeos-ui@google.com",
		},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:mainline"},
	})
}

func SessionManagerRespawn(ctx context.Context, s *testing.State) {
	if err := upstart.EnsureJobRunning(ctx, "ui"); err != nil {
		s.Fatal("Failed to ensure ui job is running: ", err)
	}
	_, _, pid, err := upstart.JobStatus(ctx, "ui")
	if err != nil {
		s.Fatal("Failed to get ui job status: ", err)
	}

	respawnStopped := false
	const (
		maxRespawns    = 30 // very high upper bound; see ui-respawn script for actual logic
		respawnTimeout = 30 * time.Second
	)
	s.Log("Repeatedly killing session_manager to check that ui-respawn stops restarting it eventually")
	for i := 0; i < maxRespawns && !respawnStopped; i++ {
		s.Logf("Killing %d and watching for respawn", pid)
		if err := unix.Kill(pid, unix.SIGKILL); err != nil {
			s.Fatalf("Failed to kill %d: %v", pid, err)
		}
		err = testing.Poll(ctx, func(ctx context.Context) error {
			g, st, p, err := upstart.JobStatus(ctx, "ui")
			if err != nil {
				return testing.PollBreak(err)
			}
			if p == pid {
				return errors.Errorf("waiting for %d to terminate", pid)
			}
			if g == ups.StartGoal && st == ups.RunningState {
				// session_manager was respawned, record new pid.
				pid = p
				return nil
			}
			if g == ups.StopGoal && st == ups.WaitingState {
				s.Log("session_manager (correctly) not respawned")
				respawnStopped = true
				return nil
			}
			// Some other transient state, wait for things to settle.
			return errors.Errorf("status %v/%v", g, st)
		}, &testing.PollOptions{Timeout: respawnTimeout})
		if err != nil {
			s.Fatal("Failed to wait for ui job to change state: ", err)
		}
	}
	if !respawnStopped {
		s.Errorf("session_manager was still respawned after being killed %d times", maxRespawns)
	}

	if err := upstart.EnsureJobRunning(ctx, "ui"); err != nil {
		s.Fatal("Failed to ensure ui job is running: ", err)
	}
}

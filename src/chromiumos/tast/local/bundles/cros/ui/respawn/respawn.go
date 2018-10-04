// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package respawn contains shared code to verify that processes respawn after exiting.
package respawn

import (
	"context"
	"syscall"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/testing"
)

// GetPIDFunc returns a running process's PID.
// An error should be returned if the process is not found.
type GetPIDFunc func() (int, error)

// WaitForProc waits for f to return a process not equal to oldPID.
// If timeout is positive, it limits the maximum amount of time to wait.
// The new process's PID is returned.
func WaitForProc(ctx context.Context, f GetPIDFunc, timeout time.Duration, oldPID int) (newPID int, err error) {
	if timeout > 0 {
		var cancel func()
		ctx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}

	err = testing.Poll(ctx, func(ctx context.Context) error {
		var err error
		newPID, err = f()
		if err == nil {
			if newPID != oldPID {
				return nil
			}
			return errors.Errorf("old process %d still running", oldPID)
		}
		return err
	}, &testing.PollOptions{Timeout: timeout})
	return newPID, err
}

// TestRespawn kills the process initially returned by f and then verifies that
// a new process is returned by f. name is a human-readable string describing the process,
// e.g. "Chrome" or "session_manager". The respawned PID is returned.
func TestRespawn(ctx context.Context, s *testing.State, name string, f GetPIDFunc) int {
	s.Logf("Getting initial %s process", name)
	oldPID, err := WaitForProc(ctx, f, 0, -1)
	if err != nil {
		s.Fatalf("Failed getting initial %s process: %v", name, err)
	}
	s.Logf("Initial %s process is %d", name, oldPID)

	s.Log("Killing ", oldPID)
	if err := syscall.Kill(oldPID, syscall.SIGKILL); err != nil {
		s.Fatalf("Failed to kill %d: %v", oldPID, err)
	}

	s.Logf("Waiting for %s to respawn", name)
	newPID, err := WaitForProc(ctx, f, 0, oldPID)
	if err != nil {
		s.Fatalf("Failed waiting for %s to respawn: %v", name, err)
	}
	s.Logf("Respawned %s process is %d", name, newPID)
	return newPID
}

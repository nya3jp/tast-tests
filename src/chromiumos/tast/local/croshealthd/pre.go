// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package croshealthd

import (
	"context"
	"time"

	"chromiumos/tast/local/upstart"
	"chromiumos/tast/testing"
)

// crosHealthdJobName is the name of the cros_healthd upstart job
const crosHealthdJobName = "cros_healthd"

// preImpl implements testing.Precondition.
type preImpl struct {
	name    string
	timeout time.Duration
	run     bool
	pid     int
}

// NewPrecondition creates a new precondition that can be shared by multiple
// states.
func NewPrecondition(suffix string, run bool) *preImpl {
	return &preImpl{
		name:    "cros_healthd_" + suffix,
		timeout: 10 * time.Second,
		run:     run,
	}
}

// RunningPre is a precondition that expects the daemon cros_healthd to be
// running.
var RunningPre = NewPrecondition("running", true)

func (p *preImpl) String() string         { return p.name }
func (p *preImpl) Timeout() time.Duration { return p.timeout }

// Prepare is called by the test framework at the beginning of every test using
// this precondition. It returns a PreData containing the current state that can
// be used by the test.
func (p *preImpl) Prepare(ctx context.Context, s *testing.PreState) interface{} {
	if p.run {
		// Starts the cros_healthd daemon if it is not running. If it is already
		// running, this is a no-op.
		if err := upstart.EnsureJobRunning(ctx, crosHealthdJobName); err != nil {
			s.Fatalf("Failed to start %s daemon: %s", crosHealthdJobName, err)
		}
	} else {
		// Stops the cros_healthd daemon if it is running. If it is already
		// stopped, this is a no-op.
		if err := upstart.StopJob(ctx, crosHealthdJobName); err != nil {
			s.Fatalf("Failed to stop %s daemon: %s", crosHealthdJobName, err)
		}
	}

	_, _, pid, err := upstart.JobStatus(ctx, crosHealthdJobName)
	if err != nil {
		s.Fatalf("Unable to get %s PID: %s", crosHealthdJobName, err)
	}

	// If the daemon has already been started, make sure it has not crashed or
	// restarted.
	if p.pid != 0 && p.run {
		if pid != p.pid {
			s.Fatalf("%s PID changed: want %s, got %s", crosHealthdJobName, p.pid, pid)
		}
	}

	p.pid = pid
	return p.pid
}

// Close is called by the test framework after the last test that uses this precondition.
func (p *preImpl) Close(ctx context.Context, s *testing.PreState) {
	// cros_healthd should be running be default on all systems. Ensure it is
	// left running.
	if err := upstart.EnsureJobRunning(ctx, crosHealthdJobName); err != nil {
		s.Fatalf("Failed to start %s daemon: %s", crosHealthdJobName, err)
	}
}

// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package croshealthd

import (
	"context"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/upstart"
	"chromiumos/tast/testing"
)

// crosHealthdJobName is the name of the cros_healthd upstart job
const crosHealthdJobName = "cros_healthd"

func init() {
	testing.AddFixture(&testing.Fixture{
		Name: "crosHealthdRunning",
		Desc: "The croshealthd daemon is available and running",
		Contacts: []string{
			"kerker@google.com", // Fixture maintainer
			"pmoy@google.com",
			"cros-tdm@google.com",         // team mailing list
			"cros-tdm-tpe-eng@google.com", // team mailing list
		},
		SetUpTimeout:    30 * time.Second,
		ResetTimeout:    5 * time.Second,
		PreTestTimeout:  5 * time.Second,
		PostTestTimeout: 5 * time.Second,
		TearDownTimeout: 5 * time.Second,
		Impl:            newCrosHealthdFixture(true),
	})
}

// crosHealthdFixture implements testing.FixtureImpl.
type crosHealthdFixture struct {
	run bool
	pid int
}

func newCrosHealthdFixture(run bool) testing.FixtureImpl {
	return &crosHealthdFixture{
		run: run,
	}
}

func (f *crosHealthdFixture) SetUp(ctx context.Context, s *testing.FixtState) interface{} {
	if err := f.Reset(ctx); err != nil {
		s.Fatal("Unable to reset fixture: ", err)
	}

	// Restart the "ui" job to ensure that Chrome is running and wait Chrome to
	// bootstrap the cros_healthd mojo services.
	if err := upstart.RestartJob(ctx, "ui"); err != nil {
		s.Fatal("Unable to ensure 'ui' upstart service is running: ", err)
	}
	if err := waitForMojoBootstrap(ctx); err != nil {
		s.Fatal("Unable to wait for mojo bootstrap: ", err)
	}
	return nil
}

func (f *crosHealthdFixture) Reset(ctx context.Context) error {
	if f.run {
		// Starts the cros_healthd daemon if it is not running. If it is already
		// running, this is a no-op.
		if err := upstart.EnsureJobRunning(ctx, crosHealthdJobName); err != nil {
			return errors.Wrapf(err, "failed to start %s daemon", crosHealthdJobName)
		}
	} else {
		// Stops the cros_healthd daemon if it is running. If it is already
		// stopped, this is a no-op.
		if err := upstart.StopJob(ctx, crosHealthdJobName); err != nil {
			return errors.Wrapf(err, "failed to stop %s daemon", crosHealthdJobName)
		}
	}
	return nil
}

func (f *crosHealthdFixture) PreTest(ctx context.Context, s *testing.FixtTestState) {
	_, _, pid, err := upstart.JobStatus(ctx, crosHealthdJobName)
	if err != nil {
		s.Fatalf("Unable to get %s PID: %s", crosHealthdJobName, err)
	}

	f.pid = pid
}

func (f *crosHealthdFixture) PostTest(ctx context.Context, s *testing.FixtTestState) {
	_, _, pid, err := upstart.JobStatus(ctx, crosHealthdJobName)
	if err != nil {
		s.Fatalf("Unable to get %s PID: %s", crosHealthdJobName, err)
	}

	// If the daemon has already been started, make sure it has not crashed or
	// restarted.
	if f.run && pid != f.pid {
		s.Fatalf("%s PID changed: want %v, got %v", crosHealthdJobName, f.pid, pid)
	}
}

func (f *crosHealthdFixture) TearDown(ctx context.Context, s *testing.FixtState) {
	// cros_healthd should be running be default on all systems. Ensure it is
	// left running.
	if err := upstart.EnsureJobRunning(ctx, crosHealthdJobName); err != nil {
		s.Fatalf("Failed to start %s daemon: %s", crosHealthdJobName, err)
	}
}

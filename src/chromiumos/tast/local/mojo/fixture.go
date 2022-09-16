// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package mojo provides methods for running mojo service manager.
package mojo

import (
	"context"
	"time"

	"chromiumos/tast/common/fixture"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/upstart"
	"chromiumos/tast/testing"
)

// The name of the mojo_service_manager upstart job.
const serviceManagerJobName = "mojo_service_manager"

func init() {
	testing.AddFixture(&testing.Fixture{
		Name: fixture.MojoServiceManagerRunning,
		Desc: "The mojo service manager daemon is available and running",
		Contacts: []string{
			"chungsheng@google.com",                    // Fixture maintainer
			"chromeos-mojo-service-manager@google.com", // team mailing list
		},
		SetUpTimeout:    30 * time.Second,
		ResetTimeout:    5 * time.Second,
		PreTestTimeout:  5 * time.Second,
		PostTestTimeout: 5 * time.Second,
		TearDownTimeout: 5 * time.Second,
		Impl:            newServiceManagerFixture(),
	})
}

// serviceManagerFixture implements testing.FixtureImpl.
type serviceManagerFixture struct {
	// The pid of mojo_service_manager, for check if it crashed or restarted
	// within a single test.
	pid int
}

func newServiceManagerFixture() testing.FixtureImpl {
	return &serviceManagerFixture{}
}

func ensureServiceManagerRunning(ctx context.Context) error {
	if err := upstart.EnsureJobRunning(ctx, serviceManagerJobName); err != nil {
		return errors.Wrapf(err, "failed to start %s", serviceManagerJobName)
	}
	return nil
}

func (f *serviceManagerFixture) SetUp(ctx context.Context, s *testing.FixtState) interface{} {
	return ensureServiceManagerRunning(ctx)
}

func getServiceManagerPid(ctx context.Context, s *testing.FixtTestState) int {
	_, _, pid, err := upstart.JobStatus(ctx, serviceManagerJobName)
	if err != nil {
		s.Fatalf("Unable to get %s PID: %s", serviceManagerJobName, err)
	}
	return pid
}

func (f *serviceManagerFixture) PreTest(ctx context.Context, s *testing.FixtTestState) {
	f.pid = getServiceManagerPid(ctx, s)
}

func (f *serviceManagerFixture) PostTest(ctx context.Context, s *testing.FixtTestState) {
	// Make sure it has not crashed or restarted.
	if pid := getServiceManagerPid(ctx, s); pid != f.pid {
		s.Fatalf("%s PID changed: want %v, got %v", serviceManagerJobName, f.pid, pid)
	}
}

func (f *serviceManagerFixture) Reset(ctx context.Context) error {
	return ensureServiceManagerRunning(ctx)
}

func (f *serviceManagerFixture) TearDown(ctx context.Context, s *testing.FixtState) {
	// mojo_service_manager should keep running after test finished.
	if err := ensureServiceManagerRunning(ctx); err != nil {
		s.Fatalf("Failed to run service manager: %s", err)
	}
}

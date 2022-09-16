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
		Name: fixture.MojoServiceManagerCheckPid,
		Desc: "The mojo service manager daemon is available and the pid doesn't change",
		Contacts: []string{
			"chungsheng@google.com",                    // Fixture maintainer
			"chromeos-mojo-service-manager@google.com", // team mailing list
		},
		SetUpTimeout:    5 * time.Second,
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

func getServiceManagerPidAndCheckIsRunning(ctx context.Context) (int, error) {
	_, _, pid, err := upstart.JobStatus(ctx, serviceManagerJobName)
	if err != nil {
		return 0, errors.Wrapf(err, "unable to get status of %s", serviceManagerJobName)
	}
	if pid == 0 {
		return 0, errors.Errorf("service %s is not running", serviceManagerJobName)
	}
	return pid, nil
}

func (f *serviceManagerFixture) SetUp(ctx context.Context, s *testing.FixtState) interface{} {
	return nil
}

func (f *serviceManagerFixture) PreTest(ctx context.Context, s *testing.FixtTestState) {
	pid, err := getServiceManagerPidAndCheckIsRunning(ctx)
	if err != nil {
		s.Fatal("Failed to get pid: ", err)
	}
	f.pid = pid
}

func (f *serviceManagerFixture) PostTest(ctx context.Context, s *testing.FixtTestState) {
	// Make sure it has not crashed or restarted.
	pid, err := getServiceManagerPidAndCheckIsRunning(ctx)
	if err != nil {
		s.Fatal("Failed to get pid: ", err)
	}
	if pid != f.pid {
		s.Fatalf("%s PID changed: want %v, got %v", serviceManagerJobName, f.pid, pid)
	}
}

func (f *serviceManagerFixture) Reset(ctx context.Context) error {
	return nil
}

func (f *serviceManagerFixture) TearDown(ctx context.Context, s *testing.FixtState) {}

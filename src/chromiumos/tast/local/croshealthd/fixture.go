// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package croshealthd

import (
	"context"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/dbusutil"
	"chromiumos/tast/local/upstart"
	"chromiumos/tast/testing"
)

// crosHealthdJobName is the name of the cros_healthd upstart job
const crosHealthdJobName = "cros_healthd"

// crosHealthdServiceName is the name of the cros_healthd D-Bus service
const crosHealthdServiceName = "org.chromium.CrosHealthd"

func init() {
	testing.AddFixture(&testing.Fixture{
		Name: "crosHealthdRunning",
		Desc: "The croshealthd daemon is available and running",
		Contacts: []string{
			"kerker@google.com",           // Fixture maintainer
			"menghuan@google.com",         // Fixture maintainer
			"cros-tdm-tpe-eng@google.com", // team mailing list
		},
		SetUpTimeout:    30 * time.Second,
		ResetTimeout:    5 * time.Second,
		PreTestTimeout:  5 * time.Second,
		PostTestTimeout: 5 * time.Second,
		TearDownTimeout: 5 * time.Second,
		Impl:            newCrosHealthdFixture(),
	})
}

// crosHealthdFixture implements testing.FixtureImpl.
type crosHealthdFixture struct {
	// pid of cros_healthd, for check if it crashed or restarted within a single test.
	pid        int
	forceReset bool
}

func newCrosHealthdFixture() testing.FixtureImpl {
	return &crosHealthdFixture{}
}

func (f *crosHealthdFixture) SetUp(ctx context.Context, s *testing.FixtState) interface{} {
	if err := f.RestartHealthdService(ctx); err != nil {
		s.Fatal("Fail to setup crosHealthdFixture: ", err)
	}
	return nil
}

func (f *crosHealthdFixture) PreTest(ctx context.Context, s *testing.FixtTestState) {
	_, _, pid, err := upstart.JobStatus(ctx, crosHealthdJobName)
	if err != nil {
		s.Fatalf("Unable to get %s PID: %s", crosHealthdJobName, err)
	}

	f.pid = pid
	f.forceReset = false
}

func (f *crosHealthdFixture) PostTest(ctx context.Context, s *testing.FixtTestState) {
	_, _, pid, err := upstart.JobStatus(ctx, crosHealthdJobName)
	if err != nil {
		s.Fatalf("Unable to get %s PID: %s", crosHealthdJobName, err)
	}

	// Make sure it has not crashed or restarted.
	if pid != f.pid {
		f.forceReset = true
		s.Fatalf("%s PID changed: want %v, got %v", crosHealthdJobName, f.pid, pid)
	}
}

func (f *crosHealthdFixture) Reset(ctx context.Context) error {
	if f.forceReset {
		f.forceReset = false

		if err := f.RestartHealthdService(ctx); err != nil {
			return errors.Wrap(err, "Fail to reset crosHealthdFixture")
		}
	}
	return nil
}

func (f *crosHealthdFixture) TearDown(ctx context.Context, s *testing.FixtState) {
	// cros_healthd should be running be default on all systems. Ensure it is
	// left running.
	if err := f.RestartHealthdService(ctx); err != nil {
		s.Fatalf("Fail to reset cros_healthd enivronment: %s", err)
	}
}

func (f *crosHealthdFixture) RestartHealthdService(ctx context.Context) error {
	// Stop "cros_healthd" to ensure "ui" is not using it.
	if err := upstart.StopJob(ctx, "cros_healthd"); err != nil {
		return errors.Wrapf(err, "unable to stop %q upstart service", crosHealthdJobName)
	}
	// Restart the "ui" job to ensure that Chrome is running. Chrome internally will wait "cros_healthd" to be up.
	if err := upstart.RestartJob(ctx, "ui"); err != nil {
		return errors.Wrap(err, "unable to ensure 'ui' upstart service is running")
	}
	// Ensure cros_healthd is up.
	if err := upstart.EnsureJobRunning(ctx, "cros_healthd"); err != nil {
		return errors.Wrap(err, "failed to start cros_healthd")
	}
	// It is possible that cros_healthd actually crashes but the upstart job is regarded as running.
	// Wait for the D-Bus service to be available to detect this case.
	bus, err := dbusutil.SystemBus()
	if err != nil {
		return errors.Wrap(err, "failed to connect to the D-Bus system bus")
	}
	if err := dbusutil.WaitForService(ctx, bus, crosHealthdServiceName); err != nil {
		return errors.Wrapf(err, "failed to wait for %q D-Bus service", crosHealthdServiceName)
	}
	// Wait until the Mojo bootstrap flow is done.
	if err := waitForMojoBootstrap(ctx); err != nil {
		return errors.Wrap(err, "unable to wait for mojo bootstrap")
	}
	return nil
}

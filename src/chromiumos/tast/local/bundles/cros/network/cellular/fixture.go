// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package cellular

import (
	"context"
	"os"
	"time"

	"chromiumos/tast/common/shillconst"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/shill"
	"chromiumos/tast/local/upstart"
	"chromiumos/tast/testing"
)

// The Cellular test fixture ensures that modemfwd is stopped.
const resetShillTimeout = 30 * time.Second

func init() {
	testing.AddFixture(&testing.Fixture{
		Name: "cellular",
		Desc: "Cellular tests are safe to run",
		Contacts: []string{
			"stevenjb@google.com",
			"chromeos-cellular-team@google.com",
		},
		SetUpTimeout:    5 * time.Second,
		ResetTimeout:    5 * time.Second,
		PreTestTimeout:  1 * time.Second,
		PostTestTimeout: 1 * time.Second,
		TearDownTimeout: 5 * time.Second,
		Impl:            newCellularFixture(),
	})
}

// cellularFixture implements testing.FixtureImpl.
type cellularFixture struct {
	modemfwdStopped bool
}

func newCellularFixture() testing.FixtureImpl {
	return &cellularFixture{modemfwdStopped: false}
}

const modemfwdJobName = "modemfwd"
const shillJobName = "shill"
const shillLogLevel = "-4"
const shillLogScope = "cellular+modem+connection+profile+portal"
const modemmanagerJobName = "modemmanager"
const modemmanagerLogLevel = "DEBUG"

func resetShill(ctx context.Context) []error {
	var errs []error
	if err := upstart.StopJob(ctx, shillJobName); err != nil {
		errs = append(errs, errors.Wrap(err, "failed to stop shill"))
	}
	if err := os.Remove(shillconst.DefaultProfilePath); err != nil && !os.IsNotExist(err) {
		errs = append(errs, errors.Wrap(err, "failed to remove default profile"))
	}
	if err := upstart.RestartJob(ctx, shillJobName, upstart.WithArg("SHILL_LOG_LEVEL", shillLogLevel), upstart.WithArg("SHILL_LOG_SCOPES", shillLogScope)); err != nil {
		// No more can be done if shill doesn't start
		return append(errs, errors.Wrap(err, "failed to restart shill"))
	}
	manager, err := shill.NewManager(ctx)
	if err != nil {
		// No more can be done if a manger interface cannot be created
		return append(errs, errors.Wrap(err, "failed to create new shill manager"))
	}
	if err = manager.PopAllUserProfiles(ctx); err != nil {
		errs = append(errs, errors.Wrap(err, "failed to pop all user profiles"))
	}

	// Wait until a service is connected.
	expectProps := map[string]interface{}{
		shillconst.ServicePropertyIsConnected: true,
	}
	if _, err := manager.WaitForServiceProperties(ctx, expectProps, resetShillTimeout); err != nil {
		errs = append(errs, errors.Wrap(err, "failed to wait for connected service"))
	}

	return errs
}

func (f *cellularFixture) SetUp(ctx context.Context, s *testing.FixtState) interface{} {
	stopped, err := stopJob(ctx, modemfwdJobName)
	if err != nil {
		s.Fatalf("Failed to stop job: %q, %s", modemfwdJobName, err)
	}
	if stopped {
		s.Logf("Stopped %q", modemfwdJobName)
	} else {
		s.Logf("%q not running", modemfwdJobName)
	}
	f.modemfwdStopped = stopped

	if err := upstart.StopJob(ctx, modemmanagerJobName); err != nil {
		s.Fatalf("Failed to stop job: %q, %s", modemmanagerJobName, err)
	}

	if errs := resetShill(ctx); len(errs) != 0 {
		for _, err := range errs {
			s.Error("resetShill error: ", err)
		}
		s.Fatal("Failed resetting shill in PreTest")
	}

	if err := upstart.StartJob(ctx, modemmanagerJobName, upstart.WithArg("MM_LOGLEVEL", modemmanagerLogLevel)); err != nil {
		s.Fatalf("Failed to start job: %q, %s", "modemmanager", err)
	}
	return nil
}

func (f *cellularFixture) Reset(ctx context.Context) error { return nil }

func (f *cellularFixture) PreTest(ctx context.Context, s *testing.FixtTestState) {}

func (f *cellularFixture) PostTest(ctx context.Context, s *testing.FixtTestState) {}

func (f *cellularFixture) TearDown(ctx context.Context, s *testing.FixtState) {
	if f.modemfwdStopped {
		err := upstart.StartJob(ctx, modemfwdJobName)
		if err != nil {
			s.Fatalf("Failed to start %q: %s", modemfwdJobName, err)
		}
		s.Logf("Started %q", modemfwdJobName)
	}
}

func stopJob(ctx context.Context, job string) (bool, error) {
	if !upstart.JobExists(ctx, job) {
		return false, nil
	}
	_, _, pid, err := upstart.JobStatus(ctx, job)
	if err != nil {
		return false, errors.Wrapf(err, "failed to run upstart.JobStatus for %q", job)
	}
	if pid == 0 {
		return false, nil
	}
	err = upstart.StopJob(ctx, job)
	if err != nil {
		return false, errors.Wrapf(err, "failed to stop %q", job)
	}
	return true, nil

}

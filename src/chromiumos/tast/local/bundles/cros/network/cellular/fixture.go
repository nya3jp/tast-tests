// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package cellular

import (
	"context"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/upstart"
	"chromiumos/tast/testing"
)

// The Cellular test fixture ensures that modemfwd is stopped.

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
	hermesStopped   bool
}

func newCellularFixture() testing.FixtureImpl {
	return &cellularFixture{modemfwdStopped: false}
}

const modemfwdJobName = "modemfwd"
const hermesJobName = "hermes"

func (f *cellularFixture) SetUp(ctx context.Context, s *testing.FixtState) interface{} {
	modemfwdStopped, err := stopJob(ctx, modemfwdJobName)
	if err != nil {
		s.Fatalf("Failed to stop job: %q, %s", modemfwdJobName, err)
	}
	if modemfwdStopped {
		s.Logf("Stopped %q", modemfwdJobName)
	} else {
		s.Logf("%q not running", modemfwdJobName)
	}
	f.modemfwdStopped = modemfwdStopped

	hermesStopped, err := stopJob(ctx, hermesJobName)
	if err != nil {
		s.Fatalf("Failed to stop job: %q, %s", hermesJobName, err)
	}

	if hermesStopped {
		s.Logf("Stopped %q", hermesJobName)
	} else {
		s.Logf("%q not running", hermesJobName)
	}
	f.hermesStopped = hermesStopped

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

	if f.hermesStopped {
		err := upstart.StartJob(ctx, hermesJobName)
		if err != nil {
			s.Fatalf("Failed to start %q: %s", hermesJobName, err)
		}
		s.Logf("Started %q", hermesJobName)
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

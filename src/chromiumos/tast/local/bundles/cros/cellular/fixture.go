// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package cellular

import (
	"context"
	"io/ioutil"
	"strconv"
	"strings"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/hermes"
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
		SetUpTimeout:    3 * time.Minute,
		ResetTimeout:    5 * time.Second,
		PreTestTimeout:  1 * time.Second,
		PostTestTimeout: 1 * time.Second,
		TearDownTimeout: 5 * time.Second,
		Impl:            NewCellularFixture(),
	})
}

// cellularFixture implements testing.FixtureImpl.
type cellularFixture struct {
	modemfwdStopped bool
}

func NewCellularFixture() *cellularFixture {
	return &cellularFixture{modemfwdStopped: false}
}

const modemfwdJobName = "modemfwd"
const hermesJobName = "hermes"

func (f *cellularFixture) SetUp(ctx context.Context, s *testing.FixtState) interface{} {

	// Give some time for cellular daemons to perform any modem operations. Stopping them via upstart might leave the modem in a bad state.
	ensureUptime(ctx, s, 2*time.Minute)

	var err error
	if f.modemfwdStopped, err = stopJob(ctx, modemfwdJobName); err != nil {
		s.Fatalf("Failed to stop job: %q, %s", modemfwdJobName, err)
	}
	if f.modemfwdStopped {
		s.Logf("Stopped %q", modemfwdJobName)
	} else {
		s.Logf("%q not running", modemfwdJobName)
	}
	if !upstart.JobExists(ctx, hermesJobName) {
		return nil
	}
	// Hermes is usually idle 2 minutes after boot, so go on with the test even if we cannot be sure.
	if err := hermes.WaitForHermesIdle(ctx, 30*time.Second); err != nil {
		s.Logf("Could not confirm if Hermes is idle: %s", err)
	}

	return nil
}

func (f *cellularFixture) Reset(ctx context.Context) error { return nil }

func (f *cellularFixture) PreTest(ctx context.Context, s *testing.FixtTestState) {}

func (f *cellularFixture) PostTest(ctx context.Context, s *testing.FixtTestState) {}

func (f *cellularFixture) TearDown(ctx context.Context, s *testing.FixtState) {
	if f.modemfwdStopped {
		err := upstart.EnsureJobRunning(ctx, modemfwdJobName)
		if err != nil {
			s.Fatalf("Failed to start %q: %s", modemfwdJobName, err)
		}
		s.Logf("Started %q", modemfwdJobName)
	}
}

func ensureUptime(ctx context.Context, s *testing.FixtState, duration time.Duration) {
	uptimeStr, err := ioutil.ReadFile("/proc/uptime")
	if err != nil {
		s.Errorf("Failed to read system uptime: %s", err)
	}
	uptimeFloat, err := strconv.ParseFloat(strings.Fields(string(uptimeStr))[0], 64)
	if err != nil {
		s.Errorf("Failed to parse system uptime %s : %s", string(uptimeStr), err)
	}
	uptime := time.Duration(uptimeFloat) * time.Second
	if uptime < duration {
		s.Logf("Waiting for %s uptime before starting test, current uptime:  %s", duration, uptime)
		testing.Sleep(ctx, duration-uptime)
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

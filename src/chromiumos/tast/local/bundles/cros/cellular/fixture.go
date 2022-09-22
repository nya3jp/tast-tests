// Copyright 2021 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package cellular

import (
	"context"
	"io/ioutil"
	"strconv"
	"strings"
	"time"

	"chromiumos/tast/common/fixture"
	"chromiumos/tast/common/policy/fakedms"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/cellular"
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
		Impl:            &cellularFixture{modemfwdStopped: false, connectService: false},
	})
	testing.AddFixture(&testing.Fixture{
		Name: "cellularWithFakeDMSEnrolled",
		Desc: "Cellular tests are safe to run and a fake DMS (for managed eSIM profiles) is running",
		Contacts: []string{
			"jiajunzhang@google.com",
			"chromeos-cellular-team@google.com",
		},
		SetUpTimeout:    3 * time.Minute,
		ResetTimeout:    5 * time.Second,
		PreTestTimeout:  1 * time.Second,
		PostTestTimeout: 1 * time.Second,
		TearDownTimeout: 5 * time.Second,
		Impl:            &cellularFixture{modemfwdStopped: false, useFakeDMS: true},
		Parent:          fixture.FakeDMSEnrolled,
	})
	testing.AddFixture(&testing.Fixture{
		Name: "cellularConnected",
		Desc: "Cellular tests are safe to run after cellular is connected and verified",
		Contacts: []string{
			"nmarupaka@google.com",
			"chromeos-cellular-team@google.com",
		},
		SetUpTimeout:    3 * time.Minute,
		ResetTimeout:    5 * time.Second,
		PreTestTimeout:  1 * time.Second,
		PostTestTimeout: 1 * time.Second,
		TearDownTimeout: 5 * time.Second,
		Impl:            &cellularFixture{modemfwdStopped: false, connectService: true},
		Parent:          "cellular",
	})
}

// cellularFixture implements testing.FixtureImpl.
type cellularFixture struct {
	modemfwdStopped bool
	useFakeDMS      bool
	connectService  bool
}

// FixtData holds information made available to tests that specify this fixture.
type FixtData struct {
	fdms *fakedms.FakeDMS
}

// FakeDMS implements the HasFakeDMS interface.
func (fd FixtData) FakeDMS() *fakedms.FakeDMS {
	if fd.fdms == nil {
		panic("FakeDMS is called with nil fakeDMS instance")
	}
	return fd.fdms
}

const modemfwdJobName = "modemfwd"
const hermesJobName = "hermes"

func (f *cellularFixture) SetUp(ctx context.Context, s *testing.FixtState) interface{} {
	var fdms *fakedms.FakeDMS
	if f.useFakeDMS {
		var ok bool
		fdms, ok = s.ParentValue().(*fakedms.FakeDMS)
		if !ok {
			s.Fatal("Parent is not a fakeDMSEnrolled fixture")
		}
	}

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
		return &FixtData{fdms}
	}
	// Hermes is usually idle 2 minutes after boot, so go on with the test even if we cannot be sure.
	if err := hermes.WaitForHermesIdle(ctx, 30*time.Second); err != nil {
		s.Logf("Could not confirm if Hermes is idle: %s", err)
	}

	if f.connectService {
		helper, err := cellular.NewHelper(ctx)
		if err != nil {
			s.Fatal("Failed to create cellular.Helper: ", err)
		}
		// Ensure cellular is connected and has sufficient coverage.
		if err := helper.ConnectAndCheck(ctx); err != nil {
			s.Fatal("Failed to setup well connected cellular service: ", err)
		}
	}
	return &FixtData{fdms}
}

func (f *cellularFixture) Reset(ctx context.Context) error { return nil }

func (f *cellularFixture) PreTest(ctx context.Context, s *testing.FixtTestState) {}

func (f *cellularFixture) PostTest(ctx context.Context, s *testing.FixtTestState) {}

func (f *cellularFixture) TearDown(ctx context.Context, s *testing.FixtState) {
	if f.modemfwdStopped {
		err := upstart.EnsureJobRunning(ctx, modemfwdJobName, upstart.WithArg("DEBUG_MODE", "true"))
		if err != nil {
			s.Fatalf("Failed to start %q: %s", modemfwdJobName, err)
		}
		s.Logf("Started %q", modemfwdJobName)
	}
	if f.connectService {
		helper, err := cellular.NewHelper(ctx)
		if err != nil {
			s.Fatal("Failed to create cellular.Helper: ", err)
		}
		// Ensure cellular is disconnected.
		if err := helper.Disconnect(ctx); err != nil {
			s.Fatal("Failed to disconnect cellular service: ", err)
		}
	}
}

func ensureUptime(ctx context.Context, s *testing.FixtState, duration time.Duration) {
	uptimeStr, err := ioutil.ReadFile("/proc/uptime")
	if err != nil {
		s.Errorf("Failed to read system uptime: %s", err)
	}
	uptimeFloat, err := strconv.ParseFloat(strings.Fields(string(uptimeStr))[0], 64)
	if err != nil {
		s.Errorf("Failed to parse system uptime %q : %s", string(uptimeStr), err)
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

// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package cellular

import (
	"context"
	"time"

	"chromiumos/tast/common/fixture"
	"chromiumos/tast/common/policy/fakedms"
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
		Impl:            &cellularFixture{modemfwdStopped: false},
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
}

// cellularFixture implements testing.FixtureImpl.
type cellularFixture struct {
	modemfwdStopped bool
	useFakeDMS      bool
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
	if err := cellular.EnsureUptime(ctx, 2*time.Minute); err != nil {
		s.Fatal("Failed waiting for uptime")
	}

	var err error
	if f.modemfwdStopped, err = cellular.StopModemfwd(ctx); err != nil {
		s.Fatalf("Failed to stop job: %q, %s", cellular.ModemfwdJobName, err)
	}
	if f.modemfwdStopped {
		s.Logf("Stopped %q", cellular.ModemfwdJobName)
	} else {
		s.Logf("%q not running", cellular.ModemfwdJobName)
	}
	if !upstart.JobExists(ctx, hermesJobName) {
		return &FixtData{fdms}
	}
	// Hermes is usually idle 2 minutes after boot, so go on with the test even if we cannot be sure.
	if err := hermes.WaitForHermesIdle(ctx, 30*time.Second); err != nil {
		s.Logf("Could not confirm if Hermes is idle: %s", err)
	}

	return &FixtData{fdms}
}

func (f *cellularFixture) Reset(ctx context.Context) error { return nil }

func (f *cellularFixture) PreTest(ctx context.Context, s *testing.FixtTestState) {}

func (f *cellularFixture) PostTest(ctx context.Context, s *testing.FixtTestState) {}

func (f *cellularFixture) TearDown(ctx context.Context, s *testing.FixtState) {
	if f.modemfwdStopped {
		if err := cellular.StartModemfwd(ctx, true); err != nil {
			s.Fatalf("Failed to start %q: %s", cellular.ModemfwdJobName, err)
		}
		s.Logf("Started %q", cellular.ModemfwdJobName)
	}
}

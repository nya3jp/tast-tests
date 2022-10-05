// Copyright 2021 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package cellular

import (
	"context"
	"os"
	"path/filepath"
	"time"

	"chromiumos/tast/common/fixture"
	"chromiumos/tast/common/policy/fakedms"
	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/cellular"
	"chromiumos/tast/local/hermes"
	"chromiumos/tast/local/modemmanager"
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
		PreTestTimeout:  3 * time.Second,
		PostTestTimeout: 3 * time.Minute,
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
		PreTestTimeout:  3 * time.Second,
		PostTestTimeout: 3 * time.Minute,
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
const modemfwdJobName = "modemfwd"
const modemManagerJobName = "modemmanager"
const shillJobName = "shill"

const uptimeBeforeTest = 2 * time.Minute

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
	if err := cellular.EnsureUptime(ctx, uptimeBeforeTest); err != nil {
		s.Fatal("Failed to wait for system uptime: ", err)
	}

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

	return &FixtData{fdms}
}

func (f *cellularFixture) Reset(ctx context.Context) error { return nil }

func (f *cellularFixture) PreTest(ctx context.Context, s *testing.FixtTestState) {
	// Run modem status before starting the test
	outDir, ok := testing.ContextOutDir(ctx)
	if !ok {
		testing.ContextLog(ctx, "Failed to get out dir")
		return
	}

	outFile, err := os.Create(filepath.Join(outDir, "modem-status.txt"))
	if err != nil || outFile == nil {
		return
	}

	cmd := testexec.CommandContext(ctx, "modem", "status")
	cmd.Stdout = outFile
	cmd.Stderr = outFile

	if err := cmd.Run(); err != nil {
		testing.ContextLog(ctx, "Failed to start command: ", err)
	}
	if err := outFile.Close(); err != nil {
		testing.ContextLog(ctx, "Failed to start command: ", err)
	}
}

func (f *cellularFixture) PostTest(ctx context.Context, s *testing.FixtTestState) {
	if s.HasError() {
		testing.ContextLog(ctx, "Fixture detected a test failure, restarting MM and Shill")
		// stop and start jobs instead of upstart.Restart to emulate a reboot.
		if _, err := stopJob(ctx, shillJobName); err != nil {
			testing.ContextLogf(ctx, "Failed to stop job: %q, %s", shillJobName, err)
		}
		if _, err := stopJob(ctx, modemManagerJobName); err != nil {
			testing.ContextLogf(ctx, "Failed to stop job: %q, %s", modemManagerJobName, err)
		}
		if err := upstart.StartJob(ctx, shillJobName); err != nil {
			testing.ContextLogf(ctx, "Failed to restart job: %q, %s", shillJobName, err)
		}
		if err := upstart.StartJob(ctx, modemManagerJobName); err != nil {
			testing.ContextLogf(ctx, "Failed to restart job: %q, %s", modemManagerJobName, err)
		}
		if _, err := modemmanager.NewModem(ctx); err != nil {
			testing.ContextLog(ctx, "Could not find MM dbus object after restarting ModemManager: ", err)
		}
		// Delay starting the next test to avoid any transients caused by restarting MM and shill.
		testing.Sleep(ctx, uptimeBeforeTest)
	}
}

func (f *cellularFixture) TearDown(ctx context.Context, s *testing.FixtState) {
	if f.modemfwdStopped {
		err := upstart.EnsureJobRunning(ctx, modemfwdJobName, upstart.WithArg("DEBUG_MODE", "true"))
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

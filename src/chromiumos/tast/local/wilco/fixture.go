// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wilco

import (
	"context"
	"time"

	"chromiumos/tast/local/policyutil/fixtures"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddFixture(&testing.Fixture{
		Name: "wilcoDTCEnrolled",
		Desc: "Wilco DTC enrollment fixture with running Wilco DTC VM and Support Daemon",
		Contacts: []string{
			"bisakhmondal00@gmail.com", // test author
			"lamzin@google.com",        // wilco_dtc_supportd maintainer
			"chromeos-wilco@google.com",
		},
		Impl:            &wilcoEnrolledFixture{},
		Parent:          "chromeEnrolledLoggedIn",
		SetUpTimeout:    15 * time.Second,
		PreTestTimeout:  10 * time.Second,
		PostTestTimeout: 10 * time.Second,
		TearDownTimeout: 15 * time.Second,
	})
}

// wilcoEnrolledFixture implements testing.FixtureImpl.
type wilcoEnrolledFixture struct {
	wilcoDTCVMPID       int
	wilcoDTCSupportdPID int
}

func (w *wilcoEnrolledFixture) SetUp(ctx context.Context, s *testing.FixtState) interface{} {
	enrolledFixtData, ok := s.ParentValue().(*fixtures.FixtData)
	if !ok {
		s.Fatal("Failed to access chromeEnrolledLoggedIn parent fixture")
	}
	// Starting wilco DTC vm.
	if err := StartVM(ctx, &VMConfig{
		StartProcesses: false,
		TestDBusConfig: false,
	}); err != nil {
		s.Fatal("Failed to start the Wilco DTC VM: ", err)
	}

	pidVM, err := VMPID(ctx)
	if err != nil {
		s.Fatal("Failed to get Wilco DTC VM PID: ", err)
	}

	w.wilcoDTCVMPID = pidVM

	// Starting wilco DTC support daemon.
	if err := StartSupportd(ctx); err != nil {
		s.Fatal("Failed to start the Wilco DTC Support Daemon: ", err)
	}

	pidD, err := SupportdPID(ctx)
	if err != nil {
		s.Fatal("Failed to get Wilco DTC Support Daemon PID: ", err)
	}

	w.wilcoDTCSupportdPID = pidD
	return enrolledFixtData
}

func (w *wilcoEnrolledFixture) PreTest(ctx context.Context, s *testing.FixtTestState) {
	// Restart the Wilco DTC Daemon to flush the queued events.
	if err := StartSupportd(ctx); err != nil {
		s.Fatal("Failed to restart the Wilco DTC Support Daemon: ", err)
	}

	pid, err := SupportdPID(ctx)
	if err != nil {
		s.Fatal("Failed to get Wilco DTC Support Daemon PID: ", err)
	}

	w.wilcoDTCSupportdPID = pid
}

func (w *wilcoEnrolledFixture) PostTest(ctx context.Context, s *testing.FixtTestState) {
	// Ensures that test doesn't interfere with the Wilco DTC VM and Daemon.
	pid, err := VMPID(ctx)
	if err != nil {
		s.Fatal("Failed to get Wilco DTC VM PID: ", err)
	}

	if w.wilcoDTCVMPID != pid {
		s.Error("The Wilco DTC VM PID changed while testing")
	}

	pid, err = SupportdPID(ctx)
	if err != nil {
		s.Fatal("Failed to get Wilco DTC Support Daemon PID: ", err)
	}

	if w.wilcoDTCSupportdPID != pid {
		s.Error("The Wilco DTC Support Daemon PID changed while testing")
	}
}

func (w *wilcoEnrolledFixture) TearDown(ctx context.Context, s *testing.FixtState) {
	// Stopping the the Wilco DTC VM and Daemon.
	if err := StopSupportd(ctx); err != nil {
		s.Error("Failed to stop the Wilco DTC Support Daemon: ", err)
	}

	if err := StopVM(ctx); err != nil {
		s.Error("Failed to stop the Wilco DTC VM: ", err)
	}
}

func (w *wilcoEnrolledFixture) Reset(ctx context.Context) error {
	return nil
}

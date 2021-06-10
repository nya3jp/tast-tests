// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package servo

import (
	"context"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddFixture(&testing.Fixture{
		Name:            "servo",
		Desc:            "Ensure that servo is available and working",
		Impl:            &servoFixture{},
		SetUpTimeout:    30 * time.Second,
		ResetTimeout:    30 * time.Second,
		PostTestTimeout: 30 * time.Second,
		TearDownTimeout: 30 * time.Second,
		Vars:            []string{"servo"},
	})
}

type servoFixture struct {
	pxy *Proxy
}

// SetUp is called by the framework to set up the environment with possibly heavy-weight
// operations.
func (f *servoFixture) SetUp(ctx context.Context, s *testing.FixtState) interface{} {
	testing.ContextLog(ctx, "SetUp")
	// s.Fatal("SETUP Error")
	dut := s.DUT()

	// This is expected to fail in VMs, since Servo is unusable there and the "servo" var won't
	// be supplied. https://crbug.com/967901 tracks finding a way to skip tests when needed.
	servoSpec, _ := s.Var("servo")
	pxy, err := NewProxy(s.FixtContext(), servoSpec, dut.KeyFile(), dut.KeyDir())
	if err != nil {
		// This error would propagate as a "[Fixture failure]" for tests
		// that dependend on this failed fixture instance.
		s.Fatal("Failed to connect to servo: ", err)
	}
	f.pxy = pxy
	return pxy
}

// Reset is called by the framework after each test (except for the last one) to do a
// light-weight reset of the environment to the original state.
//
// An error here will trigger the fixture to TearDown and SetUp again.
// We use this routine to check the servo connection.
func (f *servoFixture) Reset(ctx context.Context) error {
	testing.ContextLog(ctx, "Reset")
	// return f.pxy.Servo().Restore(ctx)
	return errors.New("RESSSSSSSSSSSSSSSSETT error")
	// return f.pxy.Servo().VerifyConnectivity(ctx)
	// return nil
}

// PreTest is called by the framework before each test to do a light-weight set up for the test.
//
// An s.Fatal error here will not stop the test from running, but will be noted on the test.
func (f *servoFixture) PreTest(ctx context.Context, s *testing.FixtTestState) {
	testing.ContextLog(ctx, "PreTest")
	// testing.ContextLog(ctx, "TestName: ")
	// s.Fatal("PreTest failureeeeeeeeeeeeee")
	if err := f.pxy.Servo().VerifyConnectivity(ctx, ""); err != nil {
		s.Fatal("Servo failed check: ", err)
	}
}

// PostTest is called by the framework after each test to tear down changes PreTest made.
//
// An s.Fatal error here will not restart the fixture, but will be noted on the test.
func (f *servoFixture) PostTest(ctx context.Context, s *testing.FixtTestState) {
	testing.ContextLog(ctx, "PostTest")
	s.Fatal("PostTest failureeeeeeeeeeeeeeeee")
	// if err := f.pxy.Servo().Restore(ctx); err != nil {
	// 	s.Fatal("Servo failed to restore: ", err)
	// }
	// We could attribute a servo failure to the last running test here.
	// if err := f.pxy.Servo().VerifyConnectivity(ctx, ""); err != nil {
	// 	s.Fatal("Servo failed check: ", err)
	// }
}

// TearDown is called by the framework to tear down the environment SetUp set up.
func (f *servoFixture) TearDown(ctx context.Context, s *testing.FixtState) {
	testing.ContextLog(ctx, "TearDown")
	// testing.ContextLog(ctx, "ctx error ==> ", ctx.Err().Error())
	f.pxy.Close(s.FixtContext())
	s.Fatal("FAIL ON EXIT")
}

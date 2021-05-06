// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package servo

import (
	"context"
	"time"

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

func (f *servoFixture) SetUp(ctx context.Context, s *testing.FixtState) interface{} {
	dut := s.DUT()

	// This is expected to fail in VMs, since Servo is unusable there and the "servo" var won't
	// be supplied. https://crbug.com/967901 tracks finding a way to skip tests when needed.
	pxy, err := NewProxy(s.FixtContext(), s.RequiredVar("servo"), dut.KeyFile(), dut.KeyDir())
	if err != nil {
		s.Fatal("Failed to connect to servo: ", err)
	}
	f.pxy = pxy
	return pxy
}

func (f *servoFixture) Reset(ctx context.Context) error {
	return f.pxy.Servo().VerifyConnectivity(ctx)
}

func (f *servoFixture) PreTest(ctx context.Context, s *testing.FixtTestState) {}

func (f *servoFixture) PostTest(ctx context.Context, s *testing.FixtTestState) {
	// We could attribute a servo failure to the last running test here.
	if err := f.pxy.Servo().VerifyConnectivity(ctx); err != nil {
		s.Fatal("Servo failed check: ", err)
	}
}

func (f *servoFixture) TearDown(ctx context.Context, s *testing.FixtState) {
	f.pxy.Close(s.FixtContext())
}

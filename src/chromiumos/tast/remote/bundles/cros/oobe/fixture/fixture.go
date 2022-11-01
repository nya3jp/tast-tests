// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package fixture contains fixtures oobe tests use.
package fixture

import (
	"context"
	"time"

	"chromiumos/tast/common/servo"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddFixture(&testing.Fixture{
		Name:            "turnOffServo",
		Desc:            "Fixture for turning of Servo in Chrome devices",
		Contacts:        []string{"tjohnsonkanu@chromium.org", "cros-connectivity@google.com"},
		Impl:            &fixture{},
		Vars:            []string{"servo"},
		SetUpTimeout:    10 * time.Second,
		TearDownTimeout: 10 * time.Second,
		PreTestTimeout:  10 * time.Second,
		PostTestTimeout: 10 * time.Second,
	})
}

type fixture struct{}

func (*fixture) SetUp(ctx context.Context, s *testing.FixtState) interface{} {
	s.Log("SetUp TurnOffServo")

	dut := s.DUT()
	pxy, err := servo.NewProxy(ctx, s.RequiredVar("servo"), dut.KeyFile(), dut.KeyDir())
	if err != nil {
		s.Fatal("Failed to connect to servo: ", err)
	}
	defer pxy.Close(ctx)

	if err := pxy.Servo().SetOnOff(ctx, servo.USBKeyboard, servo.Off); err != nil {
		s.Fatal("Failed to turn of servo: ", err)
	}
	return nil
}

func (*fixture) TearDown(ctx context.Context, s *testing.FixtState) {
	s.Log("TearDown TurnOffServo")

	dut := s.DUT()
	pxy, err := servo.NewProxy(ctx, s.RequiredVar("servo"), dut.KeyFile(), dut.KeyDir())
	if err != nil {
		s.Fatal("Failed to connect to servo: ", err)
	}
	defer pxy.Close(ctx)

	if err := pxy.Servo().SetOnOff(ctx, servo.USBKeyboard, servo.On); err != nil {
		s.Fatal("Failed to turn on servo: ", err)
	}
}

func (*fixture) Reset(ctx context.Context) error                        { return nil }
func (*fixture) PreTest(ctx context.Context, s *testing.FixtTestState)  {}
func (*fixture) PostTest(ctx context.Context, s *testing.FixtTestState) {}

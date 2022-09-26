// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package setup

import (
	"context"
	"time"

	"chromiumos/tast/testing"
)

func init() {
	testing.AddFixture(&testing.Fixture{
		Name: "powerSetUp",
		Desc: "Set up DUT for power measurements",
		Contacts: []string{
			"jakebarnes@google.com",
			"chromeos-platform-ml@google.com",
		},
		Impl:            &powerSetUpFixture{},
		SetUpTimeout:    time.Minute,
		TearDownTimeout: time.Minute,
	})
}

type powerSetUpFixture struct {
	cleanup func(context.Context) error
}

func (f *powerSetUpFixture) SetUp(ctx context.Context, s *testing.FixtState) interface{} {
	sup, cleanup := New("powerSetUpFixture")

	// Stop UI in order to minimize the number of factors that could influence the results.
	sup.Add(DisableService(ctx, "ui"))

	sup.Add(PowerTest(ctx, nil, PowerTestOptions{
		Wifi: DisableWifiInterfaces,
		// Since we stop the UI disabling the Night Light is redundant.
		NightLight: DoNotDisableNightLight,
	}, nil))

	if err := sup.Check(ctx); err != nil {
		s.Fatal("Power setup failed: ", err)
	}

	f.cleanup = cleanup

	return nil
}

func (f *powerSetUpFixture) TearDown(ctx context.Context, s *testing.FixtState) {
	if err := f.cleanup(ctx); err != nil {
		testing.ContextLog(ctx, "Power cleanup failed: ", err)
	}
}

func (f *powerSetUpFixture) PreTest(ctx context.Context, s *testing.FixtTestState) {
}

func (f *powerSetUpFixture) PostTest(ctx context.Context, s *testing.FixtTestState) {
}

func (f *powerSetUpFixture) Reset(ctx context.Context) error {
	return nil
}

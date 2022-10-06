// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package cellular

import (
	"context"
	"time"

	"chromiumos/tast/testing"
)

// The Cellular test fixture ensures that modemfwd is stopped.

func init() {
	testing.AddFixture(&testing.Fixture{
		Name: "cellularRemote",
		Desc: "Cellular tests are safe to run",
		Contacts: []string{
			"pholla@google.com",
			"chromeos-cellular-team@google.com",
		},
		SetUpTimeout:    3 * time.Minute,
		ResetTimeout:    5 * time.Second,
		PreTestTimeout:  3 * time.Minute,
		PostTestTimeout: 3 * time.Minute,
		TearDownTimeout: 5 * time.Second,
		Impl:            &cellularRemoteFixture{},
	})
}

type cellularRemoteFixture struct{
}

func (f *cellularRemoteFixture) SetUp(ctx context.Context, s *testing.FixtState) interface{} {
	return nil
}

func (f *cellularRemoteFixture) Reset(ctx context.Context) error { return nil }

func (f *cellularRemoteFixture) PreTest(ctx context.Context, s *testing.FixtTestState) {
	testing.ContextLog(ctx, "testname ", s.TestName())
	if s.TestName() == "cellular.Smoke" {
    if err := s.DUT().Reboot(ctx); err!=nil {
		  testing.ContextLog(ctx, "Failed to reboot DUT")
    }
	}
}

func (f *cellularRemoteFixture) PostTest(ctx context.Context, s *testing.FixtTestState) {
}

func (f *cellularRemoteFixture) TearDown(ctx context.Context, s *testing.FixtState) {
}

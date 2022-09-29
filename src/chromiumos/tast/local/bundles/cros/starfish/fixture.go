// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package starfish

import (
	"context"
	"time"

	"chromiumos/tast/local/starfish"
	"chromiumos/tast/testing"
)

// The starfish test fixture ensures that the correct SIM slot is configured for the test.

func init() {
	testing.AddFixture(&testing.Fixture{
		Name: "starfish",
		Desc: "Allows preconfiguration of the Starfish module before running a test suite",
		Contacts: []string{
			"nmarupaka@google.com",
			"chromeos-cellular-team@google.com",
		},
		SetUpTimeout:    1 * time.Minute,
		ResetTimeout:    1 * time.Second,
		PreTestTimeout:  1 * time.Second,
		PostTestTimeout: 1 * time.Minute,
		TearDownTimeout: 1 * time.Second,
		Impl:            &starfishFixture{},
	})
}

// starfishFixture implements testing.FixtureImpl.
type starfishFixture struct {
	sf starfish.Starfish
}

// FixtData holds information made available to tests that specify this fixture.
type FixtData struct {
}

func (f *starfishFixture) SetUp(ctx context.Context, s *testing.FixtState) interface{} {
	if err := f.sf.Setup(ctx); err != nil {
		s.Fatalf("Failed to setup starfish: %s", err)
	}
	return &FixtData{}
}

func (f *starfishFixture) Reset(ctx context.Context) error {
	return nil
}

func (f *starfishFixture) PreTest(ctx context.Context, s *testing.FixtTestState) {
}

func (f *starfishFixture) PostTest(ctx context.Context, s *testing.FixtTestState) {
}

func (f *starfishFixture) TearDown(ctx context.Context, s *testing.FixtState) {
	if err := f.sf.Teardown(ctx); err != nil {
		s.Fatalf("Failed to teardown starfish: %s", err)
	}
}

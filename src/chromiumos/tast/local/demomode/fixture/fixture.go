// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package fixture

import (
	"context"
	"time"

	"chromiumos/tast/local/hwsec"
	"chromiumos/tast/testing"
)

const (
	// TPMReset is name for the fixture to reset TPM during SetUp and TearDown.
	TPMReset = "tpmReset"

	setUpTimeout    = 15 * time.Second
	tearDownTimeout = 25 * time.Second
)

func init() {
	testing.AddFixture(&testing.Fixture{
		Name: TPMReset,
		Desc: "Reset TPM in setup/teardown",
		Contacts: []string{
			"jacksontadie@google.com",
			"cros-demo-mode-eng@google.com",
		},
		Impl:            &fixtureImpl{},
		SetUpTimeout:    setUpTimeout,
		TearDownTimeout: tearDownTimeout,
	})
}

// fixtureImpl implements testing.FixtureImpl.
type fixtureImpl struct {
}

func (f *fixtureImpl) SetUp(ctx context.Context, s *testing.FixtState) interface{} {
	resetTPMAndSystemState(ctx, s)
	return nil
}

func (f *fixtureImpl) Reset(ctx context.Context) error {
	return nil
}

func (f *fixtureImpl) PreTest(ctx context.Context, s *testing.FixtTestState) {}

func (f *fixtureImpl) PostTest(ctx context.Context, s *testing.FixtTestState) {}

func (f *fixtureImpl) TearDown(ctx context.Context, s *testing.FixtState) {
	//resetTPMAndSystemState(ctx, s)
}

// resetTPMAndSystemState resets TPM, which can take a few seconds.
func resetTPMAndSystemState(ctx context.Context, s *testing.FixtState) {
	r := hwsec.NewCmdRunner()
	helper, err := hwsec.NewHelper(r)
	if err != nil {
		s.Fatal("Helper creation error: ", err)
	}
	s.Log("Start to reset TPM")
	if err := helper.EnsureTPMAndSystemStateAreReset(ctx); err != nil {
		s.Fatal("Failed to ensure resetting TPM: ", err)
	}
	s.Log("TPM is confirmed to be reset")
}

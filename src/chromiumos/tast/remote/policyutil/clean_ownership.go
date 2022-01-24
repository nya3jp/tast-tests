// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package policyutil

import (
	"context"
	"time"

	"chromiumos/tast/common/fixture"
	"chromiumos/tast/dut"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddFixture(&testing.Fixture{
		Name: fixture.CleanOwnership,
		Desc: "Fixture cleaning enrollment related state. Use when your local test performs REAL enrollment and proper clean up is necessary. DUT reboots before and after all tests using the fixture",
		Contacts: []string{
			"kamilszarek@google.com",
			"chromeos-commercial-remote-management@google.com"},
		Impl:            &cleanOwner{},
		SetUpTimeout:    3 * time.Minute,
		TearDownTimeout: 3 * time.Minute,
		ResetTimeout:    3 * time.Minute,
		ServiceDeps:     []string{"tast.cros.policy.PolicyService"},
	})
}

type cleanOwner struct {
	// dut holds the reference to DUT that is needed in Reset() function as
	// there is no access to the DUT.
	dut *dut.DUT
}

func (co *cleanOwner) SetUp(ctx context.Context, s *testing.FixtState) interface{} {
	if err := EnsureTPMAndSystemStateAreReset(ctx, s.DUT()); err != nil {
		s.Fatal("Failed to reset TPM: ", err)
	}
	co.dut = s.DUT()

	return nil
}

func (co *cleanOwner) TearDown(ctx context.Context, s *testing.FixtState) {
	if err := EnsureTPMAndSystemStateAreReset(ctx, s.DUT()); err != nil {
		s.Fatal("Failed to reset TPM: ", err)
	}
}

func (co *cleanOwner) Reset(ctx context.Context) error {
	// TODO: Once remote fixtures will support using Reset cleaning TPM has to
	// be called from here as well.
	return nil
}
func (*cleanOwner) PreTest(ctx context.Context, s *testing.FixtTestState)  {}
func (*cleanOwner) PostTest(ctx context.Context, s *testing.FixtTestState) {}

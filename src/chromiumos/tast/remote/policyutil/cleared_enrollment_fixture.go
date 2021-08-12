// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package policyutil

import (
	"context"
	"time"

	"chromiumos/tast/dut"
	"chromiumos/tast/errors"
	"chromiumos/tast/ssh"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddFixture(&testing.Fixture{
		Name:            "clearedEnrollment",
		Desc:            "Fixture that clears enrollment",
		Contacts:        []string{"uwyiming@google.com"},
		Impl:            &clearedEnrollmentFixt{},
		SetUpTimeout:    8 * time.Minute,
		TearDownTimeout: 5 * time.Minute,
		ResetTimeout:    15 * time.Second,
		ServiceDeps:     []string{"tast.cros.policy.PolicyService"},
	})
}

type clearedEnrollmentFixt struct {
}

func clearEnrollment(ctx context.Context, d *dut.DUT) error {
	if _, err := d.Conn().Command("/usr/sbin/update_rw_vpd", "check_enrollment", "0").Output(ctx, ssh.DumpLogOnError); err != nil {
		return errors.Wrap(err, "failed to update the VPD")
	}

	if err := EnsureTPMAndSystemStateAreReset(ctx, d); err != nil {
		return errors.Wrap(err, "failed to reset the TPM")
	}

	return nil
}

func (c *clearedEnrollmentFixt) SetUp(ctx context.Context, s *testing.FixtState) interface{} {
	if err := clearEnrollment(ctx, s.DUT()); err != nil {
		s.Fatal("Failed to clear enrollment: ", err)
	}

	return nil
}

func (c *clearedEnrollmentFixt) TearDown(ctx context.Context, s *testing.FixtState) {
	if err := clearEnrollment(ctx, s.DUT()); err != nil {
		s.Fatal("Failed to clear enrollment: ", err)
	}
}

func (*clearedEnrollmentFixt) Reset(ctx context.Context) error                        { return nil }
func (*clearedEnrollmentFixt) PreTest(ctx context.Context, s *testing.FixtTestState)  {}
func (*clearedEnrollmentFixt) PostTest(ctx context.Context, s *testing.FixtTestState) {}

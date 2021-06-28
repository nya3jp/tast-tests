// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package power is used to remotely operate power functionalities with DUTs.
package power

import (
	"context"
	"time"

	"github.com/golang/protobuf/ptypes/empty"

	"chromiumos/tast/common/servo"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/services/cros/policy"
	"chromiumos/tast/testing"
)

// EnsureBatteryWithinRange is a remote utility that takes a policy service client & a servo connection to ensure DUT battery is within provided minLevel & maxLevel.
// If force flag is set to true and there is no past enrollment, a new chrome instance will be created during battery drain (Note: this may tear the OOBE).
func EnsureBatteryWithinRange(ctx context.Context, cl policy.PolicyServiceClient, s *servo.Servo, minLevel, maxLevel float64, force bool) error {
	// Checking current status of dut battery percentage.
	status, err := cl.BatteryStatus(ctx, &empty.Empty{})
	if err != nil {
		return errors.Wrap(err, "unable to fetch battery status")
	}
	// Battery already is in provided range.
	if p := status.Percentage; p > minLevel && p < maxLevel {
		return nil
	}

	role, err := s.GetPDRole(ctx) // checking current power delivery role.
	if err != nil {
		return err
	}

	if role == servo.PDRoleNA { // servo type is not v4.
		return errors.New("requires servo type v4 for battery charge and drain through servo_pd_role")
	}

	defer func(ctx context.Context) {
		if err := s.SetPDRole(ctx, role); err != nil {
			testing.ContextLogf(ctx, "Failed to restore servo_pd_role to %s during cleanup: %v", role, err)
		}
	}(ctx)

	ctx, cancel := ctxutil.Shorten(ctx, 20*time.Second)
	defer cancel()

	if status.Percentage < minLevel { // requires charging.
		testing.ContextLog(ctx, "Battery charging has been initiated")
		if err := s.SetPDRole(ctx, servo.PDRoleSrc); err != nil {
			return errors.Wrap(err, "unable to set servo_pd_role to src")
		}
		if _, err := cl.BatteryCharge(ctx, &policy.BatteryChargeRequest{
			Percentage: minLevel,
		}); err != nil {
			return err
		}
	} else { // discharging
		if err := s.SetPDRole(ctx, servo.PDRoleSnk); err != nil {
			return errors.Wrap(err, "unable to set servo_pd_role to snk")
		}
		testing.ContextLog(ctx, "Battery discharging has been initiated")
		if _, err := cl.BatteryDrain(ctx, &policy.BatteryDrainRequest{
			Percentage: maxLevel,
			Force:      force,
		}); err != nil {
			return err
		}
	}

	return nil
}

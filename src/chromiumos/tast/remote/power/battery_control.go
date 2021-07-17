// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package power is used to remotely operate power functionalities with DUTs.
package power

import (
	"context"
	"time"

	"github.com/golang/protobuf/ptypes/empty"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/remote/servo"
	"chromiumos/tast/rpc"
	pb "chromiumos/tast/services/cros/power"
	"chromiumos/tast/testing"
)

// EnsureBatteryPercentage puts the DUT battery within minLevel and maxLevel and returns if the DUT OOBE has been teared throughout this procedure.
func EnsureBatteryPercentage(ctx context.Context, c *rpc.Client, s *servo.Servo, minLevel, maxLevel float64) (bool, error) {
	cl := pb.NewPowerServiceClient(c.Conn)
	// Checking current status of dut battery percentage.
	status, err := cl.BatteryStatus(ctx, &empty.Empty{})
	if err != nil {
		return false, errors.Wrap(err, "unable to fetch battery status")
	}
	// Battery already is in provided range.
	if p := status.Percentage; p > minLevel && p < maxLevel {
		return false, nil
	}

	role, err := s.GetPDRole(ctx) // checking current power delivery role.
	if err != nil {
		return false, err
	}

	if role == servo.PDRoleNA {
		return false, errors.New("requires servo type v4 for battery charge and drain through servo_pd_role")
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
			return false, errors.Wrap(err, "unable to set servo_pd_role to src")
		}
		if _, err := cl.BatteryCharge(ctx, &pb.BatteryChargeRequest{
			Percentage: minLevel,
		}); err != nil {
			return false, err
		}
	} else { // discharging
		if err := s.SetPDRole(ctx, servo.PDRoleSnk); err != nil {
			return false, errors.Wrap(err, "unable to set servo_pd_role to snk")
		}
		testing.ContextLog(ctx, "Battery discharging has been initiated")
		if _, err := cl.BatteryDrain(ctx, &pb.BatteryDrainRequest{
			Percentage: maxLevel,
		}); err != nil {
			return true, err
		}
	}

	return status.Percentage >= maxLevel, nil
}

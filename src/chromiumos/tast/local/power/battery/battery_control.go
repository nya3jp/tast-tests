// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package battery provides necessary functionality to perform battery charge, drain on DUT.
package battery

import (
	"context"
	"time"

	"chromiumos/tast/common/servo"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/power"
	"chromiumos/tast/testing"
)

// Drain takes a chrome connection and drains the device battery to the specified percentage by rendering a resource heavy WebGL graphics.
func Drain(ctx context.Context, cr *chrome.Chrome, percentage float64) error {
	status, err := power.GetStatus(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to acquire device power status")
	}

	// Maxing out screen brightness to drain faster.
	pm, err := power.NewPowerManager(ctx)
	if err != nil {
		return err
	}
	restore, err := pm.UpdateBrightness(ctx, 100)
	if err != nil {
		return err
	}
	defer restore(ctx)

	const url = "https://crospower.page.link/power_BatteryDrain"
	// Rendering a WebGL website to consume power quickly.
	conn, err := cr.NewConn(ctx, url)
	if err != nil {
		testing.ContextLog(ctx, "Failed to render webpage: ", err)
	}
	defer conn.Close()
	defer conn.CloseTarget(ctx)

	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	if err := testing.Poll(ctx, func(context.Context) error {
		if status, err = power.GetStatus(ctx); err != nil {
			return testing.PollBreak(errors.Wrap(err, "failed to get device power status"))
		}
		if status.LinePowerConnected {
			return testing.PollBreak(errors.New("BatteryDrain requires device to be disconnected from power source"))
		}

		if status.BatteryDisplayPercent > percentage {
			return errors.New("still discharging")
		}
		return nil
	}, &testing.PollOptions{
		Interval: time.Second,
	}); err != nil {
		return errors.Wrapf(err, "unable to drain the battery to %f percent", percentage)
	}

	return nil
}

// Charge charges the device to the specified percentage.
func Charge(ctx context.Context, percentage float64) error {
	status, err := power.GetStatus(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to acquire device power status")
	}

	if err := testing.Poll(ctx, func(context.Context) error {
		// To tackle the micro delay between switching servo role & DUT reflects the status.
		status, err = power.GetStatus(ctx)
		if err != nil {
			return testing.PollBreak(errors.Wrap(err, "failed to acquire device power status"))
		}
		if !status.LinePowerConnected {
			return errors.New("BatteryCharge requires power line connected for charging DUT")
		}
		return nil
	}, &testing.PollOptions{
		Timeout:  20 * time.Second,
		Interval: time.Second,
	}); err != nil {
		return err
	}

	pm, err := power.NewPowerManager(ctx)
	if err != nil {
		return err
	}
	restore, err := pm.UpdateBrightness(ctx, 10)
	if err != nil {
		return err
	}
	defer restore(ctx)

	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	if err := testing.Poll(ctx, func(context.Context) error {
		if status, err = power.GetStatus(ctx); err != nil {
			return testing.PollBreak(errors.Wrap(err, "failed to get device power status"))
		}
		if status.BatteryDischarging {
			return testing.PollBreak(errors.Wrap(err, "power line isn't connected, can't charge device"))
		}

		if status.BatteryDisplayPercent == 100.0 {
			return nil
		}
		if status.BatteryDisplayPercent < percentage {
			return errors.New("still charging")
		}
		return nil
	}, &testing.PollOptions{
		Interval: time.Second,
	}); err != nil {
		return errors.Wrapf(err, "unable to charge the battery to %f percent", percentage)
	}

	return nil
}

// EnsureBatteryWithinRange takes a servo & chrome connection and ensure the device battery within the specified minLevel & maxLevel.
func EnsureBatteryWithinRange(ctx context.Context, cr *chrome.Chrome, s *servo.Servo, minLevel, maxLevel float64) error {
	status, err := power.GetStatus(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to fetch device battery status")
	}
	// Battery already is in provided range.
	if p := status.BatteryDisplayPercent; p > minLevel && p < maxLevel {
		return nil
	}

	role, err := s.GetPDRole(ctx) // querying current servo role and store for future to restore back.
	if err != nil {
		return err
	}

	if role == servo.PDRoleNA {
		return errors.New("requires servo type v4 for battery charge and drain through servo_pd_role")
	}

	defer func(ctx context.Context) {
		if err := s.SetPDRole(ctx, role); err != nil {
			testing.ContextLogf(ctx, "Failed to restore servo_pd_role to %s during cleanup: %v", role, err)
		}
	}(ctx)

	ctx, cancel := ctxutil.Shorten(ctx, 20*time.Second)
	defer cancel()

	if status.BatteryDisplayPercent < minLevel { // requires charging.
		testing.ContextLog(ctx, "Battery charging has been initiated")
		if err := s.SetPDRole(ctx, servo.PDRoleSrc); err != nil {
			return errors.Wrap(err, "unable to set servo_pd_role to src")
		}
		if err := Charge(ctx, minLevel); err != nil {
			return err
		}
	} else { // discharging
		if err := s.SetPDRole(ctx, servo.PDRoleSnk); err != nil {
			return errors.Wrap(err, "unable to set servo_pd_role to snk")
		}
		testing.ContextLog(ctx, "Battery discharging has been initiated")
		if err := Drain(ctx, cr, maxLevel); err != nil {
			return err
		}
	}

	return nil
}

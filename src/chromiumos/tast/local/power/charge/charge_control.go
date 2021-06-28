// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package charge provides necessary functionality to perform battery charge, drain on DUT.
package charge

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

// EnsureBatteryWithinRange ensures the device battery within the specified min & max percentages.
// Powerd reports battery_percent & battery_display_percent and for all the operations the latter
// one has been taken into consideration.
// This function queries the current battery display percentage of DUT and initiates charging or draining as required.
func EnsureBatteryWithinRange(ctx context.Context, cr *chrome.Chrome, s *servo.Servo, minPercentage, maxPercentage float64) error {
	if minPercentage < 0.0 || minPercentage > 100.0 {
		return errors.New("invalid min percentage, it should be within [0.0, 100.0]")
	}
	if maxPercentage < 0.0 || maxPercentage > 100.0 {
		return errors.New("invalid max percentage, it should be within [0.0, 100.0]")
	}

	// Shorten deadline to leave time for cleanup.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	status, err := power.GetStatus(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to obtain DUT power status")
	}
	// Battery already is in required range.
	if p := status.BatteryDisplayPercent; p > minPercentage && p < maxPercentage {
		return nil
	}

	role, err := s.GetPDRole(ctx) // storing the current servo role to perform a deferred restore
	if err != nil {
		return errors.Wrap(err, "failed to get current servo power delivery (pd) role")
	}

	if role == servo.PDRoleNA {
		return errors.New(`requires "servo v4" for operating DUT power delivery through servo_pd_role`)
	}

	defer func(ctx context.Context) {
		if err := s.SetPDRole(ctx, role); err != nil {
			testing.ContextLogf(ctx, "Failed to restore servo_pd_role to %s during cleanup: %v", role, err)
		}
	}(cleanupCtx)

	if status.BatteryDisplayPercent < minPercentage { // charging
		if err := s.SetPDRole(ctx, servo.PDRoleSrc); err != nil {
			return errors.Wrapf(err, "unable to set servo_pd_role to %s", servo.PDRoleSrc)
		}
		testing.ContextLogf(ctx, "Battery charging has been initiated. Target percentage: %.2f %%", minPercentage)
		if err := charge(ctx, minPercentage); err != nil {
			return err
		}
	} else { // discharging
		if err := s.SetPDRole(ctx, servo.PDRoleSnk); err != nil {
			return errors.Wrapf(err, "unable to set servo_pd_role to %s", servo.PDRoleSnk)
		}
		testing.ContextLogf(ctx, "Battery discharging has been initiated. Target percentage: %.2f %%", maxPercentage)
		if err := drain(ctx, cr, maxPercentage); err != nil {
			return err
		}
	}

	return nil
}

// charge charges the device to the specified display percentage.
func charge(ctx context.Context, displayPercentage float64) error {
	// Shorten deadline to leave time for cleanup.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()

	if err := powerSourceStatus(ctx, true); err != nil {
		return err
	}

	// Dimming DUT screen.
	pm, err := power.NewPowerManager(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to create PowerManager object to reduce screen brightness for faster charging")
	}
	brightness, err := pm.GetScreenBrightnessPercent(ctx)
	if err := pm.SetScreenBrightness(ctx, 10); err != nil {
		return errors.Wrap(err, "failed to update screen brightness")
	}

	defer func(ctx context.Context) {
		if err := pm.SetScreenBrightness(ctx, brightness); err != nil {
			testing.ContextLogf(ctx, "Failed to reset screen brightness to %.2f %%: %v", brightness, err)
		}
	}(cleanupCtx)

	if err := testing.Poll(ctx, func(context.Context) error {
		status, err := power.GetStatus(ctx)
		if err != nil {
			return testing.PollBreak(errors.Wrap(err, "failed to obtain DUT power status"))
		}
		if status.BatteryDischarging {
			return testing.PollBreak(errors.Wrap(err, "power line isn't connected, can't charge device"))
		}

		if status.BatteryDisplayPercent < displayPercentage {
			return errors.Errorf("still charging from %.2f %% to %.2f %%", status.BatteryDisplayPercent, displayPercentage)
		}
		return nil
	}, &testing.PollOptions{
		Interval: time.Second,
	}); err != nil {
		return errors.Wrapf(err, "failed to charge battery to %.2f %%", displayPercentage)
	}

	return nil
}

// drain discharges the device battery to the specified display percentage by rendering a resource heavy WebGL graphics.
func drain(ctx context.Context, cr *chrome.Chrome, displayPercentage float64) error {
	// Shorten deadline to leave time for cleanup.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	if err := powerSourceStatus(ctx, false); err != nil {
		return err
	}

	// Maxing out screen brightness to drain faster.
	pm, err := power.NewPowerManager(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to create PowerManager object to maximize screen brightness for faster discharging")
	}
	brightness, err := pm.GetScreenBrightnessPercent(ctx)
	if err := pm.SetScreenBrightness(ctx, 100); err != nil {
		return errors.Wrap(err, "failed to update screen brightness")
	}

	defer func(ctx context.Context) {
		if err := pm.SetScreenBrightness(ctx, brightness); err != nil {
			testing.ContextLogf(ctx, "Failed to reset screen brightness to %.2f %%: %v", brightness, err)
		}
	}(cleanupCtx)

	// Rendering a WebGL website to consume power quickly.
	conn, err := cr.NewConn(ctx, "https://crospower.page.link/power_BatteryDrain")
	if err != nil {
		testing.ContextLog(ctx, "Failed to open page: ", err)
	}
	defer conn.Close()
	defer conn.CloseTarget(cleanupCtx)

	if err := testing.Poll(ctx, func(context.Context) error {
		status, err := power.GetStatus(ctx)
		if err != nil {
			return testing.PollBreak(errors.Wrap(err, "failed to obtain DUT power status"))
		}
		if status.LinePowerConnected {
			return testing.PollBreak(errors.New("battery draining requires device disconnected from the power source"))
		}

		if status.BatteryDisplayPercent > displayPercentage {
			return errors.Errorf("still discharging from %.2f %% to %.2f %%", displayPercentage, status.BatteryDisplayPercent)
		}
		return nil
	}, &testing.PollOptions{
		Interval: time.Second,
	}); err != nil {
		return errors.Wrapf(err, "failed to drain battery to %.2f %%", displayPercentage)
	}

	return nil
}

// powerSourceStatus polls device power source connection status to tackle the micro delay between flipping
// servo role and DUT reflects the status.
func powerSourceStatus(ctx context.Context, acConnected bool) error {
	return testing.Poll(ctx, func(context.Context) error {
		status, err := power.GetStatus(ctx)
		if err != nil {
			return testing.PollBreak(errors.Wrap(err, "failed to obtain DUT power status"))
		}
		if acConnected && !status.LinePowerConnected {
			return errors.New("battery charging requires device conntected to an active power source")
		}
		if !acConnected && status.LinePowerConnected {
			return errors.New("battery draining requires device disconnected from the power source")
		}
		return nil
	}, &testing.PollOptions{
		Timeout: 20 * time.Second,
	})
}

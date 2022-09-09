// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package power

import (
	"context"
	"strconv"
	"time"

	"chromiumos/tast/remote/firmware"
	"chromiumos/tast/remote/firmware/fixture"
	"chromiumos/tast/remote/firmware/power"
	"chromiumos/tast/remote/firmware/suspend"
	"chromiumos/tast/testing"
)

const (
	defaultDuration = 15 * time.Minute

	// default servo comm timeout is 10s, battery check requires two
	batteryCapacityTimeout  = 20 * time.Second
	batteryCapacityInterval = time.Second

	// Minimum battery life of 14 days
	minimumBatteryLife = time.Hour * 24 * 14

	varDuration = "duration"
	varNoGsc    = "no_gsc"

	reconnectTimeout = 20 * time.Second

	gsc1V8Rail = "pp1800_gsc_z1"
	gsc3V3Rail = "pp3300_gsc_z1"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: SuspendConsumption,
		Desc: "Tests that power consumption in suspend meets requirements",
		Contacts: []string{
			"robertzieba@google.com",
			"tast-users@chromium.org",
		},
		Attr:    []string{"group:firmware"},
		Fixture: fixture.NormalMode,
		Timeout: defaultDuration + 5*time.Minute, // Ensure we have enough time for test setup/teardown
		Vars:    []string{varDuration, varNoGsc}, // Duration is in seconds
		Params: []testing.Param{{
			Name: "s0ix",
			Val:  suspend.StateS0ix,
		}},
	})
}

func SuspendConsumption(ctx context.Context, s *testing.State) {
	h := s.FixtValue().(*fixture.Value).Helper

	if err := h.RequireServo(ctx); err != nil {
		s.Fatal("Failed to connect to servo: ", err)
	}

	duration := defaultDuration
	if v, ok := s.Var(varDuration); ok {
		seconds, err := strconv.Atoi(v)
		if err != nil {
			s.Fatalf("Failed to parse %s from string %s", varDuration, v)
		}

		duration = time.Duration(seconds) * time.Second
	}

	noGsc := true
	if v, ok := s.Var(varNoGsc); ok {
		var err error = nil
		noGsc, err = strconv.ParseBool(v)
		if err != nil {
			s.Fatalf("Failed to parse %s from string %s", varNoGsc, v)
		}
	}

	// Can't exceed defaultDuration because the test will timeout
	if duration > defaultDuration {
		s.Fatalf("Duration cannot exceed %d seconds", int(defaultDuration.Seconds()))
	}

	s.Logf("Suspend duration %d seconds", int(duration.Seconds()))

	// Disconnect AC power since we're using the battery rail to measure consumption
	// The original value will be automatically restored when the test ends
	if err := h.SetDUTPower(ctx, false); err != nil {
		s.Fatal("Failed to disconnect AC power")
	}

	// Create our suspend context
	suspendContext, err := suspend.NewContext(ctx, h)
	if err != nil {
		s.Fatalf("Failed to create suspend context: %s", err)
	}
	defer suspendContext.Close()

	targetState := s.Param().(suspend.State)
	s.Logf("Checking %s support", targetState)
	if err := suspendContext.VerifySupendWake(targetState); err != nil {
		s.Fatalf("Failed to determine support for %s: %s", targetState, err)
	}

	// Create our measurement context
	powerContext := power.NewDutPowerContext(ctx, h)

	// Start testing
	s.Log("Suspending DUT")
	args := suspend.DefaultSuspendArgs()
	if err := suspendContext.SuspendDUT(targetState, args); err != nil {
		s.Fatalf("Failed to suspend DUT: %s", err)
	}

	s.Log("Starting power measurements")
	results, err := powerContext.Measure(duration)
	if err != nil {
		s.Fatalf("Failed to measure power: %s", err)
	}

	s.Log("Waking DUT")
	wakeArgs := suspend.DefaultWakeArgs()
	// The DUT may not reconnect automatically after a long suspend
	// So attempt to manually reconnect if that happens
	wakeArgs.ForceReconnect = true
	if err := suspendContext.WakeDUT(wakeArgs); err != nil {
		s.Fatalf("Failed to wake DUT: %s", err)
	}

	s.Log(results)

	var consumption float32
	var exists bool
	powerConsumptionKeys := []string{
		// The sense resistor on the battery rail is usually the most accurate
		"ppvar_bat",
		"ppvar_bat_q",
		"ppvar_vbat",
	}
	for _, key := range powerConsumptionKeys {
		consumption, exists = results.GetMean(key)
		if exists {
			break
		}
	}
	if !exists {
		s.Fatal("Failed to get system power consumption on rails ", powerConsumptionKeys)
	}

	if consumption == 0.0 || consumption < 0.0 {
		s.Fatalf("Invalid system power consumption of %f", consumption)
	}

	if noGsc {
		// Communication between any onboard power sensors goes through the GSC
		// The GSC is generally asleep during suspend so its consumption should be
		// nearly zero

		gscRails := []string{
			gsc1V8Rail,
			gsc3V3Rail,
		}

		for i := 0; i < len(gscRails); i++ {
			if gscConsumption, exists := results.GetMean(gscRails[i]); exists {
				s.Logf("Subtracting %q consumption of %f mW", gscRails[i], gscConsumption)
				consumption -= gscConsumption
			}
		}
	}

	// The requirements are formulated in terms of days of battery life
	// Get the capacity of the battery to determine if the test passed
	capacityMWH, err := getBatteryCapacityMWH(ctx, h)
	if err != nil {
		s.Fatalf("Failed to get battery capacity: %s", err)
	}

	batteryLifeHours := float32(capacityMWH) / consumption
	batteryLifeDuration := time.Duration(int(batteryLifeHours)) * time.Hour
	batteryLifeDays := batteryLifeDuration.Hours() / 24.0

	s.Logf("Measured system power consumption of %f mW", consumption)
	s.Logf("Battery capacity is %d mWH, battery life in %s is %f days", capacityMWH, targetState, batteryLifeDays)

	if batteryLifeDuration < minimumBatteryLife {
		s.Fatalf("Device battery life of %f days is less than minimum of %f", batteryLifeDays, minimumBatteryLife.Hours()/24.0)
	}
}

func getBatteryCapacityMWH(ctx context.Context, h *firmware.Helper) (int, error) {
	var err error = nil
	designCapacityMAH := 0
	designVoltageMV := 0

	testing.Poll(ctx, func(ctx context.Context) error {
		designCapacityMAH, err = h.Servo.GetBatteryFullDesignMAH(ctx)
		if err != nil {
			return err
		}

		designVoltageMV, err = h.Servo.GetBatteryDesignVoltageDesignMV(ctx)
		if err != nil {
			return err
		}

		return nil

	}, &testing.PollOptions{Timeout: batteryCapacityTimeout, Interval: batteryCapacityInterval})

	if err != nil {
		return -1, err
	}

	// Capacity and voltage are milli units, multiplying them yields microwatt-hours
	// Divide by 1000 to get milliwatt-hours
	return designCapacityMAH * designVoltageMV / 1000, nil
}

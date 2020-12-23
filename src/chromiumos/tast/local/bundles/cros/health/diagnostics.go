// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package health tests the system daemon cros_healthd to ensure that telemetry
// and diagnostics calls can be completed successfully.
package health

import (
	"context"
	"time"

	"chromiumos/tast/local/croshealthd"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: Diagnostics,
		Desc: "Tests 'cros-health-tool diag' command line invocation",
		Contacts: []string{
			"pmoy@chromium.org",   // cros_healthd tool author
			"tbegin@chromium.org", // test author
		},
		SoftwareDeps: []string{"chrome", "diagnostics"},
		Attr:         []string{"group:mainline", "informational"},
		Timeout:      10 * time.Minute,
		Fixture:      "crosHealthdRunning",
	})
}

// Diagnostics runs and verifies that the 'cros-health-tool diag' command-line
// invocation can get a list of supported routines from cros_healthd, and run
// every testable routine supported across all boards.
//
// The diag command has two actions it can run. 'get_routines' returns a list of
// all routines that diag supports and 'run_routine' starts running a routine
// and waits until the routine completes and prints the result.
//
// This test verifies that 'get_routines' returns a valid list of routines and
// then runs the testable routines to make sure there is no error. The routines
// can either pass or fail.
func Diagnostics(ctx context.Context, s *testing.State) {
	// Determine if a routine is programmatically testable. Some routines take
	// too long or require physical user interaction.
	isTestable := func(routine string) bool {
		switch routine {
		case
			croshealthd.RoutineACPower,          // Interactive
			croshealthd.RoutineBatteryCharge,    // Interactive
			croshealthd.RoutineBatteryDischarge, // Interactive
			croshealthd.RoutineMemory:           // ~30 min runtime
			return false
		}

		return true
	}

	routines, err := croshealthd.GetDiagRoutines(ctx)
	if err != nil {
		s.Fatal("Failed to get diag routines: ", err)
	}

	// There are 9 routines supported on all devices.
	if len(routines) < 9 {
		s.Fatalf("Found %d routine(s) %v; want >=9", len(routines), routines)
	}

	// Run each of the routines supported on the device that are
	// programmatically testable.
	for _, routine := range routines {
		if !isTestable(routine) {
			s.Logf("Skipping untestable routine: %s", routine)
			continue
		}

		s.Logf("Running routine: %s", routine)
		ret, err := croshealthd.RunDiagRoutine(ctx, routine)
		if err != nil {
			s.Fatalf("Unable to run diagnostic routine: %s", err)
		} else if ret == nil {
			s.Fatal("nil result from RunDiagRoutine running ", routine)
		}
		result := *ret

		// Test a given routine and ensure that it runs and either passes or fails.
		// Some lab machines might have old batteries, for example, so we only want
		// to test that the routine can complete successfully without crashing or
		// throwing errors.
		if result.Status != croshealthd.StatusPassed &&
			result.Status != croshealthd.StatusFailed &&
			result.Status != croshealthd.StatusNotRun {
			s.Fatalf("%q routine has status %q; want \"Passed\", \"Failed\", or \"Not run\"", routine, result.Status)
		}

		// Check to see that if the routine was run, the progress is 100%
		if result.Progress != 100 && result.Status != croshealthd.StatusNotRun {
			s.Fatalf("%q routine has progress %d; want 100", routine, result.Progress)
		}
	}
}

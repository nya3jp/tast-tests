// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package platform

import (
	"context"
	"strconv"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/power"
	"chromiumos/tast/local/power/setup"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: PowerBenchmark,
		Desc: "Tests the resolution of power metrics for different task durations",
		Contacts: []string{
			"mblsha@google.com",
		},
		Attr: []string{
			"group:mainline",
		},
		// SoftwareDeps: []string{},
		// HardwareDeps:
		// display_backlight

		// SoftwareDeps:
		// stress-ng
		Params: []testing.Param{},
	})
}

func PowerBenchmark(ctx context.Context, s *testing.State) {
	// Shorten the test context so that even if the test times out
	// there will be time to clean up.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, time.Minute)
	defer cancel()

	// setup.Setup configures a DUT for a test, and cleans up after.
	sup, cleanup := setup.New("mblsha power benchmark")
	defer func() {
		if err := cleanup(cleanupCtx); err != nil {
			s.Fatal("Cleanup failed: ", err)
		}
	}()

	// TODO(mblsha): Set backlight to zero

	// Add the default power test configuration.
	sup.Add(setup.PowerTest(ctx, nil, setup.ForceBatteryDischarge))
	if err := sup.Check(ctx); err != nil {
		s.Fatal("Setup failed: ", err)
	}

	batteryPaths, err := power.ListSysfsBatteryPaths(ctx)
	if err != nil {
		s.Fatal("Failed to get battery paths: ", err)
	}

	if len(batteryPaths) != 1 {
		s.Fatal("device has multiple batteries: ", batteryPaths)
	}

	//----------------------------------------------------------------------------

	type Result struct {
		time          int
		rapl_watts    float64
		battery_watts float64
	}
	var results []Result

	for test_duration := 1; test_duration <= 10; test_duration += 1 {
		for i := 0; i < 5; i += 1 {
			testing.ContextLog(ctx, test_duration, "s, run #", i)

			battery_start, err := power.ReadBatteryEnergy(batteryPaths)
			if err != nil {
				s.Fatal("failed to read battery energy ", err)
			}

			// uses intel RAPL (aka simulated power metrics, supported only on Intel platforms)
			snapshot, err := power.NewRAPLSnapshot()
			if err != nil {
				s.Fatalf("failed to create RAPL Snapshot", err)
			}
			if _, err := snapshot.DiffWithCurrentRAPLAndReset(); err != nil {
				s.Fatalf("failed to collect initial RAPL metrics ", err)
			}

			human_duration := strconv.Itoa(test_duration) + "s"
			cmd := testexec.CommandContext(ctx, "stress-ng", "--cpu", "8", "--timeout", human_duration)
			if err := cmd.Run(testexec.DumpLogOnError); err != nil {
				s.Fatalf("failed to run stress-ng ", err)
			}

			energy, err := snapshot.DiffWithCurrentRAPLAndReset()
			if err != nil {
				s.Fatalf("failed to collect initial RAPL metrics", err)
			}

			battery_end, err := power.ReadBatteryEnergy(batteryPaths)
			if err != nil {
				s.Fatal("failed to read battery energy ", err)
			}

			if battery_start < battery_end {
				s.Fatalf("battery_start (", battery_start, ") < battery_end (", battery_end, "): is charger not disabled?")
			}
			battery_watts := (battery_start - battery_end) / float64(test_duration) * 3600.0
			rapl_watts := energy.Total() / float64(test_duration)

			r := Result{test_duration, rapl_watts, battery_watts}
			testing.ContextLog(ctx, "battery_start: ", battery_start, "; battery_end: ", battery_end)
			testing.ContextLog(ctx, r)
			results = append(results, r)
		}
	}

	testing.ContextLog(ctx, "\n\n\n")
	testing.ContextLog(ctx, "results:", results)
	testing.ContextLog(ctx, "\n\n\n")
}

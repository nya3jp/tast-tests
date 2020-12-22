// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package health tests the system daemon cros_healthd to ensure that telemetry
// and diagnostics calls can be completed successfully.
package health

import (
	"context"

	"chromiumos/tast/local/croshealthd"
	"chromiumos/tast/testing"
)

// newRoutineParams creates and returns a diagnostic routine with default test
// parameters.
func newRoutineParams(routine string) croshealthd.RoutineParams {
	return croshealthd.RoutineParams{
		Routine: routine,
		Cancel:  false,
	}
}

// newCancelRoutineParams creates and returns a diagnostic routine that will be
// canceled partway through running.
func newCancelRoutineParams(routine string) croshealthd.RoutineParams {
	return croshealthd.RoutineParams{
		Routine: routine,
		Cancel:  true,
	}
}

func init() {
	testing.AddTest(&testing.Test{
		Func: DiagnosticsRun,
		Desc: "Tests that the cros_healthd diagnostic routines can be run without errors",
		Contacts: []string{
			"pmoy@chromium.org",   // cros_healthd tool author
			"tbegin@chromium.org", // test author
		},
		SoftwareDeps: []string{"diagnostics"},
		Attr:         []string{"group:mainline"},
		Fixture:      "crosHealthdRunning",
		Params: []testing.Param{{
			Name:      "battery_capacity",
			Val:       newRoutineParams(croshealthd.RoutineBatteryCapacity),
			ExtraAttr: []string{"informational"},
		}, {
			Name:      "battery_health",
			Val:       newRoutineParams(croshealthd.RoutineBatteryHealth),
			ExtraAttr: []string{"informational"},
		}, {
			Name:      "urandom",
			Val:       newCancelRoutineParams(croshealthd.RoutineURandom),
			ExtraAttr: []string{"informational"},
		}, {
			Name:      "smartctl_check",
			Val:       newRoutineParams(croshealthd.RoutineSmartctlCheck),
			ExtraAttr: []string{"informational"},
		}, {
			Name:      "cpu_cache",
			Val:       newRoutineParams(croshealthd.RoutineCPUCache),
			ExtraAttr: []string{"informational"},
		}, {
			Name:      "cpu_stress",
			Val:       newRoutineParams(croshealthd.RoutineCPUStress),
			ExtraAttr: []string{"informational"},
		}, {
			Name:      "floating_point_accuracy",
			Val:       newRoutineParams(croshealthd.RoutineFloatingPointAccurary),
			ExtraAttr: []string{"informational"},
		}, {
			Name:      "nvme_self_test",
			Val:       newRoutineParams(croshealthd.RoutineNVMESelfTest),
			ExtraAttr: []string{"informational"},
		}, {
			Name:              "nvme_wear_level",
			Val:               newRoutineParams(croshealthd.RoutineNVMEWearLevel),
			ExtraAttr:         []string{"informational"},
			ExtraSoftwareDeps: []string{"wilco"},
		}, {
			Name:      "prime_search",
			Val:       newRoutineParams(croshealthd.RoutinePrimeSearch),
			ExtraAttr: []string{"informational"},
		}, {
			Name:      "lan_connectivity",
			Val:       newRoutineParams(croshealthd.RoutineLanConnectivity),
			ExtraAttr: []string{"informational"},
		}, {
			Name:      "signal_strength",
			Val:       newRoutineParams(croshealthd.RoutineSignalStrength),
			ExtraAttr: []string{"informational"},
		}, {
			Name:      "gateway_can_be_pinged",
			Val:       newRoutineParams(croshealthd.RoutineGatewayCanBePinged),
			ExtraAttr: []string{"informational"},
		}, {
			Name:      "has_secure_wifi_connection",
			Val:       newRoutineParams(croshealthd.RoutineHasSecureWifiConnection),
			ExtraAttr: []string{"informational"},
		}, {
			Name:      "dns_resolver_present",
			Val:       newRoutineParams(croshealthd.RoutineDNSResolverPresent),
			ExtraAttr: []string{"informational"},
		}, {
			Name:      "dns_latency",
			Val:       newRoutineParams(croshealthd.RoutineDNSLatency),
			ExtraAttr: []string{"informational"},
		}, {
			Name:      "dns_resolution",
			Val:       newRoutineParams(croshealthd.RoutineDNSResolverPresent),
			ExtraAttr: []string{"informational"},
		}, {
			Name:      "captive_portal",
			Val:       newRoutineParams(croshealthd.RoutineCaptivePortal),
			ExtraAttr: []string{"informational"},
		}, {
			Name:      "http_firewall",
			Val:       newRoutineParams(croshealthd.RoutineHTTPFirewall),
			ExtraAttr: []string{"informational"},
		}, {
			Name:      "https_firewall",
			Val:       newRoutineParams(croshealthd.RoutineHTTPSFirewall),
			ExtraAttr: []string{"informational"},
		}, {
			Name:      "https_latency",
			Val:       newRoutineParams(croshealthd.RoutineHTTPSLatency),
			ExtraAttr: []string{"informational"},
		}},
	})
}

// DiagnosticsRun is a paramaterized test that runs supported diagnostic
// routines through cros_healthd. The purpose of this test is to ensure that the
// routines can be run without errors, and not to check if the routines pass or
// fail.
func DiagnosticsRun(ctx context.Context, s *testing.State) {
	params := s.Param().(croshealthd.RoutineParams)
	routine := params.Routine
	s.Logf("Running routine: %s", routine)
	ret, err := croshealthd.RunDiagRoutine(ctx, params)
	if err != nil {
		s.Fatalf("Unable to run %s routine: %s", routine, err)
	} else if ret == nil {
		s.Fatal("nil result from RunDiagRoutine running ", routine)
	}
	result := *ret

	// Check that if requested, the routine was canceled.
	if params.Cancel {
		if result.Status != croshealthd.StatusCancelling &&
			result.Status != croshealthd.StatusCancelled {
			s.Fatalf("%q routine has status %q; want %q or %q",
				routine, result.Status, croshealthd.StatusCancelling, croshealthd.StatusCancelled)
		}
		return
	}

	// Test a given routine and ensure that it can complete successfully without
	// crashing or throwing errors. For example, some lab machines might have
	// old batteries that would fail the diagnostic routines, but this should
	// not fail the tast test
	if result.Status != croshealthd.StatusPassed &&
		result.Status != croshealthd.StatusFailed &&
		result.Status != croshealthd.StatusNotRun {
		s.Fatalf("%q routine has status %q; want %q, %q, or %q",
			routine, croshealthd.StatusPassed, croshealthd.StatusFailed, croshealthd.StatusNotRun, result.Status)
	}

	// Check to see that if the routine was run, the progress is 100%
	if result.Progress != 100 && result.Status != croshealthd.StatusNotRun {
		s.Fatalf("%q routine has progress %d; want 100", routine, result.Progress)
	}
}

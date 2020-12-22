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

func init() {
	testing.AddTest(&testing.Test{
		Func: DiagnosticsRun,
		Desc: "Tests that the cros_healthd diagnostic routines can be run without errors",
		Contacts: []string{
			"pmoy@chromium.org",   // cros_healthd tool author
			"tbegin@chromium.org", // test author
			"cros-tdm@google.com", // team mailing list
		},
		SoftwareDeps: []string{"diagnostics"},
		Attr:         []string{"group:mainline"},
		Fixture:      "crosHealthdRunning",
		Params: []testing.Param{{
			Name:      "battery_capacity",
			Val:       croshealthd.RoutineBatteryCapacity,
			ExtraAttr: []string{"informational"},
		}, {
			Name:      "battery_health",
			Val:       croshealthd.RoutineBatteryHealth,
			ExtraAttr: []string{"informational"},
		}, {
			Name:      "urandom",
			Val:       croshealthd.RoutineURandom,
			ExtraAttr: []string{"informational"},
		}, {
			Name:      "smartctl_check",
			Val:       croshealthd.RoutineSmartctlCheck,
			ExtraAttr: []string{"informational"},
		}, {
			Name:      "cpu_cache",
			Val:       croshealthd.RoutineCPUCache,
			ExtraAttr: []string{"informational"},
		}, {
			Name:      "cpu_stress",
			Val:       croshealthd.RoutineCPUStress,
			ExtraAttr: []string{"informational"},
		}, {
			Name:      "floating_point_accuracy",
			Val:       croshealthd.RoutineFloatingPointAccurary,
			ExtraAttr: []string{"informational"},
		}, {
			Name:      "nvme_self_test",
			Val:       croshealthd.RoutineNVMESelfTest,
			ExtraAttr: []string{"informational"},
		}, {
			Name:      "nvme_wear_level",
			Val:       croshealthd.RoutineNVMEWearLevel,
			ExtraAttr: []string{"informational"},
		}, {
			Name:      "prime_search",
			Val:       croshealthd.RoutinePrimeSearch,
			ExtraAttr: []string{"informational"},
		}, {
			Name:      "lan_connectivity",
			Val:       croshealthd.RoutineLanConnectivity,
			ExtraAttr: []string{"informational"},
		}, {
			Name:      "signal_strength",
			Val:       croshealthd.RoutineSignalStrength,
			ExtraAttr: []string{"informational"},
		}, {
			Name:      "gateway_can_be_pinged",
			Val:       croshealthd.RoutineGatewayCanBePinged,
			ExtraAttr: []string{"informational"},
		}, {
			Name:      "has_secure_wifi_connection",
			Val:       croshealthd.RoutineHasSecureWifiConnection,
			ExtraAttr: []string{"informational"},
		}, {
			Name:      "dns_resolver_present",
			Val:       croshealthd.RoutineDNSResolverPresent,
			ExtraAttr: []string{"informational"},
		}, {
			Name:      "dns_latency",
			Val:       croshealthd.RoutineDNSLatency,
			ExtraAttr: []string{"informational"},
		}, {
			Name:      "dns_resolution",
			Val:       croshealthd.RoutineDNSResolverPresent,
			ExtraAttr: []string{"informational"},
		}, {
			Name:      "captive_portal",
			Val:       croshealthd.RoutineCaptivePortal,
			ExtraAttr: []string{"informational"},
		}, {
			Name:      "http_firewall",
			Val:       croshealthd.RoutineHTTPFirewall,
			ExtraAttr: []string{"informational"},
		}, {
			Name:      "https_firewall",
			Val:       croshealthd.RoutineHTTPSFirewall,
			ExtraAttr: []string{"informational"},
		}, {
			Name:      "https_latency",
			Val:       croshealthd.RoutineHTTPSLatency,
			ExtraAttr: []string{"informational"},
		}},
	})
}

// DiagnosticsRun is a paramaterized test that runs supported diagnostic
// routines through cros_healthd. The purpose of this test is to ensure that the
// routines can be run without errors, and not to check if the routines pass or
// fail.
func DiagnosticsRun(ctx context.Context, s *testing.State) {
	routine := s.Param().(string)

	// Check that the routine is supported on the device. If not supported, exit
	// the test early.
	routines, err := croshealthd.GetDiagRoutines(ctx)
	if err != nil {
		s.Fatal("Failed to get diag routines: ", err)
	}
	supported := false
	for _, r := range routines {
		if r == routine {
			supported = true
			break
		}
	}

	s.Logf("Running routine: %s", routine)
	ret, err := croshealthd.RunDiagRoutine(ctx, routine)
	if err != nil {
		s.Fatalf("Unable to run %s routine: %s", routine, err)
	} else if ret == nil {
		s.Fatal("nil result from RunDiagRoutine running ", routine)
	}
	result := *ret

	if !supported {
		if result.Status != croshealthd.StatusUnsupported {
			s.Fatalf("%q routine has status %q; want %q",
				routine, result.Status, croshealthd.StatusUnsupported)
		}
		return
	}

	// Test a given routine and ensure that it can complete successfully without
	// crashing or throwing errors. For example, some lab machines might have
	// old batteries that would fail the diagnostic routines, but this should
	// not fail the tast test.
	if !(result.Status == croshealthd.StatusPassed ||
		result.Status == croshealthd.StatusFailed ||
		result.Status == croshealthd.StatusNotRun) {
		s.Fatalf("%q routine has status %q; want %q, %q, or %q",
			routine, croshealthd.StatusPassed, croshealthd.StatusFailed, croshealthd.StatusNotRun, result.Status)
	}

	// Check to see that if the routine was run, the progress is 100%
	if result.Progress != 100 && result.Status != croshealthd.StatusNotRun {
		s.Fatalf("%q routine has progress %d; want 100", routine, result.Progress)
	}
}

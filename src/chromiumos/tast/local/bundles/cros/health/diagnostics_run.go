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
	"chromiumos/tast/testing/hwdep"
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
			Name:              "battery_capacity",
			Val:               croshealthd.RoutineBatteryCapacity,
			ExtraAttr:         []string{"informational"},
			ExtraHardwareDeps: hwdep.D(hwdep.Battery()),
		}, {
			Name:              "battery_health",
			Val:               croshealthd.RoutineBatteryHealth,
			ExtraAttr:         []string{"informational"},
			ExtraHardwareDeps: hwdep.D(hwdep.Battery()),
		}, {
			Name:      "urandom",
			Val:       croshealthd.RoutineURandom,
			ExtraAttr: []string{"informational"},
		}, {
			Name:              "smartctl_check",
			Val:               croshealthd.RoutineSmartctlCheck,
			ExtraAttr:         []string{"informational"},
			ExtraSoftwareDeps: []string{"smartctl"},
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
			Name:              "nvme_self_test",
			Val:               croshealthd.RoutineNVMESelfTest,
			ExtraAttr:         []string{"informational"},
			ExtraSoftwareDeps: []string{"nvme"},
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
	s.Logf("Running routine: %s", routine)
	result, err := croshealthd.RunDiagRoutine(ctx, routine)
	if err != nil {
		s.Fatalf("Unable to run %s routine: %s", routine, err)
	}

	// Test a given routine and ensure that it can complete successfully without
	// crashing or throwing errors. For example, some lab machines might have
	// old batteries that would fail the diagnostic routines, but this should
	// not fail the Tast test.
	if result.Status != croshealthd.StatusPassed &&
		result.Status != croshealthd.StatusFailed &&
		result.Status != croshealthd.StatusNotRun {
		s.Fatalf("Unexpected routine status for %q: got %q; want %q, %q, or %q",
			routine, result.Status, croshealthd.StatusPassed, croshealthd.StatusFailed, croshealthd.StatusNotRun)
	}

	// Check to see that if the routine was run, the progress is 100%
	if result.Progress != 100 && result.Status != croshealthd.StatusNotRun {
		s.Fatalf("Unexpected progress value for %q routine with status %q: got %d; want 100",
			routine, result.Status, result.Progress)
	}
}

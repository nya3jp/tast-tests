// Copyright 2021 The ChromiumOS Authors
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
	"chromiumos/tast/testing/hwdep"
)

// newRoutineParams creates and returns a diagnostic routine with default test
// parameters.
func newRoutineParams(routine string) croshealthd.RoutineParams {
	return croshealthd.RoutineParams{
		Routine:            routine,
		Cancel:             false,
		WearLevelThreshold: 50,
	}
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         DiagnosticsRun,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Tests that the cros_healthd diagnostic routines can be run without errors",
		Contacts:     []string{"cros-tdm-tpe-eng@google.com"},
		SoftwareDeps: []string{"chrome", "diagnostics"},
		Attr:         []string{"group:mainline"},
		Fixture:      "crosHealthdRunning",
		Params: []testing.Param{{
			Name:              "battery_capacity",
			Val:               newRoutineParams(croshealthd.RoutineBatteryCapacity),
			ExtraHardwareDeps: hwdep.D(hwdep.Battery()),
		}, {
			Name:              "battery_health",
			Val:               newRoutineParams(croshealthd.RoutineBatteryHealth),
			ExtraHardwareDeps: hwdep.D(hwdep.Battery()),
		}, {
			Name: "urandom",
			Val:  newRoutineParams(croshealthd.RoutineURandom),
		}, {
			Name:              "smartctl_check",
			Val:               newRoutineParams(croshealthd.RoutineSmartctlCheck),
			ExtraSoftwareDeps: []string{"smartctl"},
		}, {
			Name:    "cpu_cache",
			Val:     newRoutineParams(croshealthd.RoutineCPUCache),
			Timeout: 5 * time.Minute,
		}, {
			Name:    "cpu_stress",
			Val:     newRoutineParams(croshealthd.RoutineCPUStress),
			Timeout: 5 * time.Minute,
		}, {
			Name: "floating_point_accuracy",
			Val:  newRoutineParams(croshealthd.RoutineFloatingPointAccurary),
		}, {
			Name:              "nvme_self_test",
			Val:               newRoutineParams(croshealthd.RoutineNVMESelfTest),
			Timeout:           3 * time.Minute,
			ExtraSoftwareDeps: []string{"nvme"},
			ExtraHardwareDeps: hwdep.D(hwdep.Nvme(), hwdep.NvmeSelfTest()),
		}, {
			Name: "nvme_wear_level",
			Val:  newRoutineParams(croshealthd.RoutineNVMEWearLevel),
			// nvme_wear_level requires specific offsets in the nvme log that
			// are only currently defined for wilco devices.
			ExtraSoftwareDeps: []string{"nvme", "wilco"},
			ExtraHardwareDeps: hwdep.D(hwdep.Nvme()),
		}, {
			Name: "prime_search",
			Val:  newRoutineParams(croshealthd.RoutinePrimeSearch),
		}, {
			Name: "lan_connectivity",
			Val:  newRoutineParams(croshealthd.RoutineLanConnectivity),
		}, {
			Name: "signal_strength",
			Val:  newRoutineParams(croshealthd.RoutineSignalStrength),
		}, {
			Name: "gateway_can_be_pinged",
			Val:  newRoutineParams(croshealthd.RoutineGatewayCanBePinged),
		}, {
			Name: "has_secure_wifi_connection",
			Val:  newRoutineParams(croshealthd.RoutineHasSecureWifiConnection),
		}, {
			Name: "dns_resolver_present",
			Val:  newRoutineParams(croshealthd.RoutineDNSResolverPresent),
		}, {
			Name: "dns_latency",
			Val:  newRoutineParams(croshealthd.RoutineDNSLatency),
		}, {
			Name: "dns_resolution",
			Val:  newRoutineParams(croshealthd.RoutineDNSResolverPresent),
		}, {
			Name: "captive_portal",
			Val:  newRoutineParams(croshealthd.RoutineCaptivePortal),
		}, {
			Name: "http_firewall",
			Val:  newRoutineParams(croshealthd.RoutineHTTPFirewall),
		}, {
			Name: "https_firewall",
			Val:  newRoutineParams(croshealthd.RoutineHTTPSFirewall),
		}, {
			Name: "https_latency",
			Val:  newRoutineParams(croshealthd.RoutineHTTPSLatency),
		}, {
			Name:      "sensitive_sensor",
			Val:       newRoutineParams(croshealthd.RoutineSensitiveSensor),
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
	result, err := croshealthd.RunDiagRoutine(ctx, params)
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

// Copyright 2021 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package croshealthd

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/crosconfig"
	"chromiumos/tast/shutil"
	"chromiumos/tast/testing"
)

// List of cros_healthd diagnostic routines
const (
	RoutineBatteryCapacity         string = "battery_capacity"
	RoutineBatteryHealth                  = "battery_health"
	RoutineURandom                        = "urandom"
	RoutineSmartctlCheck                  = "smartctl_check"
	RoutineACPower                        = "ac_power"
	RoutineCPUCache                       = "cpu_cache"
	RoutineCPUStress                      = "cpu_stress"
	RoutineFloatingPointAccurary          = "floating_point_accuracy"
	RoutineNVMEWearLevel                  = "nvme_wear_level"
	RoutineNVMESelfTest                   = "nvme_self_test"
	RoutineDiskRead                       = "disk_read"
	RoutinePrimeSearch                    = "prime_search"
	RoutineBatteryDischarge               = "battery_discharge"
	RoutineBatteryCharge                  = "battery_charge"
	RoutineMemory                         = "memory"
	RoutineLanConnectivity                = "lan_connectivity"
	RoutineSignalStrength                 = "signal_strength"
	RoutineGatewayCanBePinged             = "gateway_can_be_pinged"
	RoutineHasSecureWifiConnection        = "has_secure_wifi_connection"
	RoutineDNSResolverPresent             = "dns_resolver_present"
	RoutineDNSLatency                     = "dns_latency"
	RoutineDNSResolution                  = "dns_resolution"
	RoutineCaptivePortal                  = "captive_portal"
	RoutineHTTPFirewall                   = "http_firewall"
	RoutineHTTPSFirewall                  = "https_firewall"
	RoutineHTTPSLatency                   = "https_latency"
	RoutineSensitiveSensor                = "sensitive_sensor"
)

// List of possible routine statuses
const (
	StatusReady         string = "Ready"
	StatusRunning              = "Running"
	StatusWaiting              = "Waiting"
	StatusPassed               = "Passed"
	StatusFailed               = "Failed"
	StatusError                = "Error"
	StatusCancelled            = "Cancelled"
	StatusFailedToStart        = "Failed to start"
	StatusRemoved              = "Removed"
	StatusCancelling           = "Cancelling"
	StatusUnsupported          = "Unsupported"
	StatusNotRun               = "Not run"
)

// RoutineResult contains the progress of the routine as a percentage and
// the routine status.
type RoutineResult struct {
	Progress int
	Status   string
}

// RoutineParams are different configuration options for running a diagnostic
// routine.
type RoutineParams struct {
	Routine            string // The name of the routine to run
	Cancel             bool   // Boolean flag to cancel the routine
	WearLevelThreshold int    // Threshold for RoutineNVMEWearLevel. The param
	// will only be used if the corresponding field in
	// cros-config is missing.
}

// RunDiagRoutine runs the specified routine based on `params`. Returns a
// RoutineResult on success or an error.
func RunDiagRoutine(ctx context.Context, params RoutineParams) (*RoutineResult, error) {
	diagParams := []string{"--action=run_routine", fmt.Sprintf("--routine=%s", params.Routine)}
	if params.Cancel {
		diagParams = append(diagParams, "--force_cancel_at_percent=5")
	}
	if params.Routine == RoutineNVMEWearLevel {
		// Replace the default value in params.WearLevelThreshold with the
		// correspodning field found in cros-config.
		thresholdStr, err := crosconfig.Get(ctx, "/cros-healthd/routines/nvme-wear-level", "wear-level-threshold")
		if err != nil && !crosconfig.IsNotFound(err) {
			return nil, errors.Wrap(err, "failed to invoke cros_config for wear-level-threshold")
		}
		if !crosconfig.IsNotFound(err) {
			threshold, err := strconv.Atoi(thresholdStr)
			if err != nil {
				return nil, errors.Wrapf(err, "Unable to parse wear-level-threshold in cros_config %q to int", thresholdStr)
			}
			params.WearLevelThreshold = threshold
		}
		diagParams = append(diagParams, fmt.Sprintf("--wear_level_threshold=%d", params.WearLevelThreshold))
	}
	output, err := runDiag(ctx, diagParams)
	if err != nil {
		return nil, err
	}
	return parseOutput(ctx, output)
}

// GetDiagRoutines returns a list of valid routines for the device on success,
// or an error.
func GetDiagRoutines(ctx context.Context) ([]string, error) {
	output, err := runDiag(ctx, []string{"--action=get_routines"})
	if err != nil {
		return []string{}, err
	}

	re := regexp.MustCompile(`Available routine: (.*)`)
	var routines []string
	for _, line := range strings.Split(strings.TrimSpace(output), "\n") {
		match := re.FindStringSubmatch(line)
		if match != nil {
			routines = append(routines, match[1])
		}
	}
	return routines, nil
}

// runDiag is a helper function that runs the cros_healthd diag command and
// returns the raw stdout on success, or an error.
func runDiag(ctx context.Context, args []string) (string, error) {
	args = append([]string{"diag"}, args...)
	cmd := testexec.CommandContext(ctx, "cros-health-tool", args...)
	testing.ContextLogf(ctx, "Running %q", shutil.EscapeSlice(cmd.Args))
	out, err := cmd.Output()
	if err != nil {
		cmd.DumpLog(ctx)
		return "", errors.Wrapf(err, "failed to run %q", shutil.EscapeSlice(cmd.Args))
	}
	return string(out), nil
}

// parseOutput is a helper function that takes the `raw` output from running a
// diagnostic routine and returns a RoutineResult on success, or an error.
func parseOutput(ctx context.Context, raw string) (*RoutineResult, error) {
	status := ""
	progress := 0
	re := regexp.MustCompile(`([^:]+): (.*)`)
	testing.ContextLog(ctx, raw)

	for _, line := range strings.Split(strings.TrimSpace(raw), "\n") {
		match := re.FindStringSubmatch(line)
		if match == nil {
			continue
		}

		key := match[1]
		value := match[2]
		switch key {
		case "Status":
			status = value
		case "Progress":
			// Look for just the last progress value. Diag prints a single
			// line for the progress, which may contain carriage returns.
			// The line will be formatted as follows, where # is any int:
			// #\rProgress: #\rProgress: #\rProgress: # ... \rProgress: #
			// Slicing value after the last space should give us the final
			// progress percent.
			percent := value[strings.LastIndex(value, " ")+1:]
			i, err := strconv.Atoi(percent)
			if err != nil {
				testing.ContextLogf(ctx, "Failed to parse progress status: %q", value)
				return nil, errors.Wrapf(err, "Unable to parse %q value %q as int", key, percent)
			}
			progress = i
		}
	}
	return &RoutineResult{progress, status}, nil
}

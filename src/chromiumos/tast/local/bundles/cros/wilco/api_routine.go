// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wilco

import (
	"context"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/wilco/pre"
	"chromiumos/tast/local/bundles/cros/wilco/routines"
	"chromiumos/tast/testing"
	dtcpb "chromiumos/wilco_dtc"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: APIRoutine,
		Desc: "Test sending RunRoutineRequest and GetRoutineUpdate gRPC requests from Wilco DTC VM to the Wilco DTC Support Daemon daemon",
		Contacts: []string{
			"vsavu@chromium.org",  // Test author
			"pmoy@chromium.org",   // wilco_dtc_supportd author
			"lamzin@chromium.org", // wilco_dtc_supportd maintainer
			"chromeos-wilco@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"vm_host", "wilco"},
		Pre:          pre.WilcoDtcSupportdAPI,
	})
}

func APIRoutine(ctx context.Context, s *testing.State) {
	// executeRoutine sends the request in rrRequest, executing the routine, and
	// checks the result against expectedStatus. Some routines would not get back
	// to service right away, so shortening test time by cancelling them and check
	// if it is in cancelled status respectively.
	executeRoutine := func(ctx context.Context,
		rrRequest dtcpb.RunRoutineRequest,
		expectedStatus dtcpb.DiagnosticRoutineStatus) error {

		ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()

		rrResponse := dtcpb.RunRoutineResponse{}
		err := routines.CallRunRoutine(ctx, rrRequest, &rrResponse)
		if err != nil {
			return errors.Wrap(err, "unable to run routine: ")
		}

		uuid := rrResponse.Uuid
		response := dtcpb.GetRoutineUpdateResponse{}

		if expectedStatus == dtcpb.DiagnosticRoutineStatus_ROUTINE_STATUS_CANCELLED {
			err = routines.CancelRoutine(ctx, uuid)
			if err != nil {
				return errors.Wrap(err, "unable to cancel routine")
			}
		}

		err = routines.WaitUntilRoutineChangesState(ctx, uuid, dtcpb.DiagnosticRoutineStatus_ROUTINE_STATUS_RUNNING, 5*time.Second)
		if err != nil {
			return errors.Wrap(err, "routine not finished")
		}

		err = routines.GetRoutineStatus(ctx, uuid, true, &response)
		if err != nil {
			return errors.Wrap(err, "unable to get routine status: ")
		}

		s.Log("Routine status message: ", response.StatusMessage)
		if response.Status != expectedStatus {
			return errors.Errorf("invalid status; got %s, want %s", response.Status, expectedStatus)
		}

		err = routines.RemoveRoutine(ctx, uuid)
		if err != nil {
			return errors.Wrap(err, "unable to remove routine: ")
		}

		return nil
	}

	for _, param := range []struct {
		name           string
		request        dtcpb.RunRoutineRequest
		expectedStatus dtcpb.DiagnosticRoutineStatus
	}{
		{
			name: "urandom",
			request: dtcpb.RunRoutineRequest{
				Routine: dtcpb.DiagnosticRoutine_ROUTINE_URANDOM,
				Parameters: &dtcpb.RunRoutineRequest_UrandomParams{
					UrandomParams: &dtcpb.UrandomRoutineParameters{
						LengthSeconds: 1,
					},
				},
			},
			expectedStatus: dtcpb.DiagnosticRoutineStatus_ROUTINE_STATUS_PASSED,
		},
		{
			name: "battery",
			request: dtcpb.RunRoutineRequest{
				Routine: dtcpb.DiagnosticRoutine_ROUTINE_BATTERY,
				Parameters: &dtcpb.RunRoutineRequest_BatteryParams{
					BatteryParams: &dtcpb.BatteryRoutineParameters{
						LowMah:  1000,
						HighMah: 10000,
					},
				},
			},
			expectedStatus: dtcpb.DiagnosticRoutineStatus_ROUTINE_STATUS_PASSED,
		},
		{
			name: "battery_fail",
			request: dtcpb.RunRoutineRequest{
				Routine: dtcpb.DiagnosticRoutine_ROUTINE_BATTERY,
				Parameters: &dtcpb.RunRoutineRequest_BatteryParams{
					BatteryParams: &dtcpb.BatteryRoutineParameters{
						LowMah:  10,
						HighMah: 100,
					},
				},
			},
			// HighMah is 100 (all devices should have a battery larger than
			// this).
			expectedStatus: dtcpb.DiagnosticRoutineStatus_ROUTINE_STATUS_FAILED,
		},
		{
			name: "battery_sysfs",
			request: dtcpb.RunRoutineRequest{
				Routine: dtcpb.DiagnosticRoutine_ROUTINE_BATTERY_SYSFS,
				Parameters: &dtcpb.RunRoutineRequest_BatterySysfsParams{
					BatterySysfsParams: &dtcpb.BatterySysfsRoutineParameters{
						MaximumCycleCount:         5000,
						PercentBatteryWearAllowed: 50,
					},
				},
			},
			expectedStatus: dtcpb.DiagnosticRoutineStatus_ROUTINE_STATUS_PASSED,
		},
		{
			name: "smartctl",
			request: dtcpb.RunRoutineRequest{
				Routine: dtcpb.DiagnosticRoutine_ROUTINE_SMARTCTL_CHECK,
				Parameters: &dtcpb.RunRoutineRequest_SmartctlCheckParams{
					SmartctlCheckParams: &dtcpb.SmartctlCheckRoutineParameters{},
				},
			},
			expectedStatus: dtcpb.DiagnosticRoutineStatus_ROUTINE_STATUS_PASSED,
		},
		{
			name: "cpu_cache",
			request: dtcpb.RunRoutineRequest{
				Routine: dtcpb.DiagnosticRoutine_ROUTINE_CPU_CACHE,
				Parameters: &dtcpb.RunRoutineRequest_CpuParams{
					CpuParams: &dtcpb.CpuRoutineParameters{
						LengthSeconds: 1,
					},
				},
			},
			expectedStatus: dtcpb.DiagnosticRoutineStatus_ROUTINE_STATUS_PASSED,
		},
		{
			name: "cpu_cache_fail",
			request: dtcpb.RunRoutineRequest{
				Routine: dtcpb.DiagnosticRoutine_ROUTINE_CPU_CACHE,
				Parameters: &dtcpb.RunRoutineRequest_CpuParams{
					CpuParams: &dtcpb.CpuRoutineParameters{
						LengthSeconds: 0,
					},
				},
			},
			expectedStatus: dtcpb.DiagnosticRoutineStatus_ROUTINE_STATUS_FAILED,
		},
		{
			name: "cpu_stress",
			request: dtcpb.RunRoutineRequest{
				Routine: dtcpb.DiagnosticRoutine_ROUTINE_CPU_STRESS,
				Parameters: &dtcpb.RunRoutineRequest_CpuParams{
					CpuParams: &dtcpb.CpuRoutineParameters{
						LengthSeconds: 1,
					},
				},
			},
			expectedStatus: dtcpb.DiagnosticRoutineStatus_ROUTINE_STATUS_PASSED,
		},
		{
			name: "cpu_stress_fail",
			request: dtcpb.RunRoutineRequest{
				Routine: dtcpb.DiagnosticRoutine_ROUTINE_CPU_STRESS,
				Parameters: &dtcpb.RunRoutineRequest_CpuParams{
					CpuParams: &dtcpb.CpuRoutineParameters{
						LengthSeconds: 0,
					},
				},
			},
			expectedStatus: dtcpb.DiagnosticRoutineStatus_ROUTINE_STATUS_FAILED,
		},
		{
			name: "floating_point_accuracy",
			request: dtcpb.RunRoutineRequest{
				Routine: dtcpb.DiagnosticRoutine_ROUTINE_FLOATING_POINT_ACCURACY,
				Parameters: &dtcpb.RunRoutineRequest_FloatingPointAccuracyParams{
					FloatingPointAccuracyParams: &dtcpb.FloatingPointAccuracyRoutineParameters{
						LengthSeconds: 1,
					},
				},
			},
			expectedStatus: dtcpb.DiagnosticRoutineStatus_ROUTINE_STATUS_PASSED,
		},
	} {
		// Here we time how long the execution of each routine takes as they are
		// run in the same test.

		s.Run(ctx, param.name, func(ctx context.Context, s *testing.State) {
			if err := executeRoutine(ctx, param.request, param.expectedStatus); err != nil {
				s.Error("Routine test failed: ", err)
			}
		})
	}
}

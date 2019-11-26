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
	// checks the result against shouldFail.
	executeRoutine := func(ctx context.Context,
		rrRequest dtcpb.RunRoutineRequest, shouldFail bool) error {

		ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
		defer cancel()

		rrResponse := dtcpb.RunRoutineResponse{}
		err := routines.CallRunRoutine(ctx, rrRequest, &rrResponse)
		if err != nil {
			return errors.Wrap(err, "unable to run routine: ")
		}

		uuid := rrResponse.Uuid
		response := dtcpb.GetRoutineUpdateResponse{}

		err = routines.WaitUntilRoutineChangesState(ctx, uuid, dtcpb.DiagnosticRoutineStatus_ROUTINE_STATUS_RUNNING, 2*time.Second)
		if err != nil {
			return errors.Wrap(err, "routine not finished")
		}

		err = routines.GetRoutineStatus(ctx, uuid, true, &response)
		if err != nil {
			return errors.Wrap(err, "unable to get routine status: ")
		}

		if shouldFail {
			if response.Status != dtcpb.DiagnosticRoutineStatus_ROUTINE_STATUS_FAILED {
				return errors.Errorf("invalid status; got %s, want ROUTINE_STATUS_FAILED", response.Status)
			}
		} else {
			if response.Status != dtcpb.DiagnosticRoutineStatus_ROUTINE_STATUS_PASSED {
				return errors.Errorf("invalid status; got %s, want ROUTINE_STATUS_PASSED", response.Status)
			}
		}

		err = routines.RemoveRoutine(ctx, uuid)
		if err != nil {
			return errors.Wrap(err, "unable to remove routine: ")
		}

		return nil
	}

	for _, param := range []struct {
		name       string
		request    dtcpb.RunRoutineRequest
		shouldFail bool
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
			shouldFail: false,
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
			shouldFail: false,
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
			shouldFail: true,
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
			shouldFail: false,
		},
		{
			name: "battery_sysfs_fail",
			request: dtcpb.RunRoutineRequest{
				Routine: dtcpb.DiagnosticRoutine_ROUTINE_BATTERY_SYSFS,
				Parameters: &dtcpb.RunRoutineRequest_BatterySysfsParams{
					BatterySysfsParams: &dtcpb.BatterySysfsRoutineParameters{
						MaximumCycleCount:         1,
						PercentBatteryWearAllowed: 0,
					},
				},
			},
			// MaximumCycleCount is 1 (all devices should have used their
			// battery more than once).
			shouldFail: true,
		},
		{
			name: "smartctl",
			request: dtcpb.RunRoutineRequest{
				Routine: dtcpb.DiagnosticRoutine_ROUTINE_SMARTCTL_CHECK,
				Parameters: &dtcpb.RunRoutineRequest_SmartctlCheckParams{
					SmartctlCheckParams: &dtcpb.SmartctlCheckRoutineParameters{},
				},
			},
			shouldFail: false,
		},
	} {
		// Here we time how long the execution of each routine takes as they are
		// run in the same test.

		s.Run(ctx, param.name, func(s *testing.State, ctx context.Context) {
			if err := executeRoutine(ctx, param.request, param.shouldFail); err != nil {
				s.Errorf("Routine test failed for %s: %v", param.name, err)
			}
		})
	}
}

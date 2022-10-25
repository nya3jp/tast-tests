// Copyright 2021 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wilco

import (
	"context"
	"os"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/wilco/routines"
	"chromiumos/tast/local/wilco"
	"chromiumos/tast/testing"
	dtcpb "chromiumos/wilco_dtc"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         APIRoutineEnrolled,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Test sending RunRoutineRequest and GetRoutineUpdate gRPC requests from Wilco DTC VM to the Wilco DTC Support Daemon",
		Contacts: []string{
			"chromeos-oem-services@google.com", // Use team email for tickets.
			"bkersting@google.com",
			"lamzin@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"reboot", "vm_host", "wilco", "chrome"},
		Fixture:      "wilcoDTCAllowed",
	})
}

func APIRoutineEnrolled(ctx context.Context, s *testing.State) {
	// The function fetches all available routines and tests the existence of the diagnostic routines
	// that the platform may support.
	if err := func(ctx context.Context) error {
		response := dtcpb.GetAvailableRoutinesResponse{}

		if err := wilco.DPSLSendMessage(ctx, "GetAvailableRoutines", &dtcpb.GetAvailableRoutinesRequest{}, &response); err != nil {
			return errors.Wrap(err, "failed to get available routines")
		}

		contains := func(all []dtcpb.DiagnosticRoutine, expected dtcpb.DiagnosticRoutine) bool {
			for _, e := range all {
				if expected == e {
					return true
				}
			}
			return false
		}

		for _, want := range []dtcpb.DiagnosticRoutine{
			dtcpb.DiagnosticRoutine_ROUTINE_BATTERY,
			dtcpb.DiagnosticRoutine_ROUTINE_BATTERY_SYSFS,
			dtcpb.DiagnosticRoutine_ROUTINE_URANDOM,
			dtcpb.DiagnosticRoutine_ROUTINE_SMARTCTL_CHECK,
			dtcpb.DiagnosticRoutine_ROUTINE_CPU_CACHE,
			dtcpb.DiagnosticRoutine_ROUTINE_CPU_STRESS,
			dtcpb.DiagnosticRoutine_ROUTINE_FLOATING_POINT_ACCURACY,
			dtcpb.DiagnosticRoutine_ROUTINE_NVME_WEAR_LEVEL,
			dtcpb.DiagnosticRoutine_ROUTINE_NVME_SHORT_SELF_TEST,
			dtcpb.DiagnosticRoutine_ROUTINE_NVME_LONG_SELF_TEST,
		} {
			if !contains(response.Routines, want) {
				return errors.Errorf("routine %s missing", want)
			}
		}
		return nil
	}(ctx); err != nil {
		s.Error("Failed to get available diagnostic routines: ", err)
	}

	for _, tc := range []struct {
		name                         string
		request                      *dtcpb.RunRoutineRequest
		wantRoutineStatus            dtcpb.DiagnosticRoutineStatus
		postRoutineValidityCheckFunc func() error
	}{
		{
			name: "urandom",
			request: &dtcpb.RunRoutineRequest{
				Routine: dtcpb.DiagnosticRoutine_ROUTINE_URANDOM,
				Parameters: &dtcpb.RunRoutineRequest_UrandomParams{
					UrandomParams: &dtcpb.UrandomRoutineParameters{
						LengthSeconds: 1,
					},
				},
			},
			wantRoutineStatus: dtcpb.DiagnosticRoutineStatus_ROUTINE_STATUS_PASSED,
		},
		{
			name: "urandom_cancel",
			request: &dtcpb.RunRoutineRequest{
				Routine: dtcpb.DiagnosticRoutine_ROUTINE_URANDOM,
				Parameters: &dtcpb.RunRoutineRequest_UrandomParams{
					UrandomParams: &dtcpb.UrandomRoutineParameters{
						LengthSeconds: 5,
					},
				},
			},
			wantRoutineStatus: dtcpb.DiagnosticRoutineStatus_ROUTINE_STATUS_CANCELLED,
		},
		{
			name: "battery",
			request: &dtcpb.RunRoutineRequest{
				Routine: dtcpb.DiagnosticRoutine_ROUTINE_BATTERY,
				Parameters: &dtcpb.RunRoutineRequest_BatteryParams{
					BatteryParams: &dtcpb.BatteryRoutineParameters{},
				},
			},
			wantRoutineStatus: dtcpb.DiagnosticRoutineStatus_ROUTINE_STATUS_PASSED,
		},
		{
			name: "battery_sysfs",
			request: &dtcpb.RunRoutineRequest{
				Routine: dtcpb.DiagnosticRoutine_ROUTINE_BATTERY_SYSFS,
				Parameters: &dtcpb.RunRoutineRequest_BatterySysfsParams{
					BatterySysfsParams: &dtcpb.BatterySysfsRoutineParameters{},
				},
			},
			wantRoutineStatus: dtcpb.DiagnosticRoutineStatus_ROUTINE_STATUS_PASSED,
		},
		{
			name: "smartctl",
			request: &dtcpb.RunRoutineRequest{
				Routine: dtcpb.DiagnosticRoutine_ROUTINE_SMARTCTL_CHECK,
				Parameters: &dtcpb.RunRoutineRequest_SmartctlCheckParams{
					SmartctlCheckParams: &dtcpb.SmartctlCheckRoutineParameters{},
				},
			},
			wantRoutineStatus: dtcpb.DiagnosticRoutineStatus_ROUTINE_STATUS_PASSED,
		},
		// Success is not tested because the CPU cache routine takes too much time.
		{
			name: "cpu_cache_fail",
			request: &dtcpb.RunRoutineRequest{
				Routine: dtcpb.DiagnosticRoutine_ROUTINE_CPU_CACHE,
				Parameters: &dtcpb.RunRoutineRequest_CpuParams{
					CpuParams: &dtcpb.CpuRoutineParameters{
						LengthSeconds: 0,
					},
				},
			},
			// The length of seconds is zero (the length of seconds for the test
			// should larger than zero).
			wantRoutineStatus: dtcpb.DiagnosticRoutineStatus_ROUTINE_STATUS_FAILED,
		},
		{
			name: "cpu_cache_cancelled",
			request: &dtcpb.RunRoutineRequest{
				Routine: dtcpb.DiagnosticRoutine_ROUTINE_CPU_CACHE,
				Parameters: &dtcpb.RunRoutineRequest_CpuParams{
					CpuParams: &dtcpb.CpuRoutineParameters{
						LengthSeconds: 1,
					},
				},
			},
			wantRoutineStatus: dtcpb.DiagnosticRoutineStatus_ROUTINE_STATUS_CANCELLED,
		},
		// Success is not tested because the CPU stress routine takes too much time.
		{
			name: "cpu_stress_fail",
			request: &dtcpb.RunRoutineRequest{
				Routine: dtcpb.DiagnosticRoutine_ROUTINE_CPU_STRESS,
				Parameters: &dtcpb.RunRoutineRequest_CpuParams{
					CpuParams: &dtcpb.CpuRoutineParameters{
						LengthSeconds: 0,
					},
				},
			},
			// The length of seconds is zero (the length of seconds for the test
			// should larger than zero).
			wantRoutineStatus: dtcpb.DiagnosticRoutineStatus_ROUTINE_STATUS_FAILED,
		},
		{
			name: "cpu_stress_cancelled",
			request: &dtcpb.RunRoutineRequest{
				Routine: dtcpb.DiagnosticRoutine_ROUTINE_CPU_STRESS,
				Parameters: &dtcpb.RunRoutineRequest_CpuParams{
					CpuParams: &dtcpb.CpuRoutineParameters{
						LengthSeconds: 1,
					},
				},
			},
			wantRoutineStatus: dtcpb.DiagnosticRoutineStatus_ROUTINE_STATUS_CANCELLED,
		},
		{
			name: "floating_point_accuracy",
			request: &dtcpb.RunRoutineRequest{
				Routine: dtcpb.DiagnosticRoutine_ROUTINE_FLOATING_POINT_ACCURACY,
				Parameters: &dtcpb.RunRoutineRequest_FloatingPointAccuracyParams{
					FloatingPointAccuracyParams: &dtcpb.FloatingPointAccuracyRoutineParameters{
						LengthSeconds: 1,
					},
				},
			},
			wantRoutineStatus: dtcpb.DiagnosticRoutineStatus_ROUTINE_STATUS_PASSED,
		},
		{
			name: "floating_point_accuracy_cancelled",
			request: &dtcpb.RunRoutineRequest{
				Routine: dtcpb.DiagnosticRoutine_ROUTINE_FLOATING_POINT_ACCURACY,
				Parameters: &dtcpb.RunRoutineRequest_FloatingPointAccuracyParams{
					FloatingPointAccuracyParams: &dtcpb.FloatingPointAccuracyRoutineParameters{
						LengthSeconds: 5,
					},
				},
			},
			wantRoutineStatus: dtcpb.DiagnosticRoutineStatus_ROUTINE_STATUS_CANCELLED,
		},
		// Success is not tested because there are many DUTs in the lab that
		// have SSD with wear level >99%.
		{
			name: "nvme_wear_level_failed",
			request: &dtcpb.RunRoutineRequest{
				Routine: dtcpb.DiagnosticRoutine_ROUTINE_NVME_WEAR_LEVEL,
				Parameters: &dtcpb.RunRoutineRequest_NvmeWearLevelParams{
					NvmeWearLevelParams: &dtcpb.NvmeWearLevelRoutineParameters{
						WearLevelThreshold: 0,
					},
				},
			},
			// The result will fail due to the threshold of the wear level is zero as
			// well as the wear level value always larger or equal to zero.
			wantRoutineStatus: dtcpb.DiagnosticRoutineStatus_ROUTINE_STATUS_FAILED,
		},
		// Success is not tested because the NVMe short self-test routine takes too
		// much time.
		{
			name: "nvme_short_self_test_cancelled",
			request: &dtcpb.RunRoutineRequest{
				Routine: dtcpb.DiagnosticRoutine_ROUTINE_NVME_SHORT_SELF_TEST,
				Parameters: &dtcpb.RunRoutineRequest_NvmeShortSelfTestParams{
					NvmeShortSelfTestParams: &dtcpb.NvmeShortSelfTestRoutineParameters{},
				},
			},
			wantRoutineStatus: dtcpb.DiagnosticRoutineStatus_ROUTINE_STATUS_CANCELLED,
		},
		// Success is not tested because the NVMe long self-test routine takes too
		// much time.
		{
			name: "nvme_long_self_test_cancelled",
			request: &dtcpb.RunRoutineRequest{
				Routine: dtcpb.DiagnosticRoutine_ROUTINE_NVME_LONG_SELF_TEST,
				Parameters: &dtcpb.RunRoutineRequest_NvmeLongSelfTestParams{
					NvmeLongSelfTestParams: &dtcpb.NvmeLongSelfTestRoutineParameters{},
				},
			},
			wantRoutineStatus: dtcpb.DiagnosticRoutineStatus_ROUTINE_STATUS_CANCELLED,
		},
		{
			name: "disk_read_linear",
			request: &dtcpb.RunRoutineRequest{
				Routine: dtcpb.DiagnosticRoutine_ROUTINE_DISK_LINEAR_READ,
				Parameters: &dtcpb.RunRoutineRequest_DiskLinearReadParams{
					DiskLinearReadParams: &dtcpb.DiskLinearReadRoutineParameters{
						LengthSeconds: 1,
						FileSizeMb:    1,
					},
				},
			},
			wantRoutineStatus: dtcpb.DiagnosticRoutineStatus_ROUTINE_STATUS_PASSED,
		},
		{
			name: "disk_read_random",
			request: &dtcpb.RunRoutineRequest{
				Routine: dtcpb.DiagnosticRoutine_ROUTINE_DISK_RANDOM_READ,
				Parameters: &dtcpb.RunRoutineRequest_DiskRandomReadParams{
					DiskRandomReadParams: &dtcpb.DiskRandomReadRoutineParameters{
						LengthSeconds: 1,
						FileSizeMb:    1,
					},
				},
			},
			wantRoutineStatus: dtcpb.DiagnosticRoutineStatus_ROUTINE_STATUS_PASSED,
		},
		{
			name: "disk_read_linear_cancelled",
			request: &dtcpb.RunRoutineRequest{
				Routine: dtcpb.DiagnosticRoutine_ROUTINE_DISK_LINEAR_READ,
				Parameters: &dtcpb.RunRoutineRequest_DiskLinearReadParams{
					DiskLinearReadParams: &dtcpb.DiskLinearReadRoutineParameters{
						LengthSeconds: 5,
						FileSizeMb:    1,
					},
				},
			},
			wantRoutineStatus: dtcpb.DiagnosticRoutineStatus_ROUTINE_STATUS_CANCELLED,
			// Disk read test routine will create a test file, ensure it is deleted after cancellation.
			postRoutineValidityCheckFunc: func() error {
				const testFile = "/var/cache/diagnostics_disk_read_routine_data/fio-test-file"
				if _, err := os.Stat(testFile); os.IsNotExist(err) {
					return nil
				}

				return errors.Errorf("test file %s still exists", testFile)
			},
		},
	} {
		s.Run(ctx, tc.name, func(ctx context.Context, s *testing.State) {
			rrResponse := dtcpb.RunRoutineResponse{}
			// Sending the request and executing the routine.
			if err := routines.CallRunRoutine(ctx, tc.request, &rrResponse); err != nil {
				s.Fatal("Failed to call routine: ", err)
			}

			uuid := rrResponse.Uuid

			defer func(ctx context.Context) {
				if err := routines.RemoveRoutine(ctx, uuid); err != nil {
					s.Error("Failed to remove routine: ", err)
				}
			}(ctx)

			wantRoutineStatus := dtcpb.DiagnosticRoutineStatus_ROUTINE_STATUS_RUNNING
			if tc.wantRoutineStatus == dtcpb.DiagnosticRoutineStatus_ROUTINE_STATUS_CANCELLED {
				if err := routines.CancelRoutine(ctx, uuid); err != nil {
					s.Error("Unable to cancel routine: ", err)
				}
				wantRoutineStatus = dtcpb.DiagnosticRoutineStatus_ROUTINE_STATUS_CANCELLING
			}

			if tc.postRoutineValidityCheckFunc != nil {
				defer func() {
					if err := tc.postRoutineValidityCheckFunc(); err != nil {
						s.Error("Post routine validity check failed: ", err)
					}
				}()
			}

			if err := routines.WaitUntilRoutineChangesState(ctx, uuid, wantRoutineStatus, 30*time.Second); err != nil {
				s.Fatalf("Failed to wait until routine change status to %v: %v", wantRoutineStatus, err)
			}

			response := dtcpb.GetRoutineUpdateResponse{}
			if err := routines.GetRoutineStatus(ctx, uuid, true, &response); err != nil {
				s.Fatal("Failed to get routine status: ", err)
			}

			if response.Status != tc.wantRoutineStatus {
				s.Errorf("Unexpected status = got %v, want %v", response.Status, tc.wantRoutineStatus)
			}
		})
	}
}

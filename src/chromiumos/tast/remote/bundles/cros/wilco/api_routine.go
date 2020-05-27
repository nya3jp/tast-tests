// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wilco

import (
	"context"
	"encoding/json"
	"os"
	"time"

	"github.com/golang/protobuf/proto"
	"github.com/golang/protobuf/ptypes/empty"

	"chromiumos/tast/common/policy"
	"chromiumos/tast/common/policy/fakedms"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/remote/policyutil"
	"chromiumos/tast/rpc"
	ps "chromiumos/tast/services/cros/policy"
	"chromiumos/tast/services/cros/wilco"
	"chromiumos/tast/testing"
	dtcpb "chromiumos/wilco_dtc"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: APIRoutine,
		Desc: "Test sending RunRoutineRequest and GetRoutineUpdate gRPC requests from Wilco DTC VM to the Wilco DTC Support Daemon",
		Contacts: []string{
			"vsavu@chromium.org",  // Test author
			"pmoy@chromium.org",   // wilco_dtc_supportd author
			"lamzin@chromium.org", // wilco_dtc_supportd maintainer
			"chromeos-wilco@google.com",
		},
		Attr:         []string{"group:enrollment"},
		SoftwareDeps: []string{"reboot", "vm_host", "wilco", "chrome"},
		ServiceDeps:  []string{"tast.cros.wilco.WilcoService", "tast.cros.policy.PolicyService"},
		Timeout:      10 * time.Minute,
	})
}

func APIRoutine(ctx context.Context, s *testing.State) {
	defer func(ctx context.Context) {
		if err := policyutil.EnsureTPMIsResetAndPowerwash(ctx, s.DUT()); err != nil {
			s.Error("Failed to reset TPM: ", err)
		}
	}(ctx)

	ctx, cancel := ctxutil.Shorten(ctx, 3*time.Minute)
	defer cancel()

	if err := policyutil.EnsureTPMIsResetAndPowerwash(ctx, s.DUT()); err != nil {
		s.Fatal("Failed to reset TPM: ", err)
	}

	cl, err := rpc.Dial(ctx, s.DUT(), s.RPCHint(), "cros")
	if err != nil {
		s.Fatal("Failed to connect to the RPC service on the DUT: ", err)
	}
	defer cl.Close(ctx)

	wc := wilco.NewWilcoServiceClient(cl.Conn)
	pc := ps.NewPolicyServiceClient(cl.Conn)

	pb := fakedms.NewPolicyBlob()
	pb.AddPolicy(&policy.DeviceWilcoDtcAllowed{Val: true})
	// wilco_dtc and wilco_dtc_supportd only run for affiliated users
	pb.DeviceAffiliationIds = []string{"default"}
	pb.UserAffiliationIds = []string{"default"}

	pJSON, err := json.Marshal(pb)
	if err != nil {
		s.Fatal("Failed to serialize policies: ", err)
	}

	if _, err := pc.EnrollUsingChrome(ctx, &ps.EnrollUsingChromeRequest{
		PolicyJson: pJSON,
	}); err != nil {
		s.Fatal("Failed to enroll using chrome: ", err)
	}
	defer pc.StopChromeAndFakeDMS(ctx, &empty.Empty{})

	if _, err := wc.TestGetAvailableRoutines(ctx, &empty.Empty{}); err != nil {
		s.Error("Get available routines test failed: ", err)
	}

	type sanityCheckFn func() error

	// testRoutineExecution sends the request in rrRequest, executing the routine,
	// and checks the result against expectedStatus. Some routines would not get
	// back to service right away, so shortening test time by cancelling them and
	// check if they are in cancelled status respectively.
	testRoutineExecution := func(ctx context.Context,
		rrRequest dtcpb.RunRoutineRequest,
		expectedStatus wilco.DiagnosticRoutineStatus,
		postRoutineSanityCheck sanityCheckFn) error {

		ctx, cancel := context.WithTimeout(ctx, 35*time.Second)
		defer cancel()

		data, err := proto.Marshal(&rrRequest)
		if err != nil {
			return errors.Wrap(err, "failed to marshall")
		}

		if expectedStatus == wilco.DiagnosticRoutineStatus_ROUTINE_STATUS_CANCELLED {
			_, err = wc.TestRoutineCancellation(ctx, &wilco.ExecuteRoutineRequest{
				Request: data,
			})
			if err != nil {
				return errors.Wrap(err, "failed to cancel routine")
			}

			if postRoutineSanityCheck != nil {
				if err := postRoutineSanityCheck(); err != nil {
					return errors.Wrap(err, "post routine sanity check failed")
				}
			}
			return nil
		}

		resp, err := wc.ExecuteRoutine(ctx, &wilco.ExecuteRoutineRequest{
			Request: data,
		})
		if err != nil {
			return errors.Wrap(err, "failed to execute routine")
		}

		if resp.Status != expectedStatus {
			return errors.Errorf("unexpected status: got %s, want %s", resp.Status, expectedStatus)
		}

		return nil
	}

	for _, param := range []struct {
		name                string
		request             dtcpb.RunRoutineRequest
		expectedStatus      wilco.DiagnosticRoutineStatus
		sanityCheckFunction sanityCheckFn
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
			expectedStatus: wilco.DiagnosticRoutineStatus_ROUTINE_STATUS_PASSED,
		},
		{
			name: "urandom_cancel",
			request: dtcpb.RunRoutineRequest{
				Routine: dtcpb.DiagnosticRoutine_ROUTINE_URANDOM,
				Parameters: &dtcpb.RunRoutineRequest_UrandomParams{
					UrandomParams: &dtcpb.UrandomRoutineParameters{
						LengthSeconds: 5,
					},
				},
			},
			expectedStatus: wilco.DiagnosticRoutineStatus_ROUTINE_STATUS_CANCELLED,
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
			expectedStatus: wilco.DiagnosticRoutineStatus_ROUTINE_STATUS_PASSED,
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
			expectedStatus: wilco.DiagnosticRoutineStatus_ROUTINE_STATUS_FAILED,
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
			expectedStatus: wilco.DiagnosticRoutineStatus_ROUTINE_STATUS_PASSED,
		},
		{
			name: "smartctl",
			request: dtcpb.RunRoutineRequest{
				Routine: dtcpb.DiagnosticRoutine_ROUTINE_SMARTCTL_CHECK,
				Parameters: &dtcpb.RunRoutineRequest_SmartctlCheckParams{
					SmartctlCheckParams: &dtcpb.SmartctlCheckRoutineParameters{},
				},
			},
			expectedStatus: wilco.DiagnosticRoutineStatus_ROUTINE_STATUS_PASSED,
		},
		// Success is not tested because the CPU cache routine takes too much time.
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
			// The length of seconds is zero (the length of seconds for the test
			// should larger than zero).
			expectedStatus: wilco.DiagnosticRoutineStatus_ROUTINE_STATUS_FAILED,
		},
		{
			name: "cpu_cache_cancelled",
			request: dtcpb.RunRoutineRequest{
				Routine: dtcpb.DiagnosticRoutine_ROUTINE_CPU_CACHE,
				Parameters: &dtcpb.RunRoutineRequest_CpuParams{
					CpuParams: &dtcpb.CpuRoutineParameters{
						LengthSeconds: 1,
					},
				},
			},
			expectedStatus: wilco.DiagnosticRoutineStatus_ROUTINE_STATUS_CANCELLED,
		},
		// Success is not tested because the CPU stress routine takes too much time.
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
			// The length of seconds is zero (the length of seconds for the test
			// should larger than zero).
			expectedStatus: wilco.DiagnosticRoutineStatus_ROUTINE_STATUS_FAILED,
		},
		{
			name: "cpu_stress_cancelled",
			request: dtcpb.RunRoutineRequest{
				Routine: dtcpb.DiagnosticRoutine_ROUTINE_CPU_STRESS,
				Parameters: &dtcpb.RunRoutineRequest_CpuParams{
					CpuParams: &dtcpb.CpuRoutineParameters{
						LengthSeconds: 1,
					},
				},
			},
			expectedStatus: wilco.DiagnosticRoutineStatus_ROUTINE_STATUS_CANCELLED,
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
			expectedStatus: wilco.DiagnosticRoutineStatus_ROUTINE_STATUS_PASSED,
		},
		{
			name: "floating_point_accuracy_cancelled",
			request: dtcpb.RunRoutineRequest{
				Routine: dtcpb.DiagnosticRoutine_ROUTINE_FLOATING_POINT_ACCURACY,
				Parameters: &dtcpb.RunRoutineRequest_FloatingPointAccuracyParams{
					FloatingPointAccuracyParams: &dtcpb.FloatingPointAccuracyRoutineParameters{
						LengthSeconds: 5,
					},
				},
			},
			expectedStatus: wilco.DiagnosticRoutineStatus_ROUTINE_STATUS_CANCELLED,
		},
		{
			name: "nvme_wear_level",
			request: dtcpb.RunRoutineRequest{
				Routine: dtcpb.DiagnosticRoutine_ROUTINE_NVME_WEAR_LEVEL,
				Parameters: &dtcpb.RunRoutineRequest_NvmeWearLevelParams{
					NvmeWearLevelParams: &dtcpb.NvmeWearLevelRoutineParameters{
						WearLevelThreshold: 99,
					},
				},
			},
			expectedStatus: wilco.DiagnosticRoutineStatus_ROUTINE_STATUS_PASSED,
		},
		{
			name: "nvme_wear_level_failed",
			request: dtcpb.RunRoutineRequest{
				Routine: dtcpb.DiagnosticRoutine_ROUTINE_NVME_WEAR_LEVEL,
				Parameters: &dtcpb.RunRoutineRequest_NvmeWearLevelParams{
					NvmeWearLevelParams: &dtcpb.NvmeWearLevelRoutineParameters{
						WearLevelThreshold: 0,
					},
				},
			},
			// The result will fail due to the threshold of the wear level is zero as
			// well as the wear level value always larger or equal to zero.
			expectedStatus: wilco.DiagnosticRoutineStatus_ROUTINE_STATUS_FAILED,
		},
		// Success is not tested because the NVMe short self-test routine takes too
		// much time.
		{
			name: "nvme_short_self_test_cancelled",
			request: dtcpb.RunRoutineRequest{
				Routine: dtcpb.DiagnosticRoutine_ROUTINE_NVME_SHORT_SELF_TEST,
				Parameters: &dtcpb.RunRoutineRequest_NvmeShortSelfTestParams{
					NvmeShortSelfTestParams: &dtcpb.NvmeShortSelfTestRoutineParameters{},
				},
			},
			expectedStatus: wilco.DiagnosticRoutineStatus_ROUTINE_STATUS_CANCELLED,
		},
		// Success is not tested because the NVMe long self-test routine takes too
		// much time.
		{
			name: "nvme_long_self_test_cancelled",
			request: dtcpb.RunRoutineRequest{
				Routine: dtcpb.DiagnosticRoutine_ROUTINE_NVME_LONG_SELF_TEST,
				Parameters: &dtcpb.RunRoutineRequest_NvmeLongSelfTestParams{
					NvmeLongSelfTestParams: &dtcpb.NvmeLongSelfTestRoutineParameters{},
				},
			},
			expectedStatus: wilco.DiagnosticRoutineStatus_ROUTINE_STATUS_CANCELLED,
		},
		{
			name: "disk_read_linear",
			request: dtcpb.RunRoutineRequest{
				Routine: dtcpb.DiagnosticRoutine_ROUTINE_DISK_LINEAR_READ,
				Parameters: &dtcpb.RunRoutineRequest_DiskLinearReadParams{
					DiskLinearReadParams: &dtcpb.DiskLinearReadRoutineParameters{
						LengthSeconds: 1,
						FileSizeMb:    1,
					},
				},
			},
			expectedStatus: wilco.DiagnosticRoutineStatus_ROUTINE_STATUS_PASSED,
		},
		{
			name: "disk_read_random",
			request: dtcpb.RunRoutineRequest{
				Routine: dtcpb.DiagnosticRoutine_ROUTINE_DISK_RANDOM_READ,
				Parameters: &dtcpb.RunRoutineRequest_DiskRandomReadParams{
					DiskRandomReadParams: &dtcpb.DiskRandomReadRoutineParameters{
						LengthSeconds: 1,
						FileSizeMb:    1,
					},
				},
			},
			expectedStatus: wilco.DiagnosticRoutineStatus_ROUTINE_STATUS_PASSED,
		},
		{
			name: "disk_read_linear_cancelled",
			request: dtcpb.RunRoutineRequest{
				Routine: dtcpb.DiagnosticRoutine_ROUTINE_DISK_LINEAR_READ,
				Parameters: &dtcpb.RunRoutineRequest_DiskLinearReadParams{
					DiskLinearReadParams: &dtcpb.DiskLinearReadRoutineParameters{
						LengthSeconds: 5,
						FileSizeMb:    1,
					},
				},
			},
			expectedStatus: wilco.DiagnosticRoutineStatus_ROUTINE_STATUS_CANCELLED,
			// disk read test routine will create a test file
			// ensure it is deleted after cancellation
			sanityCheckFunction: func() error {
				const testFile = "/var/cache/diagnostics_disk_read_routine_data/fio-test-file"
				if _, err := os.Stat(testFile); os.IsNotExist(err) {
					return nil
				}

				return errors.Errorf("test file %s still exist", testFile)
			},
		},
	} {
		s.Run(ctx, param.name, func(ctx context.Context, s *testing.State) {
			if err := testRoutineExecution(ctx, param.request, param.expectedStatus, param.sanityCheckFunction); err != nil {
				s.Error("Routine test failed: ", err)
			}
		})
	}
}

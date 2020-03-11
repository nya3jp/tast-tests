// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wilco

import (
	"context"
	"encoding/json"
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

	// executeRoutine sends the request in rrRequest, executing the routine, and
	// checks the result against shouldFail.
	executeRoutine := func(ctx context.Context,
		rrRequest dtcpb.RunRoutineRequest, shouldFail bool) error {

		ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
		defer cancel()

		data, err := proto.Marshal(&rrRequest)
		if err != nil {
			return errors.Wrap(err, "failed to marshall")
		}

		resp, err := wc.ExecuteRoutine(ctx, &wilco.ExecuteRoutineRequest{
			Request: data,
		})
		if err != nil {
			return errors.Wrap(err, "failed to execute routine")
		}

		if shouldFail && resp.Status != wilco.DiagnosticRoutineStatus_ROUTINE_STATUS_FAILED {
			return errors.Errorf("invalid result: expected FAILED, got %s", resp.Status)
		} else if !shouldFail && resp.Status != wilco.DiagnosticRoutineStatus_ROUTINE_STATUS_PASSED {
			return errors.Errorf("invalid result: expected PASSED, got %s", resp.Status)
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
		s.Run(ctx, param.name, func(ctx context.Context, s *testing.State) {
			if err := executeRoutine(ctx, param.request, param.shouldFail); err != nil {
				s.Error("Routine test failed: ", err)
			}
		})
	}
}

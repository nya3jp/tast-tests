// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wilco

import (
	"context"

	"chromiumos/tast/local/bundles/cros/wilco/common"
	"chromiumos/tast/local/wilco"
	"chromiumos/tast/testing"
	dtcpb "chromiumos/wilco_dtc"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: APIGetAvailableRoutines,
		Desc: "Test sending GetAvailableRoutines gRPC request from Wilco DTC VM to the Wilco DTC Support Daemon daemon",
		Contacts: []string{
			"vsavu@chromium.org",  // Test author
			"pmoy@chromium.org",   // wilco_dtc_supportd author
			"lamzin@chromium.org", // wilco_dtc_supportd maintainer
			"chromeos-wilco@google.com",
		},
		Attr:         []string{"informational"},
		SoftwareDeps: []string{"vm_host", "wilco"},
	})
}

func APIGetAvailableRoutines(ctx context.Context, s *testing.State) {
	res, err := common.SetupSupportdForAPITest(ctx, s)
	ctx = res.TestContext
	defer common.TeardownSupportdForAPITest(res.CleanupContext, s)
	if err != nil {
		s.Fatal("Failed setup: ", err)
	}

	arMsg := dtcpb.GetAvailableRoutinesRequest{}
	arRes := dtcpb.GetAvailableRoutinesResponse{}

	if err := wilco.DPSLSendMessage(ctx, "GetAvailableRoutines", &arMsg, &arRes); err != nil {
		s.Fatal("unable to get Routines: ", err)
	}

	contains := func(all []dtcpb.DiagnosticRoutine, expected dtcpb.DiagnosticRoutine) bool {
		for _, e := range all {
			if expected == e {
				return true
			}
		}
		return false
	}

	expectedRoutines := []dtcpb.DiagnosticRoutine{
		dtcpb.DiagnosticRoutine_ROUTINE_BATTERY,
		dtcpb.DiagnosticRoutine_ROUTINE_BATTERY_SYSFS,
		dtcpb.DiagnosticRoutine_ROUTINE_URANDOM,
		dtcpb.DiagnosticRoutine_ROUTINE_SMARTCTL_CHECK,
	}

	// Error conditions defined by the proto definition.
	if len(arRes.Routines) == 0 {
		s.Fatal("No routines available")
	}

	for _, val := range expectedRoutines {
		if !contains(arRes.Routines, val) {
			s.Fatalf("Routine %s missing", val.String())
		}
	}
}

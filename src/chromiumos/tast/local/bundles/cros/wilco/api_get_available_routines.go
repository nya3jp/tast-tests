// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wilco

import (
	"context"

	"chromiumos/tast/local/bundles/cros/wilco/pre"
	"chromiumos/tast/local/wilco"
	"chromiumos/tast/testing"
	dtcpb "chromiumos/wilco_dtc"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: APIGetAvailableRoutines,
		Desc: "Test sending GetAvailableRoutines gRPC request from Wilco DTC VM to the Wilco DTC Support Daemon",
		Contacts: []string{
			"vsavu@chromium.org",  // Test author
			"pmoy@chromium.org",   // wilco_dtc_supportd author
			"lamzin@chromium.org", // wilco_dtc_supportd maintainer
			"chromeos-wilco@google.com",
		},
		Attr:         []string{"informational"},
		SoftwareDeps: []string{"vm_host", "wilco"},
		Pre:          pre.WilcoDtcSupportdAPI,
	})
}

func APIGetAvailableRoutines(ctx context.Context, s *testing.State) {
	request := dtcpb.GetAvailableRoutinesRequest{}
	response := dtcpb.GetAvailableRoutinesResponse{}

	if err := wilco.DPSLSendMessage(ctx, "GetAvailableRoutines", &request, &response); err != nil {
		s.Fatal("unable to get Routines: ", err)
	}

	if len(response.Routines) == 0 {
		s.Fatal("No routines available")
	}

	contains := func(all []dtcpb.DiagnosticRoutine, expected dtcpb.DiagnosticRoutine) bool {
		for _, e := range all {
			if expected == e {
				return true
			}
		}
		return false
	}

	for _, val := range []dtcpb.DiagnosticRoutine{
		dtcpb.DiagnosticRoutine_ROUTINE_BATTERY,
		dtcpb.DiagnosticRoutine_ROUTINE_BATTERY_SYSFS,
		dtcpb.DiagnosticRoutine_ROUTINE_URANDOM,
		dtcpb.DiagnosticRoutine_ROUTINE_SMARTCTL_CHECK,
	} {
		if !contains(response.Routines, val) {
			s.Fatalf("Routine %s missing", val.String())
		}
	}
}

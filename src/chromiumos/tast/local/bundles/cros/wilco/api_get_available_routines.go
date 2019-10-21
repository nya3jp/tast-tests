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
		Desc: "Test GetAvailableRoutines in WilcoDtcSupportd",
		Contacts: []string{
			"vsavu@chromium.org",  // Test author, wilco_dtc author
			"pmoy@chromium.org",   // wilco_dtc_supportd author
			"lamzin@chromium.org", // wilco_dtc_supportd maintainer
			"chromeos-wilco@google.com",
		},
		Attr:         []string{"informational"},
		SoftwareDeps: []string{"vm_host", "wilco"},
	})
}

func contains(a []dtcpb.DiagnosticRoutine, x dtcpb.DiagnosticRoutine) bool {
	for _, n := range a {
		if x == n {
			return true
		}
	}
	return false
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

	// Error conditions defined by the proto definition.
	if len(arRes.Routines) == 0 {
		s.Fatal("No routines available")
	}

	if !contains(arRes.Routines, dtcpb.DiagnosticRoutine_ROUTINE_BATTERY) {
		s.Fatal("Missing ROUTINE_BATTERY")
	}

	if !contains(arRes.Routines, dtcpb.DiagnosticRoutine_ROUTINE_BATTERY) {
		s.Fatal("Missing ROUTINE_BATTERY")
	}

	if !contains(arRes.Routines, dtcpb.DiagnosticRoutine_ROUTINE_BATTERY_SYSFS) {
		s.Fatal("Missing ROUTINE_BATTERY_SYSFS")
	}

	if !contains(arRes.Routines, dtcpb.DiagnosticRoutine_ROUTINE_URANDOM) {
		s.Fatal("Missing ROUTINE_URANDOM")
	}

	if !contains(arRes.Routines, dtcpb.DiagnosticRoutine_ROUTINE_SMARTCTL_CHECK) {
		s.Fatal("Missing ROUTINE_SMARTCTL_CHECK")
	}
}

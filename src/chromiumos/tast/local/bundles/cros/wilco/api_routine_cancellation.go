// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wilco

import (
	"context"
	"time"

	"chromiumos/tast/local/bundles/cros/wilco/pre"
	"chromiumos/tast/local/bundles/cros/wilco/routines"
	"chromiumos/tast/testing"
	"chromiumos/tast/timing"
	dtcpb "chromiumos/wilco_dtc"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: APIRoutineCancellation,
		Desc: "Test sending RunRoutineRequest and GetRoutineUpdate gRPC requests from Wilco DTC VM to the Wilco DTC Support Daemon daemon, but cancel while executing",
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

func APIRoutineCancellation(ctx context.Context, s *testing.State) {
	ctx, cancel := context.WithTimeout(ctx, 4*time.Second)
	defer cancel()

	rrRequest := dtcpb.RunRoutineRequest{
		Routine: dtcpb.DiagnosticRoutine_ROUTINE_URANDOM,
		Parameters: &dtcpb.RunRoutineRequest_UrandomParams{
			UrandomParams: &dtcpb.UrandomRoutineParameters{
				LengthSeconds: 5,
			},
		},
	}
	rrResponse := dtcpb.RunRoutineResponse{}
	if err := routines.CallRunRoutine(ctx, rrRequest, &rrResponse); err != nil {
		s.Fatal("Unable to call routine: ", err)
	}

	uuid := rrResponse.Uuid
	response := dtcpb.GetRoutineUpdateResponse{}

	if err := routines.CancelRoutine(ctx, uuid); err != nil {
		s.Fatal("Unable to cancel routine: ", err)
	}

	// Because cancellation is slow, we time how long it takes to change from
	// STATUS_CANCELLING.
	ctx, st := timing.Start(ctx, "cancel")
	err := routines.WaitUntilRoutineChangesState(ctx, uuid, dtcpb.DiagnosticRoutineStatus_ROUTINE_STATUS_CANCELLING, 4*time.Second)
	st.End()
	if err != nil {
		s.Fatal("Routine not finished: ", err)
	}

	if err := routines.GetRoutineStatus(ctx, uuid, true, &response); err != nil {
		s.Fatal("Unable to get routine status: ", err)
	}

	if response.Status != dtcpb.DiagnosticRoutineStatus_ROUTINE_STATUS_CANCELLED {
		s.Errorf("Invalid status; got %s, want ROUTINE_STATUS_CANCELLED: ", response.Status)
	}

	if err := routines.RemoveRoutine(ctx, uuid); err != nil {
		s.Error("Unable to remove routine: ", err)
	}

}

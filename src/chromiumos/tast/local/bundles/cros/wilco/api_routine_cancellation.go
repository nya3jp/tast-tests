// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wilco

import (
	"context"
	"os"
	"path/filepath"
	"time"

	"chromiumos/tast/errors"
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
	type fn func() error

	executeRoutine := func(ctx context.Context,
		rrRequest dtcpb.RunRoutineRequest,
		postRoutineSanityCheck fn) error {
		ctx, cancel := context.WithTimeout(ctx, 4*time.Second)
		defer cancel()

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
		if postRoutineSanityCheck != nil {
			if err := postRoutineSanityCheck(); err != nil {
				s.Error("post routine sanity check failed: ", err)
			}
		}

		return nil
	}

	// disk read test routine will create a test file
	// ensure it is deleted after cancellation
	diskReadHouseKeepingCheck := func() error {
		testFile := filepath.Join("/var/cache/diagnostics_disk_read_routine_data", "fio-test-file")
		_, err := os.Stat(testFile)
		if os.IsNotExist(err) {
			return nil
		}

		return errors.Errorf("test file %s still existed", testFile)
	}

	for _, param := range []struct {
		name                string
		request             dtcpb.RunRoutineRequest
		sanityCheckFunction fn
	}{
		{
			name: "urandom",
			request: dtcpb.RunRoutineRequest{
				Routine: dtcpb.DiagnosticRoutine_ROUTINE_URANDOM,
				Parameters: &dtcpb.RunRoutineRequest_UrandomParams{
					UrandomParams: &dtcpb.UrandomRoutineParameters{
						LengthSeconds: 5,
					},
				},
			},
			sanityCheckFunction: nil,
		},
		{
			name: "disk_read_linear",
			request: dtcpb.RunRoutineRequest{
				Routine: dtcpb.DiagnosticRoutine_ROUTINE_DISK_LINEAR_READ,
				Parameters: &dtcpb.RunRoutineRequest_DiskLinearReadParams{
					DiskLinearReadParams: &dtcpb.DiskLinearReadRoutineParameters{
						LengthSeconds: 5,
						FileSizeMb:    1,
					},
				},
			},
			sanityCheckFunction: diskReadHouseKeepingCheck,
		},
	} {
		// Here we time how long the execution of each routine takes as they are
		// run in the same test.

		s.Run(ctx, param.name, func(ctx context.Context, s *testing.State) {
			if err := executeRoutine(ctx, param.request, param.sanityCheckFunction); err != nil {
				s.Error("Routine test failed: ", err)
			}
		})
	}
}

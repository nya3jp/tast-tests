// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package health tests the system daemon cros_healthd to ensure that telemetry
// and diagnostics calls can be completed successfully.
package health

import (
	"context"

	"chromiumos/tast/local/croshealthd"
	"chromiumos/tast/testing"
)

// newCancelRoutineParams creates and returns a diagnostic routine that will be
// canceled partway through running.
func newCancelRoutineParams(routine string) croshealthd.RoutineParams {
	return croshealthd.RoutineParams{
		Routine: routine,
		Cancel:  true,
	}
}

func init() {
	testing.AddTest(&testing.Test{
		Func: DiagnosticsCancel,
		Desc: "Tests that the supported cros_healthd diagnostic routines can be canceled",
		Contacts: []string{
			"pmoy@chromium.org",   // cros_healthd tool author
			"tbegin@chromium.org", // test author
			"cros-tdm@google.com", // team mailing list
		},
		SoftwareDeps: []string{"diagnostics"},
		Attr:         []string{"group:mainline"},
		Fixture:      "crosHealthdRunning",
		Params: []testing.Param{{
			Name:      "urandom",
			Val:       newCancelRoutineParams(croshealthd.RoutineURandom),
			ExtraAttr: []string{"informational"},
		}},
	})
}

// DiagnosticsCancel is a paramaterized test that test canceling supported diagnostic
// routines.
func DiagnosticsCancel(ctx context.Context, s *testing.State) {
	params := s.Param().(croshealthd.RoutineParams)
	routine := params.Routine
	s.Logf("Running routine: %s", routine)
	result, err := croshealthd.RunDiagRoutine(ctx, params)
	if err != nil {
		s.Fatalf("Unable to run %s routine: %s", routine, err)
	}

	// Check that the routine was canceled.
	if params.Cancel {
		if result.Status != croshealthd.StatusCancelling &&
			result.Status != croshealthd.StatusCancelled {
			s.Fatalf("%q routine has status %q; want %q or %q",
				routine, result.Status, croshealthd.StatusCancelling, croshealthd.StatusCancelled)
		}
		return
	}
}

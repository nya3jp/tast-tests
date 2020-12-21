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

func init() {
	testing.AddTest(&testing.Test{
		Func: DiagnosticsList,
		Desc: "Tests getting supported dignostic routines from cros_healthd",
		Contacts: []string{
			"pmoy@chromium.org",   // cros_healthd tool author
			"tbegin@chromium.org", // test author
			"cros-tdm@google.com", // team mailing list
		},
		SoftwareDeps: []string{"diagnostics"},
		Attr:         []string{"group:mainline", "informational"},
		Fixture:      "crosHealthdRunning",
	})
}

// DiagnosticsList queries cros_healthd for a list of supported diagnostic
// routines.
func DiagnosticsList(ctx context.Context, s *testing.State) {
	routines, err := croshealthd.GetDiagRoutines(ctx)
	if err != nil {
		s.Fatal("Failed to get diag routines: ", err)
	}

	// There are 9 routines supported on all devices.
	if len(routines) < 9 {
		s.Fatalf("Unexpected number of routines, got %d (%v); want >=9", len(routines), routines)
	}
}

// Copyright 2021 The ChromiumOS Authors
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
		Func:         DiagnosticsList,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Tests getting supported dignostic routines from cros_healthd",
		Contacts:     []string{"cros-tdm-tpe-eng@google.com"},
		SoftwareDeps: []string{"chrome", "diagnostics"},
		Attr:         []string{"group:mainline"},
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

	// There are routines supported on all devices.
	if len(routines) == 0 {
		s.Fatal("Unexpected number of routines, got 0; want >0")
	}
}

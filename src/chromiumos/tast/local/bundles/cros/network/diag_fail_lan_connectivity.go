// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package network

import (
	"context"

	"chromiumos/tast/local/bundles/cros/network/diag"
	"chromiumos/tast/local/shill"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: DiagFailLANConnectivity,
		Desc: "Tests that the LANConnectivity network diagnostic test fails when ethernet is disabled",
		Contacts: []string{
			"tbegin@chromium.org",            // test author
			"khegde@chromium.org",            // network diagnostics author
			"stevenjb@chromium.org",          // network-health tech lead
			"cros-network-health@google.com", // network-health team
		},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:mainline", "informational"},
		Fixture:      "networkDiagnosticsShillReset",
	})
}

// DiagFailLANConnectivity tests that when the ethernet technology is disabled,
// the LANConnectivity network diagnostic routine fails.
func DiagFailLANConnectivity(ctx context.Context, s *testing.State) {
	manager, err := shill.NewManager(ctx)
	if err != nil {
		s.Fatal("Failed creating shill manager proxy: ", err)
	}

	// The re-enable callback is not needed since this is handled in the
	// networkDiagnosticsShillReset fixture.
	_, err = manager.DisableTechnologyForTesting(ctx, shill.TechnologyEthernet)
	if err != nil {
		s.Fatal("Failed to disable ethernet technology")
	}

	mojo := s.FixtValue().(*diag.MojoAPI)
	// After the property change is emitted, Chrome still needs to process it.
	// Since Chrome does not emit a change, poll to test whether the expected
	// problem occurs.
	pollParams := diag.PollRoutineParams{
		Routine:  diag.RoutineLanConnectivity,
		Verdict:  diag.VerdictProblem,
		Problems: []int{},
	}
	if err := mojo.PollRoutine(ctx, pollParams); err != nil {
		s.Fatal("Failed to poll routine: ", err)
	}
}

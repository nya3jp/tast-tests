// Copyright 2021 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package network

import (
	"context"

	diagcommon "chromiumos/tast/common/network/diag"
	"chromiumos/tast/local/bundles/cros/network/diag"
	"chromiumos/tast/local/shill"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         DiagFailLANConnectivity,
		LacrosStatus: testing.LacrosVariantNeeded,
		Desc:         "Tests that the LANConnectivity network diagnostic test fails when ethernet is disabled",
		Contacts: []string{
			"khegde@chromium.org",            // test maintainer
			"stevenjb@chromium.org",          // network-health tech lead
			"cros-network-health@google.com", // network-health team
		},
		SoftwareDeps: []string{"chrome"},
		// TODO(b/234551696): Re-enable test.
		Attr:    []string{},
		Fixture: "networkDiagnosticsShillReset",
	})
}

// DiagFailLANConnectivity tests that when the ethernet technology is disabled,
// the LANConnectivity network diagnostic routine fails.
func DiagFailLANConnectivity(ctx context.Context, s *testing.State) {
	manager, err := shill.NewManager(ctx)
	if err != nil {
		s.Fatal("Failed creating shill manager proxy: ", err)
	}

	technologies, err := manager.GetEnabledTechnologies(ctx)
	if err != nil {
		s.Fatal("Failed to get enabled technologies: ", err)
	}

	for _, t := range technologies {
		// The re-enable callback is not needed since this is handled in the
		// networkDiagnosticsShillReset fixture.
		_, err = manager.DisableTechnologyForTesting(ctx, t)
		if err != nil {
			s.Fatalf("Failed to disable %v technology: %s", t, err)
		}
	}

	mojo := s.FixtValue().(*diag.MojoAPI)
	// After the property change is emitted, Chrome still needs to process it.
	// Since Chrome does not emit a change, poll to test whether the expected
	// problem occurs.
	const problemNoLanConnectivity uint32 = 0
	expectedResult := &diagcommon.RoutineResult{
		Verdict:  diagcommon.VerdictProblem,
		Problems: []uint32{problemNoLanConnectivity},
	}
	if err := mojo.PollRoutine(ctx, diagcommon.RoutineLanConnectivity, expectedResult); err != nil {
		s.Fatal("Failed to poll routine: ", err)
	}
}

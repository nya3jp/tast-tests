// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package network

import (
	"context"

	"chromiumos/tast/local/bundles/cros/network/diag"
	"chromiumos/tast/local/bundles/cros/network/firewall"
	"chromiumos/tast/testing"
)

type failFirewallParams struct {
	Routine    string
	BlockPorts []string
}

func init() {
	testing.AddTest(&testing.Test{
		Func: DiagFailFirewall,
		Desc: "Tests that the http/s fireware network diagnostic test fails when traffic is not allowed on certain ports",
		Contacts: []string{
			"tbegin@chromium.org",            // test author
			"khegde@chromium.org",            // network diagnostics author
			"stevenjb@chromium.org",          // network-health tech lead
			"cros-network-health@google.com", // network-health team
		},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:mainline"},
		Fixture:      "networkDiagnosticsShillReset",
		Params: []testing.Param{{
			Name: "http",
			Val: failFirewallParams{
				Routine:    diag.RoutineHTTPFirewall,
				BlockPorts: []string{"80"},
			},
			ExtraAttr: []string{"informational"},
		}, {
			Name: "https",
			Val: failFirewallParams{
				Routine:    diag.RoutineHTTPSFirewall,
				BlockPorts: []string{"443"},
			},
			ExtraAttr: []string{"informational"},
		}},
	})
}

// DiagFailFirewall tests that when network traffic is blocked on port 80 and
// 443 the network diagnostic routine can detect this condition.
func DiagFailFirewall(ctx context.Context, s *testing.State) {
	params := s.Param().(failFirewallParams)

	createFirewallParams := firewall.CreateFirewallParams{
		// Drop http and https traffic.
		BlockPorts:     params.BlockPorts,
		BlockProtocols: []string{"tcp", "udp"},
		TimeoutSec:     "3",
	}

	if err := firewall.CreateFirewall(ctx, createFirewallParams); err != nil {
		s.Fatal("Failed to create firewall: ", err)
	}

	const problemFirewallDetected = 1

	mojo := s.FixtValue().(*diag.MojoAPI)
	result, err := mojo.RunRoutine(ctx, params.Routine)
	if err != nil {
		s.Fatal("Failed to run routine: ", err)
	}

	if result.Verdict != diag.VerdictProblem {
		s.Fatalf("Expected routine problem verdict; got: %v, want: %v", result.Verdict, diag.VerdictProblem)
	}

	if len(result.Problems) != 1 {
		s.Fatalf("Unexpected problems length, got: %d, want: %d", result.Problems, 1)
	}

	if result.Problems[0] != problemFirewallDetected {
		s.Fatalf("Routine reported unexpected problem; got %v, want %v", result.Problems[0], problemFirewallDetected)
	}
}

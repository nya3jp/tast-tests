// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package network

import (
	"context"
	"time"

	diagcommon "chromiumos/tast/common/network/diag"
	"chromiumos/tast/ctxutil"
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
		Desc: "Tests that the http/s firewall network diagnostic test fails when traffic is not allowed on certain ports",
		Contacts: []string{
			"tbegin@chromium.org",            // test author
			"khegde@chromium.org",            // network diagnostics author
			"cros-network-health@google.com", // network-health team
		},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:mainline"},
		Fixture:      "networkDiagnosticsShillReset",
		Params: []testing.Param{{
			Name: "http",
			Val: failFirewallParams{
				Routine:    diagcommon.RoutineHTTPFirewall,
				BlockPorts: []string{"80"},
			},
			ExtraAttr: []string{"informational"},
		}, {
			Name: "https",
			Val: failFirewallParams{
				Routine:    diagcommon.RoutineHTTPSFirewall,
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

	// Create cleanup context to ensure iptables are restored.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()

	if err := firewall.SaveIptables(ctx, s.OutDir()); err != nil {
		s.Fatal("Unable to save iptables: ", err)
	}

	createFirewallParams := firewall.CreateFirewallParams{
		// Drop http and https traffic.
		BlockPorts:     params.BlockPorts,
		BlockProtocols: []string{"tcp", "udp"},
		Timeout:        3 * time.Second,
	}
	if err := firewall.CreateFirewall(ctx, createFirewallParams); err != nil {
		s.Fatal("Failed to create firewall: ", err)
	}

	defer func() {
		if err := firewall.RestoreIptables(cleanupCtx, s.OutDir()); err != nil {
			s.Log("Failed to restore iptables: ", err)
		}
	}()

	const problemFirewallDetected = 1
	expectedResult := &diagcommon.RoutineResult{
		Verdict:  diagcommon.VerdictProblem,
		Problems: []uint32{problemFirewallDetected},
	}
	mojo := s.FixtValue().(*diag.MojoAPI)
	if err := mojo.PollRoutine(ctx, params.Routine, expectedResult); err != nil {
		s.Fatal("Failed to poll routine: ", err)
	}
}

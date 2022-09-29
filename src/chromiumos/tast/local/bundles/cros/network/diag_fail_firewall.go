// Copyright 2021 The ChromiumOS Authors
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
		Func:         DiagFailFirewall,
		LacrosStatus: testing.LacrosVariantNeeded,
		Desc:         "Tests that the http/s firewall network diagnostic test fails when traffic is not allowed on certain ports",
		Contacts: []string{
			"khegde@chromium.org",            // test maintainer
			"cros-network-health@google.com", // network-health team
		},
		SoftwareDeps: []string{"chrome", "no_qemu"},
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
	ctx, cancel := ctxutil.Shorten(ctx, firewall.IptablesCleanupTimeout)
	defer cancel()

	if errs := firewall.SaveIptables(ctx, s.OutDir()); len(errs) != 0 {
		msg := ""
		for _, err := range errs {
			msg = msg + err.Error()
		}
		s.Fatal("Unable to save iptables state: ", msg)
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
		if errs := firewall.RestoreIptables(cleanupCtx, s.OutDir()); len(errs) != 0 {
			for _, err := range errs {
				s.Log("Failed to restore iptables state: ", err)
			}
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

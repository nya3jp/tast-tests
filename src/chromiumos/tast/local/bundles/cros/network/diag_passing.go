// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package network

import (
	"context"
	"time"

	"chromiumos/tast/local/bundles/cros/network/diag"
	"chromiumos/tast/testing"
)

type netDiagParams struct {
	Routine string
}

func newNetDiagParams(routine string) netDiagParams {
	return netDiagParams{
		Routine: routine,
	}
}

func init() {
	testing.AddTest(&testing.Test{
		Func: DiagPassing,
		Desc: "Tests that the network diagnostic routines can pass in a normal environment",
		Contacts: []string{
			"tbegin@chromium.org",            // test author
			"khegde@chromium.org",            // network diagnostics author
			"stevenjb@chromium.org",          // network-health tech lead
			"cros-network-health@google.com", // network-health team
		},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:mainline", "informational"},
		Fixture:      "networkDiagnostics",
		Params: []testing.Param{{
			Name:      "lan_connectivity",
			Val:       newNetDiagParams(diag.RoutineLanConnectivity),
			ExtraAttr: []string{"informational"},
		}, {
			Name:      "dns_resolver_present",
			Val:       newNetDiagParams(diag.RoutineDNSResolverPresent),
			ExtraAttr: []string{"informational"},
		}, {
			Name:      "dns_resolution",
			Val:       newNetDiagParams(diag.RoutineDNSResolution),
			ExtraAttr: []string{"informational"},
		}, {
			Name:      "dns_latency",
			Val:       newNetDiagParams(diag.RoutineDNSLatency),
			ExtraAttr: []string{"informational"},
		}, {
			Name:      "http_firewall",
			Val:       newNetDiagParams(diag.RoutineHTTPFirewall),
			ExtraAttr: []string{"informational"},
		}, {
			Name:      "https_firewall",
			Val:       newNetDiagParams(diag.RoutineHTTPSFirewall),
			ExtraAttr: []string{"informational"},
		}, {
			Name:      "https_latency",
			Val:       newNetDiagParams(diag.RoutineHTTPSLatency),
			ExtraAttr: []string{"informational"},
		}, {
			Name:      "captive_portal",
			Val:       newNetDiagParams(diag.RoutineCaptivePortal),
			ExtraAttr: []string{"informational"},
		}, {
			Name:      "video_conferencing",
			Val:       newNetDiagParams(diag.RoutineVideoConferencing),
			ExtraAttr: []string{"informational"},
			Timeout:   5 * time.Minute,
		}},
	})
}

// DiagPassing is a parameterized test that ensures that network diagnostic
// routines will pass with a normal network connection.
func DiagPassing(ctx context.Context, s *testing.State) {
	mojo := s.FixtValue().(*diag.MojoAPI)
	params := s.Param().(netDiagParams)
	routine := params.Routine

	result, err := mojo.RunRoutine(ctx, routine)
	if err != nil {
		s.Fatal("Unable to run routine: ", err)
	}

	if err := diag.CheckRoutineVerdict(result.Verdict); err != nil {
		s.Fatal("Unexpected routine verdict: ", err)
	}

	if len(result.Problems) != 0 {
		s.Fatal("Routine reported problems: ", result.Problems)
	}
}

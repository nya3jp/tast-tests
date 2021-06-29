// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package network

import (
	"context"
	"time"

	diagcommon "chromiumos/tast/common/network/diag"
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
		Attr:         []string{"group:mainline"},
		Fixture:      "networkDiagnostics",
		Params: []testing.Param{{
			Name: "lan_connectivity",
			Val:  newNetDiagParams(diagcommon.RoutineLanConnectivity),
		}, {
			Name: "dns_resolver_present",
			Val:  newNetDiagParams(diagcommon.RoutineDNSResolverPresent),
		}, {
			Name: "dns_resolution",
			Val:  newNetDiagParams(diagcommon.RoutineDNSResolution),
		}, {
			Name: "dns_latency",
			Val:  newNetDiagParams(diagcommon.RoutineDNSLatency),
			// TODO(crbug/1203776): Promote test when stable.
			ExtraAttr: []string{"informational"},
		}, {
			Name: "http_firewall",
			Val:  newNetDiagParams(diagcommon.RoutineHTTPFirewall),
		}, {
			Name: "https_firewall",
			Val:  newNetDiagParams(diagcommon.RoutineHTTPSFirewall),
		}, {
			Name: "https_latency",
			Val:  newNetDiagParams(diagcommon.RoutineHTTPSLatency),
			// TODO(crbug/1203776): Promote test when stable.
			ExtraAttr: []string{"informational"},
		}, {
			Name: "captive_portal",
			Val:  newNetDiagParams(diagcommon.RoutineCaptivePortal),
		}, {
			Name: "video_conferencing",
			Val:  newNetDiagParams(diagcommon.RoutineVideoConferencing),
			// TODO(crbug/1178896): Promote test when stable.
			ExtraAttr: []string{"informational"},
			Timeout:   10 * time.Minute,
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

	expectedResult := &diagcommon.RoutineResult{
		Verdict:  diagcommon.VerdictNoProblem,
		Problems: []uint32{},
	}
	if err := diagcommon.CheckRoutineResult(result, expectedResult); err != nil {
		s.Fatal("Routine result did not match: ", err)
	}
}

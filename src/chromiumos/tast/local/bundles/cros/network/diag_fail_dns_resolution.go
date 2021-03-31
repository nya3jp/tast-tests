// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package network

import (
	"context"
	"time"

	"chromiumos/tast/common/shillconst"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/network/diag"
	"chromiumos/tast/local/shill"
	"chromiumos/tast/testing"
)

type dnsResolutionProblem int

const (
	// problemFailedToResolveHost means that the DNS server was unable to
	// resolve the specified host.
	problemFailedToResolveHost dnsResolutionProblem = 0
)

func init() {
	testing.AddTest(&testing.Test{
		Func: DiagFailDNSResolution,
		Desc: "Tests that the DNS resolution network diagnostic test fails when the DNS cannot resolve requests",
		Contacts: []string{
			"tbegin@chromium.org",            // test author
			"khegde@chromium.org",            // network diagnostics author
			"stevenjb@chromium.org",          // network-health tech lead
			"cros-network-health@google.com", // network-health team
		},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:mainline", "informational"},
		Fixture:      "networkDiagnosticsShill",
	})
}

// DiagFailDNSResolution tests that when the domain name server (DNS) cannot
// resolve requests the network diagnostic routine can detect this condition.
func DiagFailDNSResolution(ctx context.Context, s *testing.State) {
	manager, err := shill.NewManager(ctx)
	if err != nil {
		s.Fatal("Failed creating shill manager proxy: ", err)
	}

	props := map[string]interface{}{
		shillconst.ServicePropertyType: "ethernet",
		shillconst.ServicePropertyStaticIPConfig: map[string]interface{}{
			shillconst.IPConfigPropertyNameServers: []string{"0.0.0.0"}}}
	if _, err := manager.ConfigureServiceForProfile(ctx, shillconst.DefaultProfileObjectPath, props); err != nil {
		s.Fatal("Unable to configure shill service: ", err)
	}

	if _, err := manager.WaitForServiceProperties(ctx, props, 5*time.Second); err != nil {
		s.Fatal("Service not found: ", err)
	}

	mojo := s.FixtValue().(*diag.MojoAPI)
	// It takes some time for the change in shill to propagate through Chrome.
	// Poll the routine until we get a problem state.
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		result, err := mojo.RunRoutine(ctx, diag.RoutineDNSResolution)
		if err != nil {
			s.Fatal("Unable to run routine: ", err)
		}

		if result.Verdict != diag.VerdictProblem {
			return errors.Errorf("expected routine problem verdict; got: %v, want: %v", result.Verdict, diag.VerdictProblem)
		}

		if len(result.Problems) != 1 {
			s.Fatal("Routine didn't report problems")
		}

		if result.Problems[0] != int(problemFailedToResolveHost) {
			s.Fatalf("Routine reported unexpected problem; got %v, want %v", result.Problems[0], problemFailedToResolveHost)
		}

		return nil
	}, &testing.PollOptions{Interval: 250 * time.Millisecond, Timeout: 5 * time.Second}); err != nil {
		s.Fatal("Timout waiting for routine to fail: ", err)
	}
}

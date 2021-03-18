// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package network

import (
	"context"

	"chromiumos/tast/local/bundles/cros/network/diag"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: DiagDNSResolverPresent,
		Desc: "Tests that the DNS Resolver Present diagnostic routine can be run",
		Contacts: []string{
			"tbegin@chromium.org",            // test author
			"khegde@chromium.org",            // network diagnostics author
			"stevenjb@chromium.org",          // network-health tech lead
			"cros-network-health@google.com", // network-health team
		},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:mainline", "informational"},
		Fixture:      "networkDiagnostics",
	})
}

// DiagDNSResolverPresent tests the DNS Resolver Present diagnostic routine can be run
// through the mojo API.
func DiagDNSResolverPresent(ctx context.Context, s *testing.State) {
	mojo := s.FixtValue().(*diag.MojoAPI)

	result, err := mojo.DNSResolverPresent(ctx)
	if err != nil {
		s.Fatal("Unable to run DNSResolverPresent routine: ", err)
	}

	if err := diag.CheckRoutineVerdict(result.Verdict); err != nil {
		s.Fatal("Unexpected routine routine verdict: ", err)
	}

	if len(result.Problems) != 0 {
		s.Fatal("DNSResolverPresent routine reported problems: ", result.Problems)
	}
}

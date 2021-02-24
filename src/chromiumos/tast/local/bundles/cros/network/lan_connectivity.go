// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package network

import (
	"context"

	"chromiumos/tast/local/network/diag"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: LanConnectivity,
		Desc: "Tests that the lan connectivity test can be run",
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

// LanConnectivity tests the lan_connectivity diagnostic test can be run through
// the mojo API.
func LanConnectivity(ctx context.Context, s *testing.State) {
	mojo := s.FixtValue().(*diag.MojoAPI)

	verdict, err := mojo.LanConnectivity(ctx)
	if err != nil {
		s.Fatal("Unable to run LanConnectivity routine: ", err)
	}

	if verdict == diag.VerdictProblem {
		s.Fatal("LanConnectivity routine detected a problem")
	} else if verdict == diag.VerdictNotRun {
		s.Fatal("LanConnectivity routine did not run")
	} else if verdict == diag.VerdictUnknown {
		s.Fatal("LanConnectivity routine unknown verdict")
	} else if verdict == diag.VerdictNoProblem {
		s.Log("LanConnectivity routine completed successfully")
	}
}

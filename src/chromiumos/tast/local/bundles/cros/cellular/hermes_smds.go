// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package cellular

import (
	"context"
	"time"

	"chromiumos/tast/common/hermesconst"
	"chromiumos/tast/local/hermes"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: HermesSMDS,
		Desc: "Perform SMDS eSIM operations on test eSIM",
		Contacts: []string{
			"srikanthkumar@google.com",
			"chromeos-cellular-team@google.com",
		},
		Attr:    []string{"group:cellular", "cellular_unstable", "cellular_sim_test_esim"},
		Fixture: "cellular",
		Timeout: 5 * time.Minute,
	})
}

func HermesSMDS(ctx context.Context, s *testing.State) {
	// Get a test euicc.
	euicc, _, err := hermes.GetEUICC(ctx, true)
	if err != nil {
		s.Fatal("Unable to get Hermes euicc: ", err)
	}

	if err := euicc.Call(ctx, hermesconst.EuiccMethodUseTestCerts, true).Err; err != nil {
		s.Fatal("Failed to use test certs: ", err)
	}
	s.Log("Using test certs")

	// Please ensure there is a stork profile for this EID already before running the test.
	eid := ""
	if err := euicc.Property(ctx, hermesconst.EuiccPropertyEid, &eid); err != nil {
		s.Fatal("Failed to read euicc EID")
	}
	s.Log("EID of the euicc: ", eid)

	const numProfiles = 2
	pendingProfiles, err := euicc.PendingProfiles(ctx)
	if err != nil {
		s.Fatal("Failed to get pending profiles: ", err)
	}
	if len(pendingProfiles) < numProfiles {
		s.Fatalf("Got %d profiles, want %d profiles", len(pendingProfiles), numProfiles)
	}

	for _, profile := range pendingProfiles {
		s.Logf("Pending profile %s", profile.String())
	}
}

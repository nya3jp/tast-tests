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
		Attr:    []string{"group:cellular", "cellular_sim_test_esim"},
		Timeout: 10 * time.Minute,
	})
}

func HermesSMDS(ctx context.Context, s *testing.State) {
	// Get a test euicc
	euicc, _, err := hermes.GetEUICC(ctx, true)
	if err != nil {
		s.Fatal("Unable to get Hermes euicc: ", err)
	}

	if err := euicc.Call(ctx, hermesconst.EuiccMethodUseTestCerts, true).Err; err != nil {
		s.Fatal("Failed to use test certs: ", err)
	}
	s.Log("Using test certs")

	if err := euicc.Call(ctx, hermesconst.EuiccMethodResetMemory, 1).Err; err != nil {
		s.Fatal("Failed to reset test euicc: ", err)
	}

	// Need to create EID based profile on stork first then call RequestPendingProfiles.
	eid := ""
	if err := euicc.Get(ctx, hermesconst.EuiccPropertyEid, &eid); err != nil {
		s.Fatal("Failed to read euicc EID")
	}
	s.Log("Eid of the euicc: ", eid)

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

	s.Log("Reset ", euicc)
	if err := euicc.Call(ctx, hermesconst.EuiccMethodResetMemory, 1).Err; err != nil {
		s.Fatal("Failed to reset test euicc: ", err)
	}

	if len(pendingProfiles) != 0 {
		s.Fatalf("Unexpected number of pending profiles, got: %d, want: 0", len(pendingProfiles))
	}

	hermes.CheckNumInstalledProfiles(ctx, s, euicc, 0)
}

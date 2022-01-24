// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package policy

import (
	"context"

	"chromiumos/tast/common/fixture"
	"chromiumos/tast/common/policy"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/policyutil"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         E2eAutoUpdate, // TODO: Parameterized test with user / policies.
		LacrosStatus: testing.LacrosVariantUnknown,
		Desc:         "Test enrolling and signing in with owned test account configured in a way to have specific policies set",
		Contacts: []string{
			"kamilszarek@google.com", // Test author
			"chromeos-commercial-remote-management@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Fixture:      fixture.CleanOwnership,
		VarDeps: []string{
			"policy.e2e_au_username",
			"policy.e2e_au_password",
		},
	})
}

func E2eAutoUpdate(ctx context.Context, s *testing.State) {
	username := s.RequiredVar("policy.e2e_au_username")
	password := s.RequiredVar("policy.e2e_au_password")
	// dmServerURL := "https://crosman-alpha.sandbox.google.com/devicemanagement/data/api"

	cr, err := chrome.New(ctx,
		chrome.GAIAEnterpriseEnroll(chrome.Creds{User: username, Pass: password}),
		chrome.GAIALogin(chrome.Creds{User: username, Pass: password}),
		// chrome.DMSPolicy(dmServerURL), // TODO: Do we need to run this against alpha and prod?
		chrome.ProdPolicy(),
	)
	if err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}

	// Either this option to check policy.
	conn, err := cr.NewConn(ctx, "chrome://policy")
	if err != nil {
		s.Fatal("Failed to navigate to test website: ", err)
	}
	defer conn.Close()

	// Check policy has the right value, or maybe even exist on the list as
	// policies without values are not by default shown.

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}

	// Or this way.
	// Ensure chrome://policy shows correct policy value.
	if err := policyutil.Verify(ctx, tconn, []policy.Policy{&policy.ArcEnabled{Val: true}}); err != nil {
		s.Error("Wrong policy value: ", err)
	}

}

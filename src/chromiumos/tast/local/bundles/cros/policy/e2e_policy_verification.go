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

type systemE2eTestCase struct {
	username string
	password string
	policies []policy.Policy // policies is the policies values.
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         E2ePolicyVerification, // TODO: Parameterized test with user / policies.
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
			"policy.E2e_autoupdate_username",
			"policy.E2e_autoupdate_password",
		},
		Params: []testing.Param{
			{
				Name: "autoupdate",
				Val: systemE2eTestCase{
					username: "policy.E2e_autoupdate_username",
					password: "policy.E2e_autoupdate_password",
					policies: []policy.Policy{ // Put only policies related to auto update.
						&policy.ChromeOsReleaseChannelDelegated{Val: true},
						&policy.DeviceAutoUpdateDisabled{Val: true},
						&policy.DeviceRollbackToTargetVersion{Val: 3},
						&policy.DeviceUpdateHttpDownloadsEnabled{Val: true},
						&policy.DeviceUpdateScatterFactor{Val: 0},
						&policy.RebootAfterUpdate{Val: true},
					},
				},
			},
		},
	})
}

func E2ePolicyVerification(ctx context.Context, s *testing.State) {
	tc := s.Param().(systemE2eTestCase)
	username := s.RequiredVar(tc.username)
	password := s.RequiredVar(tc.password)
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

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}
	// Ensure chrome://policy shows correct policy values.
	if err := policyutil.Verify(ctx, tconn, tc.policies); err != nil {
		s.Error("Wrong policy value: ", err)
	}
}

// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package policy

import (
	"context"

	"chromiumos/tast/common/fixture"
	"chromiumos/tast/common/policy"
	"chromiumos/tast/common/policy/fakedms"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/policyutil"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         FakeEnrollmentRealGAIA,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Tests that real GAIA account can be used along with fake enrollment",
		Contacts: []string{
			"lamzin@google.com", // Test author
			"chromeos-commercial-remote-management@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Fixture:      fixture.FakeDMSEnrolled,
		Vars: []string{
			"policy.FakeEnrollmentRealGAIA.username",
			"policy.FakeEnrollmentRealGAIA.password",
		},
	})
}

// FakeEnrollmentRealGAIA tests that real GAIA account can be used along with fake enrollment.
func FakeEnrollmentRealGAIA(ctx context.Context, s *testing.State) {
	fdms, ok := s.FixtValue().(*fakedms.FakeDMS)
	if !ok {
		s.Fatal("Parent is not a FakeDMS fixture")
	}

	username := s.RequiredVar("policy.FakeEnrollmentRealGAIA.username")
	password := s.RequiredVar("policy.FakeEnrollmentRealGAIA.password")

	pb := fakedms.NewPolicyBlob()
	pb.PolicyUser = username
	pb.DeviceAffiliationIds = []string{"default_affiliation_id"}
	pb.UserAffiliationIds = []string{"default_affiliation_id"}

	// We have to update fake DMS policy user and affiliation IDs before starting Chrome.
	if err := fdms.WritePolicyBlob(pb); err != nil {
		s.Fatal("Failed to write policy blob before starting Chrome: ", err)
	}

	cr, err := chrome.New(ctx,
		chrome.KeepEnrollment(),
		chrome.GAIALogin(chrome.Creds{User: username, Pass: password}),
		chrome.DMSPolicy(fdms.URL),
		chrome.CustomLoginTimeout(chrome.ManagedUserLoginTimeout))
	if err != nil {
		s.Fatal("Chrome startup failed: ", err)
	}
	defer cr.Close(ctx)

	defer faillog.DumpUITreeWithScreenshotOnError(ctx, s.OutDir(), s.HasError, cr, "ui_tree")

	// Connect to Test API.
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect to test API: ", err)
	}

	// Set some policy.
	ps := []policy.Policy{&policy.AllowDinosaurEasterEgg{Val: true}}
	pb.AddPolicies(ps)

	if err := policyutil.ServeBlobAndRefresh(ctx, fdms, cr, pb); err != nil {
		s.Fatal("Failed to serve and refresh: ", err)
	}

	if err := policyutil.Verify(ctx, tconn, ps); err != nil {
		s.Fatal("Failed to serve and refresh: ", err)
	}

	conn, err := cr.NewConn(ctx, "chrome://policy")
	if err != nil {
		s.Fatal("Failed to create connection to policy page: ", err)
	}
	defer conn.Close()

	var got string
	if err := conn.Eval(ctx, `document.querySelector("#status-box-container > fieldset:nth-child(2) > div:nth-child(18) > div.is-affiliated").innerText`, &got); err != nil {
		s.Fatal("Failed to retrieve is affiliated value: ", err)
	}

	if want := " Yes"; got != want {
		s.Errorf("Unexpected is affiliated value: got %q; want %q", got, want)
	}
}

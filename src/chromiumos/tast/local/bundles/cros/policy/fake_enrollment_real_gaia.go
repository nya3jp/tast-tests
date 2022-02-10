// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package policy

import (
	"context"

	"chromiumos/tast/common/fixture"
	"chromiumos/tast/common/policy"
	"chromiumos/tast/common/policy/fakedms"
	"chromiumos/tast/local/chrome"
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
			"policy.FakeEnrollmentRealGAIA.user_name",
			"policy.FakeEnrollmentRealGAIA.password",
		},
	})
}

// FakeEnrollmentRealGAIA tests that real GAIA accouont can be used along with fake enrollment.
func FakeEnrollmentRealGAIA(ctx context.Context, s *testing.State) {
	fdms, ok := s.FixtValue().(*fakedms.FakeDMS)
	if !ok {
		s.Fatal("Parent is not a FakeDMS fixture")
	}

	username := s.RequiredVar("policy.FakeEnrollmentRealGAIA.user_name")
	password := s.RequiredVar("policy.FakeEnrollmentRealGAIA.password")

	cr, err := chrome.New(ctx,
		chrome.KeepEnrollment(),
		chrome.GAIALogin(chrome.Creds{User: username, Pass: password}),
		chrome.DMSPolicy(fdms.URL),
		chrome.ExtraArgs("--login-manager"),
		chrome.CustomLoginTimeout(chrome.ManagedUserLoginTimeout))
	if err != nil {
		s.Fatal("Chrome startup failed: ", err)
	}

	pb := fakedms.NewPolicyBlob()
	// Telemetry Extension work only for affiliated users.
	pb.DeviceAffiliationIds = []string{"default_affiliation_id"}
	pb.UserAffiliationIds = []string{"default_affiliation_id"}
	pb.AddPolicy(&policy.ExtensionInstallForcelist{Val: []string{
		"gogonhoemckpdpadfnjnpgbjpbjnodgc;https://clients2.google.com/service/update2/crx",
	}})

	if err := policyutil.ServeBlobAndRefresh(ctx, fdms, cr, pb); err != nil {
		s.Fatal("Failed to serve and refresh: ", err)
	}
}

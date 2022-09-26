// Copyright 2021 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package policy

import (
	"context"

	"chromiumos/tast/common/fixture"
	"chromiumos/tast/common/pci"
	"chromiumos/tast/common/policy"
	"chromiumos/tast/common/policy/fakedms"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/policyutil"
	"chromiumos/tast/local/policyutil/fixtures"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         DeviceEphemeralUsersEnabled,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Verifies whether the ephemeral_users_enabled policy is set on the device or not",
		Contacts: []string{
			"sergiyb@google.com",   // Migrated from autotest to tast.
			"rzakarian@google.com", // Original autotest author.
		},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:mainline", "informational"},
		Fixture:      fixture.FakeDMSEnrolled,
		VarDeps:      []string{"ui.signinProfileTestExtensionManifestKey"},
		SearchFlags: []*testing.StringPair{
			pci.SearchFlag(&policy.DeviceEphemeralUsersEnabled{}, pci.VerifiedValue),
		},
	})
}

func updatePolicyBlob(fdms *fakedms.FakeDMS, pb *policy.Blob, value *policy.DeviceEphemeralUsersEnabled) error {
	pb.AddPolicy(value)
	return fdms.WritePolicyBlob(pb)
}

func DeviceEphemeralUsersEnabled(ctx context.Context, s *testing.State) {
	fdms := s.FixtValue().(*fakedms.FakeDMS)
	pb := policy.NewBlob()

	for _, param := range []struct {
		// name is the subtest name.
		name string
		// value is the policy value.
		value *policy.DeviceEphemeralUsersEnabled
	}{
		{
			name:  "true",
			value: &policy.DeviceEphemeralUsersEnabled{Val: true},
		},
		{
			name:  "false",
			value: &policy.DeviceEphemeralUsersEnabled{Val: false},
		},
		{
			name:  "unset",
			value: &policy.DeviceEphemeralUsersEnabled{Stat: policy.StatusUnset},
		},
	} {
		s.Run(ctx, param.name, func(ctx context.Context, s *testing.State) {
			if err := updatePolicyBlob(fdms, pb, param.value); err != nil {
				s.Fatal("Failed to write policies to FakeDMS: ", err)
			}
			opts := []chrome.Option{
				chrome.FakeLogin(chrome.Creds{User: fixtures.Username, Pass: fixtures.Password}),
				chrome.DMSPolicy(fdms.URL),
				chrome.KeepEnrollment(),
				chrome.LoadSigninProfileExtension(s.RequiredVar("ui.signinProfileTestExtensionManifestKey")),
				chrome.DeferLogin(),
			}
			// If we expect mount to be ephemeral, add corresponding option so that it is correctly
			// validated when we continue login below.
			if param.value.Stat != policy.StatusUnset && param.value.Val {
				opts = append(opts, chrome.EphemeralUser())
			}
			cr, err := chrome.New(ctx, opts...)
			defer cr.Close(ctx)
			if err != nil {
				s.Fatal("Failed to start Chrome: ", err)
			}

			tconn, err := cr.SigninProfileTestAPIConn(ctx)
			if err != nil {
				s.Fatal("Failed to connect to signin test extension: ", err)
			}
			if err := policyutil.Refresh(ctx, tconn); err != nil {
				s.Fatal("Failed to update Chrome policies: ", err)
			}

			// This implicitly checks that user mount is of correct type.
			if err := cr.ContinueLogin(ctx); err != nil {
				s.Fatal("Failed to login into Chrome: ", err)
			}

			// Reset and refresh policies to avoid affecting subsequent tests.
			if err := policyutil.ServeAndRefresh(ctx, fdms, cr, []policy.Policy{}); err != nil {
				s.Fatal("Failed to reset policies in Chrome: ", err)
			}
		})
	}
}

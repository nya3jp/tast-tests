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
	"chromiumos/tast/local/cryptohome"
	"chromiumos/tast/local/policyutil/fixtures"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: DeviceEphemeralUsersEnabled,
		Desc: "Verifies whether the ephemeral_users_enabled policy is set on the device or not",
		Contacts: []string{
			"rzakarian@google.com", // Original test author.
			"sergiyb@google.com",   // Migrated from autotest to tast.
			"chromeos-commercial-remote-management@google.com",
		},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:mainline", "informational"},
		Fixture:      fixture.FakeDMSEnrolled,
		VarDeps:      []string{"ui.signinProfileTestExtensionManifestKey"},
	})
}

func DeviceEphemeralUsersEnabled(ctx context.Context, s *testing.State) {
	fdms := s.FixtValue().(*fakedms.FakeDMS)

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
			// Log in once to apply updated policy, which will only affect next login.
			pb := fakedms.NewPolicyBlob()
			pb.AddPolicy(param.value)
			if err := fdms.WritePolicyBlob(pb); err != nil {
				s.Fatal("Failed to write policies to FakeDMS: ", err)
			}
			_, err := chrome.New(ctx,
				chrome.FakeLogin(chrome.Creds{User: fixtures.Username, Pass: fixtures.Password}),
				chrome.DMSPolicy(fdms.URL),
				chrome.KeepEnrollment(),
				chrome.IgnoreUserMount())
			if err != nil {
				s.Fatal("Chrome login failed: ", err)
			}

			// Log in again to check whether user mount (cryptohome) is of correct type.
			cr, err := chrome.New(ctx,
				chrome.FakeLogin(chrome.Creds{User: fixtures.Username, Pass: fixtures.Password}),
				chrome.DMSPolicy(fdms.URL),
				chrome.KeepEnrollment(),
				chrome.IgnoreUserMount())
			if err != nil {
				s.Fatal("Chrome login failed: ", err)
			}
			defer cr.Close(ctx)
			expectPermanentMount := param.value.Stat == policy.StatusUnset || !param.value.Val
			if err := cryptohome.WaitForUserMountAndValidateType(ctx, fixtures.Username, expectPermanentMount); err != nil {
				if expectPermanentMount {
					s.Error("User mount is not mounted as permanent")
				} else {
					s.Error("User mount is not mounted as temporary")
				}
			}
		})
	}
}

// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package policy

import (
	"context"

	"chromiumos/tast/common/fixture"
	"chromiumos/tast/common/policy"
	"chromiumos/tast/common/policy/fakedms"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/policyutil/fixtures"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: DeviceEphemeralUsersEnabled,
		Desc: "Verifies whether the ephemeral_users_enabled policy is set on the device or not",
		Contacts: []string{
			"sergiyb@google.com",   // Migrated from autotest to tast.
			"rzakarian@google.com", // Original autotest author.
			"chromeos-commercial-remote-management@google.com",
		},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:mainline", "informational"},
		Fixture:      fixture.FakeDMSEnrolled,
	})
}

func setDeviceEphemeralUsersEnabledPolicy(ctx context.Context, fdms *fakedms.FakeDMS, value *policy.DeviceEphemeralUsersEnabled, loginOpts []chrome.Option) error {
	// Log in to apply the updated policy, which will affect the next login.
	pb := fakedms.NewPolicyBlob()
	pb.AddPolicy(value)
	if err := fdms.WritePolicyBlob(pb); err != nil {
		return errors.Wrap(err, "failed to write policies to FakeDMS")
	}
	if _, err := chrome.New(ctx, loginOpts...); err != nil {
		return errors.Wrap(err, "chrome login failed")
	}
	return nil
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
			// Set policy value, which only affects the next login, hence we implicitly expect a
			// permanent mount here by not adding chrome.EphemeralUser option.
			initialOpts := []chrome.Option{
				chrome.FakeLogin(chrome.Creds{User: fixtures.Username, Pass: fixtures.Password}),
				chrome.DMSPolicy(fdms.URL),
				chrome.KeepEnrollment(),
			}
			err1 := setDeviceEphemeralUsersEnabledPolicy(ctx, fdms, param.value, initialOpts)

			// Reset policy value and validate that the user mount is of correct type. Notably, we
			// attempt this call even if previous call has failed to ensure that policy is reset and
			// does not affect subsequent logins in other tests.
			updatedOpts := initialOpts
			if param.value.Stat != policy.StatusUnset && param.value.Val {
				updatedOpts = append(initialOpts, chrome.EphemeralUser())
			}
			err2 := setDeviceEphemeralUsersEnabledPolicy(
				ctx, fdms, &policy.DeviceEphemeralUsersEnabled{Stat: policy.StatusUnset}, updatedOpts)

			if err1 != nil {
				s.Fatal("Failed to set DeviceEphemeralUsersEnabled policy:", err1)
			}

			if err2 != nil {
				s.Fatal("Failed to reset DeviceEphemeralUsersEnabled policy or validate correct user mount: ", err2)
			}
		})
	}
}

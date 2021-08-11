// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package policy

import (
	"context"

	"chromiumos/tast/common/policy"
	"chromiumos/tast/local/chrome/uiauto/checked"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/restriction"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/policyutil"
	"chromiumos/tast/local/policyutil/fixtures"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: ChromeOsLockOnIdleSuspend,
		Desc: "Behavior of ChromeOsLockOnIdleSuspend policy, checking the correspoding toggle button states (restriction and checked) after setting the policy",
		Contacts: []string{
			"gabormagda@google.com", // Test author
			"chromeos-commercial-remote-management@google.com",
		},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:mainline", "informational"},
		Fixture:      "chromePolicyLoggedIn",
	})
}

// ChromeOsLockOnIdleSuspend tests the ChromeOsLockOnIdleSuspend policy.
func ChromeOsLockOnIdleSuspend(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(*fixtures.FixtData).Chrome
	fdms := s.FixtValue().(*fixtures.FixtData).FakeDMS

	for _, param := range []struct {
		name            string
		wantRestriction restriction.Restriction           // wantRestriction is the wanted restriction state of the checkboxes in Browsing history.
		wantChecked     checked.Checked                   // wantChecked is the wanted checked state of the checkboxes in Browsing history.
		value           *policy.ChromeOsLockOnIdleSuspend // value is the value of the policy.
	}{
		{
			name:            "forced",
			wantRestriction: restriction.Disabled,
			wantChecked:     checked.True,
			value:           &policy.ChromeOsLockOnIdleSuspend{Val: true},
		},
		{
			name:            "disabled",
			wantRestriction: restriction.Disabled,
			wantChecked:     checked.False,
			value:           &policy.ChromeOsLockOnIdleSuspend{Val: false},
		},
		{
			name:            "unset",
			wantRestriction: restriction.None,
			wantChecked:     checked.False,
			value:           &policy.ChromeOsLockOnIdleSuspend{Stat: policy.StatusUnset},
		},
	} {
		s.Run(ctx, param.name, func(ctx context.Context, s *testing.State) {
			defer faillog.DumpUITreeWithScreenshotOnError(ctx, s.OutDir(), s.HasError, cr, "ui_tree_"+param.name)

			// Perform cleanup.
			if err := policyutil.ResetChrome(ctx, fdms, cr); err != nil {
				s.Fatal("Failed to clean up: ", err)
			}

			// Update policies.
			if err := policyutil.ServeAndVerify(ctx, fdms, cr, []policy.Policy{param.value}); err != nil {
				s.Fatal("Failed to update policies: ", err)
			}

			// Open the Security and sign-in page where the affected toggle button can be found.
			if err := policyutil.OSSettingsPageWithPassword(ctx, cr, "osPrivacy/lockScreen", fixtures.Password).
				SelectNode(ctx, nodewith.
					Role(role.ToggleButton).
					Name("Show lock screen when waking from sleep")).
				Restriction(param.wantRestriction).
				Checked(param.wantChecked).
				Verify(); err != nil {
				s.Error("Unexpected OS settings state: ", err)
			}
		})
	}
}

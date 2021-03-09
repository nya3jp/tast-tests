// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package policy

import (
	"context"

	"chromiumos/tast/common/policy"
	"chromiumos/tast/local/chrome/ui"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/policyutil"
	"chromiumos/tast/local/policyutil/pre"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: BlockThirdPartyCookies,
		Desc: "Test the behavior of BlockThirdPartyCookies policy: check if third party cookies are allowed based on policy value",
		Contacts: []string{
			"snijhara@google.com", // Test author
			"chromeos-commercial-stability@google.com",
		},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:mainline", "informational"},
		Pre:          pre.User,
	})
}

func BlockThirdPartyCookies(ctx context.Context, s *testing.State) {
	cr := s.PreValue().(*pre.PreData).Chrome
	fdms := s.PreValue().(*pre.PreData).FakeDMS

	// Connect to Test API to use it with the UI library.
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}

	for _, param := range []struct {
		name                 string                 // name is the name of the test case.
		wantAllowCookiesAttr map[string]interface{} // wantAllowCookiesAttr are the expected attributes of
		// "Allow all cookies" radio button on settings page.
		wantBlockExternalCookiesAttr map[string]interface{} // wantBlockExternalCookiesAttr are the expected attributes of
		// "Block third-party cookies" radio button on settings page.
		policy *policy.BlockThirdPartyCookies // policy is the policy we test.
	}{
		{
			name: "unset",
			wantAllowCookiesAttr: map[string]interface{}{
				"restriction": ui.RestrictionNone,
			},
			wantBlockExternalCookiesAttr: map[string]interface{}{
				"restriction": ui.RestrictionNone,
			},
			policy: &policy.BlockThirdPartyCookies{Stat: policy.StatusUnset},
		},
		{
			name: "allow",
			wantAllowCookiesAttr: map[string]interface{}{
				"restriction": ui.RestrictionNone,
				"checked":     ui.CheckedStateTrue,
			},
			wantBlockExternalCookiesAttr: map[string]interface{}{
				"restriction": ui.RestrictionDisabled,
				"checked":     ui.CheckedStateFalse,
			},
			policy: &policy.BlockThirdPartyCookies{Val: false},
		},
		{
			name: "block",
			wantAllowCookiesAttr: map[string]interface{}{
				"restriction": ui.RestrictionDisabled,
				"checked":     ui.CheckedStateFalse,
			},
			// There is no restriction as the user can also select the option to block all cookies.
			wantBlockExternalCookiesAttr: map[string]interface{}{
				"restriction": ui.RestrictionNone,
				"checked":     ui.CheckedStateTrue,
			},
			policy: &policy.BlockThirdPartyCookies{Val: true},
		},
	} {
		s.Run(ctx, param.name, func(ctx context.Context, s *testing.State) {
			defer faillog.DumpUITreeOnErrorToFile(ctx, s.OutDir(), s.HasError, tconn, "ui_tree_"+param.name+".txt")

			// Perform cleanup.
			if err := policyutil.ResetChrome(ctx, fdms, cr); err != nil {
				s.Fatal("Failed to clean up: ", err)
			}

			// Update policies.
			if err := policyutil.ServeAndVerify(ctx, fdms, cr, []policy.Policy{param.policy}); err != nil {
				s.Fatal("Failed to update policies: ", err)
			}

			// Open cookies settings.
			conn, err := cr.NewConn(ctx, "chrome://settings/cookies")
			if err != nil {
				s.Fatal("Failed to open cookies settings: ", err)
			}
			defer conn.Close()

			// Verify the state of "Allow all cookies" radio button.
			if err := policyutil.VerifySettingsNode(ctx, tconn,
				ui.FindParams{
					Role: ui.RoleTypeRadioButton,
					Name: "Allow all cookies",
				},
				ui.FindParams{
					Attributes: param.wantAllowCookiesAttr,
				},
			); err != nil {
				s.Error("Unexpected Allow all cookies radio button state: ", err)
			}

			// Verify the state of "Block third-party cookies" radio button.
			if err := policyutil.VerifySettingsNode(ctx, tconn,
				ui.FindParams{
					Role: ui.RoleTypeRadioButton,
					Name: "Block third-party cookies",
				},
				ui.FindParams{
					Attributes: param.wantBlockExternalCookiesAttr,
				},
			); err != nil {
				s.Error("Unexpected Block third-party cookies radio button state: ", err)
			}
		})
	}
}

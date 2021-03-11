// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package policy

import (
	"context"
	"time"

	"chromiumos/tast/common/policy"
	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/chrome/ui"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/policyutil"
	"chromiumos/tast/local/policyutil/pre"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: DictationEnabled,
		Desc: "Behavior of DictationEnabled policy: checking if dictation is enabled or not",
		Contacts: []string{
			"swapnilgupta@google.com", // Test author
			"chromeos-commercial-stability@google.com",
		},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:mainline", "informational"},
		Pre:          pre.User,
	})
}

// DictationEnabled tests the DictationEnabled policy.
func DictationEnabled(ctx context.Context, s *testing.State) {
	cr := s.PreValue().(*pre.PreData).Chrome
	fdms := s.PreValue().(*pre.PreData).FakeDMS

	// Connect to Test API to use it with the UI library.
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}
	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)

	for _, param := range []struct {
		name            string
		value           *policy.DictationEnabled
		wantChecked     ui.CheckedState
		wantRestriction ui.RestrictionState
	}{
		{
			name:            "unset",
			value:           &policy.DictationEnabled{Stat: policy.StatusUnset},
			wantChecked:     ui.CheckedStateFalse,
			wantRestriction: ui.RestrictionNone,
		},
		{
			name:            "disabled",
			value:           &policy.DictationEnabled{Val: false},
			wantChecked:     ui.CheckedStateFalse,
			wantRestriction: ui.RestrictionDisabled,
		},
		{
			name:            "enabled",
			value:           &policy.DictationEnabled{Val: true},
			wantChecked:     ui.CheckedStateTrue,
			wantRestriction: ui.RestrictionDisabled,
		},
	} {
		s.Run(ctx, param.name, func(ctx context.Context, s *testing.State) {

			// Perform cleanup.
			if err := policyutil.ResetChrome(ctx, fdms, cr); err != nil {
				s.Fatal("Failed to clean up: ", err)
			}

			// Update policies.
			if err := policyutil.ServeAndVerify(ctx, fdms, cr, []policy.Policy{param.value}); err != nil {
				s.Fatal("Failed to update policies: ", err)
			}

			// Open settings page where the affected toggle button can be found.
			sconn, err := apps.LaunchOSSettings(ctx, cr, "chrome://os-settings/osAccessibility")
			if err != nil {
				s.Fatal("Failed to connect to the accessibility settings page: ", err)
			}
			defer sconn.Close()

			// Find and click manage accessibility link.
			if err := ui.StableFindAndClick(ctx, tconn, ui.FindParams{
				Role: ui.RoleTypeLink,
				Name: "Manage accessibility features Enable accessibility features",
			}, &testing.PollOptions{Timeout: 15 * time.Second}); err != nil {
				s.Fatal("Failed to find and click Manage accessibility features link: ", err)
			}

			if err := policyutil.VerifySettingsNode(ctx, tconn,
				ui.FindParams{
					Role: ui.RoleTypeToggleButton,
					Name: "Enable dictation (speak to type)",
				},
				ui.FindParams{
					Attributes: map[string]interface{}{
						"restriction": param.wantRestriction,
						"checked":     param.wantChecked,
					},
				},
			); err != nil {
				s.Error("Unexpected settings state: ", err)
			}
		})
	}
}

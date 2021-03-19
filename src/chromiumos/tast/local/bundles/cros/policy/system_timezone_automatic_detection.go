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
	"chromiumos/tast/local/policyutil/fixtures"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: SystemTimezoneAutomaticDetection,
		Desc: "Behavior of SystemTimezoneAutomaticDetection policy",
		Contacts: []string{
			"vsavu@google.com", // Test author
			"chromeos-commercial-stability@google.com",
		},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:enrollment"},
		Fixture:      "chromeEnrolledLoggedIn",
	})
}

func SystemTimezoneAutomaticDetection(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(*fixtures.FixtData).Chrome
	fdms := s.FixtValue().(*fixtures.FixtData).FakeDMS

	// Connect to Test API to use it with the UI library.
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}

	for _, param := range []struct {
		// name is the subtest name.
		name string
		// value is the policy value.
		value *policy.SystemTimezoneAutomaticDetection
		// wantRestriction indicates whether the setting can be changed by the user.
		wantRestriction ui.RestrictionState
		// selectedOption is the selected state for timezone detection.
		selectedOption string
		// selectedMethod is the selected option for timezone detection.
		selectedMethod string
	}{
		{
			name:            "all",
			value:           &policy.SystemTimezoneAutomaticDetection{Val: 4},
			wantRestriction: ui.RestrictionDisabled,
			selectedOption:  "Set automatically",
		},
		{
			name:            "wifi",
			value:           &policy.SystemTimezoneAutomaticDetection{Val: 3},
			wantRestriction: ui.RestrictionDisabled,
			selectedOption:  "Set automatically",
		},
		{
			name:            "ip",
			value:           &policy.SystemTimezoneAutomaticDetection{Val: 2},
			wantRestriction: ui.RestrictionDisabled,
			selectedOption:  "Set automatically",
		},
		{
			name:            "never",
			value:           &policy.SystemTimezoneAutomaticDetection{Val: 1},
			wantRestriction: ui.RestrictionDisabled,
			selectedOption:  "Choose from list",
		},
		{
			name:            "user",
			value:           &policy.SystemTimezoneAutomaticDetection{Val: 0},
			wantRestriction: ui.RestrictionNone,
			selectedOption:  "Set automatically",
		},
	} {
		s.Run(ctx, param.name, func(ctx context.Context, s *testing.State) {
			defer faillog.DumpUITreeOnErrorToFile(ctx, s.OutDir(), s.HasError, tconn, "ui_tree_"+param.name+".txt")

			// Perform cleanup.
			if err := policyutil.ResetChrome(ctx, fdms, cr); err != nil {
				s.Fatal("Failed to clean up: ", err)
			}

			// Update policies.
			if err := policyutil.ServeAndVerify(ctx, fdms, cr, []policy.Policy{param.value}); err != nil {
				s.Fatal("Failed to update policies: ", err)
			}

			// Open the Lockscreen page where we can set the timezone.
			conn, err := apps.LaunchOSSettings(ctx, cr, "chrome://os-settings/dateTime/timeZone")
			if err != nil {
				s.Fatal("Failed to connect to the settings page: ", err)
			}
			defer conn.Close()

			// Find the radio group node.
			rgNode, err := ui.FindWithTimeout(ctx, tconn, ui.FindParams{Role: ui.RoleTypeRadioGroup}, 15*time.Second)
			if err != nil {
				s.Fatal("Finding radio group failed: ", err)
			}
			defer rgNode.Release(ctx)

			// Find the selected radio button under the radio group.
			srbNode, err := rgNode.FindSelectedRadioButton(ctx)
			if err != nil {
				s.Fatal("Finding the selected radio button failed: ", err)
			}
			defer srbNode.Release(ctx)

			if err := policyutil.CheckNodeAttributes(srbNode, ui.FindParams{
				Attributes: map[string]interface{}{
					"restriction": param.wantRestriction,
					"name":        param.selectedOption,
				},
			}); err != nil {
				s.Error("Unexpected settings state: ", err)
			}

			// TODO(crbug.com/1190753): Add check for the selected method.
		})
	}
}

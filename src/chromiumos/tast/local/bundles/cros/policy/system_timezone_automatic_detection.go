// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package policy

import (
	"context"

	"chromiumos/tast/common/policy"
	"chromiumos/tast/local/chrome/uiauto"
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
		Func: SystemTimezoneAutomaticDetection,
		Desc: "Check of SystemTimezoneAutomaticDetection policy by checking the settings page",
		Contacts: []string{
			"vsavu@google.com", // Test author
			"chromeos-commercial-remote-management@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
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
		wantRestriction restriction.Restriction
		// selectedOption is the selected state for timezone detection.
		selectedOption string
		// selectedDetection is the state for timezone detection.
		selectedDetection string
	}{
		{
			name:              "all",
			value:             &policy.SystemTimezoneAutomaticDetection{Val: 4},
			wantRestriction:   restriction.Disabled,
			selectedOption:    "Set automatically",
			selectedDetection: "Use Wi-Fi or mobile networks to determine location",
		},
		{
			name:              "wifi",
			value:             &policy.SystemTimezoneAutomaticDetection{Val: 3},
			wantRestriction:   restriction.Disabled,
			selectedOption:    "Set automatically",
			selectedDetection: "Use only Wi-Fi to determine location",
		},
		{
			name:              "ip",
			value:             &policy.SystemTimezoneAutomaticDetection{Val: 2},
			wantRestriction:   restriction.Disabled,
			selectedOption:    "Set automatically",
			selectedDetection: "Use your IP address to determine location (default)",
		},
		{
			name:              "never",
			value:             &policy.SystemTimezoneAutomaticDetection{Val: 1},
			wantRestriction:   restriction.Disabled,
			selectedOption:    "Choose from list",
			selectedDetection: "Automatic time zone detection is disabled",
		},
		{
			name:              "user",
			value:             &policy.SystemTimezoneAutomaticDetection{Val: 0},
			wantRestriction:   restriction.None,
			selectedOption:    "Set automatically",
			selectedDetection: "Use your IP address to determine location (default)",
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

			// Open the time zone settings page.
			if err := policyutil.OSSettingsPage(ctx, cr, "dateTime/timeZone").
				SelectNode(ctx, nodewith.
					Role(role.RadioButton).
					Name(param.selectedOption)).
				Checked(checked.True).
				Restriction(param.wantRestriction).
				Verify(); err != nil {
				s.Error("Unexpected OS settings state: ", err)
			}

			// Check the currently selected detection mode.
			uia := uiauto.New(tconn)
			text, err := uia.Info(ctx, nodewith.Name("Time zone detection method"))
			if err != nil {
				s.Fatal("Could not find current detection mode: ", err)
			}
			if text.Value != param.selectedDetection {
				s.Errorf("Invalid detection mode: got %q; want %q", text.Value, param.selectedDetection)
			}
		})
	}
}

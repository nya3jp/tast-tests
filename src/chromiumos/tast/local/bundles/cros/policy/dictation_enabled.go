// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package policy

import (
	"context"

	"chromiumos/tast/common/policy"
	"chromiumos/tast/local/apps"
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
		Func: DictationEnabled,
		Desc: "Behavior of DictationEnabled policy: checking if dictation is enabled or not",
		Contacts: []string{
			"swapnilgupta@google.com", // Test author
			"chromeos-commercial-stability@google.com",
		},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:mainline", "informational"},
		Fixture:      "chromePolicyLoggedIn",
	})
}

// DictationEnabled tests the DictationEnabled policy.
func DictationEnabled(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(*fixtures.FixtData).Chrome
	fdms := s.FixtValue().(*fixtures.FixtData).FakeDMS

	// Connect to Test API to use it with the UI library.
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}
	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)

	for _, param := range []struct {
		name            string
		value           *policy.DictationEnabled
		wantButton      bool
		wantChecked     checked.Checked
		wantRestriction restriction.Restriction
	}{
		{
			name:            "unset",
			value:           &policy.DictationEnabled{Stat: policy.StatusUnset},
			wantButton:      false,
			wantChecked:     checked.False,
			wantRestriction: restriction.None,
		},
		{
			name:            "disabled",
			value:           &policy.DictationEnabled{Val: false},
			wantButton:      false,
			wantChecked:     checked.False,
			wantRestriction: restriction.Disabled,
		},
		{
			name:            "enabled",
			value:           &policy.DictationEnabled{Val: true},
			wantButton:      true,
			wantChecked:     checked.True,
			wantRestriction: restriction.Disabled,
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

			buttonExists := true
			ui := uiauto.New(tconn)
			if err = ui.Exists(nodewith.Name("Toggle dictation").Role(role.Button))(ctx); err != nil {

				buttonExists = false
			}
			if buttonExists != param.wantButton {
				s.Errorf("Unexpected existance of Toggle dictation button: got %v; want %v", buttonExists, param.wantButton)
			}

			// Find and click manage accessibility link.
			if err := ui.LeftClick(nodewith.Name("Manage accessibility features Enable accessibility features").Role(role.Link))(ctx); err != nil {

				s.Fatal("Failed to find and click Manage accessibility features link: ", err)
			}

			nodeInfo, err := ui.Info(ctx, nodewith.Name("Enable dictation (speak to type)").Role(role.ToggleButton))
			if err != nil {

				s.Fatal("Could not find Enable dictation (speak to type) button: ", err)
			}
			// Check that the button is in the correct state.
			if nodeInfo.Restriction != param.wantRestriction {
				s.Errorf("Unexpected button restriction state: got %v, want %v", nodeInfo.Restriction, param.wantRestriction)
			}
			if nodeInfo.Checked != param.wantChecked {
				s.Errorf("Unexpected button checked state: got %v, want %v", nodeInfo.Checked, param.wantChecked)
			}

		})
	}
}

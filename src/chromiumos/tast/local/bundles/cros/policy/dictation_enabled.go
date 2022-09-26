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
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/checked"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/restriction"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/policyutil"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         DictationEnabled,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Behavior of DictationEnabled policy: checking if dictation is enabled or not",
		Contacts: []string{
			"swapnilgupta@google.com", // Test author
		},
		SoftwareDeps: []string{"chrome"},
		// TODO(crbug.com/1238027): Close dialog before the next test.
		// Attr:         []string{"group:mainline", "informational"},
		Fixture: fixture.ChromePolicyLoggedIn,
		SearchFlags: []*testing.StringPair{
			pci.SearchFlag(&policy.DictationEnabled{}, pci.VerifiedFunctionalityUI),
		},
	})
}

// DictationEnabled tests the DictationEnabled policy.
func DictationEnabled(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(chrome.HasChrome).Chrome()
	fdms := s.FixtValue().(fakedms.HasFakeDMS).FakeDMS()

	// Connect to Test API to use it with the UI library.
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}

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
			defer faillog.DumpUITreeWithScreenshotOnError(ctx, s.OutDir(), s.HasError, cr, "ui_tree_"+param.name)

			// Perform cleanup.
			if err := policyutil.ResetChrome(ctx, fdms, cr); err != nil {
				s.Fatal("Failed to clean up: ", err)
			}

			// Update policies.
			if err := policyutil.ServeAndVerify(ctx, fdms, cr, []policy.Policy{param.value}); err != nil {
				s.Fatal("Failed to update policies: ", err)
			}

			// Trying to find the "Toggle dictation" button in the system tray.
			buttonExists := true
			ui := uiauto.New(tconn)
			if err = ui.Exists(nodewith.Name("Toggle dictation").Role(role.Button))(ctx); err != nil {
				buttonExists = false
			}
			if buttonExists != param.wantButton {
				s.Errorf("Unexpected existence of Toggle dictation button: got %v; want %v", buttonExists, param.wantButton)
			}

			if err := policyutil.OSSettingsPage(ctx, cr, "manageAccessibility").
				SelectNode(ctx, nodewith.
					Name("Enable dictation (speak to type)").
					Role(role.ToggleButton)).
				Restriction(param.wantRestriction).
				Checked(param.wantChecked).
				Verify(); err != nil {
				s.Error("Unexpected OS settings state: ", err)
			}
		})
	}
}

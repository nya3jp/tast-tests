// Copyright 2020 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package policy

import (
	"context"
	"time"

	"chromiumos/tast/common/fixture"
	"chromiumos/tast/common/pci"
	"chromiumos/tast/common/policy"
	"chromiumos/tast/common/policy/fakedms"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/checked"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/quicksettings"
	"chromiumos/tast/local/chrome/uiauto/restriction"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/policyutil"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         ShowAccessibilityOptionsInSystemTrayMenu,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Behavior of ShowAccessibilityOptionsInSystemTrayMenu policy: check the a11y option in the system tray, and the status of the related option in the settings",
		Contacts: []string{
			"gabormagda@google.com", // Test author
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Fixture:      fixture.ChromePolicyLoggedIn,
		SearchFlags: []*testing.StringPair{
			pci.SearchFlag(&policy.ShowAccessibilityOptionsInSystemTrayMenu{}, pci.VerifiedFunctionalityUI),
		},
	})
}

// ShowAccessibilityOptionsInSystemTrayMenu tests the ShowAccessibilityOptionsInSystemTrayMenu policy.
func ShowAccessibilityOptionsInSystemTrayMenu(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(chrome.HasChrome).Chrome()
	fdms := s.FixtValue().(fakedms.HasFakeDMS).FakeDMS()

	// Connect to Test API to use it with the UI library.
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}

	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 3*time.Second)
	defer cancel()
	defer quicksettings.Hide(cleanupCtx, tconn)

	for _, param := range []struct {
		name            string
		wantChecked     checked.Checked         // wantChecked is the expected existence of the a11y button.
		wantRestriction restriction.Restriction // wantRestriction is the wanted restriction state of the toggle button for the "Show accessibility options in Quick Settings" option.
		policy          *policy.ShowAccessibilityOptionsInSystemTrayMenu
	}{
		{
			name:            "unset",
			wantChecked:     checked.False,
			wantRestriction: restriction.None,
			policy:          &policy.ShowAccessibilityOptionsInSystemTrayMenu{Stat: policy.StatusUnset},
		},
		{
			name:            "false",
			wantChecked:     checked.False,
			wantRestriction: restriction.Disabled,
			policy:          &policy.ShowAccessibilityOptionsInSystemTrayMenu{Val: false},
		},
		{
			name:            "true",
			wantChecked:     checked.True,
			wantRestriction: restriction.Disabled,
			policy:          &policy.ShowAccessibilityOptionsInSystemTrayMenu{Val: true},
		},
	} {
		s.Run(ctx, param.name, func(ctx context.Context, s *testing.State) {
			defer faillog.DumpUITreeWithScreenshotOnError(ctx, s.OutDir(), s.HasError, cr, "ui_tree_"+param.name)

			// Perform cleanup.
			if err := policyutil.ResetChrome(ctx, fdms, cr); err != nil {
				s.Fatal("Failed to clean up: ", err)
			}

			// Update policies.
			if err := policyutil.ServeAndVerify(ctx, fdms, cr, []policy.Policy{param.policy}); err != nil {
				s.Fatal("Failed to update policies: ", err)
			}

			// Open OS Settings page where the affected button can be found.
			if err := policyutil.OSSettingsPage(ctx, cr, "osAccessibility").
				SelectNode(ctx, nodewith.
					Role(role.ToggleButton).
					Name("Show accessibility options in Quick Settings")).
				Restriction(param.wantRestriction).
				Checked(param.wantChecked).
				Verify(); err != nil {
				s.Error("Unexpected OS settings state: ", err)
			}

			// Open system tray.
			if err := quicksettings.Show(ctx, tconn); err != nil {
				s.Fatal("Failed to open the system tray: ", err)
			}

			// Get all the feature pod icons in the system tray.
			uia := uiauto.New(tconn)
			systemTrayContainer := nodewith.ClassName("SystemTrayContainer")
			podIcons, err := uia.NodesInfo(ctx, nodewith.ClassName("FeaturePodIconButton").Ancestor(systemTrayContainer))
			if err != nil {
				s.Fatal("Failed to get a list of feature pod icons: ", err)
			}

			// Look for the a11y button among the feature pod icons.
			found := false
			for _, icon := range podIcons {
				if icon.Name == "Show accessibility settings" {
					found = true
				}
			}
			if wantFound := param.wantChecked == checked.True; wantFound != found {
				s.Errorf("Unexpected accessibility button presence; got %t, want %t", found, wantFound)
			}
		})
	}
}

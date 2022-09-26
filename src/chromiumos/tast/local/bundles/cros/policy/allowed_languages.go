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
	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/policyutil"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         AllowedLanguages,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Behavior of AllowedLanguages policy, checking the correspoding checkbox states (count) after setting the policy",
		Contacts: []string{
			"mohamedaomar@google.com", // Test author
		},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:mainline"},
		Fixture:      fixture.ChromePolicyLoggedIn,
		SearchFlags: []*testing.StringPair{
			pci.SearchFlag(&policy.AllowedLanguages{}, pci.VerifiedFunctionalityUI),
		},
	})
}

// AllowedLanguages tests the AllowedLanguages policy.
func AllowedLanguages(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(chrome.HasChrome).Chrome()
	fdms := s.FixtValue().(fakedms.HasFakeDMS).FakeDMS()

	// Connect to Test API to use it with the UI library.
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}

	for _, param := range []struct {
		name     string
		minLangs int                      // minLangs is the minimum number of allowed languages in add languages dialog.
		maxLangs int                      // maxLangs is the maximum number of allowed languages in add languages dialog.
		lastLang string                   // lastLang is the last language name that appears in the dialog without scrolling.
		value    *policy.AllowedLanguages // value is the value of the policy.
	}{
		{
			name:     "unset",
			minLangs: 5,
			maxLangs: 200,
			lastLang: "Asturian - asturianu", // It could be different depending on the screen size of the device, but since 5 is the minimum, so Asturian is enough.
			value:    &policy.AllowedLanguages{Stat: policy.StatusUnset},
		},
		{
			name:     "nonempty",
			minLangs: 2,
			maxLangs: 2,
			lastLang: "German - Deutsch",
			value:    &policy.AllowedLanguages{Val: []string{"en-US", "de", "ar", "xyz"}},
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

			// In the following block we try to access "chrome://os-settings/osLanguages/details".
			// But it cannot be opened using apps.LaunchOSSettings(ctx, cr, "chrome://os-settings/osLanguages/details").
			// Instead, we navigate through "chrome://os-settings/osLanguages", then click on Languages link.
			// Open the os settings languages page.
			conn, err := apps.LaunchOSSettings(ctx, cr, "chrome://os-settings/osLanguages")
			if err != nil {
				s.Fatal("Failed to open the OS settings page: ", err)
			}
			defer conn.Close()

			ui := uiauto.New(tconn)

			if err := uiauto.Combine("Add languages and wait for the dialog",
				// Find and click on languages link.
				ui.LeftClick(nodewith.Name("Languages English (United States)").Role(role.Link)),
				// Find and clilck on Add languages button to select the preferred languages from the popup dialog.
				ui.LeftClick(nodewith.Name("Add languages").Role(role.Button)),
				// Wait for the last checkbox in the screen to appear.
				ui.WaitUntilExists(nodewith.Name(param.lastLang).Role(role.CheckBox)),
			)(ctx); err != nil {
				s.Fatal("Failed to open the languages dialog: ", err)
			}

			// Count the number of checkboxes in the dialog.
			langs, err := ui.NodesInfo(ctx, nodewith.Role(role.CheckBox))
			if err != nil {
				s.Fatal("Failed to find all checkboxes: ", err)
			}
			if (param.minLangs > len(langs)) || (len(langs) > param.maxLangs) {
				s.Errorf("The number of preferred languages doesn't match: got %d; want at least %d and at most %d", len(langs), param.minLangs, param.maxLangs)
			}
		})
	}
}

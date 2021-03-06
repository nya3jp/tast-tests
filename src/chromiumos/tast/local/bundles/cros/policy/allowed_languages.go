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
		Func: AllowedLanguages,
		Desc: "Behavior of AllowedLanguages policy, checking the correspoding checkbox states (count) after setting the policy",
		Contacts: []string{
			"mohamedaomar@google.com", // Test author
			"chromeos-commercial-stability@google.com",
		},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:mainline"},
		Fixture:      "chromePolicyLoggedIn",
	})
}

// AllowedLanguages tests the AllowedLanguages policy.
func AllowedLanguages(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(*fixtures.FixtData).Chrome
	fdms := s.FixtValue().(*fixtures.FixtData).FakeDMS

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
			// Find and click on languages link.
			if err := ui.StableFindAndClick(ctx, tconn, ui.FindParams{
				Role: ui.RoleTypeLink,
				Name: "Languages English (United States)",
			}, &testing.PollOptions{Timeout: 15 * time.Second}); err != nil {
				s.Fatal("Failed to find Languages English (United States) link: ", err)
			}

			// Find and clilck on Add languages button to select the preferred languages from the popup dialog.
			if err := ui.StableFindAndClick(ctx, tconn, ui.FindParams{
				Role: ui.RoleTypeButton,
				Name: "Add languages",
			}, &testing.PollOptions{Timeout: 15 * time.Second}); err != nil {
				s.Fatal("Failed to find Add languages button: ", err)
			}

			// Wait for the last checkbox in the screen to appear.
			if err := ui.WaitUntilExists(ctx, tconn, ui.FindParams{
				Role: ui.RoleTypeCheckBox,
				Name: param.lastLang,
			}, 15*time.Second); err != nil {
				s.Fatal("Failed to find the last checkbox: ", err)
			}

			// Count the number of checkboxes in the dialog.
			nodeSlice, err := ui.FindAll(ctx, tconn, ui.FindParams{Role: ui.RoleTypeCheckBox})
			if err != nil {
				s.Fatal("Failed to find all checkboxes: ", err)
			}
			defer nodeSlice.Release(ctx)

			if (param.minLangs > len(nodeSlice)) || (len(nodeSlice) > param.maxLangs) {
				s.Errorf("The number of preferred languages doesn't match: got %d; want at least %d and at most %d", len(nodeSlice), param.minLangs, param.maxLangs)
			}
		})
	}
}

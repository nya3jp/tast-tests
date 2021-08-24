// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package policy

import (
	"context"
	"time"

	"chromiumos/tast/common/policy"
	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/ui"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/policyutil"
	"chromiumos/tast/local/policyutil/fixtures"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: HideWebStoreIcon,
		Desc: "Behavior of HideWebStoreIcon policy, check if a Web Store Icon is displayed in app launcher based on the value of the policy",
		Contacts: []string{
			"evgenyu@google.com", // Test author
			"chromeos-commercial-remote-management@google.com",
		},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:mainline", "informational"},
		Fixture:      "chromePolicyLoggedIn",
	})
}

// HideWebStoreIcon tests the HideWebStoreIcon policy.
func HideWebStoreIcon(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(*fixtures.FixtData).Chrome
	fdms := s.FixtValue().(*fixtures.FixtData).FakeDMS

	// Connect to Test API to use it with the UI library.
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}

	pinWebStoreApp := &policy.PinnedLauncherApps{Val: []string{apps.WebStore.ID}}

	for _, param := range []struct {
		name     string
		wantIcon bool            // wantIcon is the expected existence of the "Web Store" icon.
		policies []policy.Policy // policies is the policies that will be set.
	}{
		{
			// This case fails due to https://crbug.com/1199312 -> node is
			// present in the UI tree but not visible in the UI.
			name:     "hide",
			wantIcon: false,
			policies: []policy.Policy{pinWebStoreApp, &policy.HideWebStoreIcon{Val: true}},
		},
		{
			name:     "show",
			wantIcon: true,
			policies: []policy.Policy{pinWebStoreApp, &policy.HideWebStoreIcon{Val: false}},
		},
		{
			name:     "unset",
			wantIcon: true,
			policies: []policy.Policy{pinWebStoreApp, &policy.HideWebStoreIcon{Stat: policy.StatusUnset}},
		},
	} {
		s.Run(ctx, param.name, func(ctx context.Context, s *testing.State) {
			defer faillog.DumpUITreeWithScreenshotOnError(ctx, s.OutDir(), s.HasError, cr, "ui_tree_"+param.name)

			// Perform cleanup.
			if err := policyutil.ResetChrome(ctx, fdms, cr); err != nil {
				s.Fatal("Failed to clean up: ", err)
			}

			// Update policies.
			if err := policyutil.ServeAndVerify(ctx, fdms, cr, param.policies); err != nil {
				s.Fatal("Failed to update policies: ", err)
			}

			tabletMode, err := ash.TabletModeEnabled(ctx, tconn)
			if err != nil {
				s.Fatal("Failed to get tablet mode status: ", err)
			}

			uia := uiauto.New(tconn)
			// In desktop mode user needs to bring up the application grid.
			// In tablet mode the application grid is already up.
			if !tabletMode {
				// Open Launcher and go to Apps list view page.
				// Tried to use launcher.OpenExpandedView(tconn) but it seems to be flaky, after some testing
				// it seems to be mostly flaky at ash.WaitForLauncherState(ctx, tconn, ash.FullscreenAllApps).
				if err := uiauto.Combine("Open Launcher and go to Expanded Apps list view",
					uia.WithInterval(500*time.Millisecond).LeftClickUntil(
						nodewith.Name("Launcher").ClassName("ash/HomeButton"),
						uia.Exists(nodewith.Name("Expand to all apps").ClassName("ExpandArrowView"))),
					uia.LeftClick(nodewith.Name("Expand to all apps").ClassName("ExpandArrowView")),
					uia.WaitUntilExists(nodewith.ClassName("AppsGridView")),
				)(ctx); err != nil {
					s.Fatal("Failed to open Expanded Apps list view: ", err)
				}
			}

			appName := apps.WebStore.Name

			// Confirm the status of the Web Store icon in the application launcher
			if err := policyutil.WaitUntilExistsStatus(ctx, tconn, ui.FindParams{
				Name:      appName,
				ClassName: "AppListItemView",
			}, param.wantIcon, 15*time.Second); err != nil {
				s.Error("Could not confirm the desired status of the Web Store Icon in the application launcher: ", err)
			}

			if err := policyutil.WaitUntilExistsStatus(ctx, tconn, ui.FindParams{
				Name:      appName,
				ClassName: "ash/ShelfAppButton",
			}, param.wantIcon, 15*time.Second); err != nil {
				s.Error("Could not confirm the desired status of the Web Store Icon on the system shelf: ", err)
			}
		})
	}
}

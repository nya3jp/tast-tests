// Copyright 2021 The ChromiumOS Authors
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
	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/policyutil"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         HideWebStoreIcon,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Behavior of HideWebStoreIcon policy, check if a Web Store Icon is displayed in app launcher based on the value of the policy",
		Contacts: []string{
			"kamilszarek@google.com", // Test owner
		},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:mainline", "informational"},
		Fixture:      fixture.ChromePolicyLoggedIn,
		SearchFlags: []*testing.StringPair{
			pci.SearchFlag(&policy.PinnedLauncherApps{}, pci.VerifiedFunctionalityUI),
			pci.SearchFlag(&policy.HideWebStoreIcon{}, pci.VerifiedFunctionalityUI),
		},
	})
}

// HideWebStoreIcon tests the HideWebStoreIcon policy.
func HideWebStoreIcon(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(chrome.HasChrome).Chrome()
	fdms := s.FixtValue().(fakedms.HasFakeDMS).FakeDMS()

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
						nodewith.Name("Launcher").HasClass("ash/HomeButton"),
						uia.Exists(nodewith.HasClass("AppListBubbleView")),
					),
				)(ctx); err != nil {
					s.Fatal("Failed to open Apps list view: ", err)
				}
			}

			appName := apps.WebStore.Name

			// Confirm the status of the Web Store icon in the application launcher
			if err := policyutil.WaitUntilExistsStatus(ctx, tconn, nodewith.Name(appName).HasClass("AppListItemView"), param.wantIcon, 15*time.Second); err != nil {
				s.Error("Could not confirm the desired status of the Web Store Icon in the application launcher: ", err)
			}

			// Confirm the status of the Web Store icon on the shelf
			if err := policyutil.WaitUntilExistsStatus(ctx, tconn, nodewith.Name(appName).HasClass("ash/ShelfAppButton"), param.wantIcon, 15*time.Second); err != nil {
				s.Error("Could not confirm the desired status of the Web Store Icon on the system shelf: ", err)
			}
		})
	}
}

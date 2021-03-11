// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package policy

import (
	"context"

	"chromiumos/tast/common/policy"
	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/policyutil"
	"chromiumos/tast/local/policyutil/fixtures"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: PinnedLauncherApps,
		Desc: "Test the behavior of PinnedLauncherApps policy: apps in the policy are pinned on the shelf and cannot be unpinned",
		Contacts: []string{
			"snijhara@google.com", // Test author
			"chromeos-commercial-stability@google.com",
		},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:mainline", "informational"},
		Fixture:      "chromePolicyLoggedIn",
	})
}

func PinnedLauncherApps(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(*fixtures.FixtData).Chrome
	fdms := s.FixtValue().(*fixtures.FixtData).FakeDMS

	// Connect to Test API to use it with the UI library.
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}

	s.Run(ctx, "pin Files app", func(ctx context.Context, s *testing.State) {
		defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)

		// Perform cleanup.
		if err := policyutil.ResetChrome(ctx, fdms, cr); err != nil {
			s.Fatal("Failed to clean up: ", err)
		}

		// Update the policy to pin the files app.
		policyValue := policy.PinnedLauncherApps{Val: []string{apps.Files.ID}}
		if err := policyutil.ServeAndVerify(ctx, fdms, cr, []policy.Policy{&policyValue}); err != nil {
			s.Fatal("Failed to update policies: ", err)
		}

		// Check that files app is a pinned app on the shelf.
		shelfItems, err := ash.ShelfItems(ctx, tconn)
		if err != nil {
			s.Fatal("Failed to get shelf items: ", err)
		}
		found := false
		for _, shelfItem := range shelfItems {
			if shelfItem.AppID == apps.Files.ID && shelfItem.Title == apps.Files.Name && shelfItem.Type == ash.ShelfItemTypePinnedApp {
				found = true
				break
			}
		}
		if !found {
			s.Fatal("Files was not found in the list of pinned shelf items: ", err)
		}

		ui := uiauto.New(tconn)
		filesAppShelfButton := nodewith.Name(apps.Files.Name).ClassName("ash/ShelfAppButton")
		unpinContextMenuItem := nodewith.Name("Unpin").ClassName("MenuItemView")
		if err := uiauto.Combine("check unpin option is not present for pinned app",
			ui.RightClick(filesAppShelfButton),
			ui.WaitUntilGone(unpinContextMenuItem),
			// This extra left click is needed to dismiss the context menu.
			ui.LeftClick(filesAppShelfButton),
		)(ctx); err != nil {
			s.Fatal("Failed to check unpin option in context menu: ", err)
		}

		// Reset the policy so that files app is no longer pinned.
		if err := policyutil.ServeAndVerify(ctx, fdms, cr, []policy.Policy{}); err != nil {
			s.Fatal("Failed to update policies: ", err)
		}

		if err := uiauto.Combine("unpin the app after policy reset",
			ui.RightClick(filesAppShelfButton),
			ui.WaitUntilExists(unpinContextMenuItem),
			ui.LeftClick(unpinContextMenuItem),
			ui.WaitUntilExists(nodewith.Name(apps.Files.Name+" was un-pinned")),
		)(ctx); err != nil {
			s.Fatal("Failed to unpin the app after policy reset: ", err)
		}
	})
}

// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package accountmanager provides functions to manage accounts in-session.
package accountmanager

import (
	"context"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/accountmanager"
	"chromiumos/tast/local/chrome/browser"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         ChangeARCAvailability,
		LacrosStatus: testing.LacrosVariantExists,
		Desc:         "Verify that ARC availability can be changed in OS Settings",
		Contacts:     []string{"anastasiian@chromium.org", "team-dent@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Params: []testing.Param{{
			ExtraSoftwareDeps: []string{"android_p", "lacros"},
			Fixture:           "loggedInToChromeAndArcWithLacros",
			Val:               browser.TypeLacros,
		}, {
			Name:              "vm",
			ExtraSoftwareDeps: []string{"android_vm", "lacros"},
			Fixture:           "loggedInToChromeAndArcWithLacros",
			Val:               browser.TypeLacros,
		}},
		VarDeps: []string{"accountmanager.username2", "accountmanager.password2"},
		Timeout: 8 * time.Minute,
	})
}

func ChangeARCAvailability(ctx context.Context, s *testing.State) {
	username := s.RequiredVar("accountmanager.username2")
	password := s.RequiredVar("accountmanager.password2")

	// Reserve one minute for various cleanup.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, time.Minute)
	defer cancel()

	cr := s.FixtValue().(accountmanager.FixtureData).Chrome()

	// Connect to Test API to use it with the UI library.
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect Test API: ", err)
	}
	defer faillog.DumpUITreeWithScreenshotOnError(ctx, s.OutDir(), s.HasError, cr, "change_arc_availability")

	defer func(ctx context.Context) {
		s.Log("Running test cleanup")
		if err := accountmanager.TestCleanup(ctx, tconn, cr, s.Param().(browser.Type)); err != nil {
			s.Fatal("Failed to do cleanup: ", err)
		}
	}(cleanupCtx)

	ui := uiauto.New(tconn).WithTimeout(time.Minute)
	a := s.FixtValue().(accountmanager.FixtureData).ARC
	defer a.DumpUIHierarchyOnError(ctx, s.OutDir(), s.HasError)

	arcDevice, err := a.NewUIDevice(ctx)
	if err != nil {
		s.Fatal("Failed to initialize UI Automator: ", err)
	}
	defer arcDevice.Close(ctx)

	addAccountButton := nodewith.Name("Add Google Account").Role(role.Button)
	moreActionsButton := nodewith.Name("More actions, " + username).Role(role.Button)
	removeFromARCButton := nodewith.Name("Stop using with Android apps").Role(role.MenuItem)
	addToARCButton := nodewith.Name("Use with Android apps").Role(role.MenuItem)

	// Open Account Manager page in OS Settings and click Add Google Account button.
	if err := uiauto.Combine("Click Add Google Account button",
		accountmanager.OpenAccountManagerSettingsAction(tconn, cr),
		ui.LeftClickUntil(addAccountButton, ui.Exists(accountmanager.AddAccountDialog())),
	)(ctx); err != nil {
		s.Fatal("Failed to click Add Google Account button: ", err)
	}

	// ARC toggle should be checked.
	if err := accountmanager.CheckArcToggleStatus(ctx, tconn, browser.TypeLacros, true); err != nil {
		s.Fatal("Failed to check ARC toggle status: ", err)
	}

	s.Log("Adding a secondary Account")
	if err := accountmanager.AddAccount(ctx, tconn, username, password); err != nil {
		s.Fatal("Failed to add a secondary Account: ", err)
	}

	// Make sure that the settings page is focused again.
	if err := ui.WaitUntilExists(addAccountButton)(ctx); err != nil {
		s.Fatal("Failed to find Add Google Account button: ", err)
	}
	// Find "More actions, <email>" button to make sure that account was added.
	if err := ui.WaitUntilExists(moreActionsButton)(ctx); err != nil {
		s.Fatal("Failed to find More actions button: ", err)
	}

	// Check that account is present in ARC.
	if err := accountmanager.CheckIsAccountPresentInArcAction(tconn, arcDevice, username, true /*expectedPresentInArc*/)(ctx); err != nil {
		s.Fatal("Failed to check that account is present in ARC: ", err)
	}

	if err := uiauto.Combine("Remove account from ARC",
		// Open OS Settings again.
		accountmanager.OpenAccountManagerSettingsAction(tconn, cr),
		// Find and click "More actions, <email>" > "Stop using with Android apps" button.
		ui.WaitUntilExists(moreActionsButton),
		ui.LeftClick(moreActionsButton),
		ui.WaitUntilExists(removeFromARCButton),
		ui.LeftClick(removeFromARCButton),
		// Check that account is not present in ARC.
		accountmanager.CheckIsAccountPresentInArcAction(tconn, arcDevice, username, false /*expectedPresentInArc*/),
	)(ctx); err != nil {
		s.Fatal("Failed to remove account from ARC: ", err)
	}

	if err := uiauto.Combine("Add account to ARC",
		// Open OS Settings again.
		accountmanager.OpenAccountManagerSettingsAction(tconn, cr),
		// Find and click "More actions, <email>" > "Use with Android apps" button.
		ui.WaitUntilExists(moreActionsButton),
		ui.LeftClick(moreActionsButton),
		ui.WaitUntilExists(addToARCButton),
		ui.LeftClick(addToARCButton),
		// Check that account is present in ARC.
		accountmanager.CheckIsAccountPresentInArcAction(tconn, arcDevice, username, true /*expectedPresentInArc*/),
	)(ctx); err != nil {
		s.Fatal("Failed to add account to ARC: ", err)
	}
}

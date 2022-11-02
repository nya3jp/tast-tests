// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package accountmanager provides functions to manage accounts in-session.
package accountmanager

import (
	"context"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/accountmanager"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/chrome/browser"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         ARCAccountPickerNewAccount,
		LacrosStatus: testing.LacrosVariantExists,
		Desc:         "Verify new account addition from ARC account picker",
		Contacts:     []string{"anastasiian@chromium.org", "team-dent@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome", "lacros"},
		Params: []testing.Param{{
			ExtraSoftwareDeps: []string{"android_p"},
			Fixture:           "loggedInToChromeAndArcWithLacros",
		}, {
			Name:              "vm",
			ExtraSoftwareDeps: []string{"android_vm"},
			Fixture:           "loggedInToChromeAndArcWithLacros",
		}},
		VarDeps: []string{
			"accountmanager.ARCAccountPickerNewAccount.username1",
			"accountmanager.ARCAccountPickerNewAccount.password1",
			"accountmanager.ARCAccountPickerNewAccount.username2",
			"accountmanager.ARCAccountPickerNewAccount.password2",
		},
		Timeout: 6 * time.Minute,
	})
}

func ARCAccountPickerNewAccount(ctx context.Context, s *testing.State) {
	username1 := s.RequiredVar("accountmanager.ARCAccountPickerNewAccount.username1")
	password1 := s.RequiredVar("accountmanager.ARCAccountPickerNewAccount.password1")
	username2 := s.RequiredVar("accountmanager.ARCAccountPickerNewAccount.username2")
	password2 := s.RequiredVar("accountmanager.ARCAccountPickerNewAccount.password2")

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

	defer func(ctx context.Context) {
		s.Log("Running test cleanup")
		if err := accountmanager.TestCleanup(ctx, tconn, cr); err != nil {
			s.Fatal("Failed to do cleanup: ", err)
		}
	}(cleanupCtx)

	defer faillog.DumpUITreeWithScreenshotOnError(cleanupCtx, s.OutDir(), s.HasError, cr, "arc_account_picker")

	ui := uiauto.New(tconn).WithTimeout(time.Minute)
	a := s.FixtValue().(accountmanager.FixtureData).ARC
	defer a.DumpUIHierarchyOnError(ctx, s.OutDir(), s.HasError)

	d, err := a.NewUIDevice(ctx)
	if err != nil {
		s.Fatal("Failed initializing UI Automator: ", err)
	}
	defer d.Close(ctx)

	addAccountButton := nodewith.Name("Add Google Account").Role(role.Button)
	addAccountDialog := accountmanager.AddAccountDialog()
	arcToggle := nodewith.NameStartingWith("Use this account with Android apps").Role(role.ToggleButton).Ancestor(addAccountDialog)

	// Open Account Manager page in OS Settings and click Add Google Account button.
	if err := uiauto.Combine("click add Google Account button",
		accountmanager.OpenAccountManagerSettingsAction(tconn, cr),
		ui.LeftClickUntil(addAccountButton, ui.Exists(accountmanager.AddAccountDialog())),
		// Uncheck ARC toggle.
		ui.LeftClickUntil(arcToggle, accountmanager.CheckARCToggleStatusAction(tconn, browser.TypeLacros, false /*expectedVal*/)),
	)(ctx); err != nil {
		s.Fatal("Failed to click add Google Account button: ", err)
	}

	s.Log("Adding a secondary Account")
	if err := accountmanager.AddAccount(ctx, tconn, username1, password1); err != nil {
		s.Fatal("Failed to add a secondary Account: ", err)
	}

	moreActionsButton := nodewith.Name("More actions, " + username1).Role(role.Button)
	if err := uiauto.Combine("confirm account addition",
		// Make sure that the settings page is focused again.
		ui.WaitUntilExists(addAccountButton),
		// Find "More actions, <email>" button to make sure that account was added.
		ui.WaitUntilExists(moreActionsButton),
		// Check that account is not present in ARC.
		accountmanager.CheckIsAccountPresentInARCAction(tconn, d,
			accountmanager.NewARCAccountOptions(username1).ExpectedPresentInARC(false)),
	)(ctx); err != nil {
		s.Fatal("Failed to confirm account addition: ", err)
	}

	if err := arc.ClickAddAccountInSettings(ctx, d, tconn); err != nil {
		s.Fatal("Failed to open Add account dialog from ARC")
	}

	addAccountItem := nodewith.Name("Add Google Account").Role(role.Button).Focusable().Ancestor(addAccountDialog)
	if err := uiauto.Combine("add account to ARC from account picker",
		ui.WaitUntilExists(addAccountItem),
		ui.LeftClick(addAccountItem),
	)(ctx); err != nil {
		s.Fatal("Failed to add account to ARC from account picker: ", err)
	}

	s.Log("Adding a secondary Account")
	if err := accountmanager.AddAccount(ctx, tconn, username2, password2); err != nil {
		s.Fatal("Failed to add a secondary Account: ", err)
	}

	// Check that account is present in ARC.
	s.Log("Verifying that account is present in ARC")
	if err := accountmanager.CheckIsAccountPresentInARCAction(tconn, d,
		accountmanager.NewARCAccountOptions(username2).ExpectedPresentInARC(true))(ctx); err != nil {
		s.Fatal("Failed to check that account is present in ARC: ", err)
	}
}

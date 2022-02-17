// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package accountmanager provides functions to manage accounts in-session.
package accountmanager

import (
	"context"
	"time"

	"chromiumos/tast/common/action"
	androidui "chromiumos/tast/common/android/ui"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/accountmanager"
	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/browser"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         ARCAccountPicker,
		LacrosStatus: testing.LacrosVariantExists,
		Desc:         "Verify ARC account picker behavior",
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
		VarDeps: []string{"accountmanager.username2", "accountmanager.password2"},
		Timeout: 6 * time.Minute,
	})
}

func ARCAccountPicker(ctx context.Context, s *testing.State) {
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
	defer faillog.DumpUITreeWithScreenshotOnError(ctx, s.OutDir(), s.HasError, cr, "arc_account_picker")

	defer func(ctx context.Context) {
		s.Log("Running test cleanup")
		if err := accountmanager.TestCleanup(ctx, tconn, cr, browser.TypeLacros); err != nil {
			s.Fatal("Failed to do cleanup: ", err)
		}
	}(cleanupCtx)

	ui := uiauto.New(tconn).WithTimeout(time.Minute)
	a := s.FixtValue().(accountmanager.FixtureData).ARC
	defer a.DumpUIHierarchyOnError(ctx, s.OutDir(), s.HasError)

	d, err := a.NewUIDevice(ctx)
	if err != nil {
		s.Fatal("Failed initializing UI Automator: ", err)
	}
	defer d.Close(ctx)

	addAccountButton := nodewith.Name("Add Google Account").Role(role.Button)
	moreActionsButton := nodewith.Name("More actions, " + username).Role(role.Button)
	addAccountDialog := accountmanager.AddAccountDialog()
	arcToggle := nodewith.NameStartingWith("Use this account with Android apps").Role(role.ToggleButton).Ancestor(addAccountDialog)

	// Open Account Manager page in OS Settings and click Add Google Account button.
	if err := uiauto.Combine("click add Google Account button",
		accountmanager.OpenAccountManagerSettingsAction(tconn, cr),
		ui.LeftClickUntil(addAccountButton, ui.Exists(accountmanager.AddAccountDialog())),
		// Uncheck ARC toggle.
		ui.LeftClickUntil(arcToggle, checkARCToggleStatusAction(tconn, false)),
	)(ctx); err != nil {
		s.Fatal("Failed to click add Google Account button: ", err)
	}

	s.Log("Adding a secondary Account")
	if err := accountmanager.AddAccount(ctx, tconn, username, password); err != nil {
		s.Fatal("Failed to add a secondary Account: ", err)
	}

	if err := uiauto.Combine("confirm account addition",
		// Make sure that the settings page is focused again.
		ui.WaitUntilExists(addAccountButton),
		// Find "More actions, <email>" button to make sure that account was added.
		ui.WaitUntilExists(moreActionsButton),
		// Check that account is not present in ARC.
		accountmanager.CheckIsAccountPresentInArcAction(tconn, d, username, false),
	)(ctx); err != nil {
		s.Fatal("Failed to confirm account addition: ", err)
	}

	accountPickerItem := nodewith.NameContaining(username).Role(role.Button).Focusable().Ancestor(addAccountDialog)
	if err := uiauto.Combine("add account to ARC from account picker",
		openAddAccountDialogFromARCAction(d, tconn),
		ui.WaitUntilExists(accountPickerItem),
		// Click on account to add it to ARC.
		ui.LeftClick(accountPickerItem),
		// Check that account is present in ARC.
		accountmanager.CheckIsAccountPresentInArcAction(tconn, d, username, true),
	)(ctx); err != nil {
		s.Fatal("Failed to add account to ARC from account picker: ", err)
	}
}

// checkARCToggleStatusAction returns an action that runs accountmanager.CheckArcToggleStatus.
func checkARCToggleStatusAction(tconn *chrome.TestConn, expectedVal bool) action.Action {
	return func(ctx context.Context) error {
		if err := accountmanager.CheckArcToggleStatus(ctx, tconn, browser.TypeLacros, expectedVal); err != nil {
			return errors.Wrap(err, "failed to check ARC toggle status")
		}
		return nil
	}
}

// openAddAccountDialogFromARCAction returns an action that clicks 'Add account' button in ARC settings.
func openAddAccountDialogFromARCAction(d *androidui.Device, tconn *chrome.TestConn) action.Action {
	return func(ctx context.Context) error {
		const scrollClassName = "android.widget.ScrollView"

		if err := apps.Launch(ctx, tconn, apps.AndroidSettings.ID); err != nil {
			return errors.Wrap(err, "failed to launch AndroidSettings")
		}

		scrollLayout := d.Object(androidui.ClassName(scrollClassName),
			androidui.Scrollable(true))
		accounts := d.Object(androidui.ClassName("android.widget.TextView"),
			androidui.TextMatches("(?i)Accounts"), androidui.Enabled(true))
		if err := scrollLayout.WaitForExists(ctx, accountmanager.DefaultUITimeout); err == nil {
			scrollLayout.ScrollTo(ctx, accounts)
		}

		if err := accounts.Click(ctx); err != nil {
			return errors.Wrap(err, "failed to click on System")
		}

		addAccount := d.Object(androidui.ClassName("android.widget.TextView"),
			androidui.TextMatches("(?i)Add account"), androidui.Enabled(true))

		if err := addAccount.WaitForExists(ctx, accountmanager.DefaultUITimeout); err != nil {
			return errors.Wrap(err, "failed finding addAccount Text View")
		}

		if err := addAccount.Click(ctx); err != nil {
			return errors.Wrap(err, "failed to click addAccount")
		}
		return nil
	}
}

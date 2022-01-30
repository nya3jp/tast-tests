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
		LacrosStatus: testing.LacrosVariantUnknown,
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
		Timeout: 6 * time.Minute,
	})
}

func ChangeARCAvailability(ctx context.Context, s *testing.State) {
	username := s.RequiredVar("accountmanager.username2")
	password := s.RequiredVar("accountmanager.password2")

	addAccountButton := nodewith.Name("Add Google Account").Role(role.Button)
	moreActionsButton := nodewith.Name("More actions, " + username).Role(role.Button)
	removeFromARCButton := nodewith.Name("Stop using with Android apps").Role(role.MenuItem)
	addToARCButton := nodewith.Name("Use with Android apps").Role(role.MenuItem)

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

	d, err := a.NewUIDevice(ctx)
	if err != nil {
		s.Fatal("Failed initializing UI Automator: ", err)
	}
	defer d.Close(ctx)

	if err := uiauto.Combine("Test ARC availability controls",
		uiauto.Combine("Add a secondary account",
			accountmanager.OpenAccountManagerSettingsAction(tconn, cr),
			ui.LeftClick(addAccountButton),
			accountmanager.CheckArcToggleStatusAction(tconn, true),
			accountmanager.AddAccountAction(tconn, username, password),
			// Make sure that the settings page is focused again.
			ui.WaitUntilExists(addAccountButton),
			// Find "More actions, <email>" button to make sure that account was added.
			ui.WaitUntilExists(moreActionsButton),
			// Check that account is present in ARC.
			accountmanager.CheckAccountPresentInArcAction(tconn, d, username),
		),
		uiauto.Combine("Remove account from ARC",
			accountmanager.OpenAccountManagerSettingsAction(tconn, cr),
			ui.WaitUntilExists(moreActionsButton),
			ui.LeftClick(moreActionsButton),
			ui.WaitUntilExists(removeFromARCButton),
			ui.LeftClick(removeFromARCButton),
			// Check that account is not present in ARC.
			accountmanager.CheckAccountNotPresentInArcAction(tconn, d, username),
		),
		uiauto.Combine("Add account to ARC",
			accountmanager.OpenAccountManagerSettingsAction(tconn, cr),
			ui.WaitUntilExists(moreActionsButton),
			ui.LeftClick(moreActionsButton),
			ui.WaitUntilExists(addToARCButton),
			ui.LeftClick(addToARCButton),
			// Check that account is present in ARC.
			accountmanager.CheckAccountPresentInArcAction(tconn, d, username),
		),
	)(ctx); err != nil {
		s.Fatal("Failed to test ARC availability controls: ", err)
	}
}

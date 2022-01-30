// Copyright 2021 The Chromium OS Authors. All rights reserved.
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
	"chromiumos/tast/local/chrome/browser/browserfixt"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         AddAccountOSSettings,
		LacrosStatus: testing.LacrosVariantUnknown,
		Desc:         "Verify that a secondary account can be added and removed from OS Settings",
		Contacts:     []string{"anastasiian@chromium.org", "team-dent@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Params: []testing.Param{{
			ExtraSoftwareDeps: []string{"android_p"},
			Fixture:           "loggedInToChromeAndArc",
			Val:               browser.TypeAsh,
		}, {
			Name:              "vm",
			ExtraSoftwareDeps: []string{"android_vm"},
			Fixture:           "loggedInToChromeAndArc",
			Val:               browser.TypeAsh,
		}, {
			Name:              "lacros",
			ExtraSoftwareDeps: []string{"android_p", "lacros"},
			Fixture:           "loggedInToChromeAndArcWithLacros",
			Val:               browser.TypeLacros,
		}, {
			Name:              "vm_lacros",
			ExtraSoftwareDeps: []string{"android_vm", "lacros"},
			Fixture:           "loggedInToChromeAndArcWithLacros",
			Val:               browser.TypeLacros,
		}},
		VarDeps: []string{"accountmanager.username2", "accountmanager.password2"},
		Timeout: 6 * time.Minute,
	})
}

func AddAccountOSSettings(ctx context.Context, s *testing.State) {
	username := s.RequiredVar("accountmanager.username2")
	password := s.RequiredVar("accountmanager.password2")

	// Reserve one minute for various cleanup.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, time.Minute)
	defer cancel()

	cr := s.FixtValue().(accountmanager.FixtureData).Chrome()

	// Setup the browser.
	br, closeBrowser, err := browserfixt.SetUp(ctx, s.FixtValue(), s.Param().(browser.Type))
	if err != nil {
		s.Fatal("Failed to setup chrome: ", err)
	}
	defer closeBrowser(cleanupCtx)

	// Connect to Test API to use it with the UI library.
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect Test API: ", err)
	}
	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)

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

	addAccountButton := nodewith.Name("Add Google Account").Role(role.Button)
	moreActionsButton := nodewith.Name("More actions, " + username).Role(role.Button)
	secondaryAccountListItem := nodewith.NameContaining(username).Role(role.Link)

	s.Log("Adding a secondary Account")
	if err := uiauto.Combine("Add a secondary account",
		accountmanager.OpenAccountManagerSettingsAction(tconn, cr),
		// Click the button to open account addition dialog.
		ui.LeftClick(addAccountButton),
		accountmanager.CheckArcToggleStatusAction(tconn, true),
		accountmanager.AddAccountAction(tconn, username, password),
		// Make sure that the settings page is focused again.
		ui.WaitUntilExists(addAccountButton),
		// Find "More actions, <email>" button to make sure that account was added.
		ui.WaitUntilExists(moreActionsButton),
		// Check that account is present in OGB.
		accountmanager.CheckOneGoogleBarAction(tconn, br, ui.WaitUntilExists(secondaryAccountListItem)),
		// Check that account is present in ARC.
		accountmanager.CheckAccountPresentInArcAction(tconn, d, username),
	)(ctx); err != nil {
		s.Fatal("Failed to add a secondary account: ", err)
	}

	if err := uiauto.Combine("Remove account",
		accountmanager.OpenAccountManagerSettingsAction(tconn, cr),
		ui.WaitUntilExists(moreActionsButton),
		ui.LeftClick(moreActionsButton),
		accountmanager.RemoveAccountFromOSSettingsAction(tconn, s.Param().(browser.Type)),
		ui.WaitUntilGone(moreActionsButton),
		accountmanager.CheckOneGoogleBarAction(tconn, br, ui.WaitUntilGone(secondaryAccountListItem)),
		// Check that account is not present in ARC.
		accountmanager.CheckAccountNotPresentInArcAction(tconn, d, username),
	)(ctx); err != nil {
		s.Fatal("Failed to remove account: ", err)
	}
}

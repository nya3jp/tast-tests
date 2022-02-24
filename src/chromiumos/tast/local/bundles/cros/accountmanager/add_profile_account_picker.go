// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package accountmanager provides functions to manage accounts in-session.
package accountmanager

import (
	"context"
	"strings"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/accountmanager"
	"chromiumos/tast/local/chrome/browser"
	"chromiumos/tast/local/chrome/lacros"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         AddProfileAccountPicker,
		LacrosStatus: testing.LacrosVariantExists,
		Desc:         "Addition of a secondary profile with account from a profile picker",
		Contacts:     []string{"anastasiian@chromium.org", "team-dent@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome", "lacros"},
		Fixture:      "loggedInToLacros",
		VarDeps:      []string{"accountmanager.username1", "accountmanager.password1"},
		Timeout:      6 * time.Minute,
	})
}

func AddProfileAccountPicker(ctx context.Context, s *testing.State) {
	username := s.RequiredVar("accountmanager.username1")
	password := s.RequiredVar("accountmanager.password1")

	// Reserve one minute for various cleanup.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, time.Minute)
	defer cancel()

	// Setup the browser.
	cr, l, cs, err := lacros.Setup(ctx, s.FixtValue(), browser.TypeLacros)
	if err != nil {
		s.Fatal("Failed to initialize test: ", err)
	}
	defer lacros.CloseLacros(cleanupCtx, l)

	// Connect to Test API to use it with the UI library.
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect Test API: ", err)
	}

	defer func(ctx context.Context) {
		s.Log("Running test cleanup")
		if err := accountmanager.TestCleanup(ctx, tconn, cr, browser.TypeLacros); err != nil {
			s.Fatal("Failed to do cleanup: ", err)
		}
	}(cleanupCtx)

	defer faillog.DumpUITreeWithScreenshotOnError(ctx, s.OutDir(), s.HasError, cr, "add_profile_account_picker")

	ui := uiauto.New(tconn).WithTimeout(accountmanager.DefaultUITimeout)

	addAccountButton := nodewith.Name("Add Google Account").Role(role.Button)
	moreActionsButton := nodewith.Name("More actions, " + username).Role(role.Button)
	if err := uiauto.Combine("Add a secondary account in OS Settings",
		accountmanager.OpenAccountManagerSettingsAction(tconn, cr),
		ui.LeftClick(addAccountButton),
		func(ctx context.Context) error {
			return accountmanager.AddAccount(ctx, tconn, username, password)
		},
		ui.WaitUntilExists(addAccountButton),
		// Check that account was added.
		ui.WaitUntilExists(moreActionsButton),
	)(ctx); err != nil {
		s.Fatal("Failed to add an account: ", err)
	}

	// Open a new tab
	conn, err := cs.NewConn(ctx, "chrome://version/")
	if err != nil {
		s.Fatal("Failed to open a new tab in Lacros browser: ", err)
	}
	defer conn.Close()

	// Browser controls to open a profile:
	profileToolbarButton := nodewith.ClassName("AvatarToolbarButton").Role(role.Button).Focusable()
	profileMenu := nodewith.NameStartingWith("Accounts and sync").Role(role.Menu)
	addProfileButton := nodewith.Name("Add").Role(role.Button).Focusable().Ancestor(profileMenu)

	// Nodes in the profile addition dialog:
	accountPicker := nodewith.Name("Choose an account").Role(role.RootWebArea)
	addProfileRoot := nodewith.Name("Set up your new Chrome profile").Role(role.RootWebArea)
	nextButton := nodewith.Name("Next").Role(role.Button).Focusable().Ancestor(addProfileRoot)
	accountEntry := nodewith.NameContaining(username).Role(role.Button).Focusable().Ancestor(accountPicker)

	// Nodes on the last screen of the profile addition dialog:
	finishAddProfileRoot := nodewith.Name("Chrome browser sync is on").Role(role.RootWebArea)
	finishAddProfileHeading := nodewith.Name("Chrome browser sync is on").Role(role.Heading).Ancestor(finishAddProfileRoot)
	doneButton := nodewith.Name("Done").Role(role.Button).Focusable().Ancestor(finishAddProfileRoot)

	if err := uiauto.Combine("Add a profile",
		uiauto.Combine("Click a button to add a profile",
			ui.WaitUntilExists(profileToolbarButton),
			ui.LeftClick(profileToolbarButton),
			ui.WaitUntilExists(addProfileButton),
			ui.LeftClick(addProfileButton),
		),
		uiauto.Combine("Click next and pick an account",
			ui.WaitUntilExists(nextButton),
			ui.WithInterval(time.Second).LeftClickUntil(nextButton, ui.Exists(accountPicker)),
			ui.WaitUntilExists(accountEntry),
			ui.LeftClick(accountEntry),
		),
		uiauto.Combine("Check that the final screen is open and click done",
			ui.WaitUntilExists(finishAddProfileHeading),
			ui.WaitUntilExists(doneButton),
			ui.LeftClick(doneButton),
		),
	)(ctx); err != nil {
		s.Fatal("Failed to create a new profile for secondary account: ", err)
	}

	// There are two Chrome windows open. Find the window of the new profile:
	// the name shouldn't contain "About Version" (unlike the first profile).
	newProfileWindow, err := accountmanager.GetChromeProfileWindow(ctx, tconn, func(node uiauto.NodeInfo) bool {
		return !strings.Contains(node.Name, "About Version")
	})
	if err != nil {
		s.Fatal("Failed to find new Chrome window: ", err)
	}

	// Make sure that a new profile was added for the correct account
	if err := uiauto.Combine("Check that the new profile belongs to the correct account",
		ui.WaitUntilExists(newProfileWindow),
		ui.WaitUntilExists(profileToolbarButton.Ancestor(newProfileWindow)),
		ui.LeftClick(profileToolbarButton.Ancestor(newProfileWindow)),
		// The menu should contain the username of the secondary account.
		ui.WaitUntilExists(nodewith.NameStartingWith("Accounts and sync").NameContaining(username).Role(role.Menu)),
	)(ctx); err != nil {
		s.Fatal("Failed to create a new profile for secondary account: ", err)
	}
}

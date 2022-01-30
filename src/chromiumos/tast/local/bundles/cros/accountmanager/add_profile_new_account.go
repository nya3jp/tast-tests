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
	"chromiumos/tast/local/chrome/lacros"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         AddProfileNewAccount,
		Desc:         "Addition of a secondary profile with a new account",
		Contacts:     []string{"anastasiian@chromium.org", "team-dent@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome", "lacros"},
		Fixture:      "loggedInToLacros",
		VarDeps:      []string{"accountmanager.username1", "accountmanager.password1"},
		Timeout:      6 * time.Minute,
	})
}

func AddProfileNewAccount(ctx context.Context, s *testing.State) {
	username := s.RequiredVar("accountmanager.username1")
	password := s.RequiredVar("accountmanager.password1")

	// Reserve one minute for various cleanup.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, time.Minute)
	defer cancel()

	// Set up the browser.
	cr, l, _, err := lacros.Setup(ctx, s.FixtValue(), browser.TypeLacros)
	if err != nil {
		s.Fatal("Failed to initialize test: ", err)
	}
	defer lacros.CloseLacros(cleanupCtx, l)

	// Connect to Test API to use it with the UI library.
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect Test API: ", err)
	}
	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)

	defer func(ctx context.Context) {
		s.Log("Running test cleanup")
		if err := accountmanager.TestCleanup(ctx, tconn, cr, browser.TypeLacros); err != nil {
			s.Fatal("Failed to do cleanup: ", err)
		}
	}(cleanupCtx)

	ui := uiauto.New(tconn).WithTimeout(accountmanager.DefaultUITimeout)

	// Browser controls to open a profile:
	profileToolbarButton := nodewith.ClassName("AvatarToolbarButton").Role(role.Button).Focusable()
	profileMenu := nodewith.NameStartingWith("Accounts and sync").Role(role.Menu)
	addProfileButton := nodewith.Name("Add").Role(role.Button).Focusable().Ancestor(profileMenu)

	// Nodes in the profile addition dialog:
	addAccountDialog := accountmanager.GetAddAccountDialog()
	addProfileRoot := nodewith.Name("Set up your new Chrome profile").Role(role.RootWebArea)
	nextButton := nodewith.Name("Next").Role(role.Button).Focusable().Ancestor(addProfileRoot)
	finishAddProfileRoot := nodewith.Name("Chrome browser sync is on").Role(role.RootWebArea)
	doneButton := nodewith.Name("Done").Role(role.Button).Focusable().Ancestor(finishAddProfileRoot)

	// Nodes that belong to the new profile:
	newProfileWindow := nodewith.NameContaining("Google Chrome").Role(role.Window).Focused()
	newProfileMenu := nodewith.NameStartingWith("Accounts and sync").NameContaining(username).Role(role.Menu)

	s.Log("Adding a new profile")
	if err := uiauto.Combine("Add a new profile",
		uiauto.Combine("Click a button to add a profile",
			ui.WaitUntilExists(profileToolbarButton),
			ui.LeftClick(profileToolbarButton),
			ui.WaitUntilExists(addProfileButton),
			ui.LeftClick(addProfileButton),
		),
		uiauto.Combine("Click next and add an account",
			ui.WaitUntilExists(nextButton),
			ui.WithInterval(time.Second).LeftClickUntil(nextButton, ui.Exists(addAccountDialog)),
			accountmanager.AddAccountAction(tconn, username, password),
		),
		uiauto.Combine("Check that the final screen is open and click done",
			ui.WaitUntilExists(nodewith.Name("Chrome browser sync is on").Role(role.Heading).Ancestor(finishAddProfileRoot)),
			ui.WaitUntilExists(doneButton),
			ui.LeftClick(doneButton),
		),
		uiauto.Combine("Check that the new profile was created and belongs to the correct account",
			ui.WaitUntilExists(newProfileWindow),
			ui.WaitUntilExists(profileToolbarButton.Ancestor(newProfileWindow)),
			ui.LeftClick(profileToolbarButton.Ancestor(newProfileWindow)),
			ui.WaitUntilExists(newProfileMenu),
		),
	)(ctx); err != nil {
		s.Fatal("Failed to create a new profile for secondary account: ", err)
	}
}

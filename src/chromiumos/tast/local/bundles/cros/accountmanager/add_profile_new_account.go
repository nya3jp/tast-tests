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

	profileToolbarButton := nodewith.ClassName("AvatarToolbarButton").Role(role.Button).Focusable()
	if err := uiauto.Combine("Click on profileToolbarButton",
		ui.WaitUntilExists(profileToolbarButton),
		ui.LeftClick(profileToolbarButton),
	)(ctx); err != nil {
		s.Fatal("Failed to click on profileToolbarButton: ", err)
	}

	profileMenu := nodewith.NameStartingWith("Accounts and sync").Role(role.Menu)
	addProfileButton := nodewith.Name("Add").Role(role.Button).Focusable().Ancestor(profileMenu)
	if err := uiauto.Combine("Click on addProfileButton",
		ui.WaitUntilExists(addProfileButton),
		ui.LeftClick(addProfileButton),
	)(ctx); err != nil {
		s.Fatal("Failed to click on addProfileButton: ", err)
	}

	s.Log("Adding a new profile")
	addAccountDialog := accountmanager.GetAddAccountDialog()
	addProfileRoot := nodewith.Name("Set up your new Chrome profile").Role(role.RootWebArea)
	nextButton := nodewith.Name("Next").Role(role.Button).Focusable().Ancestor(addProfileRoot)
	if err := uiauto.Combine("Click on nextButton",
		ui.WaitUntilExists(nextButton),
		ui.WithInterval(time.Second).LeftClickUntil(nextButton, ui.Exists(addAccountDialog)),
	)(ctx); err != nil {
		s.Fatal("Failed to click on nextButton: ", err)
	}

	s.Log("Adding a secondary account")
	if err := accountmanager.AddAccount(ctx, tconn, username, password); err != nil {
		s.Fatal("Failed to add a secondary account: ", err)
	}

	s.Log("Finish profile addition")
	finishAddProfileRoot := nodewith.Name("Chrome browser sync is on").Role(role.RootWebArea)
	doneButton := nodewith.Name("Done").Role(role.Button).Focusable().Ancestor(finishAddProfileRoot)
	if err := uiauto.Combine("Click on doneButton",
		ui.WaitUntilExists(nodewith.Name("Chrome browser sync is on").Role(role.Heading).Ancestor(finishAddProfileRoot)),
		ui.WaitUntilExists(doneButton),
		ui.LeftClick(doneButton),
	)(ctx); err != nil {
		s.Fatal("Failed to click on doneButton: ", err)
	}

	s.Log("Make sure that a new profile was added for the correct account")
	// Window of the new profile should be focused now.
	newProfileWindow := nodewith.NameContaining("Google Chrome").Role(role.Window).Focused()
	if err := uiauto.Combine("Click on profileToolbarButton in new profile",
		ui.WaitUntilExists(newProfileWindow),
		ui.WaitUntilExists(profileToolbarButton.Ancestor(newProfileWindow)),
		ui.LeftClick(profileToolbarButton.Ancestor(newProfileWindow)),
	)(ctx); err != nil {
		s.Fatal("Failed to click on profileToolbarButton in new profile: ", err)
	}

	newProfileMenu := nodewith.NameStartingWith("Accounts and sync").NameContaining(username).Role(role.Menu)
	if err := ui.WaitUntilExists(newProfileMenu)(ctx); err != nil {
		s.Fatalf("Failed to check that a new profile for the secondary account %v was created: %v", username, err)
	}
}

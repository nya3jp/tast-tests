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
	"chromiumos/tast/local/chrome/uiauto/ossettings"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         AddProfileAccountPicker,
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
	defer lacros.CloseLacrosChrome(cleanupCtx, l)

	// Connect to Test API to use it with the UI library.
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect Test API: ", err)
	}
	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)

	s.Log("Runing test cleanup")
	if err := accountmanager.TestCleanup(ctx, tconn, cr, browser.TypeLacros); err != nil {
		s.Fatal("Failed to do cleanup: ", err)
	}

	ui := uiauto.New(tconn).WithTimeout(accountmanager.DefaultUITimeout)

	// Open Account Manager page in OS Settings and find Add Google Account button.
	addAccountButton := nodewith.Name("Add Google Account").Role(role.Button)
	if _, err := ossettings.LaunchAtPageURL(ctx, tconn, cr, "accountManager", ui.Exists(addAccountButton)); err != nil {
		s.Fatal("Failed to launch Account Manager page: ", err)
	}

	// Click the button to open account addition dialog.
	if err := ui.LeftClick(addAccountButton)(ctx); err != nil {
		s.Fatal("Failed to click Add Google Account button: ", err)
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
	moreActionsButton := nodewith.Name("More actions, " + username).Role(role.Button)
	if err := ui.WaitUntilExists(moreActionsButton)(ctx); err != nil {
		s.Fatal("Failed to find More actions button: ", err)
	}

	// Open a new tab
	conn, err := cs.NewConn(ctx, "chrome://newtab")
	if err != nil {
		s.Fatal("Failed to open a new tab in Lacros browser: ", err)
	}
	defer conn.Close()

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
	accountPicker := nodewith.Name("Choose an account").Role(role.RootWebArea)
	addProfileRoot := nodewith.Name("Set up your new Chrome profile").Role(role.RootWebArea)
	nextButton := nodewith.Name("Next").Role(role.Button).Focusable().Ancestor(addProfileRoot)
	if err := uiauto.Combine("Click on nextButton",
		ui.WaitUntilExists(nextButton),
		ui.WithInterval(time.Second).LeftClickUntil(nextButton, ui.Exists(accountPicker)),
	)(ctx); err != nil {
		s.Fatal("Failed to click on nextButton: ", err)
	}

	accountEntry := nodewith.NameContaining(username).Role(role.Button).Focusable().Ancestor(accountPicker)
	if err := uiauto.Combine("Click on accountEntry",
		ui.WaitUntilExists(accountEntry),
		ui.LeftClick(accountEntry),
	)(ctx); err != nil {
		s.Fatal("Failed to click on accountEntry: ", err)
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

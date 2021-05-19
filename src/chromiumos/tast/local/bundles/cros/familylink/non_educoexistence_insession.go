// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package familylink is used for writing Family Link tests.
package familylink

import (
	"context"
	"time"

	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/familylink"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         NonEducoexistenceInsession,
		Desc:         "Checks that Unicorn account trying to add a non-EDU secondary account fails",
		Contacts:     []string{"tobyhuang@chromium.org", "cros-families-eng@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Timeout:      chrome.GAIALoginTimeout + 5*time.Minute,
		Vars:         []string{"unicorn.parentUser", "unicorn.parentPassword", "unicorn.parentFirstName", "unicorn.parentLastName", "geller.parentUser", "geller.parentPassword"},
		Fixture:      "familyLinkUnicornLogin",
	})
}

func NonEducoexistenceInsession(ctx context.Context, s *testing.State) {
	tconn := s.FixtValue().(*familylink.FixtData).TestConn

	unicornParentUser := s.RequiredVar("unicorn.parentUser")
	unicornParentPass := s.RequiredVar("unicorn.parentPassword")
	unicornParentFirstName := s.RequiredVar("unicorn.parentFirstName")
	unicornParentLastName := s.RequiredVar("unicorn.parentLastName")
	gellerParentUser := s.RequiredVar("geller.parentUser")
	gellerParentPass := s.RequiredVar("geller.parentPassword")

	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)

	s.Log("Launching the settings app")
	if err := apps.Launch(ctx, tconn, apps.Settings.ID); err != nil {
		s.Fatal("Failed to launch the settings app: ", err)
	}

	ui := uiauto.New(tconn)

	s.Log("Opening the in-session EDU Coexistence flow")
	googleAccountsButton := nodewith.Name("Google Accounts").Role(role.Button)
	addSchoolAccountButton := nodewith.Name("Add school account").Role(role.Button)
	selectParentOption := nodewith.NameStartingWith(unicornParentFirstName + " " + unicornParentLastName).Role(role.ListBoxOption)
	if err := uiauto.Combine("open in-session edu coexistence flow",
		ui.WaitUntilExists(googleAccountsButton),
		ui.LeftClickUntil(googleAccountsButton, ui.Exists(addSchoolAccountButton)),
		ui.WithInterval(time.Second).LeftClickUntil(addSchoolAccountButton, ui.Exists(selectParentOption)),
	)(ctx); err != nil {
		s.Fatal("Failed to open in-session edu coexistence flow: ", err)
	}

	s.Log("Clicking button that matches parent email: ", unicornParentUser)
	parentPasswordText := nodewith.Name("Parent password").Role(role.TextField)
	if err := ui.LeftClickUntil(selectParentOption, ui.Exists(parentPasswordText))(ctx); err != nil {
		s.Fatal("Failed to click button that matches parent email: ", err)
	}

	s.Log("Clicking the parent password text field")
	if err := ui.LeftClick(parentPasswordText)(ctx); err != nil {
		s.Fatal("Failed to click parent password text: ", err)
	}

	s.Log("Setting up keyboard")
	kb, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to get keyboard: ", err)
	}
	defer kb.Close()

	s.Log("Typing the parent password")
	if err := kb.Type(ctx, unicornParentPass+"\n"); err != nil {
		s.Fatal("Failed to type parent password: ", err)
	}

	s.Log("Clicking next on school account information for parents and Google workspace for education information pages")
	nextButton := nodewith.Name("Next").Role(role.Button)
	enterSchoolEmailText := nodewith.Name("School email").Role(role.TextField)
	if err := uiauto.Combine("Clicking next",
		ui.WaitUntilExists(nextButton),
		ui.LeftClickUntil(nextButton, ui.Exists(enterSchoolEmailText)))(ctx); err != nil {
		s.Fatal("Failed to click next button: ", err)
	}

	s.Log("Clicking school email text field")
	if err := ui.LeftClick(enterSchoolEmailText)(ctx); err != nil {
		s.Fatal("Failed to click school email text field: ", err)
	}

	s.Log("Typing Geller parent (non-EDU) email")
	if err := kb.Type(ctx, gellerParentUser+"\n"); err != nil {
		s.Fatal("Failed to type Geller parent email: ", err)
	}

	s.Log("Clicking school account password text field")
	schoolPasswordText := nodewith.Name("School password").Role(role.TextField)
	if err := uiauto.Combine("Clicking school account password text field",
		ui.WaitUntilExists(schoolPasswordText), ui.LeftClick(schoolPasswordText))(ctx); err != nil {
		s.Fatal("Failed to click school account password text field: ", err)
	}

	s.Log("Typing Geller parent (non-EDU) password")
	if err := kb.Type(ctx, gellerParentPass+"\n"); err != nil {
		s.Fatal("Failed to type Geller parent password: ", err)
	}

	s.Log("Verifying the attempt to add a non-EDU secondary account failed")
	cantAddAccountHeader := nodewith.NameContaining("add account").Role(role.Heading)
	if err := ui.WaitUntilExists(cantAddAccountHeader)(ctx); err != nil {
		s.Fatal("Failed to detect can't add acccount error message")
	}
}

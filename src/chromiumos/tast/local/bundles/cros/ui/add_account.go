// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/ui/cuj"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/ossettings"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         AddAccount,
		Desc:         "Test add new account scenario",
		Contacts:     []string{"xliu@cienet.com"},
		SoftwareDeps: []string{"chrome", "arc"},
		Fixture:      "loggedInToCUJUser",
		Vars: []string{
			"ui.second_account_name",
			"ui.second_account_password",
		},
	})
}

func AddAccount(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(cuj.FixtureData).Chrome

	secondUser := s.RequiredVar("ui.second_account_name")
	secondPass := s.RequiredVar("ui.second_account_password")

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect to test API: ", err)
	}

	defer faillog.DumpUITreeWithScreenshotOnError(ctx, s.OutDir(), func() bool { return true }, cr, "ui_dump")

	AccountsSetting := nodewith.Name("Accounts").Role(role.Link)

	_, err = ossettings.LaunchAtPage(ctx, tconn, AccountsSetting)
	if err != nil {
		s.Fatal("Failed to launch Settings: ", err)
	}

	ui := uiauto.New(tconn)
	accountIcon := nodewith.Name("Google Accounts").ClassName("subpage-arrow")
	addAccountButton := nodewith.Name("Add Google Account").Role(role.Button)
	okButton := nodewith.Name("OK").Role(role.Button)

	if err := uiauto.Combine("add account",
		ui.LeftClick(accountIcon),
		ui.LeftClick(addAccountButton),
		ui.LeftClick(okButton),
	)(ctx); err != nil {
		s.Fatal("Failed to add account: ", err)
	}

	if err := enterAccount(ctx, tconn, secondUser, secondPass); err != nil {
		s.Fatal("Failed to enter second account: ", err)
	}

	/*
		conn, err := cr.NewConn(ctx, "https://www.google.com")
		if err != nil {
			s.Fatal("Failed to navigate to google.com ", err)
		}
		// defer conn.Close()
	*/
}

// enterAccount enter account email and password.
func enterAccount(ctx context.Context, tconn *chrome.TestConn, username, password string) error {
	ui := uiauto.New(tconn)

	kb, err := input.Keyboard(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to initialize keyboard input")
	}
	defer kb.Close()

	emailContent := nodewith.NameContaining(username).Editable()
	emailField := nodewith.Name("Email or phone").Role(role.TextField)
	emailFieldFocused := nodewith.Name("Email or phone").Role(role.TextField).Focused()
	nextButton := nodewith.Name("Next").Role(role.Button)
	passwordField := nodewith.Name("Enter your password").Role(role.TextField)
	passwordFieldFocused := nodewith.Name("Enter your password").Role(role.TextField).Focused()
	iAgree := nodewith.Name("I agree").Role(role.Button)

	var actions []uiauto.Action
	if err := ui.WaitUntilExists(emailContent)(ctx); err != nil {
		// Email has not been entered into the text box yet.
		actions = append(actions,
			// Make sure text area is focused before typing. This is especially necessary on low-end DUTs.
			ui.LeftClickUntil(emailField, ui.Exists(emailFieldFocused)),
			kb.TypeAction(username),
		)
	}
	actions = append(actions,
		ui.LeftClick(nextButton),
		// Make sure text area is focused before typing. This is especially necessary on low-end DUTs.
		ui.LeftClickUntil(passwordField, ui.Exists(passwordFieldFocused)),
		kb.TypeAction(password),
		ui.LeftClick(nextButton),
		ui.LeftClickUntil(iAgree, ui.WithTimeout(time.Second).WaitUntilGone(iAgree)),
	)
	return uiauto.Combine("add second account",
		actions...,
	)(ctx)
}

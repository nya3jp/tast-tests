// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package login

import (
	"context"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

const samlDefaultUITimeout = 20 * time.Second

func init() {
	testing.AddTest(&testing.Test{
		Func:         ChromeSAML,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Checks that Chrome can make real SAML logins",
		Contacts: []string{
			"lmasopust@google.com",
			"cros-3pidp@google.com",
		},
		SoftwareDeps: []string{
			"chrome",
			"chrome_internal",
		},
		Attr: []string{
			"group:mainline", "informational",
		},
		VarDeps: []string{
			"accountmanager.samlusername",
			"accountmanager.samlpassword",
			"ui.signinProfileTestExtensionManifestKey",
		},
		Timeout: chrome.GAIALoginTimeout + time.Minute,
	})
}

// handleMicrosoftLogin navigates through the Microsoft login page and handles the login using username and password.
func handleMicrosoftLogin(ctx context.Context, tconn *chrome.TestConn, username, password string) error {
	kb, err := input.VirtualKeyboard(ctx)
	if err != nil {
		return errors.New("failed to get keyboard")
	}
	defer kb.Close()

	ui := uiauto.New(tconn).WithTimeout(samlDefaultUITimeout)

	root := nodewith.Role(role.RootWebArea).NameContaining("Sign in")

	samlEmailField := nodewith.NameContaining("Enter your email, phone, or Skype").Role(role.TextField).Ancestor(root)
	passwordField := nodewith.NameContaining("Enter the password").Role(role.TextField).Ancestor(root)
	noButton := nodewith.Name("No").Role(role.Button).Ancestor(root).Focusable()

	if err := uiauto.Combine("Enter SAML email and password",
		// Enter the User Name on the SAML page.
		ui.WaitUntilExists(samlEmailField),
		ui.LeftClickUntil(samlEmailField, ui.Exists(samlEmailField.Focused())),
		kb.TypeAction(username+"\n"),
		// Enter the Password.
		ui.WaitUntilExists(passwordField),
		ui.LeftClickUntil(passwordField, ui.Exists(passwordField.Focused())),
		kb.TypeAction(password+"\n"),
		// On "Stay signed in?" screen select "No".
		ui.WaitUntilExists(noButton),
		ui.DoDefault(noButton),
	)(ctx); err != nil {
		return errors.Wrap(err, "failed to enter SAML email and password")
	}

	return nil
}

func ChromeSAML(ctx context.Context, s *testing.State) {
	username := s.RequiredVar("accountmanager.samlusername")
	password := s.RequiredVar("accountmanager.samlpassword")

	cr, err := chrome.New(
		ctx,
		chrome.LoadSigninProfileExtension(s.RequiredVar("ui.signinProfileTestExtensionManifestKey")),
		chrome.SAMLLogin((chrome.Creds{User: username, Pass: ""})),
	)
	if err != nil {
		s.Fatal("Chrome login failed: ", err)
	}
	defer cr.Close(ctx)

	tconn, err := cr.SigninProfileTestAPIConn(ctx)
	if err != nil {
		s.Fatal("Creating login test API connection failed")
	}

	if err := handleMicrosoftLogin(ctx, tconn, username, password); err != nil {
		s.Fatal("Failed to handle Microsoft login: ", err)
	}

	if err := cr.WaitForCryptohome(ctx); err != nil {
		s.Fatal("Failed to wait for cryptohome: ", err)
	}

	if err := cr.WaitForOOBEConnectionToBeDismissed(ctx); err != nil {
		s.Fatal("Failed to wait for OOBE to be dismissed: ", err)
	}

	if err := cr.RemoveNotifications(ctx); err != nil {
		s.Fatal("Failed to remove notifications: ", err)
	}
}

// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package saml

import (
	"context"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/input"
)

// LoginFunc is used for the login callback during LoginWithSAMLAccount
type LoginFunc func(ctx context.Context, cr *chrome.Chrome) error

const samlDefaultUITimeout = 20 * time.Second

// HandleMicrosoftLogin navigates through the Microsoft login page and handles the login using username and password.
// The function is using a TestAPIConn and thus, requires the signin profile extension to be loaded.
func HandleMicrosoftLogin(username, password string) LoginFunc {
	return func(ctx context.Context, cr *chrome.Chrome) error {
		tconn, err := cr.SigninProfileTestAPIConn(ctx)
		if err != nil {
			return errors.Wrap(err, "Creating login test API connection failed")
		}

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
}

// LoginWithSAMLAccount handles real SAML logins by starting a SAML redirection using username and navigating through the IdP using loginFunc.
func LoginWithSAMLAccount(ctx context.Context, username string, loginFunc LoginFunc, opts ...chrome.Option) (c *chrome.Chrome, retErr error) {
	opts = append(opts, chrome.SAMLLogin((chrome.Creds{User: username, Pass: ""})))

	cr, err := chrome.New(ctx, opts...)
	if err != nil {
		return nil, errors.Wrap(err, "Chrome login failed")
	}

	if err := loginFunc(ctx, cr); err != nil {
		return nil, errors.Wrap(err, "failed to login with loginFunc")
	}

	cr.FinishUserLogin(ctx)

	return cr, nil
}

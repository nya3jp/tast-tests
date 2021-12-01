// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package login

import (
	"context"

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
		Func:         RestrictSignin,
		Desc:         "Test checking that signin restrictions are applied",
		Contacts:     []string{"raleksandrov@google.com", "chromeos-sw-engprod@google.com", "cros-oac@google.com"},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:mainline", "informational"},
		VarDeps: []string{
			"allowlist.username",
			"ui.oac_username",
			"ui.signinProfileTestExtensionManifestKey",
		},
	})
}

func RestrictSignin(ctx context.Context, s *testing.State) {
	cr, err := chrome.New(ctx)
	if err != nil {
		s.Fatal("Chrome login failed: ", err)
	}

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Getting test API connection failed: ", err)
	}

	allowedUser := s.RequiredVar("allowlist.username")
	if err = ossettings.RestrictSigninToUsers(ctx, tconn, cr, allowedUser); err != nil {
		s.Fatal("Failed to add allowed user: ", err)
	}

	// Sign out.
	cr.Close(ctx)

	// Restart chrome on login screen.
	cr2, err := chrome.New(
		ctx,
		chrome.NoLogin(),
		chrome.KeepState(),
		chrome.DontSkipOOBEAfterLogin(),
		chrome.LoadSigninProfileExtension(s.RequiredVar("ui.signinProfileTestExtensionManifestKey")),
	)
	if err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}
	tconn, err = cr2.SigninProfileTestAPIConn(ctx)
	if err != nil {
		s.Fatal("Getting signing test API connection failed: ", err)
	}

	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)

	ui := uiauto.New(tconn)

	// Trying to signin into some (not-allowed) user.
	enterUserEmailToSignin(ctx, s, ui, s.RequiredVar("ui.oac_username"))

	// Check Try again button exists. It should be shown on the error screen.
	tryAgainErrorButton := nodewith.Name("Try again").Role(role.Button)
	if err := ui.WaitUntilExists(tryAgainErrorButton)(ctx); err != nil {
		s.Fatal("Failed to wait for Try again button ")
	}

	if err := uiauto.Combine("Click on Try again button",
		ui.LeftClick(tryAgainErrorButton),
		ui.WaitUntilGone(tryAgainErrorButton),
	)(ctx); err != nil {
		s.Fatal(err, "failed to click on Try again button: ", err)
	}

	kb, err := input.Keyboard(ctx)
	if err := kb.Accel(ctx, "Esc"); err != nil {
		s.Fatal("Failed to emulate shortcut press: ", err)
	}
	defer kb.Close()

	// Trying to signin into allowed user.
	enterUserEmailToSignin(ctx, s, ui, allowedUser)

	// Check Back button is presented - means we have reached password view.
	backButton := nodewith.Name("Back").Role(role.Button)
	if err := ui.WaitUntilExists(backButton)(ctx); err != nil {
		s.Fatal("Failed to wait for Back button: ", err)
	}
}

func enterUserEmailToSignin(ctx context.Context, s *testing.State, ui *uiauto.Context, email string) {
	signinButton := nodewith.Name("Sign in").Role(role.Button)
	nextButton := nodewith.Name("Next").Role(role.Button)
	emailField := nodewith.Name("Email or phone").Role(role.TextField)
	emailFieldFocused := nodewith.Name("Email or phone").Role(role.TextField).Focused()
	if err := uiauto.Combine("Click on Sign in button",
		ui.LeftClick(signinButton),
		ui.WaitUntilExists(emailField),
	)(ctx); err != nil {
		s.Fatal(err, "failed signin : ", err)
	}

	if err := uiauto.Combine("Select email input field",
		ui.LeftClickUntil(emailField, ui.Exists(emailFieldFocused)),
	)(ctx); err != nil {
		s.Fatal("Failed to select email input field: ", err)
	}

	kb, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to get keyboard: ", err)
	}
	defer kb.Close()
	if err := kb.Accel(ctx, "Ctrl+A"); err != nil {
		s.Fatal("Failed emulate shortcut press: ", err)
	}

	if err := kb.Type(ctx, email); err != nil {
		s.Fatal("Failed to type in email: ", err)
	}

	if err := uiauto.Combine("Click on Next button",
		ui.LeftClick(nextButton),
		ui.WaitUntilGone(emailField),
	)(ctx); err != nil {
		s.Fatal(err, "failed to click on Next button: ", err)
	}
}

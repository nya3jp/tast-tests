// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package login

import (
	"context"
	"time"

	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash/ashproc"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/ossettings"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/procutil"
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
	cr, err := chrome.New(ctx, chrome.ExtraArgs("--force-tablet-mode=clamshell", "--disable-virtual-keyboard"))
	if err != nil {
		s.Fatal("Chrome login failed: ", err)
	}
	defer cr.Close(ctx)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Getting test API connection failed: ", err)
	}

	// Open Settings, Security and Privacy section, Manage other people subsection.
	settings, err := ossettings.LaunchAtPageURL(ctx, tconn, cr, "osPrivacy", func(context.Context) error { return nil })
	if err != nil {
		s.Fatal("Failed to launch Settings: ", err)
	}
	if err := settings.WaitForSearchBox()(ctx); err != nil {
		s.Fatal("Failed waiting for Settings to load: ", err)
	}

	subsectionName := "Manage other people"
	optionName := "Restrict sign-in to the following users:"
	if err := uiauto.Combine("Open Manage other people subsection",
		settings.LeftClick(nodewith.Name(subsectionName)),
		settings.WaitUntilExists(nodewith.Name(optionName)),
	)(ctx); err != nil {
		s.Fatal(err, "failed to open subsection 'Manage other people': ", err)
	}

	addUserButton := "Add user"
	if err := uiauto.Combine("Toggle Restrict sign-in to the following users option",
		settings.LeftClick(nodewith.Name(optionName).Role(role.ToggleButton)),
		settings.WaitUntilExists(nodewith.Name(addUserButton).Role(role.Link)),
	)(ctx); err != nil {
		s.Fatal(err, "failed to toggle 'Restrict sign-in to the following users' option: ", err)
	}

	// Add user to restriction list:
	if err := uiauto.Combine("Click on Add user button",
		settings.LeftClick(nodewith.Name(addUserButton).Role(role.Link)),
		settings.WaitUntilExists(nodewith.Name("Add user").Role(role.StaticText)),
	)(ctx); err != nil {
		s.Fatal(err, "failed to click on 'Add user' button: ", err)
	}

	kb, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to get keyboard: ", err)
	}
	defer kb.Close()

	allowedUser := s.RequiredVar("allowlist.username")

	if err := kb.Type(ctx, allowedUser); err != nil {
		s.Fatal("Failed to type in email: ", err)
	}

	addButton := nodewith.Name("Add").Role(role.Button)
	if err := uiauto.Combine("Add user to restriction list",
		settings.LeftClick(addButton),
		settings.WaitUntilGone(addButton),
	)(ctx); err != nil {
		s.Fatal(err, "failed to add user to restriction list: ", err)
	}

	// Sign out.
	oldProc, err := ashproc.Root()
	if err != nil {
		s.Fatal("Failed to get Chrome root PID: ", err)
	}

	if err := kb.Accel(ctx, "Ctrl+Shift+Q"); err != nil {
		s.Fatal("Failed emulate shortcut 1st press: ", err)
	}

	if err := kb.Accel(ctx, "Ctrl+Shift+Q"); err != nil {
		s.Fatal("Failed emulate shortcut 2nd press: ", err)
	}

	// Wait for Chrome restart.
	if err := procutil.WaitForTerminated(ctx, oldProc, 30*time.Second); err != nil {
		s.Fatal("Timeout waiting for Chrome to shutdown: ", err)
	}
	if _, err := ashproc.WaitForRoot(ctx, 30*time.Second); err != nil {
		s.Fatal("Timeout waiting for Chrome to start: ", err)
	}

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
	if err := ui.Exists(tryAgainErrorButton)(ctx); err != nil {
		s.Fatal("Failed to find Try again button ")
	}

	if err := uiauto.Combine("Click on Try again button",
		ui.LeftClick(tryAgainErrorButton),
		ui.WaitUntilGone(tryAgainErrorButton),
	)(ctx); err != nil {
		s.Fatal(err, "failed to click on Try again button: ", err)
	}

	if err := kb.Accel(ctx, "Esc"); err != nil {
		s.Fatal("Failed to emulate shortcut press: ", err)
	}

	// Trying to signin into allowed user.
	enterUserEmailToSignin(ctx, s, ui, allowedUser)

	// Check Back button is presented - means we have reached password view.
	backButton := nodewith.Name("Back").Role(role.Button)
	if err := ui.Exists(backButton)(ctx); err != nil {
		s.Fatal("Failed to find Back button: ", err)
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

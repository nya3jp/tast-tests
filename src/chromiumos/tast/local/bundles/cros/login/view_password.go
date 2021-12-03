// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package login

import (
	"context"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/lockscreen"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         ViewPassword,
		Desc:         "Test view password flow on the login/lock screens",
		Contacts:     []string{"rsorokin@google.com", "chromeos-sw-engprod@google.com", "cros-oac@google.com"},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:mainline", "informational"},
		VarDeps:      []string{"ui.signinProfileTestExtensionManifestKey"},
		Timeout:      2*chrome.LoginTimeout + 40*time.Second,
	})
}

func ViewPassword(ctx context.Context, s *testing.State) {
	var creds chrome.Creds

	// Isolate the first step to leverage `defer` pattern.
	func() {
		// Create user on the device.
		cr, err := chrome.New(ctx)
		if err != nil {
			s.Fatal("Chrome login failed: ", err)
		}
		defer cr.Close(ctx)
		creds = cr.Creds()

		tconn, err := cr.TestAPIConn(ctx)
		defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)
		if err = lockscreen.Lock(ctx, tconn); err != nil {
			s.Fatal("Failed to lock the screen")
		}
		runChecks(ctx, s, tconn, creds)
	}()

	cr, err := chrome.New(ctx,
		chrome.ExtraArgs("--skip-force-online-signin-for-testing"),
		chrome.NoLogin(),
		chrome.KeepState(),
		chrome.LoadSigninProfileExtension(s.RequiredVar("ui.signinProfileTestExtensionManifestKey")))

	tconn, err := cr.SigninProfileTestAPIConn(ctx)
	if err != nil {
		s.Fatal("Getting test Signin Profile API connection failed: ", err)
	}

	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)

	runChecks(ctx, s, tconn, creds)
}

func runChecks(ctx context.Context, s *testing.State, tconn *chrome.TestConn, creds chrome.Creds) {
	ui := uiauto.New(tconn)
	showPasswordButton := nodewith.ClassName("ToggleImageButton").Name("Show password")
	hidePasswordButton := nodewith.ClassName("ToggleImageButton").Name("Hide password")
	submitButton := nodewith.ClassName("ArrowButtonView").Name("Submit")

	keyboard, err := input.VirtualKeyboard(ctx)
	if err != nil {
		s.Fatal("Failed to get virtual keyboard: ", err)
	}
	defer keyboard.Close()
	if err = lockscreen.TypePassword(ctx, tconn, creds.User, creds.Pass, keyboard); err != nil {
		s.Fatal("Failed to type password: ", err)
	}

	if err := ui.LeftClick(showPasswordButton)(ctx); err != nil {
		s.Fatal("Failed to click on the show password button: ", err)
	}

	verifyPasswordProtected := func(expectedValue bool) error {
		isProtected, err := lockscreen.IsPasswordProtected(ctx, tconn, creds.User)
		if err != nil {
			return err
		}

		if isProtected == expectedValue {
			return nil
		}

		if expectedValue {
			return errors.New("Password input must be masked")
		}
		return errors.New("Password input must be unmasked")
	}

	if err := verifyPasswordProtected(false); err != nil {
		s.Fatal("Failed to switch password input state: ", err)
	}

	passwordHiddenAfter := 5 * time.Second
	if err := ui.Sleep(passwordHiddenAfter)(ctx); err != nil {
		s.Fatal("Failed to sleep: ", err)
	}

	if err := verifyPasswordProtected(true); err != nil {
		s.Fatal("Password input state did not switch after the timeout: ", err)
	}

	// Show password
	if err := ui.LeftClick(showPasswordButton)(ctx); err != nil {
		s.Fatal("Failed to click on the show password button: ", err)
	}

	if err := verifyPasswordProtected(false); err != nil {
		s.Fatal("Failed to switch password input state: ", err)
	}

	// Hide password
	if err := ui.LeftClick(hidePasswordButton)(ctx); err != nil {
		s.Fatal("Failed to click on the show password button: ", err)
	}

	if err := verifyPasswordProtected(true); err != nil {
		s.Fatal("Failed to switch password input state back: ", err)
	}

	// Show password
	if err := ui.LeftClick(showPasswordButton)(ctx); err != nil {
		s.Fatal("Failed to click on the show password button: ", err)
	}

	if err := ui.LeftClick(submitButton)(ctx); err != nil {
		s.Fatal("Failed to click on the submit button: ", err)
	}

	if st, err := lockscreen.WaitState(ctx, tconn, func(st lockscreen.State) bool { return st.LoggedIn }, chrome.LoginTimeout); err != nil {
		s.Fatalf("Waiting for login failed: %v (last status %+v)", err, st)
	}
}

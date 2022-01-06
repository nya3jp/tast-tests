// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package login

import (
	"context"
	"strings"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/lockscreen"
	"chromiumos/tast/local/chrome/uiauto/ossettings"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

type testParams struct {
	EnablePIN  bool
	Autosubmit bool
}

const hiddenPwdChar = "•"

func init() {
	testing.AddTest(&testing.Test{
		Func:         ShowPassword,
		Desc:         "Test Show/Hide password functionality on Password field and \"PIN or password\" field of login screen",
		Contacts:     []string{"chromeos-sw-engprod@google.com", "cros-oac@google.com"},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:mainline", "informational"},
		VarDeps:      []string{"ui.signinProfileTestExtensionManifestKey"},
		Params: []testing.Param{{
			Val: testParams{false, false},
		}, {
			Name: "pin",
			Val:  testParams{true, false},
		}, {
			Name: "switch_to_password",
			Val:  testParams{true, true},
		}},
	})
}

// ShowPassword tests viewing PIN / Password on login screen using the "Show password" button and that it goes hidden using the "Hide password" button.
func ShowPassword(ctx context.Context, s *testing.State) {
	const PIN = "123456789012"
	enablePIN := s.Param().(testParams).EnablePIN
	autosubmit := s.Param().(testParams).Autosubmit
	var creds chrome.Creds

	// Log in and log out to create a user pod on the login screen.
	func() {
		cr, err := chrome.New(ctx)
		if err != nil {
			s.Fatal("Chrome login failed: ", err)
		}
		defer cr.Close(ctx)
		creds = cr.Creds()

		tconn, err := cr.TestAPIConn(ctx)
		if err != nil {
			s.Fatal("Getting test API connection failed: ", err)
		}
		defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)

		if enablePIN {
			// Set up PIN through a connection to the Settings page.
			settings, err := ossettings.Launch(ctx, tconn)
			if err != nil {
				s.Fatal("Failed to launch Settings app: ", err)
			}
			s.Log("Performing PIN set up")
			if err := settings.EnablePINUnlock(cr, creds.Pass, PIN, autosubmit)(ctx); err != nil {
				s.Fatal("Failed to enable PIN unlock: ", err)
			}
		}
	}()

	// chrome.NoLogin() and chrome.KeepState() are needed to show the login screen with a user pod (instead of the OOBE login screen).
	cr, err := chrome.New(ctx,
		chrome.ExtraArgs("--skip-force-online-signin-for-testing"),
		chrome.NoLogin(),
		chrome.KeepState(),
		chrome.LoadSigninProfileExtension(s.RequiredVar("ui.signinProfileTestExtensionManifestKey")),
	)
	if err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}
	defer cr.Close(ctx)

	tconn, err := cr.SigninProfileTestAPIConn(ctx)
	if err != nil {
		s.Fatal("Creating login test API connection failed: ", err)
	}
	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)

	// Wait for the login screen to be ready for PIN / Password entry.
	if st, err := lockscreen.WaitState(ctx, tconn, func(st lockscreen.State) bool { return st.ReadyForPassword }, 30*time.Second); err != nil {
		s.Fatalf("Failed waiting for the login screen to be ready for PIN / Password entry: %v, last state: %+v", err, st)
	}

	// Clicking the "Switch to password" button to view the Password field when PIN autosubmit is enabled
	if autosubmit {
		if err := lockscreen.SwitchToPassword(ctx, tconn); err != nil {
			s.Fatal("Failed to click the Switch to password button: ", err)
		}
	}

	// Test the working of "Show password" and "Hide password" button on login screen.
	if enablePIN && !autosubmit {
		if err := showAndHidePassword(ctx, tconn, creds.User, PIN, true); err != nil {
			s.Fatal("Failed to Show/Hide PIN on login screen: ", err)
		}
	} else {
		if err := showAndHidePassword(ctx, tconn, creds.User, creds.Pass, false); err != nil {
			s.Fatal("Failed to Show/Hide Password on login screen: ", err)
		}
	}
}

// showAndHidePassword tests the working of "Show password" button and "Hide password" button on Password field and "PIN or password" field.
func showAndHidePassword(ctx context.Context, tconn *chrome.TestConn, username, password string, pin bool) error {
	hiddenPwd := strings.Repeat(hiddenPwdChar, len(password))

	if pin {
		// Enter the PIN on login screen when PIN is enabled.
		testing.ContextLog(ctx, "Entering PIN on \"PIN or password\" field of login screen")
		if err := lockscreen.EnterPIN(ctx, tconn, password); err != nil {
			return errors.Wrap(err, "failed to enter in PIN")
		}
	} else {
		// Enter password on login screen.
		kb, err := input.Keyboard(ctx)
		if err != nil {
			return errors.Wrap(err, "failed to get keyboard")
		}
		defer kb.Close()

		testing.ContextLog(ctx, "Entering password on login screen")
		if err := lockscreen.TypePassword(ctx, tconn, username, password, kb); err != nil {
			return errors.Wrap(err, "failed to type password")
		}
	}

	// Click the "Show password" button and verify that the viewed PIN / Password matches the user entered value.
	if err := lockscreen.ShowPassword(ctx, tconn); err != nil {
		return errors.Wrap(err, "failed to click the Show password button")
	}
	passwordField, err := lockscreen.UserPassword(ctx, tconn, username, pin)
	if err != nil {
		return errors.New("failed to read PIN / Password")
	}
	if passwordField.Value != password {
		return errors.New("PIN / Password revealed after clicking the Show password button is not matching with the user entered value")
	}

	// Verify that the PIN / Password goes hidden after clicking the "Hide password" button.
	if err := lockscreen.HidePassword(ctx, tconn); err != nil {
		return errors.Wrap(err, "failed to click the Hide password button")
	}
	passwordVal, err := lockscreen.UserPassword(ctx, tconn, username, pin)
	if err != nil {
		return errors.New("failed to read PIN / Password that is hidden")
	}
	if passwordVal.Value != hiddenPwd {
		return errors.New("PIN / Password is not hidden after clicking the Hide password button")
	}

	// Verify that when the user clicks the "Show password" button, the viewed PIN / Password goes hidden automatically after 5s timeout.
	if err := waitForPasswordHidden(ctx, tconn); err != nil {
		return errors.Wrap(err, "PIN / Password is not hidden after the timeout")
	}
	return nil
}

// waitForPasswordHidden checks that the PIN / Password is auto hidden 5s after "Show password" button is pressed.
func waitForPasswordHidden(ctx context.Context, tconn *chrome.TestConn) error {
	if err := lockscreen.ShowPassword(ctx, tconn); err != nil {
		return errors.Wrap(err, "failed to click the Show password button")
	}
	ui := uiauto.New(tconn)
	if err := ui.WithTimeout(6 * time.Second).WaitUntilGone(lockscreen.HidePasswordButton)(ctx); err != nil {
		return err
	}
	if err := ui.Exists(lockscreen.ShowPasswordButton)(ctx); err != nil {
		return errors.New("failed to find the Show password button")
	}
	return nil
}

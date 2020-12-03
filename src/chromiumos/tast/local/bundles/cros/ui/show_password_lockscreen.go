// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"
	"time"

	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ui/faillog"
	"chromiumos/tast/local/chrome/ui/lockscreen"
	"chromiumos/tast/local/chrome/ui/ossettings"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         ShowPasswordLockscreen,
		Desc:         "Test to view password, pin on lockscreen using Show password button",
		Contacts:     []string{"chromeos-sw-engprod@google.com"},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:mainline", "informational"},
		Params: []testing.Param{{
			Val: false,
		}, {
			Name: "pin",
			Val:  true,
		}},
	})
}

// ShowPasswordLockscreen function tests to view password, pin on lockscreen using Show password button
func ShowPasswordLockscreen(ctx context.Context, s *testing.State) {
	const (
		username = "testuser@gmail.com"
		password = "good"
		gaiaID   = "1234"
		PIN      = "1234567890"
	)

	pin := s.Param().(bool)

	// Login to user account
	cr, err := chrome.New(ctx, chrome.Auth(username, password, gaiaID))
	if err != nil {
		s.Fatal("Chrome login failed: ", err)
	}
	defer cr.Close(ctx)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Getting test API connection failed: ", err)
	}
	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)

	// Check the password only field
	if !pin {

		// Lock the screen.
		if err := lockscreen.Lock(ctx, tconn); err != nil {
			s.Fatal("Failed to lock the screen: ", err)
		}

		if st, err := lockscreen.WaitState(ctx, tconn, func(st lockscreen.State) bool { return st.Locked && st.ReadyForPassword }, 30*time.Second); err != nil {
			s.Fatalf("Waiting for screen to be locked failed: %v (last status %+v)", err, st)
		}

		// Verify Show password button on password only field
		if err := verifyPasswordField(ctx, tconn, s, username, password, false); err != nil {
			s.Fatal("Failed to verify password only field: ", err)
		}

		if err := lockscreen.SubmitPIN(ctx, tconn); err != nil {
			s.Fatal("Failed to submit PIN: ", err)
		}
	}

	// Set up PIN through a connection to the Settings page with autosubmit disabled.
	if err := openSettings(ctx, tconn, s, cr, password, PIN, false); err != nil {
		s.Fatal("Failed to set up PIN through a connection to the Settings page: ", err)
	}

	if pin {

		// Verify Show password button on pin or password field using the Pin value
		if err := verifyPasswordField(ctx, tconn, s, username, PIN, pin); err != nil {
			s.Fatal("Failed to verify pin/ password field using the pin value: ", err)
		}
	} else {

		// Verify Show password button on pin or password field using the user password
		if err := verifyPasswordField(ctx, tconn, s, username, password, pin); err != nil {
			s.Fatal("Failed to verify pin/ password field using the user password: ", err)
		}

		if err := lockscreen.SubmitPIN(ctx, tconn); err != nil {
			s.Fatal("Failed to submit PIN: ", err)
		}
	}

	/* Set up PIN through a connection to the Settings page with autosubmit enabled.
	With auto submit enabled, Show password button appears only on password field.*/
	if !pin {

		//Goto settings, enable pin and auto submit
		if err := openSettings(ctx, tconn, s, cr, password, PIN, true); err != nil {
			s.Fatal("Failed to set up PIN through a connection to the Settings page: ", err)
		}

		//Click the 'Switch to Password' button to view password only field
		s.Log("Clicking Switch to password button")
		if err := lockscreen.SwitchToPassword(ctx, tconn); err != nil {
			s.Fatal("Failed to click 'Switch to password' button: ", err)
		}

		// Verify Show password button on password only field after 'Switch to Password'
		if err := verifyPasswordField(ctx, tconn, s, username, password, false); err != nil {
			s.Fatal("Failed to verify password only field after 'Switch to Password': ", err)
		}
	}
}

// verifyPasswordField function tests the working of 'Show password' button and 'Hide password' button on
// 'password only' field and 'pin or password' field for pin and user password values. Show password button should
// show the field value and Hide password button should hide the field value */
func verifyPasswordField(ctx context.Context, tconn *chrome.TestConn, s *testing.State, username, pwdValue string, bPin bool) error {
	const (
		hidePwd = "••••"
		hidePin = "••••••••••"
	)

	if !bPin {
		// Enter password on lockscreen
		kb, err := input.Keyboard(ctx)
		if err != nil {
			s.Fatal("Failed to get keyboard: ", err)
		}
		defer kb.Close()

		s.Log("Entering password on lockscreen")
		if err := kb.Type(ctx, pwdValue); err != nil {
			s.Fatal("Entering password failed: ", err)
		}
	} else {
		// Enter the PIN on lockscreen when pin is enabled.
		s.Log("Entering PIN on lockscreen pin/ password field")
		if err := lockscreen.EnterPIN(ctx, tconn, pwdValue); err != nil {
			s.Fatal("Failed to enter in PIN: ", err)
		}
	}

	// Click the Show Password button to view pin/ password
	s.Log("Clicking Show Password button")
	if err := lockscreen.ShowPassword(ctx, tconn); err != nil {
		s.Fatal("Failed to click Show password button: ", err)
	}

	passwordField, err := lockscreen.ReadPassword(ctx, tconn, username)
	if err != nil {
		s.Fatal("Failed to read pin/ password value: ", err)
	}
	s.Log(passwordField.Value)

	// Verify that the viewed password matches with the user password
	if passwordField.Value != pwdValue {
		s.Error("Pin/ Password field is not showing the correct value after clicking the Show password button")
	}

	// Click the Hide Password button to hide pin/ password
	s.Log("Clicking Hide Password button")
	if err := lockscreen.HidePassword(ctx, tconn); err != nil {
		s.Fatal("Failed to click Hide password button: ", err)
	}

	passwordVal, err := lockscreen.ReadPassword(ctx, tconn, username)
	if err != nil {
		s.Fatal("Failed to read pin/ password value after clicking the Hide password button: ", err)
	}
	s.Log(passwordVal.Value)

	// Verify that the password or pin value is in hidden state after clicking Hide Password button
	if !bPin {
		if passwordVal.Value != hidePwd {
			s.Error("Password value is not hidden")
		}
	} else {
		if passwordVal.Value != hidePin {
			s.Error("Pin value is not hidden")
		}
	}

	/* Verify that once the user clicks on Show password button, the viewed pin/ password
	goes hidden automatically after 5s timeout*/
	if !lockscreen.WaitForHidePassword(ctx, tconn) {
		s.Error("Pin/ Password field didn't switch to hide state after 5s")
	} else {
		s.Log("Pin/ Password field switched to hide state automatically after 5s")
	}

	return nil
}

// openSettings is a common function to launch settings app and perform pin set up
func openSettings(ctx context.Context, tconn *chrome.TestConn, s *testing.State, cr *chrome.Chrome, password, PIN string, autosubmit bool) error {

	//Open Settings window and set up PIN through a connection to the Settings page
	if err := ossettings.Launch(ctx, tconn); err != nil {
		s.Fatal("Failed to launch Settings app: ", err)
	}

	settingsConn, err := ossettings.ChromeConn(ctx, cr)
	if err != nil {
		s.Fatal("Failed to get Chrome connection to Settings app: ", err)
	}
	defer settingsConn.Close()

	// Perform pin set up here
	if err := ossettings.EnablePINUnlock(ctx, settingsConn, password, PIN, autosubmit); err != nil {
		s.Fatal("Failed to enable PIN unlock: ", err)
	}

	// Lock the screen.
	if err := lockscreen.Lock(ctx, tconn); err != nil {
		s.Fatal("Failed to lock the screen: ", err)
	}

	if st, err := lockscreen.WaitState(ctx, tconn, func(st lockscreen.State) bool { return st.Locked && st.ReadyForPassword }, 30*time.Second); err != nil {
		s.Fatalf("Waiting for screen to be locked failed: %v (last status %+v)", err, st)
	}

	return nil
}

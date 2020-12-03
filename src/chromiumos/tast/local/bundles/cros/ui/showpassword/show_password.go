// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package showpassword contains functionality shared by show password
// and show pin related tests.
package showpassword

import (
	"context"

	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ui/lockscreen"
	"chromiumos/tast/local/chrome/ui/ossettings"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

// VerifyPasswordField tests the working of 'Show password' button and 'Hide password' button on
// 'password only' field and 'pin or password' field for pin and user password values. Show password button should
// show the field value and Hide password button should hide the field value
func VerifyPasswordField(ctx context.Context, tconn *chrome.TestConn, s *testing.State, username, pwdValue string, pin bool) error {
	const (
		hidePwd = "••••"
		hidePin = "••••••••••"
	)

	if !pin {
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

	passwordField, err := lockscreen.GetUserPassword(ctx, tconn, username)
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

	passwordVal, err := lockscreen.GetUserPassword(ctx, tconn, username)
	if err != nil {
		s.Fatal("Failed to read pin/ password value after clicking the Hide password button: ", err)
	}
	s.Log(passwordVal.Value)

	// Verify that the password or pin value is in hidden state after clicking Hide Password button
	if !pin {
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

// SetupPIN is a utility function to launch settings app and perform pin set up
func SetupPIN(ctx context.Context, tconn *chrome.TestConn, s *testing.State, cr *chrome.Chrome, password, PIN string, autosubmit bool) error {

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

	return nil
}

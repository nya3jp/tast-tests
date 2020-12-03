// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package showpassword contains functionality shared by show password
// and show pin related tests.
package showpassword

import (
	"context"

	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ui/lockscreen"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

// ShowAndHidePassword tests the working of 'Show password' button and 'Hide password' button on
// 'password' field and 'pin or password' field for pin and user password values.
func ShowAndHidePassword(ctx context.Context, tconn *chrome.TestConn, s *testing.State, username, pwdValue string, pin bool) error {
	const (
		hidePwd = "••••"
		hidePin = "••••••••••"
	)

	if pin {
		// Enter the PIN on lockscreen when pin is enabled.
		s.Log("Entering PIN on lockscreen 'pin or password' field")
		if err := lockscreen.EnterPIN(ctx, tconn, pwdValue); err != nil {
			s.Fatal("Failed to enter in PIN: ", err)
		}
	} else {
		// Enter password on lockscreen.
		kb, err := input.Keyboard(ctx)
		if err != nil {
			s.Fatal("Failed to get keyboard: ", err)
		}
		defer kb.Close()

		s.Log("Entering password on lockscreen")
		if err := kb.Type(ctx, pwdValue); err != nil {
			s.Fatal("Entering password failed: ", err)
		}
	}

	// Click the Show Password button and verify that the viewed pin / password value matches
	// with the user entered values.
	passwordField := ShowPassword(ctx, tconn, s, username)
	if passwordField != pwdValue {
		s.Error("Pin / Password field is not showing the correct value after clicking the Show password button")
	}

	// Verify that the pin / password value is in hidden state after clicking Hide Password button.
	passwordVal := HidePassword(ctx, tconn, s, username)
	if pin {
		if passwordVal != hidePin {
			s.Error("Pin value is not hidden")
		}
	} else {
		if passwordVal != hidePwd {
			s.Error("Password value is not hidden")
		}
	}

	// Verify that once the user clicks on Show password button, the viewed pin / password
	// goes hidden automatically after 5s timeout.
	if !lockscreen.WaitForPasswordHidden(ctx, tconn) {
		s.Error("Pin / Password field didn't switch to hide state after the timeout")
	} else {
		s.Log("Pin / Password field switched to hide state automatically after the timeout")
	}
	return nil
}

// ShowPassword clicks the Show Password button and returns the revealed pin / password value.
func ShowPassword(ctx context.Context, tconn *chrome.TestConn, s *testing.State, username string) string {
	s.Log("Clicking Show Password button")
	if err := lockscreen.ShowPassword(ctx, tconn); err != nil {
		s.Fatal("Failed to click Show password button: ", err)
	}

	passwordField := lockscreen.GetUserPassword(ctx, tconn, username)
	s.Log(passwordField.Value)

	return passwordField.Value
}

// HidePassword clicks the Hide Password button and returns the pin / password value that is hidden.
func HidePassword(ctx context.Context, tconn *chrome.TestConn, s *testing.State, username string) string {
	s.Log("Clicking Hide Password button")
	if err := lockscreen.HidePassword(ctx, tconn); err != nil {
		s.Fatal("Failed to click Hide password button: ", err)
	}

	passwordVal := lockscreen.GetUserPassword(ctx, tconn, username)
	s.Log(passwordVal.Value)

	return passwordVal.Value
}

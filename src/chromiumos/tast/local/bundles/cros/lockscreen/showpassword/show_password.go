// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package showpassword contains functionality shared by Show password and Show PIN related tests.
package showpassword

import (
	"context"
	"strings"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/lockscreen"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

const hiddenPwdChar = "•"

// ShowAndHidePassword tests the working of "Show password" button and "Hide password" button on Password field.
func ShowAndHidePassword(ctx context.Context, tconn *chrome.TestConn, username, password string) error {
	hiddenPwd := strings.Repeat(hiddenPwdChar, len(password))
	// Enter password on lockscreen.
	kb, err := input.Keyboard(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to get keyboard")
	}
	defer kb.Close()

	testing.ContextLog(ctx, "Entering password on lockscreen")
	if err := lockscreen.TypePassword(ctx, tconn, username, password, kb); err != nil {
		return errors.Wrap(err, "failed to type password")
	}

	// Click the "Show password" button and verify that the viewed password matches the user entered value.
	if err := lockscreen.ShowPassword(ctx, tconn); err != nil {
		return errors.Wrap(err, "failed to click the Show password button")
	}
	passwordField, err := lockscreen.UserPassword(ctx, tconn, username)
	if err != nil {
		return errors.New("failed to read password")
	}
	if passwordField.Value != password {
		return errors.New("Password revealed after clicking the Show password button is not matching with the user entered value")
	}

	// Verify that the password goes hidden after clicking the "Hide password" button.
	if err := lockscreen.HidePassword(ctx, tconn); err != nil {
		return errors.Wrap(err, "failed to click the Hide password button")
	}
	passwordVal, err := lockscreen.UserPassword(ctx, tconn, username)
	if err != nil {
		return errors.New("failed to read password that is hidden")
	}
	if passwordVal.Value != hiddenPwd {
		return errors.New("Password is not hidden after clicking the Hide password button")
	}

	// Verify that when user clicks the "Show password" button, the viewed password goes hidden automatically after 5s timeout.
	if err := WaitForPasswordHidden(ctx, tconn); err != nil {
		return errors.Wrap(err, "Password is not hidden after the timeout")
	}
	return nil
}

// ShowAndHidePIN tests the working of "Show password" button and "Hide password" button on "PIN or password" field for PIN value.
func ShowAndHidePIN(ctx context.Context, tconn *chrome.TestConn, username, PIN string) error {
	hiddenPIN := strings.Repeat(hiddenPwdChar, len(PIN))
	// Enter the PIN on lockscreen when PIN is enabled.
	testing.ContextLog(ctx, "Entering PIN on lockscreen \"PIN or password\" field")
	if err := lockscreen.EnterPIN(ctx, tconn, PIN); err != nil {
		return errors.Wrap(err, "failed to enter in PIN")
	}

	// Click the "Show password" button and verify that the viewed PIN matches with the user entered value.
	if err := lockscreen.ShowPassword(ctx, tconn); err != nil {
		return errors.Wrap(err, "failed to click the Show password button")
	}
	pinField, err := lockscreen.UserPIN(ctx, tconn, username)
	if err != nil {
		return errors.New("failed to read PIN value")
	}
	if pinField.Value != PIN {
		return errors.New("PIN value revealed after clicking the Show password button is not matching with the user entered value")
	}

	// Verify that the PIN goes hidden after clicking the "Hide password" button.
	if err := lockscreen.HidePassword(ctx, tconn); err != nil {
		return errors.Wrap(err, "failed to click the Hide password button")
	}
	pinVal, err := lockscreen.UserPIN(ctx, tconn, username)
	if err != nil {
		return errors.New("failed to read the PIN value that is hidden")
	}
	if pinVal.Value != hiddenPIN {
		return errors.New("PIN value is not hidden after clicking the Hide password button")
	}

	// Verify that when user clicks the "Show password" button, the viewed PIN goes hidden automatically after 5s timeout.
	if err := WaitForPasswordHidden(ctx, tconn); err != nil {
		return errors.Wrap(err, "PIN is not hidden after the timeout")
	}
	return nil
}

// WaitForPasswordHidden checks that PIN / Password field is autohidden 5s after "Show password" button is pressed.
func WaitForPasswordHidden(ctx context.Context, tconn *chrome.TestConn) error {
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

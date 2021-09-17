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
	"chromiumos/tast/local/chrome/ui"
	"chromiumos/tast/local/chrome/ui/lockscreen"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

// ShowAndHidePassword tests the working of "Show password" button and "Hide password" button on password field and "PIN or password" field for PIN and user password values.
func ShowAndHidePassword(ctx context.Context, tconn *chrome.TestConn, username, pwdValue string, pin bool) error {
	hiddenPwd := strings.Repeat("•", len(pwdValue))
	if pin {
		// Enter the PIN on lockscreen when PIN is enabled.
		testing.ContextLog(ctx, "Entering PIN on lockscreen \"PIN or password\" field")
		if err := lockscreen.EnterPIN(ctx, tconn, pwdValue); err != nil {
			return errors.Wrap(err, "failed to enter in PIN")
		}
	} else {
		// TODO (b/189597597): After fixing this bug, replace the below lines of code in the else block with a function for typing in the password.
		// Enter password on lockscreen.
		kb, err := input.Keyboard(ctx)
		if err != nil {
			return errors.Wrap(err, "failed to get keyboard")
		}
		defer kb.Close()

		testing.ContextLog(ctx, "Entering password on lockscreen")
		if err := kb.Type(ctx, pwdValue); err != nil {
			return errors.Wrap(err, "entering password failed")
		}
	}

	// Click the "Show password" button and verify that the viewed PIN / Password matches with the user entered values.
	passwordField, err := ShowPassword(ctx, tconn, username)
	if err != nil {
		return err
	}
	if passwordField != pwdValue {
		return errors.New("PIN / Password revealed after clicking \"Show password\" button is not matching with the user entered values")
	}

	// Verify that the PIN / Password is hidden after clicking "Hide password" button.
	passwordVal, err := HidePassword(ctx, tconn, username)
	if err != nil {
		return err
	}
	if passwordVal != hiddenPwd {
		return errors.New("PIN / Password is not hidden after clicking \"Hide password\" button")
	}

	// Verify that when the user clicks on "Show password" button, the viewed PIN / Password goes hidden automatically after 5s timeout.
	if err := WaitForPasswordHidden(ctx, tconn); err != nil {
		return errors.Wrap(err, "PIN / Password is not hidden after the timeout")
	}

	testing.ContextLog(ctx, "PIN / Password goes hidden automatically after the timeout")
	return nil
}

// ShowPassword clicks the "Show password" button and returns the revealed PIN / Password.
func ShowPassword(ctx context.Context, tconn *chrome.TestConn, username string) (string, error) {
	if err := lockscreen.ShowPassword(ctx, tconn); err != nil {
		return "", errors.Wrap(err, "failed to click \"Show password\" button")
	}

	passwordField, err := lockscreen.UserPassword(ctx, tconn, username)
	if err != nil {
		return "", errors.New("failed to read PIN / Password")
	}

	testing.ContextLog(ctx, "PIN / Password revealed after clicking the \"Show password\" button is: ", passwordField.Value)
	return passwordField.Value, nil
}

// HidePassword clicks the "Hide password" button and returns the PIN / Password that is hidden.
func HidePassword(ctx context.Context, tconn *chrome.TestConn, username string) (string, error) {
	if err := lockscreen.HidePassword(ctx, tconn); err != nil {
		return "", errors.Wrap(err, "failed to click \"Hide password\" button")
	}

	passwordVal, err := lockscreen.UserPassword(ctx, tconn, username)
	if err != nil {
		return "", errors.New("failed to read PIN / Password that is hidden")
	}

	testing.ContextLog(ctx, "PIN / Password after clicking \"Hide password\" button is: ", passwordVal.Value)
	return passwordVal.Value, nil
}

// WaitForPasswordHidden checks that PIN / Password field is autohidden 5s after "Show password" button is pressed.
func WaitForPasswordHidden(ctx context.Context, tconn *chrome.TestConn) error {
	testing.ContextLog(ctx, "Clicking \"Show password\" button")

	if err := lockscreen.ShowPassword(ctx, tconn); err != nil {
		return errors.Wrap(err, "failed to click \"Show password\" button")
	}

	if err := testing.Poll(ctx, func(ctx context.Context) error {
		flag, err := ui.Exists(ctx, tconn, lockscreen.HidePasswordBtnParams)
		if err != nil {
			return testing.PollBreak(err)
		}
		if flag {
			return errors.New("\"Hide password\" button was found")
		}
		return nil
	}, &testing.PollOptions{Timeout: 6 * time.Second}); err != nil {
		return err
	}

	showPwd, err := ui.Exists(ctx, tconn, lockscreen.ShowPasswordBtnParams)
	if err != nil {
		return err
	}
	if !showPwd {
		return errors.New("failed to find \"Show password\" button")
	}

	return nil
}

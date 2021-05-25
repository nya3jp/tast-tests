// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package showpassword contains functionality shared by Show password
// and Show PIN related tests.
package showpassword

import (
	"context"
	"strings"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ui/lockscreen"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

// ShowAndHidePassword tests the working of 'Show password' button and 'Hide password' button on 'password' field and 'pin or password' field for PIN and user password values.
func ShowAndHidePassword(ctx context.Context, tconn *chrome.TestConn, username, pwdValue string, pin bool) error {
	hidePwd := strings.Repeat("•", len(pwdValue))

	if pin {
		// Enter the PIN on lockscreen when PIN is enabled.
		testing.ContextLog(ctx, "Entering PIN on lockscreen 'pin or password' field")
		if err := lockscreen.EnterPIN(ctx, tconn, pwdValue); err != nil {
			return errors.Wrap(err, "failed to enter in PIN")
		}
	} else {
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

	// Click the Show password button and verify that the viewed PIN / Password value matches with the user entered values.
	passwordField, err := ShowPassword(ctx, tconn, username)
	if err != nil {
		return err
	}
	if passwordField != pwdValue {
		return errors.New("PIN / Password field is not showing the correct value after clicking the Show password button")
	}

	// Verify that the PIN / Password value is in hidden state after clicking Hide password button.
	passwordVal, err := HidePassword(ctx, tconn, username)
	if err != nil {
		return err
	}
	if passwordVal != hidePwd {
		return errors.New("PIN / Password value is not hidden")
	}

	// Verify that once the user clicks on Show password button, the viewed PIN / Password goes hidden automatically after 5s timeout.
	if err := lockscreen.WaitForPasswordHidden(ctx, tconn); err != nil {
		return errors.Wrap(err, "PIN / Password field didn't switch to hide state after the timeout")
	}

	testing.ContextLog(ctx, "PIN / Password field switched to hide state automatically after the timeout")
	return nil
}

// ShowPassword clicks the Show password button and returns the revealed PIN / Password value.
func ShowPassword(ctx context.Context, tconn *chrome.TestConn, username string) (string, error) {
	testing.ContextLog(ctx, "Clicking Show password button")

	if err := lockscreen.ShowPassword(ctx, tconn); err != nil {
		return "", errors.Wrap(err, "failed to click Show password button")
	}

	passwordField, err := lockscreen.GetUserPassword(ctx, tconn, username)
	if err != nil {
		return "", errors.New("failed to read PIN / Password value")
	}

	testing.ContextLog(ctx, passwordField.Value)
	return passwordField.Value, nil
}

// HidePassword clicks the Hide password button and returns the PIN / Password value that is hidden.
func HidePassword(ctx context.Context, tconn *chrome.TestConn, username string) (string, error) {
	testing.ContextLog(ctx, "Clicking Hide password button")

	if err := lockscreen.HidePassword(ctx, tconn); err != nil {
		return "", errors.Wrap(err, "failed to click Hide password button")
	}

	passwordVal, err := lockscreen.GetUserPassword(ctx, tconn, username)
	if err != nil {
		return "", errors.New("failed to read PIN / Password value that is hidden")
	}

	testing.ContextLog(ctx, passwordVal.Value)
	return passwordVal.Value, nil
}

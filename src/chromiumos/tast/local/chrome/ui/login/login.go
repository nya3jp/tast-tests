// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package login provides utilities for Chrome OS user login.
package login

import (
	"context"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ui"
	"chromiumos/tast/testing"
)

// UI params for the password field.
var passwordFieldParams ui.FindParams = ui.FindParams{
	Role: ui.RoleTypeTextField,
}

// Status corresponds to 'LoginStatusDict' as defined in autotest_privateidl.
type Status struct {
	LoggedIn     bool   `json:"isLoggedIn"`
	Owner        bool   `json:"isOwner"`
	Locked       bool   `json:"isScreenLocked"`
	Ready        bool   `json:"isReadyForPassword"`
	RegularUser  bool   `json:"isRegularUser"`
	Guest        bool   `json:"isGuest"`
	Kiosk        bool   `json:"isKiosk"`
	Email        string `json:"email"`
	DisplayEmail string `json:"displayEmail"`
	UserImage    string `json:"userImage"`
}

// WaitStatus repeatedly calls chrome.autotestPrivate.loginStatus and passes the returned
// state to f until it returns true or timeout is reached. The last-seen state is returned.
func WaitStatus(ctx context.Context, tconn *chrome.TestConn, condition func(st Status) bool, timeout time.Duration) (Status, error) {
	var st Status
	err := testing.Poll(ctx, func(ctx context.Context) error {
		if err := tconn.EvalPromise(ctx, `tast.promisify(chrome.autotestPrivate.loginStatus)()`, &st); err != nil {
			return err
		} else if !condition(st) {
			return errors.New("wrong status")
		}
		return nil
	}, &testing.PollOptions{Timeout: timeout})
	return st, err
}

// WaitForPasswordField waits for the password text field to appear in the UI.
func WaitForPasswordField(ctx context.Context, tconn *chrome.TestConn, timeout time.Duration) error {
	return ui.WaitUntilExists(ctx, tconn, passwordFieldParams, timeout)
}

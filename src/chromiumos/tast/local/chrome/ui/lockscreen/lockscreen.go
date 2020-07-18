// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package lockscreen is used to get information about the lock screen directly through the UI.
package lockscreen

import (
	"context"
	"fmt"
	"regexp"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ui"
	"chromiumos/tast/testing"
)

// State contains the state returned by chrome.autotestPrivate.loginStatus,
// corresponding to 'LoginStatusDict' as defined in autotest_private.idl.
type State struct {
	LoggedIn         bool   `json:"isLoggedIn"`
	Owner            bool   `json:"isOwner"`
	Locked           bool   `json:"isScreenLocked"`
	ReadyForPassword bool   `json:"isReadyForPassword"` // Login screen may not be ready to receive a password, even if this is true (crbug/1109381)
	RegularUser      bool   `json:"isRegularUser"`
	Guest            bool   `json:"isGuest"`
	Kiosk            bool   `json:"isKiosk"`
	Email            string `json:"email"`
	DisplayEmail     string `json:"displayEmail"`
	UserImage        string `json:"userImage"`
}

// GetState returns the login status information from chrome.autotestPrivate.loginStatus
func GetState(ctx context.Context, tconn *chrome.TestConn) (State, error) {
	var st State
	if err := tconn.EvalPromise(ctx, `tast.promisify(chrome.autotestPrivate.loginStatus)()`, &st); err != nil {
		return st, errors.Wrap(err, "failed calling chrome.autotestPrivate.loginStatus")
	}
	return st, nil
}

// WaitState repeatedly calls GetState and passes the returned state to check
// until it returns true or timeout is reached. The last-seen state is returned.
func WaitState(ctx context.Context, tconn *chrome.TestConn, check func(st State) bool, timeout time.Duration) (State, error) {
	var st State
	err := testing.Poll(ctx, func(ctx context.Context) error {
		var e error
		st, e = GetState(ctx, tconn)
		if e != nil {
			return testing.PollBreak(e)
		}
		if !check(st) {
			return errors.New("wrong status")
		}
		return nil
	}, &testing.PollOptions{Timeout: timeout})
	return st, err
}

// WaitForPasswordField waits for the password text field for a given user pod to appear in the UI.
// The password field node can be uniquely identified by its name attribute, which includes the username,
// such as "Password for username@gmail.com". We'll wait for the node whose name matches the regex
// /Password for <username>/, so the domain can be omitted, or the username argument can be an empty
// string to wait for any password field to appear.
func WaitForPasswordField(ctx context.Context, tconn *chrome.TestConn, username string, timeout time.Duration) error {
	r, err := regexp.Compile(fmt.Sprintf("Password for %v", username))
	if err != nil {
		return errors.Wrap(err, "failed to compile regexp for name attribute")
	}
	attributes := map[string]interface{}{
		"name": r,
	}
	params := ui.FindParams{
		Role:       ui.RoleTypeTextField,
		Attributes: attributes,
	}
	return ui.WaitUntilExists(ctx, tconn, params, timeout)
}

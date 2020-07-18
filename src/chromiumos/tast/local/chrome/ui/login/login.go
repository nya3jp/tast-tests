// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package login provides utilities for Chrome OS user login.
package login

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

// Status corresponds to 'LoginStatusDict' as defined in autotest_private.idl.
type Status struct {
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

// StatusType corresponds to the boolean values in Status
type StatusType int

// StatusType enum
const (
	LoggedIn StatusType = iota
	Owner
	Locked
	ReadyForPassword
	RegularUser
	Guest
	Kiosk
)

// GetStatus returns the login status information from chrome.autotestPrivate.loginStatus
func GetStatus(ctx context.Context, tconn *chrome.TestConn) (*Status, error) {
	var st Status
	if err := tconn.EvalPromise(ctx, `tast.promisify(chrome.autotestPrivate.loginStatus)()`, &st); err != nil {
		return nil, errors.Wrap(err, "failed calling chrome.autotestPrivate.loginStatus")
	}
	return &st, nil
}

// WaitForStatus repeatedly calls GetStatus and checks if the property specified by statusType to be the wanted value.
// For example, if waiting for Chrome to be logged in:
//   WaitForStatus(ctx, tconn, login.LoggedIn, true, 10*time.Second)
func WaitForStatus(ctx context.Context, tconn *chrome.TestConn, statusType StatusType, wanted bool, timeout time.Duration) error {
	return testing.Poll(ctx, func(ctx context.Context) error {
		status, err := GetStatus(ctx, tconn)
		if err != nil {
			return testing.PollBreak(err)
		}

		// Make a map from the boolean statuses so we can check the one we're interested in
		stmap := map[StatusType]bool{
			LoggedIn:         status.LoggedIn,
			Owner:            status.Owner,
			Locked:           status.Locked,
			ReadyForPassword: status.ReadyForPassword,
			RegularUser:      status.RegularUser,
			Guest:            status.Guest,
			Kiosk:            status.Kiosk,
		}

		if stmap[statusType] != wanted {
			return errors.New("wrong status")
		}
		return nil
	}, &testing.PollOptions{Timeout: timeout})
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

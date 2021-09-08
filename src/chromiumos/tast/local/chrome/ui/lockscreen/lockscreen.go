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
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/chrome/uiauto/state"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

const uiTimeout = 10 * time.Second

// State contains the state returned by chrome.autotestPrivate.loginStatus,
// corresponding to 'LoginStatusDict' as defined in autotest_private.idl.
type State struct {
	LoggedIn            bool   `json:"isLoggedIn"`
	Owner               bool   `json:"isOwner"`
	Locked              bool   `json:"isScreenLocked"`
	ReadyForPassword    bool   `json:"isReadyForPassword"` // Login screen may not be ready to receive a password, even if this is true (crbug/1109381)
	RegularUser         bool   `json:"isRegularUser"`
	Guest               bool   `json:"isGuest"`
	Kiosk               bool   `json:"isKiosk"`
	Email               string `json:"email"`
	DisplayEmail        string `json:"displayEmail"`
	UserImage           string `json:"userImage"`
	HasValidOauth2Token bool   `json:"hasValidOauth2Token"`
}

// GetState returns the login status information from chrome.autotestPrivate.loginStatus
func GetState(ctx context.Context, tconn *chrome.TestConn) (State, error) {
	var st State
	if err := tconn.Call(ctx, &st, `tast.promisify(chrome.autotestPrivate.loginStatus)`); err != nil {
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

// PasswordFieldFinder generates Finder for the password field.
// The password field node can be uniquely identified by its name attribute, which includes the username,
// such as "Password for username@gmail.com". The Finder will find the node whose name matches the regex
// /Password for <username>/, so the domain can be omitted, or the username argument can be an empty
// string to find the first password field in the hierarchy.
func PasswordFieldFinder(ctx context.Context, tconn *chrome.TestConn, username string) (*nodewith.Finder, error) {
	r, err := regexp.Compile(fmt.Sprintf("Password for %v", username))
	if err != nil {
		return nil, errors.Wrap(err, "failed to compile regexp for name attribute")
	}
	return nodewith.Role(role.TextField).Attribute("name", r), nil
}

// WaitForPasswordField waits for the password text field for a given user pod to appear in the UI.
func WaitForPasswordField(ctx context.Context, tconn *chrome.TestConn, username string, timeout time.Duration) error {
	finder, err := PasswordFieldFinder(ctx, tconn, username)
	if err != nil {
		return err
	}
	return uiauto.New(tconn).WithTimeout(timeout).WaitUntilExists(finder)(ctx)
}

// EnterPassword enters and submits the given password. Refer to PasswordFieldFinder for username options.
// It doesn't make any assumptions about the password being correct, so callers should verify the login/lock state afterwards.
func EnterPassword(ctx context.Context, tconn *chrome.TestConn, username, password string, kb *input.KeyboardEventWriter) error {
	field, err := PasswordFieldFinder(ctx, tconn, username)
	if err != nil {
		return err
	}
	ui := uiauto.New(tconn)
	if err := ui.WithTimeout(uiTimeout).WaitUntilExists(field)(ctx); err != nil {
		return errors.Wrap(err, "failed to find password box")
	}
	if err := ui.LeftClick(field)(ctx); err != nil {
		return errors.Wrap(err, "failed to click password box")
	}
	// Wait for the field to be focused before entering the password.
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		info, err := ui.Info(ctx, field)
		if err != nil {
			return errors.Wrap(err, "failed to get field node info")
		}
		if !info.State[state.Focused] {
			return errors.New("password field not focused yet")
		}
		return nil
	}, nil); err != nil {
		return err
	}

	if err := kb.Type(ctx, password+"\n"); err != nil {
		return errors.Wrap(err, "failed to enter and submit password")
	}
	return nil
}

// Lock locks the screen.
func Lock(ctx context.Context, tconn *chrome.TestConn) error {
	if err := tconn.Eval(ctx, `chrome.autotestPrivate.lockScreen()`, nil); err != nil {
		return errors.Wrap(err, "failed calling chrome.autotestPrivate.lockScreen")
	}

	if st, err := WaitState(ctx, tconn, func(st State) bool { return st.Locked && st.ReadyForPassword }, 3*uiTimeout); err != nil {
		return errors.Wrapf(err, "waiting for screen to be locked failed: %v (last status %+v)", err, st)
	}

	return nil
}

// EnterPIN enters the specified PIN.
func EnterPIN(ctx context.Context, tconn *chrome.TestConn, PIN string) error {
	ui := uiauto.New(tconn)
	for i, d := range PIN {
		button := nodewith.Role(role.Button).Name(string(d))
		if err := ui.WithTimeout(uiTimeout).LeftClick(button)(ctx); err != nil {
			return errors.Wrapf(err, "failed to press %q button (Digit %v of PIN)", d, i)
		}
	}
	return nil
}

// SubmitPIN submits the entered PIN.
func SubmitPIN(ctx context.Context, tconn *chrome.TestConn) error {
	ui := uiauto.New(tconn)
	submitButton := nodewith.Name("Submit").Role(role.Button)
	return ui.WithTimeout(uiTimeout).LeftClick(submitButton)(ctx)
}

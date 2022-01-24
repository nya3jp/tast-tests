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
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

const uiTimeout = 10 * time.Second

// AuthErrorFinder is the finder for the authentication error shown on the first failure.
var AuthErrorFinder = nodewith.Role(role.AlertDialog).NameStartingWith("Your PIN or password couldn't be verified. Try again.").ClassName("LoginErrorBubble")

// ConsecutiveAuthErrorFinder is the finder for the authentication error shown on the consecutive failures.
var ConsecutiveAuthErrorFinder = nodewith.Role(role.AlertDialog).NameStartingWith("Your PIN or password still couldn't be verified. Note: If you recently changed your password, use your old password. Your new password will be applied once you sign out.").ClassName("LoginErrorBubble")

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

// WaitForLoggedIn is a wrapper around WaitState to wait for the user to be logged in.
func WaitForLoggedIn(ctx context.Context, tconn *chrome.TestConn, timeout time.Duration) error {
	if st, err := WaitState(ctx, tconn, func(st State) bool { return st.LoggedIn }, timeout); err != nil {
		return errors.Wrapf(err, "waiting for logged in state failed: (last status %+v)", st)
	}
	return nil
}

// PasswordFieldFinder generates Finder for the password field.
// The password field node can be uniquely identified by its name attribute, which includes the username,
// such as "Password for username@gmail.com". The Finder will find the node whose name matches the regex
// /Password for <username>/, so the domain can be omitted, or the username argument can be an empty
// string to find the first password field in the hierarchy.
func PasswordFieldFinder(username string) (*nodewith.Finder, error) {
	r, err := regexp.Compile(fmt.Sprintf("Password for %v", username))
	if err != nil {
		return nil, errors.Wrap(err, "failed to compile regexp for name attribute")
	}
	return nodewith.Role(role.TextField).Attribute("name", r).Attribute("placeholder", "Password"), nil
}

// WaitForPasswordField waits for the password text field for a given user pod to appear in the UI.
func WaitForPasswordField(ctx context.Context, tconn *chrome.TestConn, username string, timeout time.Duration) error {
	finder, err := PasswordFieldFinder(username)
	if err != nil {
		return err
	}
	return uiauto.New(tconn).WithTimeout(timeout).WaitUntilExists(finder)(ctx)
}

// WaitForAuthError waits for the login error bubble that password or pin was not correct.
func WaitForAuthError(ctx context.Context, tconn *chrome.TestConn, timeout time.Duration) error {
	return uiauto.New(tconn).WithTimeout(timeout).WaitUntilExists(AuthErrorFinder)(ctx)
}

// TypePassword enters the given password (without submitting). Refer to PasswordFieldFinder for username options.
// It doesn't make any assumptions about the password being correct, so callers should verify the login/lock state afterwards.
func TypePassword(ctx context.Context, tconn *chrome.TestConn, username, password string, kb *input.KeyboardEventWriter) error {
	field, err := PasswordFieldFinder(username)
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
	if err := ui.WaitUntilExists(field.Focused())(ctx); err != nil {
		return errors.Wrap(err, "password field not focused yet")
	}
	if err := kb.Type(ctx, password); err != nil {
		return errors.Wrap(err, "failed to type password")
	}
	return nil
}

// EnterPassword types password with carriage return at the end.
func EnterPassword(ctx context.Context, tconn *chrome.TestConn, username, password string, kb *input.KeyboardEventWriter) error {
	return TypePassword(ctx, tconn, username, password+"\n", kb)
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

// HasPinPad checks whether the PIN pad is present, i.e., whether PIN unlock is enabled.
func HasPinPad(ctx context.Context, tconn *chrome.TestConn) bool {
	ui := uiauto.New(tconn)
	found, _ := ui.IsNodeFound(ctx, nodewith.ClassName("LoginPinView"))
	return found
}

// SubmitPIN submits the entered PIN.
func SubmitPIN(ctx context.Context, tconn *chrome.TestConn) error {
	return uiauto.New(tconn).WithTimeout(uiTimeout).LeftClick(SubmitButton)(ctx)
}

// ClickUserImage clicks the users image to login with Smart Lock.
func ClickUserImage(ctx context.Context, tconn *chrome.TestConn) error {
	ui := uiauto.New(tconn)
	userImage := nodewith.ClassName("LoginUserImage")
	return ui.WithTimeout(uiTimeout).LeftClick(userImage)(ctx)
}

// WaitForSmartUnlockReady waits for UI signal that the chromebook is ready to be unlocked by Smart Lock.
func WaitForSmartUnlockReady(ctx context.Context, tconn *chrome.TestConn) error {
	finder := nodewith.Name("Your device can be unlocked with Smart Lock. Press Enter to unlock.").ClassName("ImageButton")
	ui := uiauto.New(tconn)
	if err := ui.WaitUntilExists(finder)(ctx); err != nil {
		return errors.Wrap(err, "failed to wait for Smart Lock UI to indicate it is ready to unlock")
	}
	return nil
}

// WaitForSmartLockPasswordPrompt waits for the login screen indication that the user must login one more time with their password to enable the Smart Lock for login feature.
func WaitForSmartLockPasswordPrompt(ctx context.Context, tconn *chrome.TestConn) error {
	finder := nodewith.NameContaining("Enter your password to enable Smart Lock.").Role(role.StaticText)
	ui := uiauto.New(tconn)
	if err := ui.WaitUntilExists(finder)(ctx); err != nil {
		return errors.Wrap(err, "failed to wait for Smart Lock UI to show password will help you with smart lock")
	}
	return nil
}

// ShowPassword clicks the "Show password" button.
func ShowPassword(ctx context.Context, tconn *chrome.TestConn) error {
	return uiauto.New(tconn).WithTimeout(uiTimeout).LeftClick(ShowPasswordButton)(ctx)
}

// HidePassword clicks the "Hide password" button.
func HidePassword(ctx context.Context, tconn *chrome.TestConn) error {
	return uiauto.New(tconn).WithTimeout(uiTimeout).LeftClick(HidePasswordButton)(ctx)
}

// SwitchToPassword clicks the "Switch to password" button which appears only when PIN autosubmit is enabled.
func SwitchToPassword(ctx context.Context, tconn *chrome.TestConn) error {
	return uiauto.New(tconn).WithTimeout(uiTimeout).LeftClick(SwitchToPasswordButton)(ctx)
}

// PINFieldFinder generates Finder for the "PIN or password" field.
func PINFieldFinder(username string) (*nodewith.Finder, error) {
	r := regexp.MustCompile(fmt.Sprintf("Password for %v", username))
	return nodewith.Role(role.TextField).Attribute("name", r).Attribute("placeholder", "PIN or password"), nil
}

// UserPassword searches the PIN / Password field for a given user pod and returns the corresponding node.
func UserPassword(ctx context.Context, tconn *chrome.TestConn, username string, pin bool) (*uiauto.NodeInfo, error) {
	var field *nodewith.Finder
	var err error
	if pin {
		field, err = PINFieldFinder(username)
		if err != nil {
			return nil, errors.Wrap(err, "failed to find the node for \"PIN or password\" field")
		}
	} else {
		field, err = PasswordFieldFinder(username)
		if err != nil {
			return nil, errors.Wrap(err, "failed to find the Password node")
		}
	}
	return uiauto.New(tconn).Info(ctx, field)
}

// Unlock unlocks the screen, assuming that PIN / Password has already been entered by the user.
func Unlock(ctx context.Context, tconn *chrome.TestConn) error {
	if err := SubmitPIN(ctx, tconn); err != nil {
		return errors.Wrap(err, "failed to click the Submit button")
	}
	if st, err := WaitState(ctx, tconn, func(st State) bool { return st.LoggedIn }, 3*uiTimeout); err != nil {
		return errors.Wrapf(err, "failed waiting to log in: %v, last state: %+v", err, st)
	}
	return nil
}

// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package lockscreen is used to get information about the lock screen directly through the UI.
package lockscreen

import (
	"context"
	"time"

	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ui/lockscreen"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/input"
)

// State is defined in See chromiumos/tast/local/chrome/ui/lockscreen/lockscreen.go
type State = lockscreen.State

// GetState returns the login status information from chrome.autotestPrivate.loginStatus
func GetState(ctx context.Context, tconn *chrome.TestConn) (State, error) {
	return lockscreen.GetState(ctx, tconn)
}

// WaitState repeatedly calls GetState and passes the returned state to check
// until it returns true or timeout is reached. The last-seen state is returned.
func WaitState(ctx context.Context, tconn *chrome.TestConn, check func(st State) bool, timeout time.Duration) (State, error) {
	return lockscreen.WaitState(ctx, tconn, check, timeout)
}

// PasswordFieldFinder generates Finder for the password field.
// The password field node can be uniquely identified by its name attribute, which includes the username,
// such as "Password for username@gmail.com". The Finder will find the node whose name matches the regex
// /Password for <username>/, so the domain can be omitted, or the username argument can be an empty
// string to find the first password field in the hierarchy.
func PasswordFieldFinder(ctx context.Context, tconn *chrome.TestConn, username string) (*nodewith.Finder, error) {
	return lockscreen.PasswordFieldFinder(ctx, tconn, username)
}

// WaitForPasswordField waits for the password text field for a given user pod to appear in the UI.
func WaitForPasswordField(ctx context.Context, tconn *chrome.TestConn, username string, timeout time.Duration) error {
	return lockscreen.WaitForPasswordField(ctx, tconn, username, timeout)
}

// EnterPassword enters and submits the given password. Refer to PasswordFieldFinder for username options.
// It doesn't make any assumptions about the password being correct, so callers should verify the login/lock state afterwards.
func EnterPassword(ctx context.Context, tconn *chrome.TestConn, username, password string, kb *input.KeyboardEventWriter) error {
	return lockscreen.EnterPassword(ctx, tconn, username, password, kb)
}

// Lock locks the screen.
func Lock(ctx context.Context, tconn *chrome.TestConn) error {
	return lockscreen.Lock(ctx, tconn)
}

// EnterPIN enters the specified PIN.
func EnterPIN(ctx context.Context, tconn *chrome.TestConn, PIN string) error {
	return lockscreen.EnterPIN(ctx, tconn, PIN)
}

// SubmitPIN submits the entered PIN.
func SubmitPIN(ctx context.Context, tconn *chrome.TestConn) error {
	return lockscreen.SubmitPIN(ctx, tconn)
}

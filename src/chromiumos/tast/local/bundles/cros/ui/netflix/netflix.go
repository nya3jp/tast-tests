// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package netflix

import (
	"context"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/ui/cuj"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ui"
	"chromiumos/tast/local/chrome/webutil"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

// Netflix implement some applications for netflix web.
type Netflix struct {
	conn *chrome.Conn
}

const (
	signInURL  = "https://www.netflix.com/login"
	signOutURL = "https://www.netflix.com/SignOut?Inkctr=mL"
)

// New creates a new netflix instance and signin to netflix.
func New(ctx context.Context, tconn *chrome.TestConn, cr *chrome.Chrome, username, password string) (*Netflix, error) {
	conn, err := cr.NewConn(ctx, signInURL)
	if err != nil {
		return nil, errors.Wrap(err, "failed to open Netflix")
	}
	if err := signIn(ctx, tconn, conn, username, password); err != nil {
		return nil, errors.Wrap(err, "failed to sign in Netflix")
	}
	return &Netflix{conn}, nil
}

func signIn(ctx context.Context, tconn *chrome.TestConn, conn *chrome.Conn, username, password string) error {
	const (
		loginFormClass = "login-form"
		timeout        = time.Second * 20
	)
	kb, err := input.Keyboard(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to open the keyboard")
	}
	defer kb.Close()

	if err := webutil.WaitForQuiescence(ctx, conn, timeout); err != nil {
		return errors.Wrap(err, "failed to wait for netflix login page to finish loading")
	}

	if _, err := ui.FindWithTimeout(ctx, tconn,
		ui.FindParams{ClassName: loginFormClass}, timeout); err != nil {
		testing.ContextLog(ctx, "User has signed in")
		return nil
	}

	accountParams := ui.FindParams{Name: "Email or phone number"}
	if err := cuj.WaitAndClick(ctx, tconn, accountParams, timeout); err != nil {
		return errors.Wrap(err, "failed to click account field")
	}

	if err := ui.WaitForLocationChangeCompleted(ctx, tconn); err != nil {
		return errors.Wrap(err, "failed to wait for animation finished")
	}

	if err := kb.Type(ctx, username); err != nil {
		return errors.Wrap(err, "failed to type email")
	}

	if err := ui.WaitForLocationChangeCompleted(ctx, tconn); err != nil {
		return errors.Wrap(err, "failed to wait for animation finished")
	}

	passwordParams := ui.FindParams{Name: "Password"}
	if err := cuj.WaitAndClick(ctx, tconn, passwordParams, timeout); err != nil {
		return errors.Wrap(err, "failed to click password field")
	}

	if err := ui.WaitForLocationChangeCompleted(ctx, tconn); err != nil {
		return errors.Wrap(err, "failed to wait for animation finished")
	}

	if err := kb.Type(ctx, password); err != nil {
		return errors.Wrap(err, "failed to type password")
	}

	if err := ui.WaitForLocationChangeCompleted(ctx, tconn); err != nil {
		return errors.Wrap(err, "failed to wait for animation finished")
	}

	if err := kb.Accel(ctx, "Enter"); err != nil {
		return errors.Wrap(err, "failed to press enter")
	}

	if err := ui.WaitForLocationChangeCompleted(ctx, tconn); err != nil {
		return errors.Wrap(err, "failed to wait for animation finished")
	}

	// Check that user was logged in successfully
	if _, err := ui.FindWithTimeout(ctx, tconn, ui.FindParams{Name: "Home"}, timeout); err != nil {
		return errors.Wrap(err, "failed to sign in")
	}

	return nil
}

// Play netflix video
func (n *Netflix) Play(ctx context.Context, videoURL string) error {
	if err := n.conn.Navigate(ctx, videoURL); err != nil {
		return errors.Wrap(err, "failed to navigate to the url")
	}

	if err := webutil.WaitForQuiescence(ctx, n.conn, time.Second*30); err != nil {
		return errors.Wrap(err, "failed to wait for netflix logout page to finish loading")
	}
	return nil
}

// WaitForLoading wait for netflix page to finish loading
func (n *Netflix) WaitForLoading(ctx context.Context, timeout time.Duration) error {
	if err := webutil.WaitForQuiescence(ctx, n.conn, timeout); err != nil {
		return errors.Wrap(err, "failed to wait for netflix page to finish loading")
	}
	return nil
}

// SignOut signout netflix and close connection
func (n *Netflix) SignOut(ctx context.Context, tconn *chrome.TestConn) error {
	if err := n.conn.Navigate(ctx, signOutURL); err != nil {
		return errors.Wrap(err, "failed to navigate to the url")
	}
	defer n.conn.CloseTarget(ctx)
	if err := webutil.WaitForQuiescence(ctx, n.conn, time.Second*30); err != nil {
		return errors.Wrap(err, "failed to wait for netflix logout page to finish loading")
	}
	// Check that user was sign out successfully
	if _, err := ui.FindWithTimeout(ctx, tconn, ui.FindParams{Name: "Sign In"}, time.Second*20); err != nil {
		return errors.Wrap(err, "failed to sign out")
	}
	return nil
}

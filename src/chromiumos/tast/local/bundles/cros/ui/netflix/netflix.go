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

// Netflix stores chrome connection of netflix web.
type Netflix struct {
	conn *chrome.Conn
}

const (
	homeURL    = "https://www.netflix.com"
	signInURL  = "https://www.netflix.com/login"
	signOutURL = "https://www.netflix.com/SignOut?Inkctr=mL"
	timeout    = time.Second * 30
)

// New creates a new netflix instance and signin to netflix.
func New(ctx context.Context, tconn *chrome.TestConn, username, password string, cr *chrome.Chrome) (*Netflix, error) {
	conn, err := cr.NewConn(ctx, homeURL)
	if err != nil {
		return nil, errors.Wrap(err, "failed to open Netflix")
	}
	if err := webutil.WaitForQuiescence(ctx, conn, timeout); err != nil {
		return nil, errors.Wrap(err, "failed to wait for netflix login page to finish loading")
	}

	// Check that user was logged in
	homeParam := ui.FindParams{Name: "Home", Role: ui.RoleTypeLink}
	if _, err := ui.FindWithTimeout(ctx, tconn, homeParam, 5*time.Second); err == nil {
		testing.ContextLog(ctx, "Find Params: Home, User has logged in")
		return &Netflix{conn}, nil
	}
	if err := signIn(ctx, tconn, conn, username, password); err != nil {
		return nil, errors.Wrap(err, "failed to sign in Netflix")
	}
	return &Netflix{conn}, nil
}

func signIn(ctx context.Context, tconn *chrome.TestConn, conn *chrome.Conn, username, password string) error {
	kb, err := input.Keyboard(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to open the keyboard")
	}
	defer kb.Close()

	if err := conn.Navigate(ctx, signInURL); err != nil {
		return errors.Wrapf(err, "failed to navigate to the sign in url: %q", signInURL)
	}

	if err := webutil.WaitForQuiescence(ctx, conn, timeout); err != nil {
		return errors.Wrap(err, "failed to wait for netflix login page to finish loading")
	}

	accountParam := ui.FindParams{Name: "Email or phone number"}
	if err := cuj.WaitAndClick(ctx, tconn, accountParam, timeout); err != nil {
		return errors.Wrap(err, "failed to click account field")
	}

	if err := kb.Type(ctx, username); err != nil {
		return errors.Wrap(err, "failed to type email")
	}

	passwordParam := ui.FindParams{Name: "Password"}
	if err := cuj.WaitAndClick(ctx, tconn, passwordParam, timeout); err != nil {
		return errors.Wrap(err, "failed to click password field")
	}

	if err := kb.Type(ctx, password); err != nil {
		return errors.Wrap(err, "failed to type password")
	}

	if err := kb.Accel(ctx, "Enter"); err != nil {
		return errors.Wrap(err, "failed to press enter")
	}

	// Check that user was logged in successfully.
	homeParam := ui.FindParams{Name: "Home", Role: ui.RoleTypeLink}
	if _, err := ui.FindWithTimeout(ctx, tconn, homeParam, timeout); err != nil {
		return errors.Wrap(err, "failed to sign in")
	}

	return nil
}

// Play starts a netflix video.
func (n *Netflix) Play(ctx context.Context, videoURL string) error {
	if err := n.conn.Navigate(ctx, videoURL); err != nil {
		return errors.Wrap(err, "failed to navigate to the url")
	}

	return n.WaitForLoading(ctx, timeout)
}

// WaitForLoading wait for netflix page to finish loading.
func (n *Netflix) WaitForLoading(ctx context.Context, timeout time.Duration) error {
	if err := webutil.WaitForQuiescence(ctx, n.conn, timeout); err != nil {
		return errors.Wrap(err, "failed to wait for netflix page to finish loading")
	}
	return nil
}

// SignOut signout netflix.
func (n *Netflix) SignOut(ctx context.Context, tconn *chrome.TestConn) error {
	if err := n.conn.Navigate(ctx, signOutURL); err != nil {
		return errors.Wrap(err, "failed to navigate to the url")
	}
	if err := webutil.WaitForQuiescence(ctx, n.conn, timeout); err != nil {
		return errors.Wrap(err, "failed to wait for netflix logout page to finish loading")
	}
	// Check that user was sign out successfully.
	if _, err := ui.FindWithTimeout(ctx, tconn, ui.FindParams{Name: "Sign In"}, timeout); err != nil {
		return errors.Wrap(err, "failed to sign out")
	}
	return nil
}

// Close closes netflix connection.
func (n *Netflix) Close(ctx context.Context) error {
	if err := n.conn.CloseTarget(ctx); err != nil {
		return errors.Wrap(err, "failed to close Netflix")
	}
	return nil
}

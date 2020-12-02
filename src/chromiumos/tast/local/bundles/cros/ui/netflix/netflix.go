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

// Netflix provides controls of Netflix website.
type Netflix struct {
	conn *chrome.Conn
}

const (
	homeURL    = "https://www.netflix.com"
	signOutURL = "https://www.netflix.com/SignOut"
	timeout    = time.Second * 30
)

// New creates a Netflix instance and signs into Netflix website.
func New(ctx context.Context, tconn *chrome.TestConn, username, password string, cr *chrome.Chrome) (n *Netflix, err error) {
	conn, err := cr.NewConn(ctx, homeURL)
	if err != nil {
		return nil, errors.Wrap(err, "failed to open Netflix")
	}
	defer func() {
		if err != nil {
			conn.Close()
		}
	}()

	if err = webutil.WaitForQuiescence(ctx, conn, timeout); err != nil {
		return nil, errors.Wrap(err, "failed to wait for Netflix login page to finish loading")
	}
	kidsParam := ui.FindParams{Name: "Kids", Role: ui.RoleTypeLink}
	if err := ui.WaitUntilExists(ctx, tconn, kidsParam, timeout); err == nil {
		testing.ContextLog(ctx, "Kids is found, assuming the login is completed already")
		return &Netflix{conn: conn}, nil
	}
	// Check that user was logged in
	homeParam := ui.FindParams{Name: "Home", Role: ui.RoleTypeLink}
	if err := ui.WaitUntilExists(ctx, tconn, homeParam, 5*time.Second); err == nil {
		testing.ContextLog(ctx, "Home is found, assuming the login is completed already")
		return &Netflix{conn: conn}, nil
	}
	if err = n.signIn(ctx, tconn, conn, username, password); err != nil {
		return nil, errors.Wrap(err, "failed to sign in Netflix")
	}
	return &Netflix{conn: conn}, nil
}

func (n *Netflix) signIn(ctx context.Context, tconn *chrome.TestConn, conn *chrome.Conn, username, password string) error {
	testing.ContextLog(ctx, "Attempting to log into Netflix")
	kb, err := input.Keyboard(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to open the keyboard")
	}
	defer kb.Close()

	signInParam := ui.FindParams{Name: "Sign In", Role: ui.RoleTypeLink}
	if err := cuj.WaitAndClick(ctx, tconn, signInParam, timeout); err != nil {
		return errors.Wrap(err, "failed to click sign in field")
	}

	if err := webutil.WaitForQuiescence(ctx, conn, timeout); err != nil {
		return errors.Wrap(err, "failed to wait for Netflix login page to finish loading")
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
	if err := ui.WaitUntilExists(ctx, tconn, homeParam, timeout); err != nil {
		return errors.Wrap(err, "failed to sign in")
	}

	return nil
}

// Play starts a Netflix video.
func (n *Netflix) Play(ctx context.Context, videoURL string) error {
	if err := n.conn.Navigate(ctx, videoURL); err != nil {
		return errors.Wrap(err, "failed to navigate to the video url")
	}
	if err := webutil.WaitForQuiescence(ctx, n.conn, timeout); err != nil {
		return errors.Wrap(err, "failed to wait for Netflix page to finish loading")
	}

	return nil
}

// SignOut signs out from Netflix website.
func (n *Netflix) SignOut(ctx context.Context, tconn *chrome.TestConn) error {
	if err := n.conn.Navigate(ctx, signOutURL); err != nil {
		return errors.Wrap(err, "failed to navigate to the signout url")
	}
	if err := webutil.WaitForQuiescence(ctx, n.conn, timeout); err != nil {
		return errors.Wrap(err, "failed to wait for Netflix logout page to finish loading")
	}
	// Check that user has signed out successfully.
	if err := ui.WaitUntilExists(ctx, tconn, ui.FindParams{Name: "Sign In"}, timeout); err != nil {
		return errors.Wrap(err, "failed to sign out")
	}
	return nil
}

// Close closes Netflix website connection
func (n *Netflix) Close() error {
	if err := n.conn.Close(); err != nil {
		return errors.Wrap(err, "failed to close Netflix")
	}
	return nil
}

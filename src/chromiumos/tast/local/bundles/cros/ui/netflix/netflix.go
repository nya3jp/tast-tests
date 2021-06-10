// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package netflix

import (
	"context"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
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
			if closeErr := conn.Close(); closeErr != nil {
				testing.ContextLog(ctx, "Failed to close the connection: ", closeErr)
			}
		}
	}()

	if err = webutil.WaitForQuiescence(ctx, conn, timeout); err != nil {
		return nil, errors.Wrap(err, "failed to wait for Netflix login page to finish loading")
	}
	ac := uiauto.New(tconn)
	kidsParam := nodewith.Name("Kids").Role(role.Link)
	if err := ac.WithTimeout(timeout).WaitUntilExists(kidsParam)(ctx); err == nil {
		testing.ContextLog(ctx, "Kids is found, assuming the login is completed already")
		return &Netflix{conn: conn}, nil
	}
	// Check that user was logged in
	homeParam := nodewith.Name("Home").Role(role.Link)
	if err := ac.WithTimeout(5 * time.Second).WaitUntilExists(homeParam)(ctx); err == nil {
		testing.ContextLog(ctx, "Home is found, assuming the login is completed already")
		return &Netflix{conn: conn}, nil
	}
	if err = n.signIn(ctx, ac, conn, username, password); err != nil {
		return nil, errors.Wrap(err, "failed to sign in Netflix")
	}
	return &Netflix{conn: conn}, nil
}

func (n *Netflix) signIn(ctx context.Context, ac *uiauto.Context, conn *chrome.Conn, username, password string) error {
	testing.ContextLog(ctx, "Attempting to log into Netflix")
	kb, err := input.Keyboard(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to open the keyboard")
	}
	defer kb.Close()

	signInParam := nodewith.Name("Sign In").Role(role.Link)
	accountParam := nodewith.Name("Email or phone number")
	passwordParam := nodewith.Name("Password")
	homeParam := nodewith.Name("Home").Role(role.Link)
	return uiauto.Combine(
		"netflix signin",
		ac.WithTimeout(timeout).LeftClick(signInParam),
		func(ctx context.Context) error { return webutil.WaitForQuiescence(ctx, conn, timeout) },
		ac.WithTimeout(timeout).LeftClick(accountParam),
		kb.TypeAction(username),
		ac.WithTimeout(timeout).LeftClick(passwordParam),
		kb.TypeAction(password),
		kb.AccelAction("Enter"),
		ac.WithTimeout(timeout).WaitUntilExists(homeParam),
	)(ctx)
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
	if err := uiauto.New(tconn).WithTimeout(timeout).WaitUntilExists(nodewith.Name("Sign In"))(ctx); err != nil {
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

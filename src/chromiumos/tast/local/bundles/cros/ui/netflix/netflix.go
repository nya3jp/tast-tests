// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package netflix

import (
	"context"
	"strings"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/ui/cuj"
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
	conn             *chrome.Conn
	ui               *uiauto.Context
	uiHdl            cuj.UIActionHandler
	isProfileClicked bool
	isSignIn         bool
}

const (
	homeURL    = "https://www.netflix.com/browse"
	signOutURL = "https://www.netflix.com/SignOut"
)

// New creates a Netflix instance and signs into Netflix website.
func New(ctx context.Context, tconn *chrome.TestConn, username, password string, cr *chrome.Chrome, uiHdl cuj.UIActionHandler, playbackSettings string) (n *Netflix, err error) {
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

	n = &Netflix{
		conn:  conn,
		ui:    uiauto.New(tconn),
		uiHdl: uiHdl,
	}

	if err = webutil.WaitForQuiescence(ctx, conn, 30*time.Second); err != nil {
		return nil, errors.Wrap(err, "failed to wait for Netflix login page to finish loading")
	}

	// 1. User has not logged in yet if error is not nil.
	// 2. Prevent test from being interrupted by who's watching page after we opened netflix.
	if err := n.switchProfile(ctx, conn, playbackSettings); err == nil {
		return n, nil
	}

	var signInSucceed bool
	// Check home label first then call signIn if user hasn't logged in yet.
	homeLabel := nodewith.Name("Home").Role(role.Link)
	if err := n.ui.WaitUntilExists(homeLabel)(ctx); err != nil {
		if signInSucceed, err = n.signIn(ctx, tconn, conn, username, password, playbackSettings); err != nil {
			return nil, errors.Wrap(err, "failed to sign in Netflix")
		}
	}
	testing.ContextLog(ctx, "Home is found, assuming the login is completed already")

	// Select profile if user already signed in.
	if !signInSucceed {
		if err := n.selectProfile(ctx, playbackSettings); err != nil {
			return nil, errors.Wrap(err, "failed to select profile")
		}
	}

	return n, nil
}

func (n *Netflix) selectProfile(ctx context.Context, playbackSettings string) error {
	// Click the profile icon to switch profile.
	if err := n.uiHdl.Click(nodewith.ClassName("account-menu-item").First())(ctx); err != nil {
		return errors.Wrap(err, "failed to find account button to switch profiles")
	}
	n.isProfileClicked = true

	// To ensure the profile is our expected.
	if err := n.switchProfile(ctx, n.conn, playbackSettings); err == nil {
		testing.ContextLogf(ctx, "You already in %s mode", playbackSettings)
	}
	return nil
}

func (n *Netflix) switchProfile(ctx context.Context, conn *chrome.Conn, playbackSettings string) error {
	targetProfile := nodewith.NameStartingWith(playbackSettings + "-").Role(role.Link)
	nodeInfo, err := n.ui.Info(ctx, targetProfile)
	if err != nil {
		// There are three scenarios if return error here :
		// 1. Already in correct profile. (Basic or Plus)
		if n.isProfileClicked {
			n.isProfileClicked = false
			return nil
		}
		// 2. Failed to find target profile.
		//   (1). user hasn't logged in yet. (Or failed to sign in)
		//   (2). who's watching page was not shown
		// 3. Failed to click the profile icon.
		return errors.Wrap(err, "failed to find profile's node info")
	}
	if err := n.uiHdl.Click(targetProfile)(ctx); err != nil {
		return errors.Wrap(err, "failed to click profile's node")
	}
	testing.ContextLog(ctx, "Profile was clicked, assuming the login is completed already")
	testing.ContextLogf(ctx, "Switch profile to %s", nodeInfo.Name)
	// Wait for switching account to finish.
	if err := webutil.WaitForQuiescence(ctx, conn, 15*time.Second); err != nil {
		return errors.Wrap(err, "failed to wait for switching account to finish")
	}
	return nil
}

func (n *Netflix) signIn(ctx context.Context, tconn *chrome.TestConn, conn *chrome.Conn, username, password, playbackSettings string) (bool, error) {
	testing.ContextLog(ctx, "Attempting to log into Netflix")

	kb, err := input.Keyboard(ctx)
	if err != nil {
		return n.isSignIn, errors.Wrap(err, "failed to open the keyboard")
	}
	defer kb.Close()

	accountField := nodewith.Name("Email or phone number").First()
	passwordField := nodewith.Name("Password").First()

	if err := uiauto.Combine("sign in Netflix",
		n.uiHdl.Click(accountField),
		// Prevent account was saved by Chrome.
		kb.AccelAction("Ctrl+A"),
		kb.TypeAction(username),
		n.uiHdl.ClickUntil(passwordField, n.ui.WaitUntilExists(passwordField.Focused())),
		// Prevent password was saved by Chrome.
		kb.AccelAction("Ctrl+A"),
		kb.TypeAction(password),
		kb.AccelAction("Enter"),
		// Avoid to enter account page.
		n.ui.WaitUntilGone(passwordField),
	)(ctx); err != nil {
		return n.isSignIn, err
	}

	// Select specific profile on who's watching page after user logged in netflix at first time.
	if err := n.switchProfile(ctx, conn, playbackSettings); err != nil {
		// Who's watching page was not shown after logged in.
		homeLabel := nodewith.Name("Home").Role(role.Link)
		if err := n.ui.WaitUntilExists(homeLabel)(ctx); err != nil {
			return n.isSignIn, errors.Wrap(err, "failed to sign in")
		}

		n.isSignIn = true

		// May failed to click profile's node or timeout.
		if errStruct, ok := err.(*errors.E); ok {
			if strings.Contains(errStruct.Error(), "click profile") || strings.Contains(errStruct.Error(), "switching") {
				return n.isSignIn, errors.Wrap(err, "failed to switch account after logged in")
			}
		}
	}

	if !n.isSignIn {
		homeLabel := nodewith.Name("Home").Role(role.Link)
		if err := n.ui.WaitUntilExists(homeLabel)(ctx); err != nil {
			return n.isSignIn, errors.Wrap(err, "failed to sign in")
		}
	}

	if err := n.selectProfile(ctx, playbackSettings); err != nil {
		return n.isSignIn, errors.Wrap(err, "failed to select profile")
	}

	return n.isSignIn, nil
}

// Play starts a Netflix video.
func (n *Netflix) Play(ctx context.Context, videoURL string) error {
	if err := n.conn.Navigate(ctx, videoURL); err != nil {
		return errors.Wrap(err, "failed to navigate to the video url")
	}
	if err := webutil.WaitForQuiescence(ctx, n.conn, 10*time.Second); err != nil {
		return errors.Wrap(err, "failed to wait for Netflix page to finish loading")
	}
	return nil
}

// SignOut signs out from Netflix website.
func (n *Netflix) SignOut(ctx context.Context, tconn *chrome.TestConn) error {
	if err := n.conn.Navigate(ctx, signOutURL); err != nil {
		return errors.Wrap(err, "failed to navigate to the signout url")
	}
	if err := webutil.WaitForQuiescence(ctx, n.conn, 10*time.Second); err != nil {
		return errors.Wrap(err, "failed to wait for Netflix logout page to finish loading")
	}
	// Check that user has signed out successfully.
	ui := uiauto.New(tconn)
	signIn := nodewith.Name("Sign In")
	if err := ui.WaitUntilExists(signIn)(ctx); err != nil {
		return errors.Wrap(err, "failed to sign in")
	}
	return nil
}

// Close closes the web content and connection to netflix.
func (n *Netflix) Close(ctx context.Context) {
	n.conn.CloseTarget(ctx)
	n.conn.Close()
}

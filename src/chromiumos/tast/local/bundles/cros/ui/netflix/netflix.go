// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package netflix

import (
	"context"
	"regexp"
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
	homeURL    = "https://www.netflix.com/browse"
	signOutURL = "https://www.netflix.com/SignOut"
	timeout    = time.Second * 30
)

// New creates a Netflix instance and signs into Netflix website.
func New(ctx context.Context, tconn *chrome.TestConn, username, password string, cr *chrome.Chrome, playbackSettings string) (n *Netflix, err error) {
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
	// prevent case be interrupted by who's watching page
	if n, profileNode, err := switchProfile(ctx, tconn, conn, playbackSettings); err == nil {
		testing.ContextLogf(ctx, "Who's watching: click %s profile", profileNode.Name)

		testing.Poll(ctx, func(ctx context.Context) error {
			return errors.New("wait for 3 second")
		}, &testing.PollOptions{Interval: time.Second, Timeout: time.Second * 3})

		return n, nil
	}

	kidsParam := ui.FindParams{Name: "Kids", Role: ui.RoleTypeLink}
	if err := ui.WaitUntilExists(ctx, tconn, kidsParam, timeout); err == nil {
		testing.ContextLog(ctx, "Kids is found, assuming the login is completed already")
	}
	// Check that user was logged in
	homeParam := ui.FindParams{Name: "Home", Role: ui.RoleTypeLink}
	homeNode, err := ui.FindWithTimeout(ctx, tconn, homeParam, time.Second*5)
	if err == nil {
		testing.ContextLog(ctx, "Home is found, assuming the login is completed already")
	}

	if homeNode != nil && homeNode.Name == "Home" {
		// click profile icon
		if err := testing.Poll(ctx, func(ctx context.Context) error {
			expr := `(() => {
				document.querySelector("#appMountPoint > div > div > div:nth-child(1) > div.bd.dark-background > div.pinning-header > div > div > div > div:nth-child(4) > div > div > a").click()
			})()`
			if err := conn.Eval(ctx, expr, nil); err != nil {
				testing.ContextLog(ctx, "Can't find account button to switch profiles")
				return errors.Wrap(err, "failed to execute Javascript to find account button to switch profiles")
			}
			return nil
		}, &testing.PollOptions{Timeout: timeout}); err != nil {
			return nil, errors.Wrap(err, "failed to find account button to switch profiles")
		}

		n, profileNode, err := switchProfile(ctx, tconn, conn, playbackSettings)
		if err != nil {
			testing.ContextLogf(ctx, "You already in %s mode", playbackSettings)
			return &Netflix{conn: conn}, nil
		}
		testing.ContextLogf(ctx, "Switch profile to %s", profileNode.Name)

		testing.Poll(ctx, func(ctx context.Context) error {
			return errors.New("wait for 3 second")
		}, &testing.PollOptions{Interval: time.Second, Timeout: time.Second * 3})

		return n, nil
	}

	if err = n.signIn(ctx, tconn, conn, username, password, playbackSettings); err != nil {
		return nil, errors.Wrap(err, "failed to sign in Netflix")
	}
	return &Netflix{conn: conn}, nil
}

func switchProfile(ctx context.Context, tconn *chrome.TestConn, conn *chrome.Conn, playbackSettings string) (n *Netflix, params ui.FindParams, err error) {
	profile, findErr := findProfileParams(ctx, tconn, playbackSettings)
	if findErr == nil {
		if err := ui.WaitUntilExists(ctx, tconn, profile, timeout); err == nil {
			testing.ContextLog(ctx, "Profile is found, assuming the login is completed already")
		}
		if err := cuj.WaitAndClick(ctx, tconn, profile, timeout); err == nil {
			testing.ContextLog(ctx, "Profile is clicked, assuming the login is completed already")
			return &Netflix{conn: conn}, profile, nil
		}
	}
	return nil, ui.FindParams{}, errors.Wrap(findErr, "failed to find profile param, may not log in yet or already in specified profile")
}

func findProfileParams(ctx context.Context, tconn *chrome.TestConn, playbackSettings string) (ui.FindParams, error) {
	params := ui.FindParams{
		Role:       ui.RoleTypeLink,
		Attributes: map[string]interface{}{"name": regexp.MustCompile("^" + playbackSettings + "-.*")},
	}
	node, err := ui.FindWithTimeout(ctx, tconn, params, time.Second*3)
	if err != nil {
		return ui.FindParams{}, errors.Wrap(err, "failed to find profile's node")
	}
	testing.ContextLogf(ctx, "Account Name : %s", node.Name)
	return ui.FindParams{Name: node.Name, Role: ui.RoleTypeLink}, nil
}

func (n *Netflix) signIn(ctx context.Context, tconn *chrome.TestConn, conn *chrome.Conn, username, password, playbackSettings string) error {
	testing.ContextLog(ctx, "Attempting to log into Netflix")
	kb, err := input.Keyboard(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to open the keyboard")
	}
	defer kb.Close()

	accountParam := ui.FindParams{Name: "Email or phone number"}
	if err := cuj.WaitAndClick(ctx, tconn, accountParam, timeout); err != nil {
		return errors.Wrap(err, "failed to click account field")
	}

	if err := kb.Accel(ctx, "Ctrl+A"); err != nil {
		return errors.Wrap(err, "failed to select all")
	}

	if err := kb.Type(ctx, username); err != nil {
		return errors.Wrap(err, "failed to type email")
	}

	passwordParam := ui.FindParams{Name: "Password"}
	if err := cuj.WaitAndClick(ctx, tconn, passwordParam, timeout); err != nil {
		return errors.Wrap(err, "failed to click password field")
	}

	if err := kb.Accel(ctx, "Ctrl+A"); err != nil {
		return errors.Wrap(err, "failed to select all")
	}

	if err := kb.Type(ctx, password); err != nil {
		return errors.Wrap(err, "failed to type password")
	}

	if err := kb.Accel(ctx, "Enter"); err != nil {
		return errors.Wrap(err, "failed to press enter")
	}

	// avoid to enter Account page
	testing.Poll(ctx, func(ctx context.Context) error {
		return errors.New("wait for 3 second")
	}, &testing.PollOptions{Interval: time.Second, Timeout: time.Second * 3})

	// Check that user was logged in successfully.
	homeParam := ui.FindParams{Name: "Home", Role: ui.RoleTypeLink}
	if _, err := ui.FindWithTimeout(ctx, tconn, homeParam, timeout); err != nil {
		return errors.Wrap(err, "failed to sign in")
	}

	if err := testing.Poll(ctx, func(ctx context.Context) error {
		expr := `(() => {
			document.querySelector("#appMountPoint > div > div > div:nth-child(1) > div.bd.dark-background > div.pinning-header > div > div > div > div:nth-child(4) > div > div > a").click()
		})()`
		if err := conn.Eval(ctx, expr, nil); err != nil {
			testing.ContextLog(ctx, "Can't find account button to switch profiles")
			return errors.Wrap(err, "failed to execute Javascript to find account button to switch profiles")
		}
		return nil
	}, &testing.PollOptions{Timeout: timeout}); err != nil {
		return errors.Wrap(err, "failed to find account button to switch profiles")
	}

	_, profileNode, err := switchProfile(ctx, tconn, conn, playbackSettings)
	if err != nil {
		testing.ContextLogf(ctx, "You already in %s mode", playbackSettings)
		return nil
	}
	testing.ContextLogf(ctx, "Switch profile to %s", profileNode.Name)

	testing.Poll(ctx, func(ctx context.Context) error {
		return errors.New("wait for 3 second")
	}, &testing.PollOptions{Interval: time.Second, Timeout: time.Second * 3})

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

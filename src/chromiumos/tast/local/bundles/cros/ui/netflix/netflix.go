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

// Netflix struct
type Netflix struct {
	conn *chrome.Conn
}

const (
	signInURL  = "https://www.netflix.com/login"
	signOutURL = "https://www.netflix.com/SignOut?Inkctr=mL"
)

// New initial netflix instance and signin netflix.
func New(ctx context.Context, s *testing.State, tconn *chrome.TestConn, cr *chrome.Chrome) (*Netflix, error) {
	conn, err := cr.NewConn(ctx, signInURL)
	if err != nil {
		return &Netflix{}, errors.Wrap(err, "failed to open Netflix")
	}
	if err := signIn(ctx, s, tconn, conn); err != nil {
		return &Netflix{}, errors.Wrap(err, "failed to sign in Netflix")
	}
	return &Netflix{conn}, nil
}

func signIn(ctx context.Context, s *testing.State, tconn *chrome.TestConn, conn *chrome.Conn) error {
	const loginFormClass = "login-form"

	kb, err := input.Keyboard(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to open the keyboard")
	}
	defer kb.Close()

	if err := webutil.WaitForQuiescence(ctx, conn, time.Second*30); err != nil {
		return errors.Wrap(err, "failed to wait for netflix login page to finish loading")
	}

	if _, err := ui.FindWithTimeout(ctx, tconn,
		ui.FindParams{ClassName: loginFormClass}, time.Second*5); err != nil {
		testing.ContextLog(ctx, "User has signed in")
		return nil
	}

	emailid := s.RequiredVar("ui.netflix_emailid")
	password := s.RequiredVar("ui.netflix_password")

	if err := testing.Sleep(ctx, time.Second); err != nil {
		return errors.Wrap(err, "failed to wait")
	}

	if err := kb.Accel(ctx, "tab"); err != nil {
		return errors.Wrap(err, "failed to press tab")
	}

	if err := testing.Sleep(ctx, time.Second); err != nil {
		return errors.Wrap(err, "failed to wait")
	}

	if err := kb.Accel(ctx, "tab"); err != nil {
		return errors.Wrap(err, "failed to press tab")
	}

	if err := testing.Sleep(ctx, time.Second*2); err != nil {
		return errors.Wrap(err, "failed to wait")
	}

	if err := kb.Type(ctx, emailid); err != nil {
		return errors.Wrap(err, "failed to type email")
	}

	if err := testing.Sleep(ctx, time.Second); err != nil {
		return errors.Wrap(err, "failed to wait")
	}

	if err := kb.Accel(ctx, "tab"); err != nil {
		return errors.Wrap(err, "failed to press tab")
	}

	if err := testing.Sleep(ctx, time.Second*2); err != nil {
		return errors.Wrap(err, "failed to wait")
	}

	if err := kb.Type(ctx, password); err != nil {
		return errors.Wrap(err, "failed to type password")
	}

	if err := testing.Sleep(ctx, time.Second); err != nil {
		return errors.Wrap(err, "failed to wait")
	}

	if err := kb.Accel(ctx, "Enter"); err != nil {
		return errors.Wrap(err, "failed to press enter")
	}

	if err := testing.Sleep(ctx, time.Second); err != nil {
		return errors.Wrap(err, "failed to wait")
	}

	cuj.WaitAndClick(ctx, tconn, ui.FindParams{ClassName: "profile-icon"}, time.Second*10)

	if err := webutil.WaitForQuiescence(ctx, conn, time.Second*30); err != nil {
		return errors.Wrap(err, "failed to wait for netflix video to finish loading")
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
func (n *Netflix) SignOut(ctx context.Context) error {
	if err := n.conn.Navigate(ctx, signOutURL); err != nil {
		return errors.Wrap(err, "failed to navigate to the url")
	}
	if err := webutil.WaitForQuiescence(ctx, n.conn, time.Second*30); err != nil {
		return errors.Wrap(err, "failed to wait for netflix logout page to finish loading")
	}
	n.conn.Close()
	return nil
}

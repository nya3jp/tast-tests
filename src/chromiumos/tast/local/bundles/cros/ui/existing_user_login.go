// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ui"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/upstart"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: ExistingUserLogin,
		Desc: "Checks that an existing device user can login from the login screen",
		Contacts: []string{
			"kyleshima@chromium.org",
			"bhansknecht@chromium.org",
			"dhaddock@chromium.org",
		},
		SoftwareDeps: []string{"chrome", "chrome_internal"},
		Attr:         []string{"group:mainline", "informational"},
		Vars: []string{
			"ui.signinProfileTestExtensionManifestKey",
			"ui.gaia_username",
			"ui.gaia_password",
		},
	})
}

// ExistingUserLogin logs in to an existing user account from the login screen.
func ExistingUserLogin(ctx context.Context, s *testing.State) {
	username := s.RequiredVar("ui.gaia_username")
	password := s.RequiredVar("ui.gaia_password")

	// Log in and log out to create a user pod on the login screen.
	func() {
		cr, err := chrome.New(ctx, chrome.Auth(username, password, ""), chrome.GAIALogin())
		if err != nil {
			s.Fatal("Chrome login failed: ", err)
		}
		defer cr.Close(ctx)

		if err := upstart.RestartJob(ctx, "ui"); err != nil {
			s.Fatal("Failed to restart ui: ", err)
		}
	}()

	// chrome.NoLogin() and chrome.KeepState() are needed to show the login
	// screen with a user pod (instead of the OOBE login screen).
	cr, err := chrome.New(
		ctx,
		chrome.NoLogin(),
		chrome.KeepState(),
		chrome.LoadSigninProfileExtension(s.RequiredVar("ui.signinProfileTestExtensionManifestKey")),
	)
	if err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}
	defer cr.Close(ctx)

	tLoginConn, err := cr.SigninProfileTestAPIConn(ctx)
	if err != nil {
		s.Fatal("Creating login test API connection failed: ", err)
	}
	defer tLoginConn.Close()

	// lockState contains a subset of the state returned by chrome.autotestPrivate.loginStatus.
	type lockState struct {
		Ready    bool `json:"isReadyForPassword"`
		LoggedIn bool `json:"isLoggedIn"`
	}

	var st lockState
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		if err := tLoginConn.EvalPromise(ctx, `tast.promisify(chrome.autotestPrivate.loginStatus)()`, &st); err != nil {
			return err
		} else if !st.Ready {
			return errors.Errorf("wrong status: %v", st)
		}
		return nil
	}, nil); err != nil {
		s.Fatal("Failed waiting for the login screen to be ready for password entry: ", err)
	}

	kb, err := input.VirtualKeyboard(ctx)
	if err != nil {
		s.Fatal("Failed creating virtual keyboard: ", err)
	}
	defer kb.Close()

	s.Log("Entering password to log in")
	if err := kb.Type(ctx, password+"\n"); err != nil {
		s.Fatal("Entering password failed: ", err)
	}

	// Check the login was successful using the API and also by looking for the shelf in the UI
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		if err := tLoginConn.EvalPromise(ctx, `tast.promisify(chrome.autotestPrivate.loginStatus)()`, &st); err != nil {
			return err
		} else if !st.LoggedIn {
			return errors.Errorf("wrong status: %v", st)
		}
		return nil
	}, nil); err != nil {
		s.Fatal("Failed waiting for login status to change: ", err)
	}

	params := ui.FindParams{Role: ui.RoleTypeToolbar, ClassName: "ShelfView"}
	if err := ui.WaitUntilExists(ctx, tLoginConn, params, 30*time.Second); err != nil {
		s.Fatal("Shelf did not appear after logging in: ", err)
	}
}

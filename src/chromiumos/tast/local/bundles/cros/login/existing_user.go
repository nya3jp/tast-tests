// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package login

import (
	"context"
	"time"

	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/ui/lockscreen"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/upstart"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: ExistingUser,
		Desc: "Checks that an existing device user can login from the login screen",
		Contacts: []string{
			"chromeos-sw-engprod@google.com",
			"cros-oac@google.com",
		},
		SoftwareDeps: []string{"chrome", "chrome_internal"},
		Attr:         []string{"group:mainline"},
		VarDeps: []string{
			"ui.signinProfileTestExtensionManifestKey",
			"ui.gaiaPoolDefault",
		},
		Timeout: 2*chrome.GAIALoginTimeout + time.Minute,
	})
}

// ExistingUser logs in to an existing user account from the login screen.
func ExistingUser(ctx context.Context, s *testing.State) {
	var creds chrome.Creds

	// Log in and log out to create a user pod on the login screen.
	func() {
		cr, err := chrome.New(ctx, chrome.GAIALoginPool(s.RequiredVar("ui.gaiaPoolDefault")))
		if err != nil {
			s.Fatal("Chrome login failed: ", err)
		}
		defer cr.Close(ctx)
		creds = cr.Creds()

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
	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tLoginConn)
	defer tLoginConn.Close()

	// Wait for the login screen to be ready for password entry.
	if st, err := lockscreen.WaitState(ctx, tLoginConn, func(st lockscreen.State) bool { return st.ReadyForPassword }, 30*time.Second); err != nil {
		s.Fatalf("Failed waiting for the login screen to be ready for password entry: %v, last state: %+v", err, st)
	}

	// TODO(crbug/1109381): the password field isn't actually ready just yet when WaitState returns.
	// This causes it to miss some of the keyboard input, so the password will be wrong.
	// We can check in the UI for the password field to exist, which seems to be a good enough indicator that
	// the field is ready for keyboard input.
	if err := lockscreen.WaitForPasswordField(ctx, tLoginConn, creds.User, 5*time.Second); err != nil {
		s.Fatal("Password text field did not appear in the UI: ", err)
	}

	kb, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to get keyboard: ", err)
	}
	defer kb.Close()

	s.Log("Entering password to log in")
	if err := kb.Type(ctx, creds.Pass+"\n"); err != nil {
		s.Fatal("Entering password failed: ", err)
	}

	// Check if the login was successful using the API and also by looking for the shelf in the UI.
	if st, err := lockscreen.WaitState(ctx, tLoginConn, func(st lockscreen.State) bool { return st.LoggedIn }, 30*time.Second); err != nil {
		s.Fatalf("Failed waiting to log in: %v, last state: %+v", err, st)
	}

	if err := ash.WaitForShelf(ctx, tLoginConn, 30*time.Second); err != nil {
		s.Fatal("Shelf did not appear after logging in: ", err)
	}
}

// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/lockscreen"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/quicksettings"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/session"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: SignOutAll,
		Desc: "Verify that the sign in page shows after signing out multi-users by clicking button from uber tray",
		Contacts: []string{
			"kyle.chen@cienet.com",
			"cienet-development@googlegroups.com",
			"chromeos-sw-engprod@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Vars:         []string{"ui.signinProfileTestExtensionManifestKey"},
		Timeout:      5 * time.Minute,
	})
}

// SignOutAll verifies whether the login page is displayed after logging out of multiple users.
func SignOutAll(ctx context.Context, s *testing.State) {
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()

	cr, creds, err := createUsers(ctx)
	if err != nil {
		s.Fatal("Failed to create users for test: ", err)
	}
	defer cr.Close(cleanupCtx)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to get Test API connection: ", err)
	}

	defer faillog.DumpUITreeWithScreenshotOnError(cleanupCtx, s.OutDir(), s.HasError, cr, "sign_in_all_ui_dump")

	if err := signinAll(ctx, tconn, creds); err != nil {
		s.Fatal("Failed to sign in all user: ", err)
	}

	if err := quicksettings.Show(ctx, tconn); err != nil {
		s.Fatal("Failed to click Uber tray: ", err)
	}

	sm, err := session.NewSessionManager(ctx)
	if err != nil {
		s.Fatal("Failed to connect to session manager: ", err)
	}

	const state = "stopped"
	sw, err := sm.WatchSessionStateChanged(ctx, state)
	if err != nil {
		s.Fatal("Failed to watch for D-Bus signals: ", err)
	}
	defer sw.Close(ctx)

	// We ignore errors here because when we click on "Sign out" button
	// Chrome shuts down and the connection is closed.
	// So we always get an error.
	uiauto.New(tconn).LeftClick(nodewith.Name("Sign out all").HasClass("PillButton"))(ctx)

	s.Logf("Waiting for SessionStateChanged %q D-Bus signal from session_manager", state)
	select {
	case <-sw.Signals:
		s.Log("Got SessionStateChanged signal")
	case <-ctx.Done():
		s.Fatal("Didn't get SessionStateChanged signal: ", ctx.Err())
	}

	if cr, err = chrome.New(ctx,
		chrome.ExtraArgs("--skip-force-online-signin-for-testing"),
		chrome.NoLogin(),
		chrome.KeepState(),
		chrome.LoadSigninProfileExtension(s.RequiredVar("ui.signinProfileTestExtensionManifestKey")),
	); err != nil {
		s.Fatal("Chrome login failed: ", err)
	}
	defer cr.Close(cleanupCtx)

	if tconn, err = cr.SigninProfileTestAPIConn(ctx); err != nil {
		s.Fatal("Failed to re-establish test API connection: ", err)
	}
	defer faillog.DumpUITreeOnErrorToFile(cleanupCtx, s.OutDir(), s.HasError, tconn, "dump_after_signOut.txt")

	if err := verifySignout(ctx, tconn, creds); err != nil {
		s.Fatal("Failed to verify user signed out: ", err)
	}

	// First one will be owner after signing out all, and owner can't be removed.
	// Remove all users except the first one.
	creds = creds[1:]
	if err := removeUsers(ctx, tconn, creds); err != nil {
		s.Fatal("Failed to remove all user except user1 in signin page: ", err)
	}
}

func createUsers(ctx context.Context) (cr *chrome.Chrome, creds []chrome.Creds, err error) {
	creds = []chrome.Creds{
		{User: "testuser1@gmail.com", Pass: "test pass 1"},
		{User: "testuser2@gmail.com", Pass: "test pass 2"},
		{User: "testuser3@gmail.com", Pass: "test pass 3"},
		{User: "testuser4@gmail.com", Pass: "test pass 4"},
		{User: "testuser5@gmail.com", Pass: "test pass 5"},
	}

	for i, cred := range creds {
		// Need the last session to do multilpe sign-in.
		var remainLogin = i == len(creds)-1

		opts := []chrome.Option{chrome.FakeLogin(cred)}
		if i != 0 {
			opts = append(opts, chrome.KeepState())
		}
		if remainLogin {
			opts = append(opts, chrome.ExtraArgs("--skip-force-online-signin-for-testing"))
		}

		err = func() error {
			if cr, err = chrome.New(ctx, opts...); err != nil {
				return errors.Wrap(err, "failed to login to Chrome")
			}
			if remainLogin {
				return nil
			}
			return cr.Close(ctx)
		}()
		if err != nil {
			return nil, creds, err
		}
	}

	return cr, creds, nil
}

// signinAll adds all accounts to multiple sign-in with quick settings.
func signinAll(ctx context.Context, tconn *chrome.TestConn, creds []chrome.Creds) error {
	var (
		lastSignedIn  = creds[len(creds)-1]  // The last signed-in user.
		aboutToSignIn = creds[:len(creds)-1] // The users haven't signed-in.

		ui            = uiauto.New(tconn)
		signInAnother = nodewith.NameStartingWith("Sign in another user").HasClass("Button")
		multiSignIn   = nodewith.Name("Multiple sign-in").HasClass("Label")
	)

	kb, err := input.Keyboard(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to create the keyboard")
	}
	defer kb.Close()

	// Proceed to do sign-in only on the users haven't signed-in.
	for _, cred := range aboutToSignIn {
		if err := quicksettings.Show(ctx, tconn); err != nil {
			return errors.Wrap(err, "failed to show quick settings")
		}

		// The current session must be the last signed-in user.
		userIcon := nodewith.NameContaining(lastSignedIn.User).Ancestor(nodewith.HasClass("TopShortcutButtonContainer"))
		if err := uiauto.Combine("sign in another user",
			ui.LeftClick(userIcon),
			ui.LeftClick(signInAnother),
			ui.IfSuccessThen(ui.WithTimeout(5*time.Second).WaitUntilExists(multiSignIn), kb.AccelAction("Enter")),
		)(ctx); err != nil {
			return err
		}

		if _, err := lockscreen.WaitState(ctx, tconn, func(st lockscreen.State) bool { return st.ReadyForPassword }, 10*time.Second); err != nil {
			return errors.Wrap(err, "failed to wait for login screen")
		}

		if err := ui.LeftClick(nodewith.NameContaining(cred.User).HasClass("Button"))(ctx); err != nil {
			return errors.Wrap(err, "failed to select the account")
		}

		pwdInputField := nodewith.Name("Password for " + cred.User).HasClass("Textfield")
		if err := uiauto.Combine("input password and sign in",
			ui.WithTimeout(30*time.Second).LeftClickUntil(pwdInputField, ui.WithTimeout(5*time.Second).WaitUntilExists(pwdInputField.Focused())),
			kb.TypeAction(cred.Pass),
			kb.AccelAction("Enter"),
			ui.WaitUntilGone(pwdInputField),
		)(ctx); err != nil {
			return err
		}

		// Update the last signed-in user.
		lastSignedIn = cred
	}

	return nil
}

func verifySignout(ctx context.Context, tconn *chrome.TestConn, creds []chrome.Creds) error {
	ui := uiauto.New(tconn)

	for _, user := range creds {
		if err := uiauto.Combine("check the password field existed",
			ui.LeftClick(nodewith.NameContaining(user.User).HasClass("Button")),
			ui.WaitUntilExists(nodewith.Name("Password for "+user.User).HasClass("Textfield")),
		)(ctx); err != nil {
			return err
		}
	}

	return nil
}

func removeUsers(ctx context.Context, tconn *chrome.TestConn, creds []chrome.Creds) error {
	ui := uiauto.New(tconn)

	for _, user := range creds {
		if err := uiauto.Combine("remove user pods from start screen",
			ui.LeftClick(nodewith.Name(user.User).HasClass("Button")),
			ui.WaitUntilExists(nodewith.Name(user.User).Role(role.Button)),
			ui.LeftClick(nodewith.Name("Open remove dialog for "+user.User).Role(role.Button)),
			ui.LeftClick(nodewith.Name("Remove account").Role(role.Button)),
			ui.LeftClick(nodewith.Name("Remove account").Role(role.Button)),
			ui.WaitUntilGone(nodewith.Name(user.User).Role(role.Button)),
		)(ctx); err != nil {
			return err
		}
	}

	return nil
}

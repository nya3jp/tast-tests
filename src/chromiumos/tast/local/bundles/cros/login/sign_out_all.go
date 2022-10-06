// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package login

import (
	"context"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/login/userutil"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/lockscreen"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/quicksettings"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/session"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         SignOutAll,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Verify that the sign in page shows after signing out multi-users by clicking button from uber tray",
		Contacts: []string{
			"vivian.tsai@cienet.com",
			"kyle.chen@cienet.com",
			"cienet-development@googlegroups.com",
			"chromeos-sw-engprod@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Vars:         []string{"ui.signinProfileTestExtensionManifestKey"},
		Timeout:      3*chrome.LoginTimeout + 3*time.Minute,
		Params: []testing.Param{{
			Name: "auth_factor_experiment_on",
			Val:  []chrome.Option{chrome.EnableFeatures("UseAuthFactors")},
		}, {
			Name: "auth_factor_experiment_off",
			Val:  []chrome.Option{chrome.DisableFeatures("UseAuthFactors")},
		}},
	})
}

// SignOutAll verifies whether the login page is displayed after logging out of multiple users.
func SignOutAll(ctx context.Context, s *testing.State) {
	creds := []chrome.Creds{
		{User: "testuser1@gmail.com", Pass: "test pass 1"},
		{User: "testuser2@gmail.com", Pass: "test pass 2"},
		{User: "testuser3@gmail.com", Pass: "test pass 3"},
	}

	if err := userutil.CreateUser(ctx, creds[0].User, creds[0].Pass); err != nil {
		s.Fatal("Failed to create new user 1: ", err)
	}
	if err := userutil.CreateUser(ctx, creds[1].User, creds[1].Pass, chrome.KeepState()); err != nil {
		s.Fatal("Failed to create new user 2: ", err)
	}
	if err := userutil.CreateUser(ctx, creds[2].User, creds[2].Pass, chrome.KeepState()); err != nil {
		s.Fatal("Failed to create new user 3: ", err)
	}

	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()

	testParamOpts := s.Param().([]chrome.Option)
	opts := append(testParamOpts,
		chrome.ExtraArgs("--skip-force-online-signin-for-testing"),
		chrome.FakeLogin(creds[len(creds)-1]),
		chrome.KeepState(),
	)

	cr, err := chrome.New(ctx, opts...)
	if err != nil {
		s.Fatal("Failed to login last user: ", err)
	}
	defer cr.Close(cleanupCtx)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to get Test API connection: ", err)
	}

	currentUser := creds[len(creds)-1].User
	if err := signInAll(ctx, tconn, creds[:len(creds)-1], currentUser); err != nil {
		s.Fatal("Failed to sign in all user: ", err)
	}

	if err := signOutAllAndWait(ctx, tconn); err != nil {
		s.Fatal("Failed to sign out all user: ", err)
	}

	if cr, err = chrome.New(ctx,
		chrome.ExtraArgs("--skip-force-online-signin-for-testing"),
		chrome.NoLogin(),
		chrome.KeepState(),
		chrome.LoadSigninProfileExtension(s.RequiredVar("ui.signinProfileTestExtensionManifestKey")),
	); err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}
	defer cr.Close(cleanupCtx)

	if tconn, err = cr.SigninProfileTestAPIConn(ctx); err != nil {
		s.Fatal("Failed to re-establish test API connection: ", err)
	}
	defer faillog.DumpUITreeOnErrorToFile(cleanupCtx, s.OutDir(), s.HasError, tconn, "dump_after_signOut.txt")

	if err := signedOut(ctx, tconn, creds); err != nil {
		s.Fatal("Failed to verify user signed out: ", err)
	}
}

// signInAll executes multiple sign-in via quick settings.
// aboutToSignIn are users who haven't signed-in.
// currentUser is the last signed-in user.
func signInAll(ctx context.Context, tconn *chrome.TestConn, aboutToSignIn []chrome.Creds, currentUser string) error {
	var (
		ui            = uiauto.New(tconn)
		signInAnother = nodewith.NameStartingWith("Sign in another user").HasClass("Button")
		multiSignIn   = nodewith.Name("Multiple sign-in").HasClass("Label")
	)

	kb, err := input.Keyboard(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to create the keyboard")
	}
	defer kb.Close()

	// Proceed to do sign-in only on the users who haven't signed-in.
	for _, cred := range aboutToSignIn {
		if err := quicksettings.Show(ctx, tconn); err != nil {
			return errors.Wrap(err, "failed to show quick settings")
		}

		// The current session must be the last signed-in user.
		userIcon := nodewith.NameContaining(currentUser).Ancestor(nodewith.HasClass("TopShortcutButtonContainer"))
		if err := uiauto.Combine("sign in another user",
			ui.LeftClick(userIcon),
			ui.LeftClick(signInAnother),
			// A prompt of "Sign in another user" will show when signing in the first non-owner user and won't pop-up again after dismissing it.
			uiauto.IfSuccessThen(ui.WithTimeout(5*time.Second).WaitUntilExists(multiSignIn), kb.AccelAction("Enter")),
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
		currentUser = cred.User
	}

	return nil
}

// signOutAllAndWait sign out all user and wait for session stopped.
func signOutAllAndWait(ctx context.Context, tconn *chrome.TestConn) error {
	if err := quicksettings.Show(ctx, tconn); err != nil {
		return errors.Wrap(err, "failed to click Uber tray")
	}

	sm, err := session.NewSessionManager(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to connect to session manager")
	}

	const state = "stopped"
	sw, err := sm.WatchSessionStateChanged(ctx, state)
	if err != nil {
		return errors.Wrap(err, "failed to watch for D-Bus signals")
	}
	defer sw.Close(ctx)

	if err := uiauto.New(tconn).MouseMoveTo(nodewith.Name("Sign out all").HasClass("PillButton"), 0)(ctx); err != nil {
		return errors.Wrap(err, "failed to move to the location of sign out button")
	}

	// TestConn will be closed once we logged out, which causes connection error if we click the button by `uiauto.Context`,
	// therefore, we avoid using `tconn` here and use input.Mouse instead.
	mouse, err := input.Mouse(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to create mouse")
	}
	defer mouse.Close()

	if err := mouse.Click(); err != nil {
		return errors.Wrap(err, "failed to click sign out button")
	}

	testing.ContextLogf(ctx, "Waiting for SessionStateChanged %q D-Bus signal from session_manager", state)
	select {
	case <-sw.Signals:
		testing.ContextLog(ctx, "Got SessionStateChanged signal")
	case <-ctx.Done():
		return errors.Wrap(ctx.Err(), "didn't get SessionStateChanged signal")
	}
	return nil
}

// signedOut verifies all users are signed out.
func signedOut(ctx context.Context, tconn *chrome.TestConn, creds []chrome.Creds) error {
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

// Copyright 2021 The Chromium OS Authors. All rights reserved.
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
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/session"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         Logout,
		Desc:         "Verify that the sign in page shows after signing out",
		Contacts:     []string{"viviantsai@cienet.com", "cienet-development@googlegroups.com", "chromeos-sw-engprod@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		VarDeps:      []string{"ui.signinProfileTestExtensionManifestKey"},
	})
}

// Logout logs out and verifies if the sign in screen is being displayed.
func Logout(ctx context.Context, s *testing.State) {
	tests := []struct {
		description string
		logout      func(ctx context.Context, tconn *chrome.TestConn) error
	}{
		{
			description: "test logout by pressing Shift+Ctrl+Q twice",
			logout:      logoutByShortcut,
		}, {
			description: "test logout by clicking Signout button",
			logout:      logoutByClickOnButton,
		},
	}

	for _, test := range tests {
		f := func(ctx context.Context, s *testing.State) {
			cleanupCtx := ctx
			ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
			defer cancel()

			cr, err := chrome.New(ctx)
			if err != nil {
				s.Fatal("Failed to start Chrome: ", err)
			}
			defer cr.Close(cleanupCtx)

			tconn, err := cr.TestAPIConn(ctx)
			if err != nil {
				s.Fatal("Failed to get test API connection")
			}

			defer faillog.DumpUITreeWithScreenshotOnError(cleanupCtx, s.OutDir(), s.HasError, cr, "logout")

			if err := test.logout(ctx, tconn); err != nil {
				s.Fatal("Failed to logout: ", err)
			}

			if cr, err = chrome.New(ctx,
				chrome.NoLogin(),
				chrome.KeepState(),
				chrome.LoadSigninProfileExtension(s.RequiredVar("ui.signinProfileTestExtensionManifestKey")),
			); err != nil {
				s.Fatal("Failed to restart Chrome for testing: ", err)
			}
			defer cr.Close(cleanupCtx)

			if tconn, err = cr.SigninProfileTestAPIConn(ctx); err != nil {
				s.Fatal("Failed to re-establish test API connection")
			}

			siginIn := nodewith.Name("Sign in").Role(role.Button)
			if err := uiauto.New(tconn).WaitUntilExists(siginIn)(ctx); err != nil {
				s.Fatal("Failed to verify sign-in screen displayed properly after logout: ", err)
			}
		}

		if !s.Run(ctx, test.description, f) {
			s.Errorf("Failed to %s", test.description)
		}
	}
}

func logoutByShortcut(ctx context.Context, tconn *chrome.TestConn) error {
	kb, err := input.Keyboard(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to create the keyboard")
	}
	defer kb.Close()

	if err := uiauto.Combine("press the keyboard to logout",
		kb.AccelAction("Shift+Ctrl+Q"),
		kb.AccelAction("Shift+Ctrl+Q"),
	)(ctx); err != nil {
		return err
	}
	return waitForLogout(ctx)
}

func logoutByClickOnButton(ctx context.Context, tconn *chrome.TestConn) error {
	ui := uiauto.New(tconn)

	uberTray := nodewith.HasClass("UnifiedSystemTray").Role(role.Button)
	if err := ui.LeftClick(uberTray)(ctx); err != nil {
		return errors.Wrap(err, "failed to click Uber tray")
	}

	signOutButton := nodewith.Name("Sign out").Role(role.Button)
	buttonFound, err := ui.IsNodeFound(ctx, signOutButton)
	if err != nil {
		return errors.Wrap(err, "failed to find the sign out button")
	}
	if !buttonFound {
		return errors.New("signout button was not found")
	}

	// We ignore errors here because when we click on "Sign out" button
	// Chrome shuts down and the connection is closed. So we will always get an
	// error.
	ui.LeftClick(signOutButton)(ctx)
	return waitForLogout(ctx)
}

func waitForLogout(ctx context.Context) error {
	sm, err := session.NewSessionManager(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to connect to session_manager via D-Bus and return a SessionManager object")
	}
	sw, err := sm.WatchSessionStateChanged(ctx, "stopped")
	if err != nil {
		return errors.Wrap(err, "failed to watch for D-Bus signals")
	}
	defer sw.Close(ctx)

	testing.ContextLog(ctx, "Waiting for SessionStateChanged \"stopped\" D-Bus signal from session_manager")
	select {
	case <-sw.Signals:
		testing.ContextLog(ctx, "Got SessionStateChanged signal")
	case <-ctx.Done():
		return errors.Wrap(ctx.Err(), "didn't get SessionStateChanged signal")
	}

	testing.ContextLog(ctx, "Sign out: done")
	return nil
}

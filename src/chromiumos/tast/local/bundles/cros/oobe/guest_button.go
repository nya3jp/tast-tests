// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package oobe

import (
	"context"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/session"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         GuestButton,
		LacrosStatus: testing.LacrosVariantUnknown,
		Desc:         "Test the guest button in the sign-in screen",
		Contacts: []string{
			"osamafathy@google.com",
			"chromeos-sw-engprod@google.com",
			"cros-oac@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		VarDeps:      []string{"ui.signinProfileTestExtensionManifestKey"},
	})
}

func GuestButton(ctx context.Context, s *testing.State) {
	cr, err := chrome.New(ctx,
		chrome.NoLogin(),
		chrome.LoadSigninProfileExtension(s.RequiredVar("ui.signinProfileTestExtensionManifestKey")))
	if err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(cleanupCtx, time.Second*10)
	defer cancel()
	defer cr.Close(cleanupCtx)

	oobeConn, err := cr.WaitForOOBEConnection(ctx)
	if err != nil {
		s.Fatal("Failed to create OOBE connection: ", err)
	}
	defer oobeConn.Close()

	tconn, err := cr.SigninProfileTestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create the signin profile test API connection: ", err)
	}
	defer faillog.DumpUITreeOnError(cleanupCtx, s.OutDir(), s.HasError, tconn)

	ui := uiauto.New(tconn).WithTimeout(10 * time.Second)

	// Skip to sign-in screen to get the guest button shown in the shelf.
	if err := oobeConn.Eval(ctx, "Oobe.skipToLoginForTesting()", nil); err != nil {
		s.Fatal("Failed to skip to login: ", err)
	}

	s.Log("Waiting for Sigin-in screen")
	if err := oobeConn.WaitForExprWithTimeout(ctx, "OobeAPI.screens.GaiaScreen.isReadyForTesting()", 60*time.Second); err != nil {
		s.Fatal("Failed to wait for sign-in to be ready: ", err)
	}

	// Watch the session to check that the session stops when the guest button is clicked.
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

	shouldSkipGuestTosScreen := false
	if err := oobeConn.Eval(ctx, "OobeAPI.screens.GuestTosScreen.shouldSkip()", &shouldSkipGuestTosScreen); err != nil {
		s.Fatal("Failed to evaluate whether to skip guest ToS screen: ", err)
	}

	guestButton := nodewith.Name("Browse as Guest").Role(role.Button).HasClass("LoginShelfButton")
	if err := ui.WaitUntilExists(guestButton)(ctx); err != nil {
		s.Error("Guest mode button did not appear: ", err)
	}
	if err := ui.LeftClick(guestButton)(ctx); err != nil {
		s.Error("Guest mode button was not clicked: ", err)
	}

	if shouldSkipGuestTosScreen {
		s.Log("Skipping guest ToS screen")
	} else {
		s.Log("Waiting for guest ToS screen")

		if err := oobeConn.WaitForExprWithTimeout(ctx, "OobeAPI.screens.GuestTosScreen.isReadyForTesting()", 60*time.Second); err != nil {
			s.Fatal("Failed to wait for the geust ToS screen to be visible: ", err)
		}
		var guestTosNextButtonText string
		if err := oobeConn.Eval(ctx, "OobeAPI.screens.GuestTosScreen.getNextButtonName()", &guestTosNextButtonText); err != nil {
			s.Fatal("Failed to get guest ToS next button name: ", err)
		}

		guestTosNextButton := nodewith.Role(role.Button).Name(guestTosNextButtonText)

		if err := uiauto.Combine("Click accept on guest ToS screen",
			ui.WaitUntilExists(guestTosNextButton),
			ui.LeftClick(guestTosNextButton),
		)(ctx); err != nil {
			s.Fatal("Failed to click guest ToS screen accept button: ", err)
		}
	}

	s.Logf("Waiting for SessionStateChanged %q D-Bus signal from session_manager", state)
	select {
	case <-sw.Signals:
		s.Log("Got SessionStateChanged signal")
	case <-ctx.Done():
		s.Fatal("Didn't get SessionStateChanged signal: ", ctx.Err())
	}
}

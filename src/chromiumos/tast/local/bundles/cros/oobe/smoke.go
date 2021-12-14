// Copyright 2021 The Chromium OS Authors. All rights reserved.
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
	"chromiumos/tast/local/chrome/uiauto/state"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         Smoke,
		LacrosStatus: testing.LacrosVariantUnknown,
		Desc:         "Smoke test that clicks through OOBE",
		Contacts: []string{
			"chromeos-sw-engprod@google.com",
			"cros-oac@google.com",
		},
		Attr:         []string{"group:mainline"},
		SoftwareDeps: []string{"chrome"},
		VarDeps:      []string{"ui.signinProfileTestExtensionManifestKey"},
	})
}

func Smoke(ctx context.Context, s *testing.State) {
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

	if err := oobeConn.WaitForExprFailOnErr(ctx, "OobeAPI.screens.WelcomeScreen.isVisible()"); err != nil {
		s.Fatal("Failed to wait for the welcome screen to be visible: ", err)
	}

	focusedButton := nodewith.State(state.Focused, true).Role(role.Button)
	if err := uiauto.Combine("Click next on the welcome screen",
		ui.WaitUntilExists(focusedButton),
		ui.LeftClick(focusedButton),
	)(ctx); err != nil {
		s.Fatal("Failed to click welcome screen next button: ", err)
	}

	shouldSkipNetworkScreen := false
	if err := oobeConn.Eval(ctx, "OobeAPI.screens.NetworkScreen.shouldSkip()", &shouldSkipNetworkScreen); err != nil {
		s.Fatal("Failed to evaluate whether to skip Network screen: ", err)
	}

	if !shouldSkipNetworkScreen {
		if err := oobeConn.WaitForExprFailOnErr(ctx, "OobeAPI.screens.NetworkScreen.isVisible()"); err != nil {
			s.Fatal("Failed to wait for the network screen to be visible: ", err)
		}
		if err := ui.LeftClickUntil(focusedButton, ui.Gone(focusedButton))(ctx); err != nil {
			s.Fatal("Failed to click network page next button: ", err)
		}
	}

	shouldSkipEulaScreen := false
	if err := oobeConn.Eval(ctx, "OobeAPI.screens.EulaScreen.shouldSkip()", &shouldSkipEulaScreen); err != nil {
		s.Fatal("Failed to evaluate whether to skip Eula screen: ", err)
	}

	if !shouldSkipEulaScreen {
		if err := oobeConn.WaitForExprFailOnErr(ctx, "OobeAPI.screens.EulaScreen.isVisible()"); err != nil {
			s.Fatal("Failed to wait for the eula screen to be visible: ", err)
		}
		eulaWebview := nodewith.State(state.Focused, true).Role(role.Iframe)
		if err := ui.WaitUntilExists(eulaWebview)(ctx); err != nil {
			s.Fatal("Failed to wait for the eula screen to be visible: ", err)
		}
		var eulaNextButton string
		if err := oobeConn.Eval(ctx, "OobeAPI.screens.EulaScreen.getNextButtonName()", &eulaNextButton); err != nil {
			s.Fatal("Failed to get eula next button name: ", err)
		}
		eulaScreenNextButton := nodewith.Role(role.Button).Name(eulaNextButton)
		if err := uiauto.Combine("Click next on the EULA screen",
			// Button is not focused on the screen. We focus the webview with EULA by default.
			ui.WaitUntilExists(eulaScreenNextButton.State(state.Focused, false)),
			ui.LeftClickUntil(eulaScreenNextButton, ui.Gone(eulaScreenNextButton)),
		)(ctx); err != nil {
			s.Fatal("Failed to click accept eula button: ", err)
		}
	}

	if err := oobeConn.WaitForExprFailOnErr(ctx, "OobeAPI.screens.UserCreationScreen.isVisible()"); err != nil {
		s.Fatal("Failed to wait for the user creation screen to be visible: ", err)
	}

	if err := ui.LeftClickUntil(focusedButton, ui.Gone(focusedButton))(ctx); err != nil {
		s.Fatal("Failed to click user creation screen next button: ", err)
	}

	gaiaInput := nodewith.State(state.Focused, true).Role(role.TextField).Ancestor(nodewith.Role(role.Iframe))
	if err := ui.WaitUntilExists(gaiaInput)(ctx); err != nil {
		s.Fatal("Failed to wait for the login screen to be visible: ", err)
	}
}

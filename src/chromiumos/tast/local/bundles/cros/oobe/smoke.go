// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package oobe

import (
	"context"
	"time"

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
	defer cr.Close(ctx)

	oobeConn, err := cr.WaitForOOBEConnection(ctx)
	if err != nil {
		s.Fatal("Failed to create OOBE connection: ", err)
	}
	defer oobeConn.Close()

	tconn, err := cr.SigninProfileTestAPIConn(ctx)
	if err != nil {
		s.Fatal("Creating login test API connection failed: ", err)
	}
	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)

	ui := uiauto.New(tconn).WithTimeout(10 * time.Second)

	welcomeScreenNextButton := nodewith.Name("Get started").Role(role.Button)
	if err := uiauto.Combine("Click next on the welcome screen",
		ui.WaitUntilExists(welcomeScreenNextButton),
		ui.LeftClick(welcomeScreenNextButton),
	)(ctx); err != nil {
		s.Fatal("Failed to click welcome screen next button: ", err)
	}

	networkScreenNextButton := nodewith.State(state.Focused, true).Role(role.Button).Ancestor(nodewith.Name("Connect to network").First())
	if err := uiauto.Combine("Click next on the network screen",
		ui.WaitUntilExists(networkScreenNextButton),
		ui.LeftClickUntil(networkScreenNextButton, ui.Gone(networkScreenNextButton)),
	)(ctx); err != nil {
		s.Fatal("Failed to click network page next button: ", err)
	}

	shouldSkipEulaScreen := false
	if err := oobeConn.Eval(ctx, "OobeAPI.screens.EulaScreen.shouldSkip()", &shouldSkipEulaScreen); err != nil {
		s.Fatal("Failed to evaluate whether to skip Eula screen: ", err)
	}

	if !shouldSkipEulaScreen {
		eulaWebview := nodewith.State(state.Focused, true).Role(role.Iframe).Ancestor(nodewith.Name("Google terms of service").First())
		if err := ui.WaitUntilExists(eulaWebview)(ctx); err != nil {
			s.Fatal("Failed to wait for the eula screen to be visible: ", err)
		}
		eulaScreenNextButton := nodewith.Role(role.Button).Name("Accept and continue")
		if err := uiauto.Combine("Click next on the EULA screen",
			// Button is not focused on the screen. We focus the webview with EULA by default.
			ui.WaitUntilExists(eulaScreenNextButton.State(state.Focused, false)),
			ui.LeftClickUntil(eulaScreenNextButton, ui.Gone(eulaScreenNextButton)),
		)(ctx); err != nil {
			s.Fatal("Failed to click accept eula button: ", err)
		}
	}

	userCreationScreenNextButton := nodewith.State(state.Focused, true).Role(role.Button).Ancestor(nodewith.NameStartingWith("Who's using").First())
	if err := uiauto.Combine("Click next on the user creation screen",
		// "Checking for updates" might show for some time. So increase the timeout.
		ui.WithTimeout(20*time.Second).WaitUntilExists(userCreationScreenNextButton),
		ui.LeftClickUntil(userCreationScreenNextButton, ui.Gone(userCreationScreenNextButton)),
	)(ctx); err != nil {
		s.Fatal("Failed to click user creation screen next button: ", err)
	}

	gaiaInput := nodewith.NameStartingWith("Email or phone").State(state.Focused, true).Role(role.TextField).Ancestor(nodewith.Role(role.Iframe))
	if err := ui.WaitUntilExists(gaiaInput)(ctx); err != nil {
		s.Fatal("Failed to wait for the login screen to be visible: ", err)
	}
}

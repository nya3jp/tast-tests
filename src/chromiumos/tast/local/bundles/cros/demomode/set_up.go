// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package demomode

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
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/session"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         SetUp,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Basic test that clicks through Demo Mode setup from OOBE",
		Contacts:     []string{"cros-demo-mode-eng@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome", "chrome_internal", "arc", "tpm"},
		VarDeps:      []string{"ui.signinProfileTestExtensionManifestKey"},
	})
}

// SetUp runs through the basic flow of entering Demo Mode from OOBE
//
// TODO(b/231472901): Deduplicate the shared code between Demo Mode and normal
// OOBE Tast tests
func SetUp(ctx context.Context, s *testing.State) {
	cr, err := chrome.New(ctx,
		chrome.NoLogin(),
		chrome.ARCEnabled(),
		chrome.ExtraArgs("--arc-start-mode=always-start"),
		chrome.LoadSigninProfileExtension(s.RequiredVar("ui.signinProfileTestExtensionManifestKey")))
	if err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}
	clearUpCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()
	defer cr.Close(clearUpCtx)

	oobeConn, err := cr.WaitForOOBEConnection(ctx)
	if err != nil {
		s.Fatal("Failed to create OOBE connection: ", err)
	}
	defer oobeConn.Close()

	tconn, err := cr.SigninProfileTestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create the signin profile test API connection: ", err)
	}
	defer faillog.DumpUITreeOnError(clearUpCtx, s.OutDir(), s.HasError, tconn)

	ui := uiauto.New(tconn).WithTimeout(50 * time.Second)

	kb, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to get keyboard handle: ", err)
	}
	defer kb.Close()

	if err := kb.Accel(ctx, "Ctrl+Alt+D"); err != nil {
		s.Fatal("Failed to enter Demo Setup dialogue with ctrl + alt + D: ", err)
	}

	if err := oobeConn.WaitForExprFailOnErr(ctx, "OobeAPI.screens.WelcomeScreen.demoModeConfirmationDialog.isVisible()"); err != nil {
		s.Fatal("Failed to wait for the demo confirmation dialog to be visible: ", err)
	}
	var demoModeOkButtonName string
	if err := oobeConn.Eval(ctx, "OobeAPI.screens.WelcomeScreen.getDemoModeOkButtonName()", &demoModeOkButtonName); err != nil {
		s.Fatal("Failed to get demo mode OK button name: ", err)
	}
	demoModeOkButton := nodewith.Role(role.Button).Name(demoModeOkButtonName)
	if err := uiauto.Combine("Click OK on demo confirmation dialog",
		ui.WaitUntilExists(demoModeOkButton),
		ui.LeftClick(demoModeOkButton),
	)(ctx); err != nil {
		s.Fatal("Failed to click OK button on demo confirmation dialog: ", err)
	}

	s.Log("Proceeding through demo preferences screen")
	if err := oobeConn.WaitForExprFailOnErr(ctx, "OobeAPI.screens.DemoPreferencesScreen.isVisible()"); err != nil {
		s.Fatal("Failed to wait for the demo preferences screen to be visible: ", err)
	}

	var demoPreferencesNextButtonName string
	if err := oobeConn.Eval(ctx, "OobeAPI.screens.DemoPreferencesScreen.getDemoPreferencesNextButtonName()", &demoPreferencesNextButtonName); err != nil {
		s.Fatal("Failed to get demo preferences next button name: ", err)
	}
	demoPreferencesNextButton := nodewith.Role(role.Button).Name(demoPreferencesNextButtonName)
	if err := uiauto.Combine("Click next from demo preferences screen",
		ui.WaitUntilExists(demoPreferencesNextButton),
		ui.LeftClick(demoPreferencesNextButton),
	)(ctx); err != nil {
		s.Fatal("Failed to click next from demo preferences screen: ", err)
	}

	shouldSkipNetworkScreen := false
	if err := oobeConn.Eval(ctx, "OobeAPI.screens.NetworkScreen.shouldSkip()", &shouldSkipNetworkScreen); err != nil {
		s.Fatal("Failed to evaluate whether to skip network screen: ", err)
	}
	if shouldSkipNetworkScreen {
		s.Log("NetworkScreen.shouldSkip() is true; skipped")
	} else {
		s.Log("Proceeding through network screen")
		if err := oobeConn.WaitForExprFailOnErr(ctx, "OobeAPI.screens.NetworkScreen.isVisible()"); err != nil {
			s.Fatal("Failed to wait for the network screen to be visible: ", err)
		}
		//(TODO, https://crbug.com/1291153): Switch to focused button.
		nextButton := nodewith.Name("Next").Role(role.Button)
		if err := ui.LeftClickUntil(nextButton, ui.Gone(nextButton))(ctx); err != nil {
			s.Fatal("Failed to click network page next button: ", err)
		}
	}

	shouldSkipEulaScreen := false
	if err := oobeConn.Eval(ctx, "OobeAPI.screens.EulaScreen.shouldSkip()", &shouldSkipEulaScreen); err != nil {
		s.Fatal("Failed to evaluate whether to skip EULA screen: ", err)
	}
	if shouldSkipEulaScreen {
		s.Log("EulaScreen.shouldSkip() is true; skipped")
	} else {
		s.Log("Proceeding through EULA screen")
		if err := oobeConn.WaitForExprFailOnErr(ctx, "OobeAPI.screens.EulaScreen.isVisible()"); err != nil {
			s.Fatal("Failed to wait for the EULA screen to be visible: ", err)
		}
		eulaWebview := nodewith.State(state.Focused, true).Role(role.Iframe)
		if err := ui.WaitUntilExists(eulaWebview)(ctx); err != nil {
			s.Fatal("Failed to wait for the EULA webview to exist: ", err)
		}
		var eulaScreenNextButtonName string
		if err := oobeConn.Eval(ctx, "OobeAPI.screens.EulaScreen.getNextButtonName()", &eulaScreenNextButtonName); err != nil {
			s.Fatal("Failed to get EULA next button name: ", err)
		}
		eulaScreenNextButton := nodewith.Role(role.Button).Name(eulaScreenNextButtonName)
		if err := uiauto.Combine("Click next on the EULA screen",
			// Button is not focused on the screen. We focus the webview with EULA by default.
			ui.WaitUntilExists(eulaScreenNextButton.State(state.Focused, false)),
			ui.LeftClickUntil(eulaScreenNextButton, ui.Gone(eulaScreenNextButton)),
		)(ctx); err != nil {
			s.Fatal("Failed to click accept EULA button: ", err)
		}
	}

	s.Log("Proceeding through ARC TOS screen")
	if err := oobeConn.WaitForExprFailOnErr(ctx, "OobeAPI.screens.ArcTosScreen.isVisible()"); err != nil {
		s.Fatal("Failed to wait for the ARC TOS screen to be visible: ", err)
	}
	// Can find the first ARC TOS button (more) by state.Focused, but we should retrieve the
	// second (accept) by name to avoid re-retrieving the first while it's still focused
	arcTosMoreButton := nodewith.State(state.Focused, true).Role(role.Button)
	if err := uiauto.Combine("Click more button from ARC TOS screen",
		ui.WaitUntilExists(arcTosMoreButton),
		ui.LeftClick(arcTosMoreButton),
	)(ctx); err != nil {
		s.Fatal("Failed to click more button from ARC TOS screen: ", err)
	}
	// Connect to session manager now for post-setup session login, before clicking
	// final accept button and entering non-interactive part of demo setup
	sm, err := session.NewSessionManager(ctx)
	if err != nil {
		s.Fatal("Failed to connect to session manager: ", err)
	}
	sw, err := sm.WatchSessionStateChanged(ctx, "started")
	if err != nil {
		s.Fatal("Failed to watch for session manager D-Bus signals: ", err)
	}

	// Click ARC TOS accept to finish interactive part of setup
	var arcTosAcceptButtonName string
	if err := oobeConn.Eval(ctx, "OobeAPI.screens.ArcTosScreen.getArcTosDemoModeAcceptButtonName()", &arcTosAcceptButtonName); err != nil {
		s.Fatal("Failed to get ARC TOS accept button name: ", err)
	}
	arcTosAcceptButton := nodewith.Role(role.Button).Name(arcTosAcceptButtonName)
	if err := uiauto.Combine("Click accept button from ARC TOS screen",
		ui.WaitUntilExists(arcTosAcceptButton),
		ui.LeftClick(arcTosAcceptButton),
	)(ctx); err != nil {
		s.Fatal("Failed to click accept button from ARC TOS screen: ", err)
	}

	s.Log("Waiting for SessionStateChanged \"started\" D-Bus signal from session_manager")
	select {
	case <-sw.Signals:
		s.Log("Got SessionStateChanged signal")
	case <-ctx.Done():
		s.Fatal("Didn't get SessionStateChanged signal: ", ctx.Err())
	}
}

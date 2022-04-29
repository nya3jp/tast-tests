// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package demomode

import (
	"context"

	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/session"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         Setup,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Basic test that clicks through Demo Mode setup from OOBE",
		Contacts:     []string{"cros-demo-mode-eng@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome", "chrome_internal", "arc"},
	})
}

func Setup(ctx context.Context, s *testing.State) {
	cr, err := chrome.New(ctx, chrome.NoLogin(), chrome.ARCEnabled(), chrome.ExtraArgs("--arc-start-mode=always-start"))
	if err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}
	defer cr.Close(ctx)

	oobeConn, err := cr.WaitForOOBEConnection(ctx)
	if err != nil {
		s.Fatal("Failed to create OOBE connection: ", err)
	}
	defer oobeConn.Close()

	kb, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to get keyboard handle: ", err)
	}
	defer kb.Close()

	if err := kb.Accel(ctx, "Ctrl+Alt+D"); err != nil {
		s.Fatal("Failed to enter Demo Setup dialogue with ctrl + alt + D")
	}

	if err := oobeConn.WaitForExprFailOnErr(ctx, "OobeAPI.screens.WelcomeScreen.demoModeConfirmationDialog.isVisible()"); err != nil {
		s.Fatal("Failed to wait for the demo confirmation dialog to be visible: ", err)
	}
	if err := oobeConn.Eval(ctx, "OobeAPI.screens.WelcomeScreen.demoModeConfirmationOkButton.click()", nil); err != nil {
		s.Fatal("Failed to click Demo Mode confirmation dialog OK button: ", err)
	}

	s.Log("Proceeding through demo preferences screen")
	if err := oobeConn.WaitForExprFailOnErr(ctx, "OobeAPI.screens.DemoPreferencesScreen.isVisible()"); err != nil {
		s.Fatal("Failed to wait for the demo preferences screen to be visible: ", err)
	}
	if err := oobeConn.Eval(ctx, "OobeAPI.screens.DemoPreferencesScreen.clickNext()", nil); err != nil {
		s.Fatal("Failed to wait for the demo preferences screen to be visible: ", err)
	}

	shouldSkipNetworkScreen := false
	if err := oobeConn.Eval(ctx, "OobeAPI.screens.NetworkScreen.shouldSkip()", &shouldSkipNetworkScreen); err != nil {
		s.Fatal("Failed to evaluate whether to skip Network screen: ", err)
	}
	if !shouldSkipNetworkScreen {
		s.Log("Proceeding through network screen")
		if err := oobeConn.WaitForExprFailOnErr(ctx, "OobeAPI.screens.NetworkScreen.isVisible()"); err != nil {
			s.Fatal("Failed to wait for the network screen to be visible: ", err)
		}
		if err := oobeConn.WaitForExprFailOnErr(ctx, "OobeAPI.screens.NetworkScreen.nextButton.isEnabled()"); err != nil {
			s.Fatal("Failed to wait for the network screen next button to be enabled: ", err)
		}
		if err := oobeConn.Eval(ctx, "OobeAPI.screens.NetworkScreen.clickNext()", nil); err != nil {
			s.Fatal("Failed to click network page next button: ", err)
		}
	} else {
		s.Log("Skipping network screen")
	}

	shouldSkipEulaScreen := false
	if err := oobeConn.Eval(ctx, "OobeAPI.screens.EulaScreen.shouldSkip()", &shouldSkipEulaScreen); err != nil {
		s.Fatal("Failed to evaluate whether to skip Eula screen: ", err)
	}
	if !shouldSkipEulaScreen {
		s.Log("Proceeding through EULA screen")
		if err := oobeConn.WaitForExprFailOnErr(ctx, "OobeAPI.screens.EulaScreen.isVisible()"); err != nil {
			s.Fatal("Failed to wait for the eula screen to be visible: ", err)
		}
		if err := oobeConn.WaitForExprFailOnErr(ctx, "OobeAPI.screens.EulaScreen.nextButton.isEnabled()"); err != nil {
			s.Fatal("Failed to wait for the accept eula button to be enabled: ", err)
		}
		if err := oobeConn.Eval(ctx, "OobeAPI.screens.EulaScreen.clickNext()", nil); err != nil {
			s.Fatal("Failed to click accept eula button: ", err)
		}
	} else {
		s.Log("Skipping EULA screen")
	}

	s.Log("Proceeding through ARC TOS screen")
	if err := oobeConn.WaitForExprFailOnErr(ctx, "OobeAPI.screens.ArcTosScreen.isVisible()"); err != nil {
		s.Fatal("Failed to wait for the ARC TOS screen to be visible: ", err)
	}
	if err := oobeConn.WaitForExprFailOnErr(ctx, "OobeAPI.screens.ArcTosScreen.moreButton.isEnabled()"); err != nil {
		s.Fatal("Failed to wait for the ARC TOS more button to be enabled: ", err)
	}
	if err := oobeConn.Eval(ctx, "OobeAPI.screens.ArcTosScreen.moreButton.click()", nil); err != nil {
		s.Fatal("Failed to click the ARC TOS more button: ", err)
	}
	if err := oobeConn.WaitForExprFailOnErr(ctx, "OobeAPI.screens.ArcTosScreen.acceptButton.isEnabled()"); err != nil {
		s.Fatal("Failed to wait for the ARC TOS accept button to be enabled: ", err)
	}

	// Connect to session manager now for post-setup session login
	sm, err := session.NewSessionManager(ctx)
	if err != nil {
		s.Fatal("Failed to connect to session manager: ", err)
	}
	sw, err := sm.WatchSessionStateChanged(ctx, "started")
	if err != nil {
		s.Fatal("Failed to watch for session manager D-Bus signals: ", err)
	}

	// Click ARC TOS accept to finish interactive part of setup
	if err := oobeConn.Eval(ctx, "OobeAPI.screens.ArcTosScreen.acceptButton.click()", nil); err != nil {
		s.Fatal("Failed to click the ARC TOS accept button: ", err)
	}

	// TODO (b/192259053): Add a regression check here to ensure that temporary 'signin-failed' screen isn't
	// shown, after the underlying issue is fixed.

	s.Log("Waiting for SessionStateChanged \"started\" D-Bus signal from session_manager")
	select {
	case <-sw.Signals:
		s.Log("Got SessionStateChanged signal")
	case <-ctx.Done():
		s.Fatal("Didn't get SessionStateChanged signal: ", ctx.Err())
	}

}

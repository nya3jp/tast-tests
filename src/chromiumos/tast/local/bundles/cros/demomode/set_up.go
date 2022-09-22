// Copyright 2022 The ChromiumOS Authors
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
	"chromiumos/tast/local/demomode/fixture"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/session"
	"chromiumos/tast/testing"

	// XXX(yaohuali)

	"chromiumos/tast/local/apps"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         SetUp,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Basic test that clicks through Demo Mode setup from OOBE",
		Contacts:     []string{"cros-demo-mode-eng@google.com"},
		// If DUT ran other tests before current one, it could have logged into a managedchrome.com account.
		// This would place a domain lock on the device and prevent it from entering demo mode (cros-demo-mode.com).
		// The solution is to reset TPM before trying to enter demo mode.
		Fixture: fixture.TPMReset,
		Attr:    []string{"group:mainline", "informational"},
		// Demo Mode uses Zero Touch Enrollment for enterprise enrollment, which
		// requires a real TPM.
		// We require "arc" and "chrome_internal" because the ARC TOS screen
		// is only shown for chrome-branded builds when the device is ARC-capable.
		SoftwareDeps: []string{"chrome", "chrome_internal", "arc", "tpm"},
		VarDeps:      []string{"ui.signinProfileTestExtensionManifestKey"},
		// XXX(yaohuali)
		Timeout: 10 * time.Minute,
	})
}

// SetUp runs through the basic flow of entering Demo Mode from OOBE
//
// TODO(b/231472901): Deduplicate the shared code between Demo Mode and normal
// OOBE Tast tests
func SetUp(ctx context.Context, s *testing.State) {
	cr, err := chrome.New(ctx,
		chrome.NoLogin(),
		chrome.ARCSupported(),
		// TODO(crbug.com/1291183): Parameterize this test to also run a version that tests the
		// Consolidated Consent screen instead of EULA + ARC TOS
		chrome.DisableFeatures("OobeConsolidatedConsent"),
		chrome.DontSkipOOBEAfterLogin(),
		//chrome.ExtraArgs("--arc-start-mode=always-start", "--enable-crash-reporter-for-testing"),
		chrome.ExtraArgs("--arc-start-mode=always-start"),
		chrome.LoadSigninProfileExtension(s.RequiredVar("ui.signinProfileTestExtensionManifestKey")),
	)
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

	// XXX(yaohuali)
	ui := uiauto.New(tconn)//.WithTimeout(50 * time.Second)

	findAndClickButton := func(buttonApiMethod string) {
		var buttonName string
		if err := oobeConn.Eval(ctx, "OobeAPI.screens."+buttonApiMethod, &buttonName); err != nil {
			s.Fatal("Failed to get button name by calling "+buttonApiMethod+": ", err)
		}

		button := nodewith.Role(role.Button).Name(buttonName)
		if err := uiauto.Combine("Click button with name: "+buttonName,
			ui.WaitUntilExists(button),
			ui.LeftClick(button),
		)(ctx); err != nil {
			s.Fatal("Failed to click button with name: "+buttonName+" - error: ", err)
		}
	}

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
	findAndClickButton("WelcomeScreen.getDemoModeOkButtonName()")

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
		// TODO(crbug.com/1291153): Switch to focused button.
		nextButton := nodewith.Name("Next").Role(role.Button)
		if err := ui.LeftClickUntil(nextButton, ui.Gone(nextButton))(ctx); err != nil {
			s.Fatal("Failed to click network page next button: ", err)
		}
	}

	s.Log("Proceeding through demo preferences screen")
	if err := oobeConn.WaitForExprFailOnErr(ctx, "OobeAPI.screens.DemoPreferencesScreen.isVisible()"); err != nil {
		s.Fatal("Failed to wait for the demo preferences screen to be visible: ", err)
	}
	findAndClickButton("DemoPreferencesScreen.getDemoPreferencesNextButtonName()")

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
		// TODO(b/244185713): Switch to uiauto based button clicking.
		if err := oobeConn.Eval(ctx, "OobeAPI.screens.EulaScreen.clickNext()", nil); err != nil {
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
	defer sw.Close(ctx)
	// Click ARC TOS accept to finish interactive part of setup
	findAndClickButton("ArcTosScreen.getArcTosDemoModeAcceptButtonName()")

	s.Log("Waiting for SessionStateChanged \"started\" D-Bus signal from session_manager")
	select {
	case <-sw.Signals:
		s.Log("Got SessionStateChanged signal")
	case <-ctx.Done():
		s.Fatal("Didn't get SessionStateChanged signal: ", ctx.Err())
	}

	s.Log("============= OutDir: ", s.OutDir())

	// Look for the Play Store icon and click.
	// Polling till the icon is found or the timeout is reached.
	uia := uiauto.New(tconn)
	/*
	notFoundError := errors.New("Play Store icon is not found yet")
	err = testing.Poll(ctx, func(ctx context.Context) error {
		if found, err := uia.IsNodeFound(ctx, nodewith.Name(apps.PlayStore.Name).ClassName("ash/ShelfAppButton")); err != nil {
			if errors.Is(err, context.DeadlineExceeded) {
				return err
			}
			return testing.PollBreak(errors.Wrap(err, "failed to check Play Store icon"))
		} else if found {
			return nil
		}
		return notFoundError
	}, &testing.PollOptions{Timeout: 15 * time.Second, Interval: time.Second})
	*/

	s.Log("=============== Start to wait for Play Store")
	if err = uia.WithTimeout(50 * time.Second).WaitUntilExists(nodewith.Name(apps.PlayStore.Name).ClassName("ash/ShelfAppButton"))(ctx); err != nil {
		s.Fatal("Failed to wait for Play Store icon on shelf: ", err)
	}
	s.Log("=============== Play Store appears")
	testing.Sleep(ctx, 3 * time.Second)
	s.Log("=============== Sleep done")

	// We expect Play icon to always appear on tablet, regardless whether ARC is enabled by policy.
	if err != nil {
		s.Fatal("Failed to confirm the Play Store icon: ", err)
	}

	// Click on Play Store icon and see what pops out.
	if err := apps.Launch(ctx, tconn, apps.PlayStore.ID); err != nil {
		s.Fatal("Failed to launch Play Store: ", err)
	}

	// ARC opt-in is expected to happen (but fail, due to the fake policy we gave).
	// This will take a long time, e.g. 30s on kakadu, thus a long timeout value.
	arcOptInUI := nodewith.Name("Google Play apps and services").Role(role.StaticText)
	if err := uia.WithTimeout(50 * time.Second).WaitUntilExists(arcOptInUI)(ctx); err != nil {
		s.Fatal("Failed to see ARC Opt-In UI: ", err)
	}
}

// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package fixture

import (
	"context"
	"os"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/chrome/uiauto/state"
	"chromiumos/tast/local/hwsec"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/session"
	"chromiumos/tast/testing"
)

const (
	// SetUpDemoMode is the name for the fixture that clicks through Demo Mode OOBE setup
	SetUpDemoMode = "setUpDemoMode"

	setUpTimeout    = 100 * time.Second
	tearDownTimeout = 25 * time.Second
)

func init() {
	testing.AddFixture(&testing.Fixture{
		Name: SetUpDemoMode,
		Desc: "Proceed through Demo Mode setup flow from OOBE",
		Contacts: []string{
			"jacksontadie@google.com",
			"cros-demo-mode-eng@google.com",
		},
		Impl:            &fixtureImpl{},
		SetUpTimeout:    setUpTimeout,
		TearDownTimeout: tearDownTimeout,
		Vars:            []string{"ui.signinProfileTestExtensionManifestKey"},
	})
}

// fixtureImpl implements testing.FixtureImpl.
type fixtureImpl struct {
}

// Run through Demo Mode setup flow from OOBE
//
// TODO(b/231472901): Deduplicate the shared code between Demo Mode and normal
// OOBE Tast tests
func (f *fixtureImpl) SetUp(ctx context.Context, s *testing.FixtState) interface{} {
	resetTPMAndSystemState(ctx, s)

	cr, err := chrome.New(ctx,
		chrome.NoLogin(),
		chrome.ARCSupported(),
		// TODO(crbug.com/1291183): Parameterize this test to also run a version that tests the
		// Consolidated Consent screen instead of EULA + ARC TOS
		chrome.DisableFeatures("OobeConsolidatedConsent"),
		chrome.DontSkipOOBEAfterLogin(),
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

	ui := uiauto.New(tconn).WithTimeout(50 * time.Second)

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
		s.Log("Got SessionStateChanged signal. Demo Session has started")
	case <-ctx.Done():
		s.Fatal("Didn't get SessionStateChanged signal: ", ctx.Err())
	}

	s.Log("Logged in to Demo Mode")

	// Override DeveloperToolsAvailability policy (devtools is completely disabled for Demo Mode)
	// so we can connect to the test API extension in the actual demo session
	if err := os.MkdirAll("/etc/opt/chrome/policies/managed", 0755); err != nil {
		s.Fatal("Failed to create /etc/opt/chrome/policies/managed: ", err)
	}
	file, err := os.Create("/etc/opt/chrome/policies/managed/policy.json")
	if err != nil {
		s.Fatal("Failed to create /etc/opt/chrome/policies/managed/policy.json: ", err)
	}
	policyBytes := []byte("{ \"DeveloperToolsAvailability\": 1 }")
	_, err = file.Write(policyBytes)
	if err != nil {
		s.Fatal("Failed to write DeveloperToolsAvailability policy to /etc/opt/chrome/policy/managed/policy.json: ", err)
	}

	return nil
}

func (f *fixtureImpl) Reset(ctx context.Context) error {
	return nil
}

func (f *fixtureImpl) PreTest(ctx context.Context, s *testing.FixtTestState) {}

func (f *fixtureImpl) PostTest(ctx context.Context, s *testing.FixtTestState) {}

func (f *fixtureImpl) TearDown(ctx context.Context, s *testing.FixtState) {
	err := os.RemoveAll("/etc/opt/chrome/policies")
	if err != nil {
		s.Error("Failed to remove /etc/opt/chrome/policies directory: ", err)
	}
	resetTPMAndSystemState(ctx, s)
}

// resetTPMAndSystemState resets TPM, which can take a few seconds.
func resetTPMAndSystemState(ctx context.Context, s *testing.FixtState) {
	r := hwsec.NewCmdRunner()
	helper, err := hwsec.NewHelper(r)
	if err != nil {
		s.Fatal("Helper creation error: ", err)
	}
	s.Log("Start to reset TPM")
	if err := helper.EnsureTPMAndSystemStateAreReset(ctx); err != nil {
		s.Fatal("Failed to ensure resetting TPM: ", err)
	}
	s.Log("TPM is confirmed to be reset")
}

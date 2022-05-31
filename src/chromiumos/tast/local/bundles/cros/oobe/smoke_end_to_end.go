// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package oobe

import (
	"context"
	"time"

	"chromiumos/tast/common/hwsec"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/chrome/uiauto/state"
	hwseclocal "chromiumos/tast/local/hwsec"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         SmokeEndToEnd,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Smoke test that goes through OOBE, Login and Onboarding using the automation tools",
		Contacts: []string{
			"chromeos-sw-engprod@google.com",
			"cros-oobe@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		VarDeps: []string{
			"ui.signinProfileTestExtensionManifestKey",
			"ui.gaiaPoolDefault",
		},
		Params: []testing.Param{{
			Val: false,
		}, {
			Name: "polymer3",
			Val:  true,
		}},
	})
}

func SmokeEndToEnd(ctx context.Context, s *testing.State) {
	polymer3 := s.Param().(bool)
	options := []chrome.Option{
		chrome.NoLogin(),
		chrome.DontSkipOOBEAfterLogin(),
		// TODO(https://crbug.com/1328790): Enable the OobeConsolidatedConsent feature.
		// TODO(https://crbug.com/1335879): Enable the EnableOobeThemeSelection feature.
		chrome.DisableFeatures("OobeConsolidatedConsent", "EnableOobeThemeSelection"),
		chrome.DeferLogin(),
		chrome.GAIALoginPool(s.RequiredVar("ui.gaiaPoolDefault")),
		chrome.LoadSigninProfileExtension(s.RequiredVar("ui.signinProfileTestExtensionManifestKey")),
	}
	if polymer3 {
		options = append(options, chrome.EnableFeatures("EnableOobePolymer3"))
	}
	cr, err := chrome.New(ctx, options...)
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

	s.Log("Waiting for the welcome screen")
	if err := oobeConn.WaitForExprFailOnErr(ctx, "OobeAPI.screens.WelcomeScreen.isVisible()"); err != nil {
		s.Fatal("Failed to wait for the welcome screen to be visible: ", err)
	}

	focusedButton := nodewith.State(state.Focused, true).Role(role.Button)
	if err := uiauto.Combine("click next on the welcome screen",
		ui.WaitUntilExists(focusedButton),
		ui.LeftClick(focusedButton),
	)(ctx); err != nil {
		s.Fatal("Failed to click welcome screen next button: ", err)
	}

	shouldSkipNetworkScreen := false
	if err := oobeConn.Eval(ctx, "OobeAPI.screens.NetworkScreen.shouldSkip()", &shouldSkipNetworkScreen); err != nil {
		s.Fatal("Failed to evaluate whether to skip Network screen: ", err)
	}

	if shouldSkipNetworkScreen {
		s.Log("Skipping the network screen")
	} else {
		s.Log("Waiting for the network screen")
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
		s.Fatal("Failed to evaluate whether to skip Eula screen: ", err)
	}

	if shouldSkipEulaScreen {
		s.Log("Skipping the EULA screen")
	} else {
		s.Log("Waiting for the EULA screen")
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
		if err := uiauto.Combine("click next on the EULA screen",
			// Button is not focused on the screen. We focus the webview with EULA by default.
			ui.WaitUntilExists(eulaScreenNextButton.State(state.Focused, false)),
			ui.LeftClickUntil(eulaScreenNextButton, ui.Gone(eulaScreenNextButton)),
		)(ctx); err != nil {
			s.Fatal("Failed to click accept eula button: ", err)
		}
	}

	s.Log("Waiting for the user creation screen")
	if err := oobeConn.WaitForExprFailOnErr(ctx, "OobeAPI.screens.UserCreationScreen.isVisible()"); err != nil {
		s.Fatal("Failed to wait for the user creation screen to be visible: ", err)
	}

	if err := uiauto.Combine("click next on the user creation screen",
		ui.WaitUntilExists(focusedButton),
		ui.LeftClick(focusedButton),
	)(ctx); err != nil {
		s.Fatal("Failed to click user creation screen next button: ", err)
	}

	s.Log("Waiting for the Gaia screen")
	gaiaInput := nodewith.State(state.Focused, true).Role(role.TextField).Ancestor(nodewith.Role(role.Iframe))
	if err := ui.WaitUntilExists(gaiaInput)(ctx); err != nil {
		s.Fatal("Failed to wait for the login screen to be visible: ", err)
	}

	if err := cr.ContinueLogin(ctx); err != nil {
		s.Fatal("Failed to continue login: ", err)
	}

	s.Log("Waiting for the sync screen")
	if err := oobeConn.WaitForExprFailOnErr(ctx, "OobeAPI.screens.SyncScreen.isReadyForTesting()"); err != nil {
		s.Fatal("Failed to wait for the sync creation screen to be visible: ", err)
	}
	if err := uiauto.Combine("click next on the sync screen",
		ui.WaitUntilExists(focusedButton),
		ui.LeftClick(focusedButton),
	)(ctx); err != nil {
		s.Fatal("Failed to continue on the sync screen: ", err)
	}

	shouldSkipFingerprint := false
	if err := oobeConn.Eval(ctx, "OobeAPI.screens.FingerprintScreen.shouldSkip()", &shouldSkipFingerprint); err != nil {
		s.Fatal("Failed to evaluate whether to skip fingerprint screen: ", err)
	}

	if shouldSkipFingerprint {
		s.Log("Skipping the fingerprint screen")
	} else {
		s.Log("Waiting for the fingerprint screen")
		if err := oobeConn.WaitForExprFailOnErr(ctx, "OobeAPI.screens.FingerprintScreen.isVisible()"); err != nil {
			s.Fatal("Failed to wait for the fingerprint screen to be visible: ", err)
		}

		if err := uiauto.Combine("click next on the fingerprint screen",
			ui.WaitUntilExists(focusedButton),
			ui.LeftClick(focusedButton),
		)(ctx); err != nil {
			s.Fatal("Failed to skip on the fingerprint screen: ", err)
		}
	}

	cmdRunner := hwseclocal.NewCmdRunner()
	cryptohome := hwsec.NewCryptohomeClient(cmdRunner)
	supportsLE := false
	if supportsLE, err = cryptohome.SupportsLECredentials(ctx); err != nil {
		s.Fatal("Failed to get supported policies: ", err)
	}

	isInTabletMode := false
	if err := oobeConn.Eval(ctx, "OobeAPI.screens.PinSetupScreen.isInTabletMode()", &isInTabletMode); err != nil {
		s.Fatal("Failed to evaluate whether the device in the table mode: ", err)
	}

	if supportsLE || isInTabletMode {
		s.Log("Waiting for the pin setup screen")
		var pinSkipButton string
		if err := oobeConn.Eval(ctx, "OobeAPI.screens.PinSetupScreen.getSkipButtonName()", &pinSkipButton); err != nil {
			s.Fatal("Failed to get pin setup skip button name: ", err)
		}
		skipButton := nodewith.Role(role.Button).Name(pinSkipButton)
		if err := ui.LeftClick(skipButton)(ctx); err != nil {
			s.Fatal("Failed to click pin setup skip button: ", err)
		}
	} else {
		s.Log("Skipping the pin setup screen")
	}

	shouldSkipAssistant := false
	if err := oobeConn.Eval(ctx, "OobeAPI.screens.AssistantScreen.shouldSkip()", &shouldSkipAssistant); err != nil {
		s.Fatal("Failed to evaluate whether to skip assistant screen: ", err)
	}

	if shouldSkipAssistant {
		s.Log("Skipping the assistant screen")
	} else {
		s.Log("Waiting for the assistant screen")
		if err := oobeConn.WaitForExprFailOnErr(ctx, "OobeAPI.screens.AssistantScreen.isReadyForTesting()"); err != nil {
			s.Fatal("Failed to wait for the assistant screen to be visible: ", err)
		}
		var assistantSkipButton string
		if err := oobeConn.Eval(ctx, "OobeAPI.screens.AssistantScreen.getSkipButtonName()", &assistantSkipButton); err != nil {
			s.Fatal("Failed to get assistant next button name: ", err)
		}
		skipButton := nodewith.Role(role.Button).Name(assistantSkipButton)
		if err := ui.LeftClickUntil(skipButton, ui.Gone(skipButton))(ctx); err != nil {
			s.Fatal("Failed to click assistant skip button: ", err)
		}
	}

	shouldSkipMarketingOptIn := false
	if err := oobeConn.Eval(ctx, "OobeAPI.screens.MarketingOptInScreen.shouldSkip()", &shouldSkipMarketingOptIn); err != nil {
		s.Fatal("Failed to evaluate whether to skip marketing opt-in screen: ", err)
	}

	if shouldSkipMarketingOptIn {
		s.Log("Skipping marketing optin screen")
	} else {
		if err := oobeConn.WaitForExprFailOnErr(ctx, "OobeAPI.screens.MarketingOptInScreen.isVisible()"); err != nil {
			s.Fatal("Failed to wait for the marketing opt-in screen to be visible: ", err)
		}
		if err := uiauto.Combine("click next on the marketing-optin screen",
			ui.WaitUntilExists(focusedButton),
			ui.LeftClick(focusedButton),
		)(ctx); err != nil {
			s.Fatal("Failed to continue on the marketing opt-in screen: ", err)
		}
	}

	if err := cr.WaitForOOBEConnectionToBeDismissed(ctx); err != nil {
		s.Fatal("Failed to wait for OOBE to be dismissed: ", err)
	}
}

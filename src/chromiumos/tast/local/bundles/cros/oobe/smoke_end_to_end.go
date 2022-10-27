// Copyright 2022 The ChromiumOS Authors
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
			"bohdanty@google.com",
			"rrsilva@google.com",
			"chromeos-sw-engprod@google.com",
			"cros-oobe@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		VarDeps: []string{
			"ui.signinProfileTestExtensionManifestKey",
			"ui.gaiaPoolDefault",
		},
		Timeout: chrome.GAIALoginTimeout + 5*time.Minute,
		Params: []testing.Param{{
			Val: false,
		}, {
			Name: "add_person_flow",
			Val:  true,
		}},
	})
}

func SmokeEndToEnd(ctx context.Context, s *testing.State) {
	// Smoke End To End covers two flows:
	// - "Normal" OOBE flow from the Welcome screen
	//
	// - "Add Person" flow from the login screen. In this flow, Chrome is first
	//   launched to create a test user and then relaunched with 'KeepState' so
	//   it lands on the login screen.

	isAddPersonFlow := s.Param().(bool)
	if isAddPersonFlow {
		// Create user on the device.
		cr, err := chrome.New(ctx)
		if err != nil {
			s.Fatal("Failed to start Chrome and create a new user: ", err)
		}
		cr.Close(ctx)
	}

	options := []chrome.Option{
		chrome.DontSkipOOBEAfterLogin(),
		chrome.DeferLogin(),
		chrome.GAIALoginPool(s.RequiredVar("ui.gaiaPoolDefault")),
		chrome.LoadSigninProfileExtension(s.RequiredVar("ui.signinProfileTestExtensionManifestKey")),
	}

	// Keep the user that was previously added for the 'AddPerson' flow.
	if isAddPersonFlow {
		options = append(options, chrome.KeepState())
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

	focusedButton := nodewith.State(state.Focused, true).Role(role.Button)

	// Click on the 'Add Person' button if we are on the login screen, otherwise step through
	// the OOBE screens that are shown during the first boot.
	if isAddPersonFlow {
		err := ui.LeftClick(nodewith.Role(role.Button).Name("Add Person"))(ctx)
		if err != nil {
			s.Fatal("Failed to click on the Add Person button")
		}
	} else {
		s.Log("Waiting for the welcome screen")
		if err := oobeConn.WaitForExprFailOnErr(ctx, "OobeAPI.screens.WelcomeScreen.isVisible()"); err != nil {
			s.Fatal("Failed to wait for the welcome screen to be visible: ", err)
		}

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
			if err := oobeConn.WaitForExprFailOnErr(ctx, "OobeAPI.screens.EulaScreen.isReadyForTesting()"); err != nil {
				s.Fatal("Failed to wait for the eula screen to be visible: ", err)
			}
			if err := uiauto.Combine("Click next on EULA screen",
				ui.WaitUntilExists(focusedButton),
				ui.LeftClick(focusedButton),
			)(ctx); err != nil {
				s.Fatal("Failed to click EULA screen next button: ", err)
			}
		}
	}

	// TODO(b/255972416) - Remove this.
	// Even though clicking on the 'Add Person' shows the User Creation screen first, the
	// flow in performGAIALogin (gaia.go) has been hardcoded to inject JS into the page
	// and click on the next button to land on the Gaia screen. It does this whenever it
	// sees the URL used for the 'Add Person' flow (chrome://oobe/gaia-signin).
	if !isAddPersonFlow {
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
	}

	if err := cr.ContinueLogin(ctx); err != nil {
		s.Fatal("Failed to continue login: ", err)
	}

	var shouldSkipConsolidatedConsentScreen bool
	if err := oobeConn.Eval(ctx, "OobeAPI.screens.ConsolidatedConsentScreen.shouldSkip()", &shouldSkipConsolidatedConsentScreen); err != nil {
		s.Fatal("Failed to evaluate whether to skip consolidated consent screen: ", err)
	}

	if shouldSkipConsolidatedConsentScreen {
		s.Log("Skipping the consolidated consent screen")
	} else {
		s.Log("Waiting for the consolidated consent screen")
		if err := oobeConn.WaitForExprFailOnErr(ctx, "OobeAPI.screens.ConsolidatedConsentScreen.isReadyForTesting()"); err != nil {
			s.Fatal("Failed to wait for the consolidated consent screen to be visible: ", err)
		}

		isReadMoreButtonShown := false
		if err := oobeConn.Eval(ctx, "OobeAPI.screens.ConsolidatedConsentScreen.isReadMoreButtonShown()", &isReadMoreButtonShown); err != nil {
			s.Fatal("Failed to evaluate whether the read more button on the consolidated consent screen is shown: ", err)
		}
		if isReadMoreButtonShown {
			if err := uiauto.Combine("Click read more button on the consolidated consent screen",
				ui.WaitUntilExists(focusedButton),
				ui.LeftClick(focusedButton),
			)(ctx); err != nil {
				s.Fatal("Failed to click the consolidated consent read more button: ", err)
			}

			if err := oobeConn.WaitForExprFailOnErr(ctx, "!OobeAPI.screens.ConsolidatedConsentScreen.isReadMoreButtonShown()"); err != nil {
				s.Fatal("Failed to wait for the consolidated consent read more to be hidden: ", err)
			}
		}

		if err := uiauto.Combine("Click accept on the consolidated consent screen",
			ui.WaitUntilExists(focusedButton),
			ui.LeftClick(focusedButton),
		)(ctx); err != nil {
			s.Fatal("Failed to click consolidated consent screen accept button: ", err)
		}
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

	if err := oobeConn.WaitForExprFailOnErr(ctx, "OobeAPI.screens.ThemeSelectionScreen.isVisible()"); err != nil {
		s.Fatal("Failed to wait for the theme selection screen to be visible: ", err)
	}
	if err := uiauto.Combine("click next on the theme selection screen",
		ui.WaitUntilExists(focusedButton),
		ui.LeftClick(focusedButton),
	)(ctx); err != nil {
		s.Fatal("Failed to continue on the theme selection screen: ", err)
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

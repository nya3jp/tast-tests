// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"time"

	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/chrome"
	chromeui "chromiumos/tast/local/chrome/ui"
	"chromiumos/tast/local/chrome/ui/faillog"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

const (
	// Test User email
	TestUser = "crtesting2021@gmail.com"

	// Test user Password
	TestPass = "  P@ssw0rd@123"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         OobeArc,
		Desc:         "Navigate throgh OOBE via UI. Verify that PlayStore can be successfully lanched",
		Contacts:     []string{"vkrishan@google.com", "rohitbm@google.com", "arc-eng@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Params: []testing.Param{{
			ExtraSoftwareDeps: []string{"android_p"},
		}, {
			Name:              "vm",
			ExtraSoftwareDeps: []string{"android_vm"},
		}},
		Timeout: chrome.LoginTimeout + arc.BootTimeout + 120*time.Second,
		Vars:    []string{"arc.signinProfileTestExtensionManifestKey"},
	})
}

func OobeArc(ctx context.Context, s *testing.State) {

	cr, err := chrome.New(ctx, chrome.NoLogin(), chrome.LoadSigninProfileExtension(s.RequiredVar("arc.signinProfileTestExtensionManifestKey")))
	if err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}
	defer cr.Close(ctx)

	tconn, err := cr.SigninProfileTestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect Test API: ", err)
	}
	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)
	defer tconn.Close()

	oobeConn, err := cr.WaitForOOBEConnection(ctx)
	if err != nil {
		s.Fatal("Failed to connect OOBE: ", err)
	}
	defer oobeConn.Close()

	if err := oobeConn.WaitForExprFailOnErr(ctx, "OobeAPI.screens.WelcomeScreen.isVisible()"); err != nil {
		s.Fatal("Failed to wait for the welcome screen to be visible: ", err)
	}
	if err := oobeConn.Exec(ctx, "OobeAPI.screens.WelcomeScreen.clickNext()"); err != nil {
		s.Fatal("Failed to click welcome page next button: ", err)
	}

	if err := oobeConn.WaitForExprFailOnErr(ctx, "OobeAPI.screens.NetworkScreen.isVisible()"); err != nil {
		s.Fatal("Failed to wait for the network screen to be visible: ", err)
	}
	if err := oobeConn.Exec(ctx, "OobeAPI.screens.NetworkScreen.clickNext()"); err != nil {
		s.Fatal("Failed to click network page next button: ", err)
	}

	shouldSkipEulaScreen := false
	if err := oobeConn.Eval(ctx, "OobeAPI.screens.EulaScreen.shouldSkip()", &shouldSkipEulaScreen); err != nil {
		s.Fatal("Failed to evaluate whether to skip Eula screen: ", err)
	}

	if !shouldSkipEulaScreen {
		if err := oobeConn.WaitForExprFailOnErr(ctx, "OobeAPI.screens.EulaScreen.isVisible()"); err != nil {
			s.Fatal("Failed to wait for the eula screen to be visible: ", err)
		}
		if err := oobeConn.WaitForExprFailOnErr(ctx, "OobeAPI.screens.EulaScreen.nextButton.isEnabled()"); err != nil {
			s.Fatal("Failed to wait for the accept eula button to be enabled: ", err)
		}
		if err := oobeConn.Exec(ctx, "OobeAPI.screens.EulaScreen.clickNext()"); err != nil {
			s.Fatal("Failed to click accept eula button: ", err)
		}
	}

	if err := oobeConn.WaitForExprFailOnErr(ctx, "OobeAPI.screens.UserCreationScreen.isVisible()"); err != nil {
		s.Fatal("Failed to wait for the user creation screen to be visible: ", err)
	}
	if err := oobeConn.Exec(ctx, "OobeAPI.screens.UserCreationScreen.clickNext()"); err != nil {
		s.Fatal("Failed to click user creation screen next button: ", err)
	}

	if err := oobeConn.WaitForExprFailOnErr(ctx, "OobeAPI.screens.GaiaScreen.isVisible()"); err != nil {
		s.Fatal("Failed to wait for the login screen to be visible: ", err)
	}

	//Find Edit text to Enter email
	emailTextField := chromeui.FindParams{
		Role: chromeui.RoleTypeTextField,
		Name: "Email or phone",
	}

	email, err := chromeui.FindWithTimeout(ctx, tconn, emailTextField, 10*time.Second)
	if err != nil {
		s.Fatal("Failed to find Email or phone text field: ", err)
	}
	defer email.Release(ctx)

	if err := email.LeftClick(ctx); err != nil {
		s.Fatal("Failed to click the Email text field: ", err)
	}

	kb, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to get keyboard: ", err)
	}
	defer kb.Close()

	s.Log("Entering username")
	if err := kb.Type(ctx, TestUser+"\n"); err != nil {
		s.Fatal("Entering password failed: ", err)
	}
	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)

	// Enter Password
	passwordTextField := chromeui.FindParams{
		Role: chromeui.RoleTypeTextField,
		Name: "Enter your password",
	}

	password, err := chromeui.FindWithTimeout(ctx, tconn, passwordTextField, 10*time.Second)
	if err != nil {
		s.Fatal("Failed to find password text field: ", err)
	}
	defer password.Release(ctx)

	if err := password.LeftClick(ctx); err != nil {
		s.Fatal("Failed to click the Password text field: ", err)
	}

	s.Log("Entering password")
	if err := kb.Type(ctx, TestPass); err != nil {
		s.Fatal("Entering password failed: ", err)
	}

	// Click Next
	NextButton := chromeui.FindParams{
		Role: chromeui.RoleTypeButton,
		Name: "Next",
	}

	next, err := chromeui.FindWithTimeout(ctx, tconn, NextButton, 10*time.Second)
	if err != nil {
		s.Fatal("Failed to find Next button: ", err)
	}
	defer next.Release(ctx)

	if err := next.LeftClick(ctx); err != nil {
		s.Fatal("Failed to click the Next button: ", err)
	}
}

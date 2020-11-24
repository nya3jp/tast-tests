// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"

	"chromiumos/tast/local/chrome"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: OOBESmoke,
		Desc: "Smoke test that clicks through OOBE",
		Contacts: []string{
			"chromeos-sw-engprod@google.com",
			"cros-oac@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
	})
}

func OOBESmoke(ctx context.Context, s *testing.State) {
	// HID detection screen is shown before the welcome screen on certain devices, thus flag is added to prevent such cases.
	cr, err := chrome.New(ctx, chrome.NoLogin(), chrome.ExtraArgs("--disable-hid-detection-on-oobe"), chrome.EnableFeatures("ChildSpecificSignin"))
	if err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}
	defer cr.Close(ctx)

	oobeConn, err := cr.WaitForOOBEConnection(ctx)
	if err != nil {
		s.Fatal("Failed to create OOBE connection: ", err)
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
}

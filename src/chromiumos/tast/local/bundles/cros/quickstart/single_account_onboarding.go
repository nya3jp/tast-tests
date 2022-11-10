// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package quickstart contains tests for the Quick Start feature in ChromeOS.
package quickstart

import (
	"context"

	"chromiumos/tast/local/chrome/crossdevice"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         SingleAccountOnboarding,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Test Quick Start onboarding flow with one user account on the phone",
		Contacts: []string{
			"jasonrhee@chromium.org",
			"chromeos-sw-engprod@google.com",
			"chromeos-cross-device-eng@google.com",
		},
		// Attr:         []string{"group:cross-device"},
		SoftwareDeps: []string{"chrome"},
		Fixture:      "crossdeviceNoSignIn",
	})
}

func SingleAccountOnboarding(ctx context.Context, s *testing.State) {
	androidDevice := s.FixtValue().(*crossdevice.FixtData).AndroidDevice
	if androidDevice == nil {
		s.Fatal("fixture not associated with an android device")
	}
	cr := s.FixtValue().(*crossdevice.FixtData).Chrome
	if cr == nil {
		s.Fatal("fixture not associated with Chrome")
	}
	oobeConn, err := cr.WaitForOOBEConnection(ctx)
	if err != nil {
		s.Fatal("failed to create OOBE connection: ", err)
	}
	defer oobeConn.Close()
	s.Log("Waiting for the welcome screen")
	if err := oobeConn.WaitForExprFailOnErr(ctx, "OobeAPI.screens.WelcomeScreen.isVisible()"); err != nil {
		s.Fatal("Failed to wait for the welcome screen to be visible: ", err)
	}
	s.Log("Navigating to the quickstart screen")
	if err := oobeConn.Eval(ctx, "OobeAPI.advanceToScreen('quick-start')", nil); err != nil {
		s.Fatal("Failed to activate Quick Start onboarding: ", err)
	}
	s.Log("Calling accept half pair sheet")
	if err := androidDevice.AcceptFastPairHalfsheet(ctx); err != nil {
		s.Fatal("Failed to accept half pair sheet: ", err)
	}
}

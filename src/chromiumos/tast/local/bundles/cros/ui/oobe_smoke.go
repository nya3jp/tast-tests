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
		Func:         OOBESmoke,
		Desc:         "Smoke test that clicks through OOBE",
		Contacts:     []string{"bhansknecht@chromium.org", "dhaddock@chromium.org"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
	})
}

func OOBESmoke(ctx context.Context, s *testing.State) {
	cr, err := chrome.New(ctx, chrome.NoLogin())
	if err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}
	defer cr.Close(ctx)

	oobeConn, err := cr.WaitForOOBEConnection(ctx)
	if err != nil {
		s.Fatal("Failed to create OOBE connection: ", err)
	}
	defer oobeConn.Close()

	if err := oobeConn.WaitForExprFailOnErr(ctx, "!document.querySelector('oobe-welcome-md[hidden]')"); err != nil {
		s.Fatal("Failed to wait for the welcome screen to be visible: ", err)
	}
	if err := oobeConn.Exec(ctx, "document.querySelector('oobe-welcome-md').$.welcomeScreen.$.welcomeNextButton.click()"); err != nil {
		s.Fatal("Failed to click welcome page next button: ", err)
	}

	if err := oobeConn.WaitForExprFailOnErr(ctx, "!document.querySelector('oobe-network-md[hidden]')"); err != nil {
		s.Fatal("Failed to wait for the network screen to be visible: ", err)
	}
	if err := oobeConn.Exec(ctx, "document.querySelector('oobe-network-md').$.nextButton.click()"); err != nil {
		s.Fatal("Failed to click network page next button: ", err)
	}

	if err := oobeConn.WaitForExprFailOnErr(ctx, "!document.querySelector('oobe-eula-md[hidden]')"); err != nil {
		s.Fatal("Failed to wait for the eula screen to be visible: ", err)
	}
	if err := oobeConn.Exec(ctx, "document.querySelector('oobe-eula-md').$.acceptButton.click()"); err != nil {
		s.Fatal("Failed to click accept eula button: ", err)
	}

	if err := oobeConn.WaitForExprFailOnErr(ctx, "!document.querySelector('gaia-signin[hidden]')"); err != nil {
		s.Fatal("Failed to wait for the login screen to be visible: ", err)
	}
}

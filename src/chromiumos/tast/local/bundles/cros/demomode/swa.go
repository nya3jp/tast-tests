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
	"chromiumos/tast/local/chrome/uiauto/pointer"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/coords"
	"chromiumos/tast/local/demomode/fixture"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         SWA,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Verify state about the core demo mode Chrome Apps",
		Contacts:     []string{"jacksontadie@google.com", "cros-demo-mode-eng@google.com"},
		Fixture:      fixture.PostDemoModeOOBE,
		Attr:         []string{"group:mainline", "informational"},
		// Demo Mode uses Zero Touch Enrollment for enterprise enrollment, which
		// requires a real TPM.
		// We require "arc" and "chrome_internal" because the ARC TOS screen
		// is only shown for chrome-branded builds when the device is ARC-capable.
		SoftwareDeps: []string{"chrome", "chrome_internal", "arc", "tpm"},
	})
}

func SWA(ctx context.Context, s *testing.State) {
	cr, err := chrome.New(ctx,
		chrome.NoLogin(),
		chrome.ARCSupported(),
		chrome.KeepEnrollment(),
		chrome.EnableFeatures("DemoModeSWA"),
		// --force-devtools-available forces devtools on regardless of policy (devtools is
		// disabled in Demo Mode policy) to support connecting to the test API extension.
		//
		// --component-updater=test-request adds a "test-request" parameter to Omaha
		// update requests, causing the fetched Demo Mode App component to come from a
		// test cohort.
		chrome.ExtraArgs("--force-devtools-available", "--component-updater=test-request"))
	if err != nil {
		s.Fatal("Failed to restart Chrome: ", err)
	}

	clearUpCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()
	defer cr.Close(clearUpCtx)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create the test API connection: ", err)
	}
	defer faillog.DumpUITreeOnError(clearUpCtx, s.OutDir(), s.HasError, tconn)
	ui := uiauto.New(tconn).WithTimeout(100 * time.Second)

	s.Log("Waiting for Demo Mode App to launch")
	demoApp := nodewith.Name("Demo Mode App").First()
	if err := ui.WaitUntilExists(demoApp)(ctx); err != nil {
		s.Fatal("Failed to wait until Demo App exists: ", err)
	}

	// Verify app is in fullscreen Attract Loop mode by lack of toolbar.
	s.Log("Confirming that app is in fullscreen mode")
	toolbar := nodewith.Role(role.Toolbar).Ancestor(demoApp)
	if err := ui.WaitUntilGone(toolbar)(ctx); err != nil {
		s.Fatal("Failed to confirm that the toolbar is not present: ", err)
	}

	pc := pointer.NewMouse(tconn)
	defer pc.Close()

	demoAppLocation, _ := ui.Location(ctx, demoApp)

	// Move mouse (arbitrarily) from center of demo app to screen corner to
	// trigger interaction, breaking fullscreen Attract Loop.
	if err := pc.Drag(
		demoAppLocation.CenterPoint(),
		pc.DragTo(coords.NewPoint(0, 0), 1*time.Second))(ctx); err != nil {
		s.Fatal("Failed to drag mouse across screen: ", err)
	}

	// Assert that app is now windowed by presence of toolbar.
	s.Log("Waiting for windowed Highlights mode")
	if err := ui.WaitUntilExists(toolbar)(ctx); err != nil {
		s.Fatal("Failed to wait for the toolbar to be present: ", err)
	}

	pillarCard := nodewith.Name("easy card")
	if err := uiauto.Combine("Click out of highlights home page",
		ui.WaitUntilExists(pillarCard),
		ui.LeftClick(pillarCard),
	)(ctx); err != nil {
		s.Fatal("Failed to click the \"easy\" pillar card: ", err)
	}

	heading1 := nodewith.Role(role.Heading).Name("Phone pairing")
	heading2 := nodewith.Role(role.Heading).Name("Long battery life")
	nextButton := nodewith.Role(role.Button).Name("Next slide")

	s.Log("Clicking to next page in pillar")
	if err := uiauto.Combine("Click to next page in pillar",
		ui.WaitUntilExists(heading1),
		ui.WaitUntilExists(nextButton),
		ui.LeftClick(nextButton),
		ui.WaitUntilExists(heading2),
	)(ctx); err != nil {
		s.Fatal("Failed to click to next page in pillar: ", err)
	}

	securePillarButton := nodewith.Role(role.Button).Name("Discover why Chromebook is Secure")
	heading3 := nodewith.Role(role.Heading).Name("Multi-user support")
	s.Log("Clicking to a different pillar")
	if err := uiauto.Combine("Clicking to a different pillar",
		ui.WaitUntilExists(securePillarButton),
		ui.LeftClick(securePillarButton),
		ui.WaitUntilExists(heading3),
	)(ctx); err != nil {
		s.Fatal("Failed to click on a different pillar: ", err)
	}
}

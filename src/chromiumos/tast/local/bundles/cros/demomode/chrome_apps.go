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
	"chromiumos/tast/local/chrome/uiauto/state"
	"chromiumos/tast/local/demomode/fixture"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         ChromeApps,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Verify state about the core demo mode Chrome Apps",
		Contacts:     []string{"cros-demo-mode-eng@google.com"},
		Fixture:      fixture.SetUpDemoMode,
		Attr:         []string{"group:mainline", "informational"},
		// Demo Mode uses Zero Touch Enrollment for enterprise enrollment, which
		// requires a real TPM.
		// We require "arc" and "chrome_internal" because the ARC TOS screen
		// is only shown for chrome-branded builds when the device is ARC-capable.
		SoftwareDeps: []string{"chrome", "chrome_internal", "arc", "tpm"},
	})
}

// ChromeApps asserts state about the core auto-launched Demo Mode Chrome Apps:
// - Attract Loop: An animated screensaver that loops while a device is idle
// - Highlights App: A windowed interactive application that showcases ChromeOS features
func ChromeApps(ctx context.Context, s *testing.State) {
	cr, err := chrome.New(ctx, chrome.NoLogin(), chrome.KeepEnrollment())
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

	// Have such a long timeout because the Attract Loop takes a while to load
	// for the first session after setup (app is cached for subsequent sessions)
	ui := uiauto.New(tconn).WithTimeout(100 * time.Second)

	s.Log("Waiting for Highlights App")
	highlightsWindow := nodewith.Name("Google Retail Chromebook").First()
	if err := ui.WaitUntilExists(highlightsWindow)(ctx); err != nil {
		s.Fatal("Failed to wait until Highlights App exists: ", err)
	}
	s.Log("Highlights App found")

	s.Log("Waiting for the Attract Loop")
	// Unfortunately both Chrome Apps share the same element name, so we have to
	// identify the Attract Loop app by the fact that it's focused on startup
	// (unlike the Highlights app)
	attractLoopView := nodewith.State(state.Focused, true).Name("Google Retail Chromebook")
	if err := ui.WaitUntilExists(attractLoopView)(ctx); err != nil {
		s.Fatal("Failed to wait until the Attract Loop exists: ", err)
	}
}

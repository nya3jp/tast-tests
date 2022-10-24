// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package spera

import (
	"context"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/cuj"
	"chromiumos/tast/local/chrome/display"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/webutil"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/typecutils"
	"chromiumos/tast/local/uidetection"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         UIDetectionOnExternalDisplay,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Debug ui detection on external display",
		Contacts:     []string{"jane.yang@cienet.com"},
		SoftwareDeps: []string{"chrome"},
		Fixture:      "loggedInAndKeepState",
		Vars: []string{
			// Required. Used for UI detection API.
			"uidetection.key_type",
			"uidetection.key",
			"uidetection.server",
		},
	})
}

func UIDetectionOnExternalDisplay(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(chrome.HasChrome).Chrome()
	// Shorten context a bit to allow for cleanup.
	cleanUpCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect to test API: ", err)
	}
	kb, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to initialize keyboard input: ", err)
	}
	defer kb.Close()

	// Unset mirrored display so two displays can show different information.
	if err := typecutils.SetMirrorDisplay(ctx, tconn, false); err != nil {
		s.Fatal("Failed to unset mirror display: ", err)
	}
	// Make sure there are two displays on DUT.
	// This procedure must be performed after display mirror is unset. Otherwise we can only
	// get one display info.
	infos, err := display.GetInfo(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to get display info: ", err)
	}
	if len(infos) != 2 {
		s.Fatalf("DUT connected with incorrect nubmer of displays - want 2, got %d: %v", len(infos), err)
	}

	conn, err := cr.NewConn(ctx, cuj.GoogleURL)
	if err != nil {
		s.Fatalf("Failed to open URL %s: %v", cuj.GoogleURL, err)
	}
	defer conn.Close()
	defer conn.CloseTarget(cleanUpCtx)
	defer faillog.DumpUITreeWithScreenshotOnError(cleanUpCtx, s.OutDir(), s.HasError, cr, "ui_dump")

	if err := webutil.WaitForQuiescence(ctx, conn, time.Minute); err != nil {
		s.Fatalf("Failed to wait for tab to achieve quiescence within %v: %v", time.Minute, err)
	}

	ud := uidetection.NewDefault(tconn)
	udi := ud.WithScreenshotStrategy(uidetection.ImmediateScreenshot)
	searchGoogleText := uidetection.TextBlockFromSentence("Google Search").First()

	if err := ud.WaitUntilExists(searchGoogleText)(ctx); err != nil {
		s.Fatal("Failed to use uidetection with stable screenshot on internal display to wait for 'Google Search': ", err)
	} else {
		s.Log("Successfully detected 'Google Search' on internal display using uidetection with stable screenshot")
	}
	if err := udi.WaitUntilExists(searchGoogleText)(ctx); err != nil {
		s.Fatal("Failed to use uidetection with immediate screenshot on internal display to wait for 'Google Search': ", err)
	} else {
		s.Log("Successfully detected 'Google Search' on internal display using uidetection with immediate screenshot")
	}

	if err := cuj.SwitchWindowToDisplay(ctx, tconn, kb, true)(ctx); err != nil {
		s.Fatal("Failed to switch window: ", err)
	}

	if err := ud.WaitUntilExists(searchGoogleText)(ctx); err != nil {
		s.Fatal("Failed to use uidetection with stable screenshot on external display to wait for 'Google Search': ", err)
	} else {
		s.Log("Successfully detected 'Google Search' on external display using uidetection with stable screenshot")
	}
	if err := udi.WaitUntilExists(searchGoogleText)(ctx); err != nil {
		s.Fatal("Failed to use uidetection with immediate screenshot on external display to wait for 'Google Search' : ", err)
	} else {
		s.Log("Successfully detected 'Google Search' on external display using uidetection with immediate screenshot")
	}
}

// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package feedback

import (
	"context"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         LaunchOldAppFromOOBE,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "User is able to launch the old feedback app from OOBE",
		Contacts: []string{
			"swifton@google.com",
			"xiangdongkong@google.com",
			"cros-feedback-app@google.com",
		},
		Timeout: 3 * time.Minute,
		VarDeps: []string{
			"ui.signinProfileTestExtensionManifestKey",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
	})
}

// LaunchOldAppFromOOBE verifies User is able to launch the old feedback app from OOBE
func LaunchOldAppFromOOBE(ctx context.Context, s *testing.State) {
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()

	cr, err := chrome.New(
		ctx,
		chrome.LoadSigninProfileExtension(s.RequiredVar("ui.signinProfileTestExtensionManifestKey")),
		chrome.NoLogin())
	if err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}
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

	if err := oobeConn.WaitForExprFailOnErr(ctx, "OobeAPI.screens.WelcomeScreen.isVisible()"); err != nil {
		s.Fatal("Failed to wait for the welcome screen to be visible: ", err)
	}

	// Set up keyboard.
	kb, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to find keyboard: ", err)
	}
	defer kb.Close()

	ui := uiauto.New(tconn).WithTimeout(20 * time.Second)

	// Launch Feedback app with alt+shift+i.
	if err := kb.Accel(ctx, "Alt+Shift+I"); err != nil {
		s.Fatal("Failed to press alt+shift+i: ", err)
	}

	// Verify the old Feedback app is launched.
	feedbackWindow := nodewith.Name("Send feedback to Google").Role(role.Window)
	if err := ui.WaitUntilExists(feedbackWindow)(ctx); err != nil {
		s.Fatal("Failed to find the feedback window: ", err)
	}
}

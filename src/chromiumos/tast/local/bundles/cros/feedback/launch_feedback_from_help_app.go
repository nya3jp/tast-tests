// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package feedback

import (
	"context"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/feedbackapp"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         LaunchFeedbackFromHelpApp,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Feedback app can be found and launched from the Help app",
		Contacts: []string{
			"swifton@google.com",
			"xiangdongkong@google.com",
			"cros-feedback-app@google.com",
		},
		Fixture:      "chromeLoggedInWithOsFeedback",
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Timeout:      2 * time.Minute,
	})
}

// LaunchFeedbackFromHelpApp verifies launching the Feedback app from
// the Help app.
func LaunchFeedbackFromHelpApp(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(*chrome.Chrome)

	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect to Test API: ", err)
	}
	defer faillog.DumpUITreeWithScreenshotOnError(cleanupCtx, s.OutDir(), s.HasError, cr,
		"ui_dump")

	ui := uiauto.New(tconn).WithTimeout(20 * time.Second)

	// Launch the Help app from essential apps.
	if err := apps.Launch(ctx, tconn, apps.Help.ID); err != nil {
		s.Fatal("Failed to launch the Help app: ", err)
	}
	if err := ash.WaitForApp(ctx, tconn, apps.Help.ID, time.Minute); err != nil {
		s.Fatal("Failed to wait for the Help app")
	}

	// Click the Send feedback button.
	sendFeedbackButton := nodewith.NameContaining("Send feedback").Role(role.Button)
	if err := ui.DoDefault(sendFeedbackButton)(ctx); err != nil {
		s.Fatal("Failed to click Send feedback button: ", err)
	}

	// Wait for the Feedback app to launch before closing the Help app.
	if err = ash.WaitForApp(ctx, tconn, apps.Feedback.ID, time.Minute); err != nil {
		s.Fatal("Could not find app in shelf after launch: ", err)
	}

	// Close the Help app, otherwise the query for links in
	// VerifyFeedbackAppIsLaunched will return too many results.
	if err := apps.Close(ctx, tconn, apps.Help.ID); err != nil {
		s.Error("Failed to close the Help app: ", err)
	}

	if err := feedbackapp.VerifyFeedbackAppIsLaunched(ctx, tconn, ui); err != nil {
		s.Fatal("Failed to verify that the Feedback app is launched: ", err)
	}
}

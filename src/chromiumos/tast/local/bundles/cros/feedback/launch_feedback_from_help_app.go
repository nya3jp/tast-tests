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
		s.Fatal("Fail to wait for the Help app app ")
	}

	// Click the Send feedback button.
	sendFeedbackButton := nodewith.NameContaining("Send feedback").Role(role.Button)
	if err := ui.DoDefault(sendFeedbackButton)(ctx); err != nil {
		s.Fatal("Failed to click Send feedback button: ", err)
	}

	// Verify Feedback app is launched.
	if err = ash.WaitForApp(ctx, tconn, apps.Feedback.ID, time.Minute); err != nil {
		s.Fatal("Could not find app in shelf after launch: ", err)
	}

	// Verify essential elements exist.
	issueDescriptionInput := nodewith.NameContaining("Description").Role(role.TextField)
	continueButton := nodewith.Name("Continue").Role(role.Button)

	if err := uiauto.Combine("Verify essential elements exist",
		ui.WaitUntilExists(issueDescriptionInput),
		ui.WaitUntilExists(continueButton),
	)(ctx); err != nil {
		s.Fatal("Failed to find element: ", err)
	}

	// Close the Help app, otherwise the query for links will return too many results.
	if err := apps.Close(ctx, tconn, apps.Help.ID); err != nil {
		s.Error("Failed to close the Help app: ", err)
	}

	// Verify five default help content links exist.
	helpLink := nodewith.Role(role.Link).Ancestor(nodewith.Role(role.Iframe))
	for i := 0; i < 5; i++ {
		item := helpLink.Nth(i)
		if err := ui.WaitUntilExists(item)(ctx); err != nil {
			s.Errorf("Failed to find five help links (link %d): %v", i, err)
		}
	}
}

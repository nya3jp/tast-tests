// Copyright 2022 The ChromiumOS Authors.
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
	"chromiumos/tast/local/chrome/uiauto/feedbackapp"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         SubmitFeedbackAgain,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "User is able to submit a report then create a new one and submit it",
		Contacts: []string{
			"zhangwenyu@google.com",
			"xiangdongkong@google.com",
			"cros-feedback-app@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
	})
}

// SubmitFeedbackAgain verifies the user can submit a report then
// create a new one and submit it.
func SubmitFeedbackAgain(ctx context.Context, s *testing.State) {
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()

	s.Log("Setting up chrome")
	cr, err := chrome.New(ctx, chrome.EnableFeatures("OsFeedback"))
	if err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}
	defer cr.Close(cleanupCtx)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect to Test API: ", err)
	}
	defer faillog.DumpUITreeWithScreenshotOnError(cleanupCtx, s.OutDir(), s.HasError, cr,
		"ui_dump")

	ui := uiauto.New(tconn).WithTimeout(20 * time.Second)

	// Launching feedback app and go to confirmation page.
	feedbackRootNode, err := feedbackapp.LaunchAndGoToConfirmationPage(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to launch feedback app and go to confirmation page: ", err)
	}

	// Verifying user is in the confirmation page and can send a new report.
	sendNewReportButton := nodewith.Name("Send new report").Role(role.Button).Ancestor(feedbackRootNode)
	if err := ui.DoDefault(sendNewReportButton)(ctx); err != nil {
		s.Fatal("Failed to find send new report button: ", err)
	}

	// Verifying previously entered issue description is cleared.
	issueDescription := nodewith.Name(feedbackapp.IssueText).Role(role.StaticText).Ancestor(feedbackRootNode)
	if err := ui.WaitUntilExists(issueDescription)(ctx); err == nil {
		s.Fatal("Previously entered issue description still exists")
	}

	// Going through submit a new report process.
	issueDescriptionInput := nodewith.Role(role.TextField).Ancestor(feedbackRootNode)
	if err := ui.EnsureFocused(issueDescriptionInput)(ctx); err != nil {
		s.Fatal("Failed to find the issue description text input: ", err)
	}

	// Setting up keyboard and entering issue description.
	kb, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to find keyboard: ", err)
	}
	defer kb.Close()

	if err := kb.Type(ctx, "This time I am not able to open camera"); err != nil {
		s.Fatal("Failed to enter issue description: ", err)
	}

	// Looking for continue button and then click.
	button := nodewith.Name("Continue").Role(role.Button).Ancestor(feedbackRootNode)
	if err := ui.DoDefault(button)(ctx); err != nil {
		s.Fatal("Failed to click continue button: ", err)
	}

	// Looking for send button and then click.
	sendButton := nodewith.Name("Send").Role(role.Button).Ancestor(feedbackRootNode)
	if err := ui.DoDefault(sendButton)(ctx); err != nil {
		s.Fatal("Failed to click send button: ", err)
	}

	// Verifying confirmation page title exists.
	confirmationPageTitle := nodewith.Name("Thanks for your feedback").Role(role.StaticText).Ancestor(feedbackRootNode)
	if err := ui.WaitUntilExists(confirmationPageTitle)(ctx); err != nil {
		s.Fatal("Failed to find confirmation page title: ", err)
	}
}

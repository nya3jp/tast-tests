// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package feedback

import (
	"context"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
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
		Func:         SubmitFeedbackAndCloseApp,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "User is able to submit a report and then close feedback app",
		Contacts: []string{
			"zhangwenyu@google.com",
			"xiangdongkong@google.com",
			"cros-feedback-app@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
	})
}

// SubmitFeedbackAndCloseApp verifies the user can submit a report and
// close Feedback app.
func SubmitFeedbackAndCloseApp(ctx context.Context, s *testing.State) {
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()

	s.Log("Setting up chrome")
	cr, err := chrome.New(ctx, chrome.EnableFeatures("OsFeedback"))
	if err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}
	defer cr.Close(cleanupCtx)

	s.Log("Setting up test API")
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect Test API: ", err)
	}
	defer faillog.DumpUITreeOnError(cleanupCtx, s.OutDir(), s.HasError, tconn)

	ui := uiauto.New(tconn)

	s.Log("Launching feedback app and go to confirmation page")
	feedbackRootNode, err := feedbackapp.LaunchAndGoToConfirmationPage(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to launch feedback app and navigating to confirmation page: ", err)
	}

	s.Log("Verifying page title exists in the confirmation page")
	title := nodewith.Name("Thanks for your feedback").Role(role.StaticText).Ancestor(
		feedbackRootNode)
	if err := ui.WithTimeout(20 * time.Second).WaitUntilExists(title)(ctx); err != nil {
		s.Fatal("Failed to find page title: ", err)
	}

	s.Log("Verifying send new report button exists in the confirmation page")
	newReportButton := nodewith.Name("Send new report").Role(role.Button).Ancestor(
		feedbackRootNode)
	if err := ui.WithTimeout(20 * time.Second).WaitUntilExists(newReportButton)(
		ctx); err != nil {
		s.Fatal("Failed to find new report button: ", err)
	}

	s.Log("Verifying explore app link exists in the confirmation page")
	exploreAppLink := nodewith.NameContaining("Explore app").Role(role.Link).Ancestor(
		feedbackRootNode)
	if err := ui.WithTimeout(20 * time.Second).WaitUntilExists(exploreAppLink)(
		ctx); err != nil {
		s.Fatal("Failed to find explore app link: ", err)
	}
	s.Log("Verifying diagnostics app link exists in the confirmation page")
	diagnosticsAppLink := nodewith.NameContaining("Diagnostics app").Role(role.Link).Ancestor(
		feedbackRootNode)
	if err := ui.WithTimeout(20 * time.Second).WaitUntilExists(diagnosticsAppLink)(
		ctx); err != nil {
		s.Fatal("Failed to find diagnostics app link: ", err)
	}

	s.Log("Looking for done button and click")
	doneButton := nodewith.Name("Done").Role(role.Button).Ancestor(feedbackRootNode)
	if err := uiauto.Combine("Click done button",
		ui.WaitUntilExists(doneButton),
		ui.LeftClick(doneButton),
	)(ctx); err != nil {
		s.Fatal("Failed to find done button: ", err)
	}

	if err := testing.Poll(ctx, func(ctx context.Context) error {
		s.Log("Verifying feedback window is closed")
		if err := ash.WaitForApp(ctx, tconn, apps.Feedback.ID, 10*time.Second); err == nil {
			return errors.Wrap(err, "feedback app is not closed")
		}

		return nil
	}, &testing.PollOptions{Timeout: time.Minute}); err != nil {
		s.Fatal("Failed to verify feedback window is closed: ", err)
	}
}

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
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         NavigateBetweenSearchAndShareDataPage,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "User is able to navigate between the search and share data page",
		Contacts: []string{
			"zhangwenyu@google.com",
			"xiangdongkong@google.com",
			"cros-feedback-app@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
	})
}

// NavigateBetweenSearchAndShareDataPage verifies the user can navigate
// between the search and share data page.
func NavigateBetweenSearchAndShareDataPage(ctx context.Context, s *testing.State) {
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
		s.Fatal("Failed to connect to Test API: ", err)
	}
	defer faillog.DumpUITreeOnError(cleanupCtx, s.OutDir(), s.HasError, tconn)

	ui := uiauto.New(tconn)

	s.Log("Launching feedback app and navigating to share data page")
	feedbackRootNode, err := feedbackapp.LaunchAndGoToShareDataPage(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to launch feedback app and go to share data page: ", err)
	}

	s.Log("Verifying send button exists in the share data page")
	sendButton := nodewith.Name("Send").Role(role.Button).Ancestor(feedbackRootNode)
	if err := ui.WithTimeout(20 * time.Second).WaitUntilExists(sendButton)(ctx); err != nil {
		s.Fatal("Failed to find send button: ", err)
	}

	s.Log("Verifying attach files section exists in the share data page")
	attachfilesTitle := nodewith.Name("Attach files").Role(role.StaticText).Ancestor(
		feedbackRootNode)
	if err := ui.WithTimeout(20 * time.Second).WaitUntilExists(attachfilesTitle)(
		ctx); err != nil {
		s.Fatal("Failed to find attach files section: ", err)
	}

	s.Log("Verifying email section exists in the share data page")
	emailTitle := nodewith.Name("Email").Role(role.StaticText).Ancestor(feedbackRootNode)
	if err := ui.WithTimeout(20 * time.Second).WaitUntilExists(emailTitle)(ctx); err != nil {
		s.Fatal("Failed to find email section: ", err)
	}

	s.Log("Verifying share diagnostic data section exists in the share data page")
	shareDiagnosticDataTitle := nodewith.Name("Share diagnostic data").Role(
		role.StaticText).Ancestor(feedbackRootNode)
	if err := ui.WithTimeout(20 * time.Second).WaitUntilExists(shareDiagnosticDataTitle)(
		ctx); err != nil {
		s.Fatal("Failed to find share diagnostic data section: ", err)
	}

	s.Log("Looking for back button and click")
	backButton := nodewith.Name("Back").Role(role.Button).Ancestor(feedbackRootNode)
	if err := uiauto.Combine("Click back button",
		ui.WaitUntilExists(backButton),
		ui.LeftClick(backButton),
	)(ctx); err != nil {
		s.Fatal("Failed to click back button: ", err)
	}

	s.Log("Verifying the issue description input stores the text user entered previously")
	issueDescription := nodewith.Name(feedbackapp.IssueText).Role(role.StaticText).Ancestor(
		feedbackRootNode)
	if err := ui.WithTimeout(20 * time.Second).WaitUntilExists(issueDescription)(
		ctx); err != nil {
		s.Fatal("Failed to find issue description user entered previously: ", err)
	}
}

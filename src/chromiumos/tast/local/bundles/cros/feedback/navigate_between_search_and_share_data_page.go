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
		Fixture:      "chromeLoggedInWithOsFeedback",
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Timeout:      2 * time.Minute,
	})
}

// NavigateBetweenSearchAndShareDataPage verifies the user can navigate
// between the search and share data page.
func NavigateBetweenSearchAndShareDataPage(ctx context.Context, s *testing.State) {
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

	// Launch feedback app and go to share data page.
	feedbackRootNode, err := feedbackapp.LaunchAndGoToShareDataPage(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to launch feedback app and go to share data page: ", err)
	}

	// Verify essential elements exist in the share data page.
	sendButton := nodewith.Name("Send").Role(role.Button).Ancestor(feedbackRootNode)
	attachfilesTitle := nodewith.Name("Attach files").Role(role.StaticText).Ancestor(
		feedbackRootNode)
	emailTitle := nodewith.Name("Email").Role(role.StaticText).Ancestor(feedbackRootNode)
	shareDiagnosticDataTitle := nodewith.Name("Share diagnostic data").Role(
		role.StaticText).Ancestor(feedbackRootNode)

	if err := uiauto.Combine("Verify essential elements exist",
		ui.WaitUntilExists(sendButton),
		ui.WaitUntilExists(attachfilesTitle),
		ui.WaitUntilExists(emailTitle),
		ui.WaitUntilExists(shareDiagnosticDataTitle),
	)(ctx); err != nil {
		s.Fatal("Failed to find element: ", err)
	}

	// Find back button and click.
	backButton := nodewith.Name("Back").Role(role.Button).Ancestor(feedbackRootNode)
	if err := ui.DoDefault(backButton)(ctx); err != nil {
		s.Fatal("Failed to click back button: ", err)
	}

	// Verify the issue description input stores the text user entered previously.
	issueDescription := nodewith.Name(feedbackapp.IssueText).Role(role.StaticText).Ancestor(
		feedbackRootNode)
	if err := ui.WaitUntilExists(issueDescription)(ctx); err != nil {
		s.Fatal("Failed to find issue description user entered previously: ", err)
	}
}

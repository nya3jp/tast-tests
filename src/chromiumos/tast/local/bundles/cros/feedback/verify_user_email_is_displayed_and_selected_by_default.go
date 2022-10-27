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

const defaultEmailName = "user email"

func init() {
	testing.AddTest(&testing.Test{
		Func:         VerifyUserEmailIsDisplayedAndSelectedByDefault,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Verify user email is displayed and selected by default",
		Contacts: []string{
			"wangdanny@google.com",
			"zhangwenyu@google.com",
			"xiangdongkong@google.com",
			"cros-feedback-app@google.com",
		},
		Fixture:      "chromeLoggedInWithOsFeedback",
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Timeout:      3 * time.Minute,
	})
}

// VerifyUserEmailIsDisplayedAndSelectedByDefault verifies user email is
// displayed and selected by default.
func VerifyUserEmailIsDisplayedAndSelectedByDefault(ctx context.Context, s *testing.State) {
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

	// Verify user email is displayed by default.
	emailDropdown := nodewith.Name("Select email").Role(
		role.ListBox).Ancestor(feedbackRootNode)
	emailDropdownInfo, err := ui.Info(ctx, emailDropdown)
	if err != nil {
		s.Fatal("Failed to get email dropdown info: ", err)
	}
	if emailDropdownInfo.Value != defaultEmailName {
		s.Fatal("Failed to verify user email is displayed by default")
	}

	// Verify user email is selected by default.
	userEmail := nodewith.Name(defaultEmailName).Role(role.ListBoxOption)
	if err := ui.LeftClickUntil(emailDropdown, ui.WithTimeout(
		2*time.Second).WaitUntilExists(userEmail))(ctx); err != nil {
		s.Fatal("Failed to get user email: ", err)
	}
	userEmailInfo, err := ui.Info(ctx, userEmail)
	if err != nil {
		s.Fatal("Failed to get user email info: ", err)
	}
	if !userEmailInfo.Selected {
		s.Fatal("Failed to verify user email is selected by default")
	}
}

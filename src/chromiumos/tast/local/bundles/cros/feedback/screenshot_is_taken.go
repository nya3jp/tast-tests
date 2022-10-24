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
		Func:         ScreenshotIsTaken,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Verify the screenshot is taken in share data page",
		Contacts: []string{
			"zhangwenyu@google.com",
			"xiangdongkong@google.com",
			"cros-feedback-app@google.com",
		},
		Fixture:      "chromeLoggedInWithOsFeedback",
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Timeout:      5 * time.Minute,
	})
}

// ScreenshotIsTaken verifies the screenshot is taken when user navigates to share data page.
func ScreenshotIsTaken(ctx context.Context, s *testing.State) {
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

	// Verify screenshot checkbox and image exist.
	// Verify clicking screenshot will open screenshot diaglog.
	screenshotCheckBox := nodewith.Name("Screenshot").Role(role.CheckBox).Ancestor(
		feedbackRootNode)
	screenshotImg := nodewith.Role(role.Image).Ancestor(feedbackRootNode)
	screenshotDialog := nodewith.Role(role.Dialog).Ancestor(feedbackRootNode).First()

	if err := uiauto.Combine("Verify screenshot exists",
		ui.WaitUntilExists(screenshotCheckBox),
		ui.DoDefault(screenshotImg),
		ui.WaitUntilExists(screenshotDialog),
	)(ctx); err != nil {
		s.Fatal("Failed to verify screenshot exists: ", err)
	}

	// Verify clicking screenshot button will close screenshot diaglog.
	screenshotButton := nodewith.Name("Back").Role(role.Button).Ancestor(feedbackRootNode)

	if err := uiauto.Combine("Verify clicking screenshot button closes dialog",
		ui.DoDefault(screenshotButton),
		ui.WaitUntilGone(screenshotDialog),
	)(ctx); err != nil {
		s.Fatal("Failed to verify clicking screenshot button closes dialog: ", err)
	}
}

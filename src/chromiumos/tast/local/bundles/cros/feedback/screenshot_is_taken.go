// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package feedback

import (
	"context"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
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
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Timeout:      5 * time.Minute,
	})
}

// ScreenshotIsTaken verifies the screenshot is taken when user navigates to share data page.
func ScreenshotIsTaken(ctx context.Context, s *testing.State) {
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

	// Launching feedback app and go to share data page.
	feedbackRootNode, err := feedbackapp.LaunchAndGoToShareDataPage(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to launch feedback app and go to share data page: ", err)
	}

	// Verifying screenshot label and image exist.
	// Verifying clicking screenshot will open screenshot diaglog.
	screenshotLabel := nodewith.Name("Screenshot").Role(role.StaticText).Ancestor(
		feedbackRootNode)
	screenshotImg := nodewith.Name("$i18n{screenshotA11y}").Role(role.Image).Ancestor(
		feedbackRootNode)
	screenshotDialog := nodewith.Role(role.Dialog).Ancestor(feedbackRootNode).First()
	screenshotButton := nodewith.Name("Screenshot").Role(role.Button).Ancestor(feedbackRootNode)

	if err := uiauto.Combine("Verify screenshot exists",
		ui.WaitUntilExists(screenshotLabel),
		ui.DoDefault(screenshotImg),
		ui.WaitUntilExists(screenshotDialog),
		ui.DoDefault(screenshotButton),
	)(ctx); err != nil {
		s.Fatal("Failed to verify screenshot exists: ", err)
	}

	// Verifying clicking screenshot button will close screenshot diaglog.
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		if err := ui.WaitUntilExists(screenshotDialog)(ctx); err == nil {
			return errors.Wrap(err, "failed to close screenshot diaglog")
		}

		return nil
	}, &testing.PollOptions{Timeout: time.Minute}); err != nil {
		s.Fatal("Failed to close screenshot diaglog: ", err)
	}
}

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
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         ShowMissingDescriptionErrorMessage,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Show missing description error when continue without description",
		Contacts: []string{
			"wangdanny@google.com",
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

// ShowMissingDescriptionErrorMessage verifies the user click continue button
// with no issue description will show error message. Now enter text, error message will disappear.
func ShowMissingDescriptionErrorMessage(ctx context.Context, s *testing.State) {
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

	// Set up keyboard.
	kb, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to find keyboard: ", err)
	}
	defer kb.Close()

	// Launch feedback app.
	feedbackRootNode, err := feedbackapp.Launch(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to launch feedback app: ", err)
	}

	// Find continue button and click it.
	continueButton := nodewith.Name("Continue").Role(role.Button)
	if err := ui.DoDefault(continueButton)(ctx); err != nil {
		s.Fatal("Failed to click continue button: ", err)
	}

	// Verify missing description error message exists.
	missingDescriptionErrorMessage := nodewith.Name("Description is required").Role(
		role.StaticText).Ancestor(feedbackRootNode)
	if err := ui.WaitUntilExists(missingDescriptionErrorMessage)(ctx); err != nil {
		s.Fatal("Failed to find missing description error message: ", err)
	}

	// Find the issue description text input.
	issueDescriptionInput := nodewith.Role(role.TextField).Ancestor(feedbackRootNode)
	if err := ui.EnsureFocused(issueDescriptionInput)(ctx); err != nil {
		s.Fatal("Failed to find the issue description text input: ", err)
	}

	// Type issue description.
	if err := kb.Type(ctx, "Make the Description is required disappear"); err != nil {
		s.Fatal("Failed to type issue description: ", err)
	}

	// Verify missing description error message is disappeared.
	if err := ui.WaitUntilGone(missingDescriptionErrorMessage)(ctx); err != nil {
		s.Fatal("Failed to make missing description error message disappeared: ", err)
	}
}

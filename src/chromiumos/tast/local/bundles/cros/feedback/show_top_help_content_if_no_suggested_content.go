// Copyright 2022 The ChromiumOS Authors
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
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

const descriptionWithoutSuggestedContent = "$$$$$$$$$$$$$$$$$$$$$$$$$"

func init() {
	testing.AddTest(&testing.Test{
		Func:         ShowTopHelpContentIfNoSuggestedContent,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "User can get top help content if no suggested content shows",
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

// ShowTopHelpContentIfNoSuggestedContent verifies the user enter long description
// or anything with no possible help content will show top help content.
func ShowTopHelpContentIfNoSuggestedContent(ctx context.Context, s *testing.State) {
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

	// Find the issue description text input.
	issueDescriptionInput := nodewith.Role(role.TextField).Ancestor(feedbackRootNode)
	if err := ui.EnsureFocused(issueDescriptionInput)(ctx); err != nil {
		s.Fatal("Failed to find the issue description text input: ", err)
	}

	if err := testing.Poll(ctx, func(ctx context.Context) error {
		// Type issue description.
		if err := kb.Type(ctx, descriptionWithoutSuggestedContent); err != nil {
			return errors.Wrap(err, "failed to type issue description")
		}
		// Verify top help content title exists.
		title := nodewith.Name("No suggested content. See top help content.").Role(
			role.StaticText).Ancestor(feedbackRootNode)
		if err := ui.WaitUntilExists(title)(ctx); err != nil {
			return errors.Wrap(err, "failed to find the no suggested content title")
		}
		return nil
	}, &testing.PollOptions{Timeout: 20 * time.Second}); err != nil {
		s.Fatal("Failed to show top help content: ", err)
	}
}

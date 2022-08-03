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
		Func:         LongDescriptionShowTopHelpContent,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "User is able to navigate between the search and share data page",
		Contacts: []string{
			"wangdanny@google.com",
			"zhangwenyu@google.com",
			"xiangdongkong@google.com",
			"cros-feedback-app@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
	})
}

// LongDescriptionShowTopHelpContent verifies the user enter long description
// or anything with no possible help content will show top help content.
func LongDescriptionShowTopHelpContent(ctx context.Context, s *testing.State) {
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

	// Type issue description with long description.
	if err := kb.Type(ctx, "long description with no suggested help content: abcdefghi"); err != nil {
		s.Fatal("Failed to type long issue description: ", err)
	}

	// Verify help content title exists.
	title := nodewith.Name("No suggested content. See top help content.").Role(role.StaticText).Ancestor(feedbackRootNode)
	if err := ui.WaitUntilExists(title)(ctx); err != nil {
		s.Fatal("Failed to find the help content title: ", err)
	}

	// Clear the text field
	if err := kb.Accel(ctx, "Ctrl+a+Backspace"); err != nil {
		s.Fatal("Failed pressing Ctrl+a+Backspace: ", err)
	}

	// Type issue description with no possible help content.
	if err := kb.Type(ctx, "$$$"); err != nil {
		s.Fatal("Failed to type issue description with no possible help content: ", err)
	}

	// Verify help content title exists.
	updatedTitle := nodewith.Name("No suggested content. See top help content.").Role(role.StaticText).Ancestor(feedbackRootNode)
	if err := ui.WaitUntilExists(updatedTitle)(ctx); err != nil {
		s.Fatal("Failed to find the help content title: ", err)
	}
}

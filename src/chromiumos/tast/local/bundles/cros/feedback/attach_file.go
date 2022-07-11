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
		Func:         AttachFile,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Verify user can attach a file",
		Contacts: []string{
			"zhangwenyu@google.com",
			"xiangdongkong@google.com",
			"cros-feedback-app@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
	})
}

// AttachFile verifies user can attach a file.
func AttachFile(ctx context.Context, s *testing.State) {
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

	// Take a screenshot and generate a png file for uploading purpose.
	if err := kb.Accel(ctx, "Ctrl+F4"); err != nil {
		s.Fatal("Failed to take a screenshot: ", err)
	}

	// Launch feedback app and go to share data page.
	feedbackRootNode, err := feedbackapp.LaunchAndGoToShareDataPage(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to launch feedback app and go to share data page: ", err)
	}

	// Find add file button and click.
	addFileButton := nodewith.NameContaining("Add file").Role(
		role.Button).Ancestor(feedbackRootNode)
	if err := ui.DoDefault(addFileButton)(ctx); err != nil {
		s.Fatal("Failed to click add file button: ", err)
	}

	// Open Downloads dir and select the screenshot file to upload.
	if err := uiauto.Combine("Open Downloads dir and select PNG file",
		ui.LeftClick(nodewith.Name("Downloads").Role(role.TreeItem)),
		ui.LeftClick(nodewith.NameStartingWith("Screenshot").Role(role.StaticText).Ancestor(
			nodewith.Role(role.ListBox)).First()),
		ui.LeftClick(nodewith.Name("Open").Role(role.Button)),
	)(ctx); err != nil {
		s.Fatal("Failed to open Downloads dir and select PNG file: ", err)
	}

	// Verify the uploaded screenshot exists.
	screenshot := nodewith.NameContaining(".png").Role(
		role.StaticText).Ancestor(feedbackRootNode)
	if err := ui.WaitUntilExists(screenshot)(ctx); err != nil {
		s.Fatal("Failed to find screenshot: ", err)
	}

	// Find replace button and click.
	replaceButton := nodewith.NameContaining("Replace").Role(
		role.Button).Ancestor(feedbackRootNode)
	if err := ui.DoDefault(replaceButton)(ctx); err != nil {
		s.Fatal("Failed to click replace button: ", err)
	}

	// Change file name.
	if err := uiauto.Combine("Open Downloads dir and click PNG file",
		ui.LeftClick(nodewith.Name("Downloads").Role(role.TreeItem)),
		ui.LeftClick(nodewith.NameStartingWith("Screenshot").Role(role.StaticText).Ancestor(
			nodewith.Role(role.ListBox)).First()),
	)(ctx); err != nil {
		s.Fatal("Failed to open Downloads dir and click PNG file: ", err)
	}

	if err := kb.Accel(ctx, "Ctrl+Enter"); err != nil {
		s.Fatal("Failed to begin changing file name: ", err)
	}

	if err := kb.Type(ctx, "my-file"); err != nil {
		s.Fatal("Failed to type new file name: ", err)
	}

	if err := kb.Accel(ctx, "Enter"); err != nil {
		s.Fatal("Failed to confirm name change: ", err)
	}

	// Select new file to upload.
	if err := ui.LeftClick(nodewith.Name("Open").Role(role.Button))(ctx); err != nil {
		s.Fatal("Failed to select file: ", err)
	}

	// Verify new uploaded file exists.
	newFile := nodewith.Name("my-file.png").Role(role.StaticText).Ancestor(feedbackRootNode)
	if err := ui.WaitUntilExists(newFile)(ctx); err != nil {
		s.Fatal("Failed to find new file: ", err)
	}
}

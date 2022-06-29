// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package notifications

import (
	"context"
	"fmt"
	"regexp"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/chrome/webutil"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         InlineReply,
		LacrosStatus: testing.LacrosVariantNeeded,
		Desc:         "Verify inline reply for Chrome notification works",
		Contacts: []string{
			"sun.tsai@cienet.com",
			"cienet-development@googlegroups.com",
			"chromeos-sw-engprod@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Fixture:      "chromeLoggedIn",
	})
}

// InlineReply verifies that inline reply for Chrome notification works.
func InlineReply(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(*chrome.Chrome)

	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()

	// TODO(crbug.com/1311030): Update this website with the notification test page when it is fully functional.
	conn, err := cr.NewConn(ctx, "https://tests.peter.sh/notification-generator")
	if err != nil {
		s.Fatal("Failed to open the web page: ", err)
	}
	defer conn.Close()
	defer conn.CloseTarget(cleanupCtx)
	defer faillog.DumpUITreeWithScreenshotOnError(cleanupCtx, s.OutDir(), s.HasError, cr, "ui_dump")

	if err := webutil.WaitForQuiescence(ctx, conn, 15*time.Second); err != nil {
		s.Fatal("Failed to wait for page to achieve quiescence: ", err)
	}

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect Test API: ", err)
	}

	ui := uiauto.New(tconn)
	permissionBubble := nodewith.Name("tests.peter.sh wants to").HasClass("PermissionPromptBubbleView").Role(role.Window)
	if err := uiauto.IfSuccessThen(
		ui.WaitUntilExists(permissionBubble),
		ui.LeftClick(nodewith.Name("Allow").Ancestor(permissionBubble).Role(role.Button)),
	)(ctx); err != nil {
		s.Fatal("Failed to allow the notification permission: ", err)
	}

	selectSettingExpr := `
		var setting = document.getElementById("%s");
		var options = setting.options;
		var targetOptionText = "%s";
		Array.from(options).forEach(option => {
			if (option.text == targetOptionText) {
				setting.value = option.value;
				return;
			}
		});

		var currentOption = options[options.selectedIndex]
		if (currentOption.text != targetOptionText) {
			throw new Error("expected: " + targetOptionText + ", got: " + currentOption.text)
		}
	`
	// Select notification setting.
	if err := conn.Eval(ctx, fmt.Sprintf(selectSettingExpr, "actions", "One action (type text)"), nil); err != nil {
		s.Fatal("Failed to select notification setting: ", err)
	}
	// Select reaction setting.
	if err := conn.Eval(ctx, fmt.Sprintf(selectSettingExpr, "action", "Display an alert()."), nil); err != nil {
		s.Fatal("Failed to select reaction setting: ", err)
	}

	sendBtn := nodewith.Name("Display the notification").Role(role.Button)
	if err := uiauto.Combine("send notification",
		ui.WaitUntilExists(sendBtn), // This is to make sure that the node exists in the UI tree before calling MakeVisible on it.
		ui.MakeVisible(sendBtn),
		ui.LeftClick(sendBtn),
	)(ctx); err != nil {
		s.Fatal("Failed to complete the combined steps: ", err)
	}

	kb, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to get the keyboard: ", err)
	}
	defer kb.Close()

	const replyMsg = "Test reply message"
	notificationWindow := nodewith.HasClass("ash/message_center/MessagePopup").Role(role.Window)
	browserWindow := nodewith.Name("Chrome - Notification Generator | Peter.sh").HasClass("BrowserFrame").Role(role.Window)
	if err := uiauto.Combine("send reply",
		ui.WaitUntilExists(notificationWindow),
		ui.LeftClick(nodewith.NameRegex(regexp.MustCompile("(?i)reply")).Role(role.Button).Ancestor(notificationWindow)),
		ui.EnsureFocused(nodewith.HasClass("Textfield").Role(role.TextField).Ancestor(notificationWindow)),
		kb.TypeAction(replyMsg),
		kb.AccelAction("Enter"),
		ui.WaitUntilExists(nodewith.Name(fmt.Sprintf(`Clicked on "Notification title" (action: "0", reply: "%s")`, replyMsg)).Ancestor(browserWindow)),
	)(ctx); err != nil {
		s.Fatal("Failed to complete the combined steps: ", err)
	}
}

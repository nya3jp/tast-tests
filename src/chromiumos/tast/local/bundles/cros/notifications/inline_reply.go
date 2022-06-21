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
	"chromiumos/tast/local/chrome/browser"
	"chromiumos/tast/local/chrome/browser/browserfixt"
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
		LacrosStatus: testing.LacrosVariantExists,
		Desc:         "Verify inline reply for Chrome notification works",
		Contacts: []string{
			"sun.tsai@cienet.com",
			"cienet-development@googlegroups.com",
			"chromeos-sw-engprod@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Params: []testing.Param{{
			Fixture: "chromeLoggedIn",
			Val:     browser.TypeAsh,
		}, {
			Name:              "lacros",
			ExtraSoftwareDeps: []string{"lacros"},
			Fixture:           "lacros",
			Val:               browser.TypeLacros,
		}},
	})
}

// InlineReply verifies that inline reply for Chrome notification works.
func InlineReply(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(chrome.HasChrome).Chrome()

	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	br, closeBrowser, err := browserfixt.SetUp(ctx, cr, s.Param().(browser.Type))
	if err != nil {
		s.Fatal("Failed to set up browser: ", err)
	}
	defer closeBrowser(cleanupCtx)

	// TODO(crbug.com/1311030): Update this website with the notification test page when it is fully functional.
	conn, err := br.NewConn(ctx, "https://tests.peter.sh/notification-generator")
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
		// TODO(b/236799853): Use ui.LeftClick after node position mismatching issue is resolved.
		ui.DoDefault(nodewith.Name("Allow").Ancestor(permissionBubble).Role(role.Button)),
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
		// TODO(b/236799853): Use ui.LeftClick after node position mismatching issue is resolved.
		ui.DoDefault(sendBtn),
	)(ctx); err != nil {
		s.Fatal("Failed to complete the combined steps: ", err)
	}

	kb, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to get the keyboard: ", err)
	}
	defer kb.Close()

	const replyMsg = "Test reply message"
	ashNotificationWindow := nodewith.HasClass("ash/message_center/MessagePopup").Role(role.Window)
	browserNotificationWindow := nodewith.Name("tests.peter.sh says").HasClass("JavaScriptTabModalDialogViewViews").Role(role.Window)
	if err := uiauto.Combine("send reply",
		ui.WaitUntilExists(ashNotificationWindow),
		ui.LeftClick(nodewith.NameRegex(regexp.MustCompile("(?i)REPLY")).Role(role.Button).Ancestor(ashNotificationWindow)),
		ui.EnsureFocused(nodewith.HasClass("Textfield").Role(role.TextField).Ancestor(ashNotificationWindow)),
		kb.TypeAction(replyMsg),
		kb.AccelAction("Enter"),
		ui.WaitUntilExists(nodewith.Name(fmt.Sprintf(`Clicked on "Notification title" (action: "0", reply: "%s")`, replyMsg)).Ancestor(browserNotificationWindow)),
	)(ctx); err != nil {
		s.Fatal("Failed to complete the combined steps: ", err)
	}
}

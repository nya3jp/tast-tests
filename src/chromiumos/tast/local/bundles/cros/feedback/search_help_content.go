// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package feedback

import (
	"context"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
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
		Func:         SearchHelpContent,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Suggested help content is updated as user enters issue description",
		Contacts: []string{
			"zhangwenyu@google.com",
			"xiangdongkong@google.com",
			"cros-feedback-app@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
	})
}

// SearchHelpContent verifies the suggested help content will be updated as user
// enters issue description.
func SearchHelpContent(ctx context.Context, s *testing.State) {
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()

	s.Log("Set up chrome")
	cr, err := chrome.New(ctx, chrome.EnableFeatures("OsFeedback"))
	if err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}
	defer cr.Close(cleanupCtx)

	s.Log("Set up test API")
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect Test API: ", err)
	}
	defer faillog.DumpUITreeOnError(cleanupCtx, s.OutDir(), s.HasError, tconn)

	ui := uiauto.New(tconn)

	s.Log("Set up keyboard")
	kb, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to find keyboard: ", err)
	}
	defer kb.Close()

	s.Log("Launch feedback app")
	feedbackRootNode, err := feedbackapp.Launch(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to launch feedback app: ", err)
	}

	s.Log("Verify help content title")
	title := nodewith.Name("Top help content").Role(role.StaticText).Ancestor(feedbackRootNode)
	if err := ui.WaitUntilExists(title)(ctx); err != nil {
		s.Fatal("Failed to find the help content title: ", err)
	}

	s.Log("Verify there are five default help links")
	helpLink := nodewith.Role(role.Link).Ancestor(nodewith.Role(role.Iframe))
	for i := 0; i < 5; i++ {
		item := helpLink.Nth(i)
		if err := ui.WithTimeout(20 * time.Second).WaitUntilExists(item)(ctx); err != nil {
			s.Fatal("Failed to find five help links: ", err)
		}
	}

	s.Log("Find the issue description text input")
	issueDescriptionInput := nodewith.Role(role.TextField).Ancestor(feedbackRootNode)
	if err := uiauto.Combine("Focus text field",
		ui.WaitUntilExists(issueDescriptionInput),
		ui.EnsureFocused(issueDescriptionInput),
	)(ctx); err != nil {
		s.Fatal("Failed to find the issue description text input: ", err)
	}

	s.Log("Type issue description")
	if err := kb.Type(ctx, "I am not able to connect to Bluetooth"); err != nil {
		s.Fatal("Failed to type issue description: ", err)
	}

	s.Log("Verify help content title has changed")
	updatedTitle := nodewith.Name("Suggested help content").Role(role.StaticText).Ancestor(
		feedbackRootNode)
	if err := ui.WaitUntilExists(updatedTitle)(ctx); err != nil {
		s.Fatal("Failed to find the updated help content title: ", err)
	}

	s.Log("Verify there are five help content link")
	updatedHelpLink := nodewith.Role(role.Link).Ancestor(nodewith.Role(role.Iframe))
	for i := 0; i < 5; i++ {
		item := updatedHelpLink.Nth(i)
		if err := ui.WithTimeout(20 * time.Second).WaitUntilExists(item)(ctx); err != nil {
			s.Fatal("Failed to find five help links: ", err)
		}
	}

	s.Log("Verify the link can be opened")
	if err := ui.LeftClick(updatedHelpLink.First())(ctx); err != nil {
		s.Fatal("Failed to open help link: ", err)
	}

	err = ash.WaitForApp(ctx, tconn, apps.Chrome.ID, time.Minute)
	if err != nil {
		s.Fatal("Could not find app in shelf after launch: ", err)
	}
}

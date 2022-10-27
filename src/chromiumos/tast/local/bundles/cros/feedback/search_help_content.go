// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package feedback

import (
	"context"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/browser"
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
		LacrosStatus: testing.LacrosVariantExists,
		Desc:         "Suggested help content is updated as user enters issue description",
		Contacts: []string{
			"wangdanny@google.com",
			"zhangwenyu@google.com",
			"xiangdongkong@google.com",
			"cros-feedback-app@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Timeout:      2 * time.Minute,
		Params: []testing.Param{{
			Name:    "ash",
			Fixture: "chromeLoggedInWithOsFeedback",
			Val:     browser.TypeAsh,
		}, {
			Name:    "lacros",
			Fixture: "lacrosOsFeedback",
			Val:     browser.TypeLacros,
		}},
	})
}

// SearchHelpContent verifies the suggested help content will be updated as user
// enters issue description.
func SearchHelpContent(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(chrome.HasChrome).Chrome()

	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect to Test API: ", err)
	}
	defer faillog.DumpUITreeWithScreenshotOnError(cleanupCtx, s.OutDir(), s.HasError, cr,
		"ui_dump")

	ui := uiauto.New(tconn)

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

	// Verify help content title exists.
	title := nodewith.Name("Top help content").Role(role.StaticText).Ancestor(feedbackRootNode)
	if err := ui.WaitUntilExists(title)(ctx); err != nil {
		s.Fatal("Failed to find the help content title: ", err)
	}

	// Verify there are five default help links.
	_, err = verifyLinks(ctx, tconn, 5)
	if err != nil {
		s.Fatal("Failed to find five help links: ", err)
	}

	// Find the issue description text input.
	issueDescriptionInput := nodewith.Role(role.TextField).Ancestor(feedbackRootNode)
	if err := ui.EnsureFocused(issueDescriptionInput)(ctx); err != nil {
		s.Fatal("Failed to find the issue description text input: ", err)
	}

	// Type issue description.
	if err := kb.Type(ctx, "I am not able to connect to Bluetooth"); err != nil {
		s.Fatal("Failed to type issue description: ", err)
	}

	// Verify help content title has changed.
	updatedTitle := nodewith.Name("Suggested help content").Role(role.StaticText).Ancestor(
		feedbackRootNode)
	if err := ui.WaitUntilExists(updatedTitle)(ctx); err != nil {
		s.Fatal("Failed to find the updated help content title: ", err)
	}

	// Verify there are five help content link.
	updatedHelpLink, err := verifyLinks(ctx, tconn, 5)
	if err != nil {
		s.Fatal("Failed to find five help links: ", err)
	}

	// Verify the link can be opened.
	if err := ui.LeftClick(updatedHelpLink.First())(ctx); err != nil {
		s.Fatal("Failed to open help link: ", err)
	}

	bt := s.Param().(browser.Type)

	// Verify browser is opened.
	id := apps.Chrome.ID
	if bt != browser.TypeAsh {
		id = apps.LacrosID
	}
	if err = ash.WaitForApp(ctx, tconn, id, time.Minute); err != nil {
		s.Fatal("Could not find browser in shelf after launch: ", err)
	}
}

// verifyLinks function verifies there are n links existed.
func verifyLinks(ctx context.Context, tconn *chrome.TestConn, n int) (
	*nodewith.Finder, error) {
	ui := uiauto.New(tconn)

	link := nodewith.Role(role.Link).Ancestor(nodewith.Role(role.Iframe))
	for i := 0; i < n; i++ {
		item := link.Nth(i)
		if err := ui.WithTimeout(20 * time.Second).WaitUntilExists(item)(ctx); err != nil {
			return nil, errors.Wrap(err, "failed to find help links")
		}
	}
	return link, nil
}

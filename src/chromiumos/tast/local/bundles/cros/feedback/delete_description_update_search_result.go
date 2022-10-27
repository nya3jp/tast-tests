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

const issueDescriptionText = "I am not able to connect to Bluetooth"

func init() {
	testing.AddTest(&testing.Test{
		Func:         DeleteDescriptionUpdateSearchResult,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Verify help content update when delete part of issue description",
		Contacts: []string{
			"wangdanny@google.com",
			"zhangwenyu@google.com",
			"xiangdongkong@google.com",
			"cros-feedback-app@google.com",
		},
		Fixture:      "chromeLoggedInWithOsFeedback",
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Timeout:      3 * time.Minute,
	})
}

// DeleteDescriptionUpdateSearchResult verifies the help content will update
// when user deletes part of the issue description.
func DeleteDescriptionUpdateSearchResult(ctx context.Context, s *testing.State) {
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

	// Find the first link name in the help content.
	defaultFirstLinkName, err := firstLinkName(ctx, ui)
	if err != nil {
		s.Fatal("Failed to find the first link name: ", err)
	}

	// Find the issue description text input.
	issueDescriptionInput := nodewith.Role(role.TextField).Ancestor(feedbackRootNode)
	if err := ui.EnsureFocused(issueDescriptionInput)(ctx); err != nil {
		s.Fatal("Failed to find the issue description text input: ", err)
	}

	// Type issue description.
	if err := kb.Type(ctx, issueDescriptionText); err != nil {
		s.Fatal("Failed to type issue description: ", err)
	}

	// Find the updated first link name.
	updatedFirstLinkName, err := firstLinkName(ctx, ui)
	if err != nil {
		s.Fatal("Failed to find the updated first link name: ", err)
	}

	// Compare link names to check if the help content is updated.
	if defaultFirstLinkName == updatedFirstLinkName {
		s.Fatal("Failed to test because the link name isn't updated")
	}

	// Delete part of the issue description.
	for i := 0; i < 15; i++ {
		if err := kb.Accel(ctx, "Backspace"); err != nil {
			s.Fatal("Failed to delete part of issue description: ", err)
		}
	}

	// Find the new first link name.
	newFirstLinkName, err := firstLinkName(ctx, ui)
	if err != nil {
		s.Fatal("Failed to find the new first link name: ", err)
	}

	// Compare link names to check if the help content is updated after
	// deleting part of the issue description.
	if updatedFirstLinkName == newFirstLinkName {
		s.Fatal("Failed to test because the link name is still the same as before")
	}
}

// firstLinkName function returns the first link name in the help content.
func firstLinkName(ctx context.Context, ui *uiauto.Context) (
	string, error) {
	link := nodewith.Role(role.Link).Ancestor(nodewith.Role(role.Iframe)).Nth(0)
	var preName string
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		currentNodeInfo, err := ui.Info(ctx, link)
		if err != nil {
			return errors.Wrap(err, "failed to find the first link info during update")
		}
		currentName := currentNodeInfo.Name
		if preName != currentName {
			preName = currentName
			return errors.New("failed to stop polling because still typing text")
		}
		return nil
	}, &testing.PollOptions{
		Interval: 5 * time.Second,
		Timeout:  30 * time.Second,
	}); err != nil {
		return "", errors.Wrap(err, "failed to find the first link name")
	}
	return preName, nil
}

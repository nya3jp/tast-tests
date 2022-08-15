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
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         DeleteDescriptionUpdateSearchResult,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Verify url matches the current website where Feedback app is opened",
		Contacts: []string{
			"zhangwenyu@google.com",
			"xiangdongkong@google.com",
			"cros-feedback-app@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
	})
}

// DeleteDescriptionUpdateSearchResult verifies the url matches the current website from where
// user opens the Feedback app.
func DeleteDescriptionUpdateSearchResult(ctx context.Context, s *testing.State) {
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

	//Find the first link name in the help content.
	firstLinkName, err := getFirstLinkName(ctx, tconn, s)
	if err != nil {
		s.Fatal("Failed to find the first link name: ", err)
	}
	// s.Log("---------------")
	// s.Log(firstLinkName)

	// Find the issue description text input.
	issueDescriptionInput := nodewith.Role(role.TextField).Ancestor(feedbackRootNode)
	if err := ui.EnsureFocused(issueDescriptionInput)(ctx); err != nil {
		s.Fatal("Failed to find the issue description text input: ", err)
	}

	// Type issue description.
	if err := kb.Type(ctx, "I am not able to connect to Bluetooth"); err != nil {
		s.Fatal("Failed to type issue description: ", err)
	}

	// Find the updated first link name
	updatedFirstLinkName, err := getFirstLinkName(ctx, tconn, s)
	if err != nil {
		s.Fatal("Failed to find the updated first link name: ", err)
	}
	// s.Log("---------------")
	// s.Log(updatedFirstLinkName)

	// Compare link names to check if the help content is updated.
	if firstLinkName == updatedFirstLinkName {
		s.Fatal("Failed to test because the link name isn't updated")
	}

	// Delete part of the issue description.
	// newFirstLinkName, err := getFirstLinkName(ctx, tconn, s)
	// if err != nil {
	// 	s.Fatal("Failed to find the new first link name: ", err)
	// }
	// s.Log("---------------")
	// s.Log(newFirstLinkName)

	// Compare link names to check if the help content is updated after
	// deleting part of the issue description.

}

// getFirstLinkName function returns the first link name in the help content.
func getFirstLinkName(ctx context.Context, tconn *chrome.TestConn, s *testing.State) (
	string, error) {
	ui := uiauto.New(tconn)

	link := nodewith.Role(role.Link).Ancestor(nodewith.Role(role.Iframe)).Nth(0)
	currentNodeInfo, err := ui.Info(ctx, link)
	firstLinkName := currentNodeInfo.Name
	if err != nil {
		s.Fatal("Failed to find the first link info: ", err)
	}
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		currentNodeInfo, err := ui.Info(ctx, link)
		updatedFirstLinkName := currentNodeInfo.Name
		if err != nil {
			return errors.Wrap(err, "failed to find the first link info during update")
		}
		s.Log("----------first name: ", firstLinkName)
		s.Log("----------update name: ", updatedFirstLinkName)
		if firstLinkName != updatedFirstLinkName {
			firstLinkName = updatedFirstLinkName
			s.Log("-----------------here here different")
			return errors.New("the names are different")
		}
		return nil
	}, &testing.PollOptions{
		Interval: time.Minute,
		Timeout:  3 * time.Minute,
	}); err != nil {
		s.Fatal("Failed to find the first link name: ", err)
	}
	s.Log("----------------------in func")
	s.Log(firstLinkName)
	return firstLinkName, nil
}

// compareNames function
// func compareNames(name string, tconn *chrome.TestConn) (
// 	string, error) {
// 	ui := uiauto.New(tconn)

// 	link := nodewith.Role(role.Link).Ancestor(nodewith.Role(role.Iframe)).Nth(0)
// 	currentNodeInfo, err := ui.WithTimeout(200 * time.Second).Info(ctx, link)
// 	if err != nil {
// 		return "", errors.Wrap(err, "failed to find the first link info")
// 	}
// 	return currentNodeInfo.Name, nil
// }

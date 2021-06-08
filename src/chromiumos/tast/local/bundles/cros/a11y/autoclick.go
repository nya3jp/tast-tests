// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package a11y provides functions to assist with interacting with accessibility
// features and settings.
package a11y

import (
	"context"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/a11y"
	"chromiumos/tast/local/action"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: Autoclick,
		Desc: "Tests that the automatic clicks feature can be turned on and used to click buttons without physically pressing the mouse",
		Contacts: []string{
			"akihiroota@chromium.org",      // Test author
			"chromeos-a11y-eng@google.com", // Backup mailing list
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Pre:          chrome.LoggedIn(),
	})
}

func Autoclick(ctx context.Context, s *testing.State) {
	cr := s.PreValue().(*chrome.Chrome)
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}

	// Shorten deadline to leave time for cleanup
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()

	// Enable Autoclick.
	if err := a11y.SetFeatureEnabled(ctx, tconn, a11y.Autoclick, true); err != nil {
		s.Fatal("Failed to enable autoclick: ", err)
	}

	enabled := true
	defer func(ctx context.Context) {
		// Verify that autoclick is off at the end of this test.
		if enabled {
			if err := a11y.SetFeatureEnabled(ctx, tconn, a11y.Autoclick, false); err != nil {
				s.Error("Failed to disable autoclick: ", err)
			}
		}
	}(cleanupCtx)

	defer faillog.DumpUITreeOnError(cleanupCtx, s.OutDir(), s.HasError, tconn)

	ui := uiauto.New(tconn)

	// Returns a function that moves the mouse to a node defined by finder.
	// The returned function waits for the node's location to stabilize, then
	// moves the mouse to the node's center point.
	moveMouseToNode := func(finder *nodewith.Finder) action.Action {
		return func(ctx context.Context) error {
			if err := uiauto.Combine("moving mouse to click node",
				ui.WithTimeout(10*time.Second).WaitForLocation(finder),
				ui.MouseMoveTo(finder, time.Second),
			)(ctx); err != nil {
				return errors.Wrap(err, "failed to move mouse to node")
			}
			return nil
		}
	}

	menu := nodewith.ClassName("AutoclickMenuView").Onscreen()
	scrollButton := nodewith.Name("Scroll").ClassName("FloatingMenuButton").Onscreen()
	scrollView := nodewith.ClassName("AutoclickScrollBubbleView").Onscreen().First()
	leftClickButton := nodewith.Name("Left click").ClassName("FloatingMenuButton").Onscreen()

	if err := uiauto.Combine("change Autoclick mode",
		ui.WithTimeout(10*time.Second).WaitUntilExists(menu),
		// Change Autoclick to scroll mode by hovering on the scroll button in the menu.
		moveMouseToNode(scrollButton),
		// Autoclick is in scroll mode once the scroll view appears.
		ui.WithTimeout(10*time.Second).WaitUntilExists(scrollView),
		// Change back to left click mode by hovering on the left click button in the menu.
		moveMouseToNode(leftClickButton),
		// Autoclick is in left click mode once the scroll view disappears.
		ui.WithTimeout(10*time.Second).WaitUntilGone(scrollView),
	)(ctx); err != nil {
		s.Fatal("Failed to change the Autoclick mode: ", err)
	}

	// Turn off autoclick.
	if err := a11y.SetFeatureEnabled(ctx, tconn, a11y.Autoclick, false); err != nil {
		s.Error("Failed to disable autoclick: ", err)
	}

	enabled = false

	dialog := nodewith.Name("Are you sure you want to turn off automatic clicks?").Onscreen()
	yesButton := nodewith.Name("Yes").ClassName("MdTextButton").Onscreen()

	if err := uiauto.Combine("close Autoclick confirmation dialog",
		// Deactivating autoclick should show a confirmation dialog.
		ui.WithTimeout(10*time.Second).WaitUntilExists(dialog),
		// Hovering over the "Yes" button should deactivate autoclick.
		moveMouseToNode(yesButton),
		ui.WithTimeout(10*time.Second).WaitUntilGone(dialog),
	)(ctx); err != nil {
		s.Fatal("Failed to close the Autoclick confirmation dialog: ", err)
	}
}

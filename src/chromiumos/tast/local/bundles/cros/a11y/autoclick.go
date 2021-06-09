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
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	ui := uiauto.New(tconn)

	// Returns a function that moves the mouse to a node defined by finder.
	// The returned function waits for the node's location to stabilize, then
	// moves the mouse to the node's center point.
	moveMouseToNode := func(finder *nodewith.Finder) action.Action {
		return uiauto.Combine("moving mouse to node location",
			ui.WithTimeout(5*time.Second).WaitForLocation(finder),
			ui.MouseMoveTo(finder, time.Second),
		)
	}

	// A function that turns off Autoclick and closes the confirmation dialog.
	// If useAutoclick is true, then we should use Autoclick to close the
	// confirmation dialog e.g. hover over the "yes" button and wait for the
	// dialog to close.
	// If useAutoclick is false, then we should simply click the "yes" button
	// to close the dialog.
	deactivateAutoclick := func(ctx context.Context, useAutoclick bool) error {
		if err := a11y.SetFeatureEnabled(ctx, tconn, a11y.Autoclick, false); err != nil {
			return errors.Wrap(err, "failed to disable autoclick")
		}

		dialog := nodewith.Name("Are you sure you want to turn off automatic clicks?").Onscreen()
		yesButton := nodewith.Name("Yes").ClassName("MdTextButton").Onscreen()

		if useAutoclick {
			if err := uiauto.Combine("close Autoclick confirmation dialog using Autoclick",
				// Deactivating autoclick should show a confirmation dialog.
				ui.WithTimeout(5*time.Second).WaitUntilExists(dialog),
				// Hovering over the "Yes" button should deactivate autoclick.
				moveMouseToNode(yesButton),
				ui.WithTimeout(5*time.Second).WaitUntilGone(dialog),
			)(ctx); err != nil {
				return errors.Wrap(err, "failed to close the Autoclick confirmation dialog using Autoclick")
			}
		} else {
			if err := uiauto.Combine("close Autoclick confirmation dialog using left click",
				ui.WithTimeout(5*time.Second).WaitUntilExists(dialog),
				ui.WithInterval(500*time.Millisecond).LeftClickUntil(yesButton, ui.Gone(dialog)),
			)(ctx); err != nil {
				return errors.Wrap(err, "failed to close the Autoclick confirmation dialog using left click")
			}
		}
		return nil
	}

	// Enable Autoclick.
	if err := a11y.SetFeatureEnabled(ctx, tconn, a11y.Autoclick, true); err != nil {
		s.Fatal("Failed to enable autoclick: ", err)
	}

	enabled := true
	defer func(ctx context.Context) {
		// Verify that autoclick is off at the end of this test.
		if enabled {
			if err := deactivateAutoclick(ctx, false); err != nil {
				s.Fatal("Failed to deactive Autoclick and close the confirmation dialog: ", err)
			}
		}
	}(cleanupCtx)

	defer faillog.DumpUITreeOnError(cleanupCtx, s.OutDir(), s.HasError, tconn)

	menu := nodewith.ClassName("AutoclickMenuView").Onscreen()
	scrollButton := nodewith.Name("Scroll").ClassName("FloatingMenuButton").Onscreen()
	scrollView := nodewith.ClassName("AutoclickScrollBubbleView").Onscreen().First()
	leftClickButton := nodewith.Name("Left click").ClassName("FloatingMenuButton").Onscreen()

	if err := uiauto.Combine("change Autoclick mode",
		ui.WithTimeout(5*time.Second).WaitUntilExists(menu),
		// Change Autoclick to scroll mode by hovering on the scroll button in the menu.
		moveMouseToNode(scrollButton),
		// Autoclick is in scroll mode once the scroll view appears.
		ui.WithTimeout(5*time.Second).WaitUntilExists(scrollView),
		// Change back to left click mode by hovering on the left click button in the menu.
		moveMouseToNode(leftClickButton),
		// Autoclick is in left click mode once the scroll view disappears.
		ui.WithTimeout(5*time.Second).WaitUntilGone(scrollView),
	)(ctx); err != nil {
		s.Fatal("Failed to change the Autoclick mode: ", err)
	}

	if err := deactivateAutoclick(ctx, true); err != nil {
		s.Fatal("Failed to deactivate Autoclick and close the confirmation dialog: ", err)
	}

	enabled = false
}

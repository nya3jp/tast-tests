// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package a11y provides functions to assist with interacting with accessibility
// features and settings.
package a11y

import (
	"context"
	"time"

	"chromiumos/tast/local/a11y"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ui"
	"chromiumos/tast/local/chrome/ui/mouse"
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

	if err := a11y.SetFeatureEnabled(ctx, tconn, a11y.Autoclick, true); err != nil {
		s.Fatal("Failed to enable autoclick: ", err)
	}

	enabled := true
	defer func() {
		// Verify that autoclick is off at the end of this test.
		if enabled {
			if err := a11y.SetFeatureEnabled(ctx, tconn, a11y.Autoclick, false); err != nil {
				s.Error("Failed to disable autoclick: ", err)
			}
		}
	}()

	// Ensure the presence of the floating autoclick menu.
	menu, err := ui.FindWithTimeout(ctx, tconn, ui.FindParams{
		ClassName: "AutoclickMenuView",
		State:     map[ui.StateType]bool{ui.StateTypeOffscreen: false},
	}, 10*time.Second)
	if err != nil {
		s.Fatal("Failed to find the autoclick menu view: ", err)
	}
	defer menu.Release(ctx)

	// Ensure the presence of the scroll button within the autoclick menu and make
	// sure that it's not offscreen.
	scroll, err := ui.FindWithTimeout(ctx, tconn, ui.FindParams{
		Name:      "Scroll",
		ClassName: "FloatingMenuButton",
		State:     map[ui.StateType]bool{ui.StateTypeOffscreen: false},
	}, 10*time.Second)
	if err != nil {
		s.Fatal("Failed to find the autoclick scroll button: ", err)
	}
	defer scroll.Release(ctx)

	// We need to hover the mouse over the scroll button, so make sure the
	// location is stable before we proceed. Otherwise, this can cause undesired
	// failures.
	if err := scroll.WaitLocationStable(ctx, &testing.PollOptions{Interval: 1 * time.Second, Timeout: 10 * time.Second}); err != nil {
		s.Fatal("Failed to wait for the scroll button to have a stable location: ", err)
	}

	// Move the mouse to the middle of the scroll button, which changes autoclick
	// to scroll mode.
	if err := mouse.Move(ctx, tconn, scroll.Location.CenterPoint(), 0); err != nil {
		s.Fatal("Failed to move the mouse to the autoclick scroll button: ", err)
	}

	// Autoclick is in scroll mode once the scroll view appears.
	if err := ui.WaitUntilExists(ctx, tconn, ui.FindParams{ClassName: "AutoclickScrollBubbleView"}, 10*time.Second); err != nil {
		actualScroll, err := ui.FindWithTimeout(ctx, tconn, ui.FindParams{
			Name:      "Scroll",
			ClassName: "FloatingMenuButton",
			State:     map[ui.StateType]bool{ui.StateTypeOffscreen: false},
		}, 10*time.Second)
		if err != nil {
			s.Fatal("Failed to find the autoclick scroll button: ", err)
		}
		defer actualScroll.Release(ctx)
		s.Fatalf("Failed to click the scroll button; expected coordinates: %v, acutal coordinates: %v", scroll.Location.CenterPoint(), actualScroll.Location.CenterPoint())
	}

	// Change back to left click mode by finding the left click button and hovering.
	leftClick, err := ui.FindWithTimeout(ctx, tconn, ui.FindParams{
		Name:      "Left click",
		ClassName: "FloatingMenuButton",
	}, 10*time.Second)
	if err != nil {
		s.Fatal("Failed to find the autoclick left click button: ", err)
	}
	defer leftClick.Release(ctx)

	if err := mouse.Move(ctx, tconn, leftClick.Location.CenterPoint(), 0); err != nil {
		s.Fatal("Failed to move the mouse to the left click button: ", err)
	}

	// Autoclick is in left click mode once the scroll view disappears.
	if err := ui.WaitUntilGone(ctx, tconn, ui.FindParams{ClassName: "AutoclickScrollBubbleView"}, 10*time.Second); err != nil {
		s.Fatal("Failed to change back to left click mode and close the scroll view: ", err)
	}

	// Turn off autoclick.
	if err := a11y.SetFeatureEnabled(ctx, tconn, a11y.Autoclick, false); err != nil {
		s.Error("Failed to disable autoclick: ", err)
	}

	enabled = false

	// Deactivating autoclick should show a confirmation dialog.
	dialog, err := ui.FindWithTimeout(ctx, tconn, ui.FindParams{
		Name:  "Are you sure you want to turn off automatic clicks?",
		State: map[ui.StateType]bool{ui.StateTypeOffscreen: false},
	}, 10*time.Second)
	if err != nil {
		s.Fatal("Failed to find the autoclick confirmation dialog: ", err)
	}
	defer dialog.Release(ctx)

	// Hovering over the "Yes" button should deactivate autoclick.
	yesButton, err := ui.FindWithTimeout(ctx, tconn, ui.FindParams{
		Name:      "Yes",
		ClassName: "MdTextButton",
		State:     map[ui.StateType]bool{ui.StateTypeOffscreen: false},
	}, 10*time.Second)
	if err != nil {
		s.Fatal("Failed to find the yes button in the autoclick confirmation dialog: ", err)
	}
	defer yesButton.Release(ctx)

	if err := mouse.Move(ctx, tconn, yesButton.Location.CenterPoint(), 0); err != nil {
		s.Fatal("Failed to move the mouse to the yes button in the autoclick confirmation dialog: ", err)
	}

	// Wait for the confirmation dialog to disappear.
	if err := ui.WaitUntilGone(ctx, tconn, ui.FindParams{
		Name: "Are you sure you want to turn off automatic clicks?",
	}, 10*time.Second); err != nil {
		s.Fatal("Failed to close the autoclick confirmation dialog: ", err)
	}
}

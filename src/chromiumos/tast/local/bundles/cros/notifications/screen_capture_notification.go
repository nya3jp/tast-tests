// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package notifications

import (
	"context"
	"strings"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/ui"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/clipboard"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/screenshot"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         ScreenCaptureNotification,
		Desc:         "Test the behavior of screen capture notification and make sure that the clipboard and actions buttons work correctly after taking the screenshot",
		Contacts:     []string{"leandre@chromium.org", "cros-status-area-eng@google.com", "chromeos-sw-engprod@google.com"},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:mainline", "informational"},
		Fixture:      "chromeLoggedIn",
	})
}

func ScreenCaptureNotification(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(*chrome.Chrome)

	// Connect to Test API to use it with the UI library.
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}

	// Remove all screenshots at the beginning.
	if err = screenshot.RemoveScreenshots(); err != nil {
		s.Fatal("Failed to remove screenshots: ", err)
	}

	// Initially, the clipboard size should be zero.
	size, err := clipboard.GetClipboardItemsSize(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to get clipboard size: ", err)
	}
	if size != 0 {
		s.Error("Clipboard size should initially be zero")
	}

	// Take a screenshot to show a notification. Using the virtual keyboard is required since
	// different physical keyboards can require different key combinations to take a screenshot.
	vkb, err := input.VirtualKeyboard(ctx)
	if err != nil {
		s.Fatal("Failed to get virtual keyboard: ", err)
	}
	defer vkb.Close()

	if err := vkb.Accel(ctx, "Ctrl+F5"); err != nil {
		s.Fatal("Failed to take a screenshot: ", err)
	}

	params := ui.FindParams{
		Role:      ui.RoleTypeWindow,
		ClassName: "ash/message_center/MessagePopup",
	}

	// Verify that the screen capture popup notification is shown after taking the screenshot.
	if err := ui.WaitUntilExists(ctx, tconn, params, 30*time.Second); err != nil {
		s.Fatal("Failed to find popup notification: ", err)
	}

	// Verify that screenshot is saved in Download folder.
	has, err := screenshot.HasScreenshots()
	if err != nil {
		s.Fatal("Failed to check whether screenshot is present: ", err)
	}
	if !has {
		s.Error("Screenshot should be present in Download folder")
	}

	// Verify that the image has been copied to clipboard
	// The clipboard size should not be zero now.
	size, err = clipboard.GetClipboardItemsSize(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to get clipboard size: ", err)
	}
	if size == 0 {
		s.Error("Clipboard size should not be zero")
	}

	t, err := clipboard.GetClipboardFirstItemType(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to get clipboard item's type: ", err)
	}

	// The item on top of the clipboard should be an image.
	if !strings.Contains(t, "image") {
		s.Error("Clipboard item should be an image instead of ", t)
	}

	ac := uiauto.New(tconn)

	// Click "delete" button.
	deleteButton := nodewith.Name("DELETE").Role(role.Button)
	if err := ac.LeftClick(deleteButton)(ctx); err != nil {
		s.Fatal("Failed to click delete button: ", err)
	}

	if err := testing.Poll(ctx, func(ctx context.Context) error {
		// Verify that our screenshot is deleted (not present in the Download folder).
		has, err = screenshot.HasScreenshots()
		if err != nil {
			return errors.Wrap(err, "failed to check whether screenshot is present")
		}
		if has {
			return errors.New("screenshot should not be present in Download folder")
		}
		return nil
	}, &testing.PollOptions{Timeout: 10 * time.Second}); err != nil {
		s.Fatal("Failed to verify that screenshot is deleted: ", err)
	}

	// Take another screenshot.
	if err := vkb.Accel(ctx, "Ctrl+F5"); err != nil {
		s.Fatal("Failed to take a screenshot: ", err)
	}

	// Click edit button, check if Gallery app open up.
	editButton := nodewith.Name("EDIT").Role(role.Button)
	if err := ac.LeftClick(editButton)(ctx); err != nil {
		s.Fatal("Failed to click edit button: ", err)
	}

	if err := ash.WaitForApp(ctx, tconn, apps.Gallery.ID, time.Minute); err != nil {
		s.Error("Failed to wait for Gallery app to open: ", err)
	}
}

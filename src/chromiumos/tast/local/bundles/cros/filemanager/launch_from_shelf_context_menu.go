// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package filemanager

import (
	"context"
	"strings"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: LaunchFromShelfContextMenu,
		Desc: "Verify Files app opens a single window using New Window from Shelf",
		Contacts: []string{
			"benreich@chromium.org",
			"chromeos-files-syd@google.com",
		},
		Attr:         []string{"group:mainline"},
		SoftwareDeps: []string{"chrome"},
		Fixture:      "chromeLoggedIn",
	})
}

func LaunchFromShelfContextMenu(ctx context.Context, s *testing.State) {
	const (
		newWindowText      = "New window"
		newWindowClassName = "MenuItemView"
	)

	cr := s.FixtValue().(*chrome.Chrome)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to get test API Connection: ", err)
	}
	defer faillog.DumpUITreeWithScreenshotOnError(ctx, s.OutDir(), s.HasError, cr, "multiple_windows")

	// Verify that the Files app should be pinned by default to the shelf for this device.
	if pinned, err := isFilesAppPinnedByDefault(ctx, tconn); err != nil {
		s.Fatal("Failed to retrieve list of pinned apps: ", err)
	} else if !pinned {
		s.Log("Files app is not pinned to the shelf by default on this device")
		return
	}

	kb, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to get keyboard: ", err)
	}
	defer kb.Close()

	ui := uiauto.New(tconn)
	filesAppShelfButton := nodewith.Name(apps.Files.Name).ClassName("ash/ShelfAppButton")
	newWindowContextMenuItem := nodewith.Name(newWindowText).ClassName(newWindowClassName)
	if err := uiauto.Combine("click new window context menu item",
		ui.WaitUntilExists(filesAppShelfButton),
		ui.RightClick(filesAppShelfButton),
		ui.WaitUntilExists(newWindowContextMenuItem),
		ui.LeftClick(newWindowContextMenuItem),
	)(ctx); err != nil {
		s.Fatal("Failed to click New Window on Files app shelf icon: ", err)
	}

	pollOpts := &testing.PollOptions{Timeout: 5 * time.Second, Interval: time.Second}
	filesWindowMatcher := func(w *ash.Window) bool {
		return strings.HasPrefix(w.Title, "Files")
	}

	if err := ash.WaitForCondition(ctx, tconn, filesWindowMatcher, pollOpts); err != nil {
		s.Fatal("Failed to find initial Files app window: ", err)
	}

	// Ensure that no duplicate Files app windows appear after the first one.
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		if _, err := ash.FindOnlyWindow(ctx, tconn, filesWindowMatcher); errors.Is(err, ash.ErrMultipleWindowsFound) {
			return testing.PollBreak(errors.New("failed due to multiple Files app windows"))
		} else if err != nil {
			return testing.PollBreak(errors.Wrap(err, "failed to find the only Files app window"))
		}
		return nil
	}, pollOpts); err != nil {
		s.Fatal("Failed as multiple or no Files app windows visible: ", err)
	}
}

// isFilesAppPinnedByDefault verifies that Files app should be pinned by default
// and is used to ensure Files app will appear in the shelf.
func isFilesAppPinnedByDefault(ctx context.Context, tconn *chrome.TestConn) (bool, error) {
	appIDs, err := ash.GetDefaultPinnedAppIDs(ctx, tconn)
	if err != nil {
		return false, errors.Wrap(err, "failed to get the list of default pinned app IDs")
	}
	for _, id := range appIDs {
		if id == apps.Files.ID {
			return true, nil
		}
	}
	return false, nil
}

// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package browser allows interactions with browser window.
package browser

import (
	"context"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ui"
	"chromiumos/tast/local/chrome/ui/browser/bookmarksbar"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

// GetAddressBarText returns the address bar text.
func GetAddressBarText(ctx context.Context, tconn *chrome.TestConn) (string, error) {
	omniboxItem, err := ui.Find(ctx, tconn, ui.FindParams{
		ClassName: "OmniboxViewViews",
		Name:      "Address and search bar",
	})
	defer omniboxItem.Release(ctx)

	if err != nil {
		return "", errors.Wrap(err, "could not find address bar")
	}
	return omniboxItem.Value, nil
}

// triggerBookmarkThisTab opens bookmark menu for the current tab and
// waits for its element to exist. It adds current tab as a bookmark. If
// bookmarks already exists it opens the bookmark dialog.
func triggerBookmarkThisTab(ctx context.Context, tconn *chrome.TestConn) error {
	var bdFindParams ui.FindParams = ui.FindParams{
		Role: ui.RoleTypeTextField,
		Name: "Bookmark name",
	}

	visible, err := ui.Exists(ctx, tconn, bdFindParams)
	if err != nil {
		return errors.Wrap(err, "failed to check existence of bookmark dialog")
	} else if visible {
		testing.ContextLog(ctx, "bookmark dialog is already visible")
		return nil
	}

	if err := useHotKey(ctx, "Ctrl+d"); err != nil {
		return errors.Wrap(err, "could not use hotkey")
	}

	if err := ui.WaitUntilExists(ctx, tconn, bdFindParams, 5*time.Second); err != nil {
		// Shorter timeout. If bookmark menu is opened this is already rendered.
		return errors.Wrap(err, "failed to find the Bookmark name text field")
	}
	return nil
}

// BookmarkCurrentTab adds currently opened tab as a bookmark.
func BookmarkCurrentTab(ctx context.Context, tconn *chrome.TestConn) error {
	if err := triggerBookmarkThisTab(ctx, tconn); err != nil {
		return err
	}

	if err := useHotKey(ctx, "Enter"); err != nil {
		return errors.Wrap(err, "could not use hotkey")
	}
	return nil
}

// IsBookmarkStarIconVisible checks if the bookmark icon is visible.
func IsBookmarkStarIconVisible(ctx context.Context, tconn *chrome.TestConn) (bool, error) {
	visible, err := ui.Exists(ctx, tconn, ui.FindParams{
		Role:      ui.RoleTypeButton,
		ClassName: "StarView",
	})
	if err != nil {
		return false, errors.Wrap(err, "failed to check existence of bookmark star button")
	}
	return visible, nil
}

// RemoveBookmarkForCurrentTab removes bookmark using bookmark menu.
func RemoveBookmarkForCurrentTab(ctx context.Context, tconn *chrome.TestConn) error {
	if err := triggerBookmarkThisTab(ctx, tconn); err != nil {
		return err
	}

	pollOpts := testing.PollOptions{Interval: time.Second, Timeout: 5 * time.Second}
	if err := ui.StableFindAndClick(ctx, tconn, ui.FindParams{
		Role: ui.RoleTypeButton,
		Name: "Remove",
	}, &pollOpts); err != nil {
		return errors.Wrap(err, "failed to interact with Remove button")
	}
	return nil
}

// RenameBookmarkForCurrentTab renames the bookmark name for the current tab.
func RenameBookmarkForCurrentTab(ctx context.Context, tconn *chrome.TestConn, newName string) error {
	if err := triggerBookmarkThisTab(ctx, tconn); err != nil {
		return err
	}

	keyboard, err := input.Keyboard(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to open keyboard device")
	}
	defer keyboard.Close()

	testing.ContextLog(ctx, "Change bookmark name to "+newName)
	if err := keyboard.Type(ctx, newName+"\n"); err != nil {
		return errors.Wrap(err, "failed to write events")
	}
	return nil
}

// GetCurrentTabBookmarkName returns the name of the bookmark for the current tab.
func GetCurrentTabBookmarkName(ctx context.Context, tconn *chrome.TestConn) (string, error) {
	if err := triggerBookmarkThisTab(ctx, tconn); err != nil {
		return "", err
	}

	bookmarkNameTbx, err := ui.FindWithTimeout(ctx, tconn, ui.FindParams{
		Role: ui.RoleTypeTextField,
		Name: "Bookmark name",
	}, 15*time.Second)
	if err != nil {
		return "", errors.Wrap(err, "failed to find the Bookmark name text field")
	}
	defer bookmarkNameTbx.Release(ctx)

	if err := useHotKey(ctx, "Esc"); err != nil {
		return "", errors.Wrap(err, "could not use hotkey")
	}
	return bookmarkNameTbx.Value, nil
}

// ShowBookmarksBar makes bookmarks bar UI element visible.
func ShowBookmarksBar(ctx context.Context, tconn *chrome.TestConn) error {
	return bookmarksbar.Show(ctx, tconn)
}

// ToggleBookmarksBar toggles bookmarks bar using keyboard shortcut.
func ToggleBookmarksBar(ctx context.Context) error {
	return bookmarksbar.Toggle(ctx)
}

// GetVisibleBookmarksCount returns count of visible bookmarks on the the bookmarks bar.
func GetVisibleBookmarksCount(ctx context.Context, tconn *chrome.TestConn) (int, error) {
	return bookmarksbar.GetVisibleBookmarksCount(ctx, tconn)
}

// OpenBookmark opens bookmark with a given name from the bookmark bar.
func OpenBookmark(ctx context.Context, tconn *chrome.TestConn, bookmarkName string) error {
	return bookmarksbar.OpenBookmark(ctx, tconn, bookmarkName)
}

// IsBookmarkVisible checks if bookmark with a given name is visible on the bookmarks bar.
func IsBookmarkVisible(ctx context.Context, tconn *chrome.TestConn, bookmarkName string) (bool, error) {
	return bookmarksbar.IsBookmarkVisible(ctx, tconn, bookmarkName)
}

func useHotKey(ctx context.Context, hotkey string) error {
	keyboard, err := input.Keyboard(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to open keyboard device")
	}
	defer keyboard.Close()

	testing.ContextLogf(ctx, "Execute %s", hotkey)
	if err := keyboard.Accel(ctx, hotkey); err != nil {
		return errors.Wrapf(err, "failed to write events %s", hotkey)
	}
	return nil
}

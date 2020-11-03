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
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

// GetAddressBarText returns the address bar text.
func GetAddressBarText(ctx context.Context, tconn *chrome.TestConn) (string, error) {
	bookmarkedItem, err := ui.Find(ctx, tconn, ui.FindParams{
		ClassName: "OmniboxViewViews",
		Name:      "Address and search bar",
	})

	if err != nil {
		return "", errors.Wrap(err, "could not find address bar")
	}
	return bookmarkedItem.Value, nil
}

// OpenBookmarkMenu opens bookmark menu for the current tab.
func OpenBookmarkMenu(ctx context.Context, tconn *chrome.TestConn) error {
	// TODO: use strategy pattern to either accept hotkey or UI interaction

	if err := useHotKey(ctx, "Ctrl+d"); err != nil {
		return errors.Wrap(err, "could not use hotkey")
	}
	return nil
}

// AddCurrentTabAsBookmark adds currently opened tab as a bookmark.
func AddCurrentTabAsBookmark(ctx context.Context, tconn *chrome.TestConn) error {
	if err := OpenBookmarkMenu(ctx, tconn); err != nil {
		return err
	}

	if err := useHotKey(ctx, "Enter"); err != nil {
		return errors.Wrap(err, "could not use hotkey")
	}

	return nil
}

// IsBookmarkStarIconVisible checks if the bookmark icon is visible.
func IsBookmarkStarIconVisible(ctx context.Context, tconn *chrome.TestConn) (bool, error) {
	result, err := ui.Exists(ctx, tconn, ui.FindParams{
		Role:      ui.RoleTypeButton,
		ClassName: "StarView",
	})
	if err != nil {
		return false, errors.Wrap(err, "failed to check existence of bookmark star button")
	}
	return result, nil
}

// RemoveBookmark removes bookmark using bookmark menu.
func RemoveBookmark(ctx context.Context, tconn *chrome.TestConn) error {
	if err := OpenBookmarkMenu(ctx, tconn); err != nil {
		return err
	}

	removeBtn, err := ui.FindWithTimeout(ctx, tconn, ui.FindParams{
		Role: ui.RoleTypeButton,
		Name: "Remove",
	}, 5*time.Second) // Shorter timeout. If bookmark menu is opened this is already rendered.
	if err != nil {
		return errors.Wrap(err, "failed to find Remove button")
	}
	defer removeBtn.Release(ctx)

	testing.ContextLog(ctx, "Click on Remove button")
	pollOpts := testing.PollOptions{Interval: time.Second, Timeout: 5 * time.Second}
	if err := removeBtn.StableLeftClick(ctx, &pollOpts); err != nil {
		return errors.Wrap(err, "failed to left click on the Remove button")
	}
	return nil
}

// RenameBookmarkForCurrentTab renames the bookmark name for the current tab.
func RenameBookmarkForCurrentTab(ctx context.Context, tconn *chrome.TestConn, newName string) error {
	if err := OpenBookmarkMenu(ctx, tconn); err != nil {
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
	if err := OpenBookmarkMenu(ctx, tconn); err != nil {
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

// ShowBookmarkBar makes bookmarks bar UI element visible.
func ShowBookmarkBar(ctx context.Context, tconn *chrome.TestConn) error {
	visible, err := ui.Exists(ctx, tconn, ui.FindParams{
		Name:      "Bookmarks",
		ClassName: "BookmarkBarView",
	})
	if err != nil {
		return errors.Wrap(err, "failed to check existence of bookmark bar")
	}

	// Bar is already visible.
	if visible {
		testing.ContextLog(ctx, "Bookmark bar is already visible")
		return nil
	}

	return ToggleBookmarksBar(ctx)
}

// ToggleBookmarksBar toggles bookmarks bar using keyboard shortcut.
func ToggleBookmarksBar(ctx context.Context) error {
	// TODO: make it use strategy pattern to either accept hotkey or UI interaction
	if err := useHotKey(ctx, "ctrl+shift+b"); err != nil {
		return errors.Wrap(err, "could not use hotkey")
	}
	return nil
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

// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package bookmarksbar allows access to the bookmarks bar in the browser.
package bookmarksbar

import (
	"context"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ui"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

// Show makes bookmarks bar UI element visible.
func Show(ctx context.Context, tconn *chrome.TestConn) error {
	visible, err := ui.Exists(ctx, tconn, ui.FindParams{
		Name:      "Bookmarks",
		ClassName: "BookmarkBarView",
	})
	if err != nil {
		return errors.Wrap(err, "failed to check existence of bookmark bar")
	} else if visible {
		testing.ContextLog(ctx, "Bookmark bar is already visible")
		return nil
	}

	if err := useHotKey(ctx, "ctrl+shift+b"); err != nil {
		return errors.Wrap(err, "could not use hotkey")
	}

	return nil
}

// Hide makes bookmarks bar UI element not visible.
func Hide(ctx context.Context, tconn *chrome.TestConn) error {
	visible, err := ui.Exists(ctx, tconn, ui.FindParams{
		Name:      "Bookmarks",
		ClassName: "BookmarkBarView",
	})
	if err != nil {
		return errors.Wrap(err, "failed to check existence of bookmark bar")
	} else if !visible {
		testing.ContextLog(ctx, "Bookmark bar is already not visible")
		return nil
	}

	if err := useHotKey(ctx, "ctrl+shift+b"); err != nil {
		return errors.Wrap(err, "could not use hotkey")
	}

	return nil
}

// Toggle toggles bookmarks bar using keyboard shortcut.
func Toggle(ctx context.Context) error {
	if err := useHotKey(ctx, "ctrl+shift+b"); err != nil {
		return errors.Wrap(err, "could not use hotkey")
	}
	return nil
}

// GetVisibleBookmarksCount returns count of visible bookmarks on bookmarks bar.
func GetVisibleBookmarksCount(ctx context.Context, tconn *chrome.TestConn) (int, error) {
	bookmarks, err := ui.FindAll(ctx, tconn, ui.FindParams{
		ClassName: "BookmarkButton",
	})
	defer bookmarks.Release(ctx)

	if err != nil {
		return -1, errors.Wrap(err, "failed to find bookmark's buttons on the bookmark bar")
	}
	return len(bookmarks), nil
}

// OpenBookmark opens bookmark with a given name from the bookmark bar.
func OpenBookmark(ctx context.Context, tconn *chrome.TestConn, bookmarkName string) error {
	bookmarkedItem, err := ui.Find(ctx, tconn, ui.FindParams{
		ClassName: "BookmarkButton",
		Name:      bookmarkName,
	})
	defer bookmarkedItem.Release(ctx)

	if err != nil {
		return errors.Wrapf(err, "failed to find %s bookmark", bookmarkName)
	}
	return bookmarkedItem.StableLeftClick(ctx, &testing.PollOptions{Interval: time.Second, Timeout: 5 * time.Second})
}

// IsBookmarkVisible checks if bookmark with a given name is visible on the bookmark bar.
func IsBookmarkVisible(ctx context.Context, tconn *chrome.TestConn, bookmarkName string) (bool, error) {
	visible, err := ui.Exists(ctx, tconn, ui.FindParams{
		ClassName: "BookmarkButton",
		Name:      bookmarkName,
	})
	return visible, err
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

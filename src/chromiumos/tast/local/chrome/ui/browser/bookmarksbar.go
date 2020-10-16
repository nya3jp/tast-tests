// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package browser allows access to the bookmarks bar in the browser.
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

var bbFindParams ui.FindParams = ui.FindParams{
	Name:      "Bookmarks",
	ClassName: "BookmarkBarView",
}

const timeout = 5 * time.Second

// ShowBookmarksBar makes bookmarks bar UI element visible.
func ShowBookmarksBar(ctx context.Context, tconn *chrome.TestConn) error {
	visible, err := ui.Exists(ctx, tconn, bbFindParams)
	if err != nil {
		return errors.Wrap(err, "failed to check existence of bookmarks bar")
	} else if visible {
		return nil
	}

	if err := ToggleBookmarksBar(ctx); err != nil {
		return errors.Wrap(err, "could not toggle bookmarks bar")
	}

	if err := ui.WaitUntilExists(ctx, tconn, bbFindParams, timeout); err != nil {
		return errors.Wrapf(err, "bookmarks bar didn't appear within %s", timeout)
	}

	return nil
}

// Hide makes bookmarks bar UI element not visible.
func Hide(ctx context.Context, tconn *chrome.TestConn) error {
	visible, err := ui.Exists(ctx, tconn, bbFindParams)
	if err != nil {
		return errors.Wrap(err, "failed to check existence of bookmarks bar")
	} else if !visible {
		return nil
	}

	if err := ToggleBookmarksBar(ctx); err != nil {
		return errors.Wrap(err, "could not toggle bookmarks bar")
	}

	if err := ui.WaitUntilGone(ctx, tconn, bbFindParams, timeout); err != nil {
		return errors.Wrapf(err, "bookmarks bar didn't disappear within %s", timeout)
	}

	return nil
}

// ToggleBookmarksBar toggles bookmarks bar using keyboard shortcut.
func ToggleBookmarksBar(ctx context.Context) error {
	keyboard, err := input.Keyboard(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to open keyboard device")
	}
	defer keyboard.Close()

	if err := keyboard.Accel(ctx, "ctrl+shift+b"); err != nil {
		return errors.Wrap(err, "failed to write events ctrl+shift+b")
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

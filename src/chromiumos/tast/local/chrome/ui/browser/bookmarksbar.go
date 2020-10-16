// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

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

// ShowBookmarksBar makes bookmarks bar UI element visible.
func ShowBookmarksBar(ctx context.Context, tconn *chrome.TestConn, keyboard *input.KeyboardEventWriter) error {
	visible, err := ui.Exists(ctx, tconn, bbFindParams)
	if err != nil {
		return errors.Wrap(err, "failed to check existence of bookmarks bar")
	} else if visible {
		return nil
	}

	if err := ToggleBookmarksBar(ctx, keyboard); err != nil {
		return errors.Wrap(err, "could not toggle bookmarks bar")
	}

	if err := ui.WaitUntilExists(ctx, tconn, bbFindParams, shortTimeout); err != nil {
		return errors.Wrapf(err, "bookmarks bar didn't appear within %s", shortTimeout)
	}

	return nil
}

// Hide makes bookmarks bar UI element not visible.
func Hide(ctx context.Context, tconn *chrome.TestConn, keyboard *input.KeyboardEventWriter) error {
	visible, err := ui.Exists(ctx, tconn, bbFindParams)
	if err != nil {
		return errors.Wrap(err, "failed to check existence of bookmarks bar")
	} else if !visible {
		return nil
	}

	if err := ToggleBookmarksBar(ctx, keyboard); err != nil {
		return errors.Wrap(err, "could not toggle bookmarks bar")
	}

	if err := ui.WaitUntilGone(ctx, tconn, bbFindParams, shortTimeout); err != nil {
		return errors.Wrapf(err, "bookmarks bar didn't disappear within %s", shortTimeout)
	}

	return nil
}

// ToggleBookmarksBar toggles bookmarks bar using keyboard shortcut.
func ToggleBookmarksBar(ctx context.Context, keyboard *input.KeyboardEventWriter) error {
	if err := keyboard.Accel(ctx, "ctrl+shift+b"); err != nil {
		return errors.Wrap(err, "failed to write events ctrl+shift+b")
	}
	return nil
}

// VisibleBookmarksCount returns count of visible bookmarks on bookmarks bar.
func VisibleBookmarksCount(ctx context.Context, tconn *chrome.TestConn) (int, error) {
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
	return bookmarkedItem.StableLeftClick(ctx, &testing.PollOptions{Interval: time.Second, Timeout: shortTimeout})
}

// IsBookmarkVisible checks if bookmark with a given name is visible on the bookmark bar.
func IsBookmarkVisible(ctx context.Context, tconn *chrome.TestConn, bookmarkName string) (bool, error) {
	visible, err := ui.Exists(ctx, tconn, ui.FindParams{
		ClassName: "BookmarkButton",
		Name:      bookmarkName,
	})
	return visible, err
}

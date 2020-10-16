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
	"chromiumos/tast/testing"
)

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

// OpenBookmark opens bookmark from the bookmark bar.
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

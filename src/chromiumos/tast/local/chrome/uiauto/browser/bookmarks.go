// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package browser

import (
	"context"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

var bookmarksNode = nodewith.Name("Bookmarks").ClassName("BookmarkBarView")

// ShowBookmarksBar makes bookmarks bar UI element visible.
func (b *Browser) ShowBookmarksBar(ctx context.Context, keyboard *input.KeyboardEventWriter) error {

	visible, err := b.ui.IsNodeFound(ctx, bookmarksNode)
	if err != nil {
		return errors.Wrap(err, "failed to check existence of bookmarks bar")
	} else if visible {
		return nil
	}

	if err := b.ToggleBookmarksBar(ctx, keyboard); err != nil {
		return errors.Wrap(err, "could not toggle bookmarks bar")
	}

	if err := b.ui.WaitUntilExists(bookmarksNode)(ctx); err != nil {
		return errors.Wrap(err, "bookmarks bar didn't appear")
	}

	return nil
}

// Hide makes bookmarks bar UI element not visible.
func (b *Browser) Hide(ctx context.Context, keyboard *input.KeyboardEventWriter) error {
	visible, err := b.ui.IsNodeFound(ctx, bookmarksNode)
	if err != nil {
		return errors.Wrap(err, "failed to check existence of bookmarks bar")
	} else if !visible {
		return nil
	}

	if err := b.ToggleBookmarksBar(ctx, keyboard); err != nil {
		return errors.Wrap(err, "could not toggle bookmarks bar")
	}

	if err := b.ui.WaitUntilGone(bookmarksNode)(ctx); err != nil {
		return errors.Wrapf(err, "bookmarks bar didn't disappear within %s", shortTimeout)
	}

	return nil
}

// ToggleBookmarksBar toggles bookmarks bar using keyboard shortcut.
func (b *Browser) ToggleBookmarksBar(ctx context.Context, keyboard *input.KeyboardEventWriter) error {
	if err := keyboard.Accel(ctx, "ctrl+shift+b"); err != nil {
		return errors.Wrap(err, "failed to write events ctrl+shift+b")
	}

	if err := b.ui.WaitForLocation(nodewith.Root())(ctx); err != nil {
		return errors.Wrap(err, "failed waiting for animation to finish after toggling bookmark")
	}

	return nil
}

// VisibleBookmarksCount returns count of visible bookmarks on bookmarks bar.
func (b *Browser) VisibleBookmarksCount(ctx context.Context) (int, error) {
	bookmarks, err := b.ui.NodesInfo(ctx, nodewith.ClassName("BookmarkButton"))
	if err != nil {
		return -1, errors.Wrap(err, "failed to find bookmark's buttons on the bookmark bar")
	}
	return len(bookmarks), nil
}

// OpenBookmark opens bookmark with a given name from the bookmark bar.
func (b *Browser) OpenBookmark(ctx context.Context, bookmarkName string) error {
	if err := b.ui.WithPollOpts(testing.PollOptions{
		Interval: time.Second,
		Timeout:  shortTimeout,
	}).LeftClick(nodewith.ClassName("BookmarkButton").Name(bookmarkName))(ctx); err != nil {
		return errors.Wrapf(err, "failed to click %s bookmark", bookmarkName)
	}
	return nil
}

// IsBookmarkVisible checks if bookmark with a given name is visible on the bookmark bar.
func (b *Browser) IsBookmarkVisible(ctx context.Context, bookmarkName string) (bool, error) {
	visible, err := b.ui.IsNodeFound(ctx, nodewith.Name(bookmarkName).ClassName("BookmarkButton"))
	return visible, err
}

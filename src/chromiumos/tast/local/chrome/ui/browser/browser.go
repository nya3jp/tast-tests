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

const shortTimeout = 5 * time.Second
const mediumTimeout = 15 * time.Second

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

// openBookmarkThisTabDialog opens bookmark dialog for the current tab and
// waits for its element to exist. It adds current tab as a bookmark. If
// bookmarks already exists it opens the bookmark dialog.
func openBookmarkThisTabDialog(ctx context.Context, tconn *chrome.TestConn, keyboard *input.KeyboardEventWriter) error {
	var bookmarkNameTextFieldFP ui.FindParams = ui.FindParams{
		Role: ui.RoleTypeTextField,
		Name: "Bookmark name",
	}

	visible, err := ui.Exists(ctx, tconn, bookmarkNameTextFieldFP)
	if err != nil {
		return errors.Wrap(err, "failed to check existence of bookmark dialog")
	} else if visible {
		return nil
	}

	if err := keyboard.Accel(ctx, "Ctrl+d"); err != nil {
		return errors.Wrapf(err, "failed to write events %s", "Ctrl+d")
	}

	if err := ui.WaitUntilExists(ctx, tconn, bookmarkNameTextFieldFP, shortTimeout); err != nil {
		// Shorter timeout. If bookmark menu is opened this is already rendered.
		return errors.Wrap(err, "failed to find the Bookmark name text field")
	}
	return nil
}

// BookmarkCurrentTab opens bookmark dialog for the current tab and adds it as
// a bookmark.
func BookmarkCurrentTab(ctx context.Context, tconn *chrome.TestConn, keyboard *input.KeyboardEventWriter) error {
	if err := openBookmarkThisTabDialog(ctx, tconn, keyboard); err != nil {
		return err
	}

	if err := keyboard.Accel(ctx, "Enter"); err != nil {
		return errors.Wrapf(err, "failed to write events %s", "Enter")
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

// RemoveBookmarkForCurrentTab opens bookmark dialog for the current tab and
// removes bookmark.
func RemoveBookmarkForCurrentTab(ctx context.Context, tconn *chrome.TestConn, keyboard *input.KeyboardEventWriter) error {
	if err := openBookmarkThisTabDialog(ctx, tconn, keyboard); err != nil {
		return err
	}

	pollOpts := testing.PollOptions{Interval: time.Second, Timeout: shortTimeout}
	if err := ui.StableFindAndClick(ctx, tconn, ui.FindParams{
		Role: ui.RoleTypeButton,
		Name: "Remove",
	}, &pollOpts); err != nil {
		return errors.Wrap(err, "failed to interact with Remove button")
	}
	return nil
}

// RenameBookmarkForCurrentTab opens bookmark dialog for the current tab and
// renames the bookmark name.
func RenameBookmarkForCurrentTab(ctx context.Context, tconn *chrome.TestConn, keyboard *input.KeyboardEventWriter, newName string) error {
	if err := openBookmarkThisTabDialog(ctx, tconn, keyboard); err != nil {
		return err
	}

	if err := keyboard.Type(ctx, newName+"\n"); err != nil {
		return errors.Wrap(err, "failed to write events")
	}
	return nil
}

// CurrentTabBookmarkName opens bookmark dialog for the current tab and returns
// the name of the bookmark for the current tab.
func CurrentTabBookmarkName(ctx context.Context, tconn *chrome.TestConn, keyboard *input.KeyboardEventWriter) (string, error) {
	if err := openBookmarkThisTabDialog(ctx, tconn, keyboard); err != nil {
		return "", err
	}

	bookmarkNameTbx, err := ui.FindWithTimeout(ctx, tconn, ui.FindParams{
		Role: ui.RoleTypeTextField,
		Name: "Bookmark name",
	}, mediumTimeout)
	if err != nil {
		return "", errors.Wrap(err, "failed to find the Bookmark name text field")
	}
	defer bookmarkNameTbx.Release(ctx)

	if err := keyboard.Accel(ctx, "Esc"); err != nil {
		return "", errors.Wrapf(err, "failed to write events %s", "Esc")
	}

	return bookmarkNameTbx.Value, nil
}

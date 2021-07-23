// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package browser allows interactions with browser window.
package browser

import (
	"context"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/input"
)

const shortTimeout = 5 * time.Second

// Browser represents an instance of the browser.
type Browser struct {
	ui   *uiauto.Context
	conn *chrome.Conn
}

// Launch launches a browser with the given url.
// An error is returned if the browser fails to launch.
func Launch(ctx context.Context, tconn *chrome.TestConn, cr *chrome.Chrome, url string) (*Browser, error) {
	conn, err := cr.NewConn(ctx, url)
	if err != nil {
		return nil, errors.Wrap(err, "failed to open browser")
	}
	return &Browser{ui: uiauto.New(tconn), conn: conn}, nil
}

// Close closes the web content, the connection and frees related resources.
// Tests should typically defer calls to this method and ignore the returned error.
func (b *Browser) Close(ctx context.Context) error {
	b.conn.CloseTarget(ctx)
	return b.conn.Close()
}

// Navigate navigates the browser to a url.
func (b *Browser) Navigate(ctx context.Context, url string) error {
	return b.conn.Navigate(ctx, url)
}

// GetAddressBarText returns the address bar text.
func (b *Browser) GetAddressBarText(ctx context.Context) (string, error) {
	addressbarInfo, err := b.ui.Info(ctx, nodewith.Name("Address and search bar").ClassName("OmniboxViewViews"))
	if err != nil {
		return "", errors.Wrap(err, "could not find address bar")
	}
	return addressbarInfo.Value, nil
}

// openBookmarkThisTabDialog opens bookmark dialog for the current tab and
// waits for its element to exist. It adds current tab as a bookmark. If
// bookmarks already exists it opens the bookmark dialog.
func (b *Browser) openBookmarkThisTabDialog(ctx context.Context, keyboard *input.KeyboardEventWriter) error {
	bookmarkNameTextField := nodewith.Name("Bookmark name").Role(role.TextField)

	visible, err := b.ui.IsNodeFound(ctx, bookmarkNameTextField)
	if err != nil {
		return errors.Wrap(err, "failed to check existence of bookmark dialog")
	} else if visible {
		return nil
	}

	if err := keyboard.Accel(ctx, "Ctrl+d"); err != nil {
		return errors.Wrapf(err, "failed to write events %s", "Ctrl+d")
	}

	if err := b.ui.WaitUntilExists(bookmarkNameTextField)(ctx); err != nil {
		return errors.Wrap(err, "failed to find the Bookmark name text field")
	}

	return nil
}

// BookmarkCurrentTab opens bookmark dialog for the current tab and adds it as
// a bookmark.
func (b *Browser) BookmarkCurrentTab(ctx context.Context, keyboard *input.KeyboardEventWriter) error {
	if err := b.openBookmarkThisTabDialog(ctx, keyboard); err != nil {
		return err
	}

	if err := keyboard.Accel(ctx, "Enter"); err != nil {
		return errors.Wrapf(err, "failed to write events %s", "Enter")
	}

	if err := b.ui.WaitForLocation(nodewith.Root())(ctx); err != nil {
		return errors.Wrap(err, "failed waiting for animation to finish after bookmarking current tab")
	}

	return nil
}

// IsBookmarkStarIconVisible checks if the bookmark icon is visible.
func (b *Browser) IsBookmarkStarIconVisible(ctx context.Context) (bool, error) {
	visible, err := b.ui.IsNodeFound(ctx, nodewith.ClassName("StarView").Role(role.Button))
	if err != nil {
		return false, errors.Wrap(err, "failed to check existence of bookmark dialog")
	}
	return visible, nil
}

// RemoveBookmarkForCurrentTab opens bookmark dialog for the current tab and
// removes bookmark.
func (b *Browser) RemoveBookmarkForCurrentTab(ctx context.Context, keyboard *input.KeyboardEventWriter) error {
	if err := b.openBookmarkThisTabDialog(ctx, keyboard); err != nil {
		return err
	}

	if err := b.ui.WithTimeout(shortTimeout).WithInterval(time.Second).LeftClick(nodewith.Name("Remove").Role(role.Button))(ctx); err != nil {
		return errors.Wrap(err, "failed to interact with Remove button")
	}
	return nil
}

// RenameBookmarkForCurrentTab opens bookmark dialog for the current tab and
// renames the bookmark name.
func (b *Browser) RenameBookmarkForCurrentTab(ctx context.Context, keyboard *input.KeyboardEventWriter, newName string) error {
	if err := b.openBookmarkThisTabDialog(ctx, keyboard); err != nil {
		return err
	}

	if err := keyboard.Type(ctx, newName+"\n"); err != nil {
		return errors.Wrap(err, "failed to write events")
	}

	if err := b.ui.WaitForLocation(nodewith.Root())(ctx); err != nil {
		return errors.Wrap(err, "failed waiting for animation to finish after renaming bookmark")
	}

	return nil
}

// CurrentTabBookmarkName opens bookmark dialog for the current tab and returns
// the name of the bookmark for the current tab.
func (b *Browser) CurrentTabBookmarkName(ctx context.Context, keyboard *input.KeyboardEventWriter) (string, error) {
	if err := b.openBookmarkThisTabDialog(ctx, keyboard); err != nil {
		return "", err
	}

	bookmarkNameInfo, err := b.ui.Info(ctx, nodewith.Name("Bookmark name").Role(role.TextField))
	if err != nil {
		return "", errors.Wrap(err, "failed to find the Bookmark name text field")
	}

	if err := keyboard.Accel(ctx, "Esc"); err != nil {
		return "", errors.Wrapf(err, "failed to write events %s", "Esc")
	}

	if err := b.ui.WaitForLocation(nodewith.Root())(ctx); err != nil {
		return "", errors.Wrap(err, "failed waiting for animation to finish after dismissing bookmark dialog")
	}

	return bookmarkNameInfo.Value, nil
}

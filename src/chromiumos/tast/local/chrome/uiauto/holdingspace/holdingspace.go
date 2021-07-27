// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package holdingspace

import (
	"context"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/filesapp"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/state"
)

// FindChip returns a finder which locates a holding space chip node with the
// specified name.
func FindChip(name string) *nodewith.Finder {
	return nodewith.ClassName("HoldingSpaceItemChipView").Name(name)
}

// FindContextMenuItem returns a finder which locates a holding space context
// menu item node with the specified name.
func FindContextMenuItem(name string) *nodewith.Finder {
	return nodewith.ClassName("MenuItemView").Name(name)
}

// FindDownloadChip returns a finder which locates a holding space download chip
// node with the specified name.
func FindDownloadChip(name string) *nodewith.Finder {
	return nodewith.Ancestor(nodewith.ClassName("DownloadsSection")).
		ClassName("HoldingSpaceItemChipView").
		Name(name)
}

// FindPinnedFileChip returns a finder which locates a holding space pinned file
// chip node with the specified name.
func FindPinnedFileChip(name string) *nodewith.Finder {
	return nodewith.Ancestor(nodewith.ClassName("PinnedFilesSection")).
		ClassName("HoldingSpaceItemChipView").
		Name(name)
}

// FindTray returns a finder which locates the holding space tray node.
func FindTray() *nodewith.Finder {
	return nodewith.ClassName("HoldingSpaceTray")
}

// UnpinItemViaContextMenu unpins an item from holding space.
func UnpinItemViaContextMenu(ctx context.Context, tconn *chrome.TestConn,
	name string) error {
	uia := uiauto.New(tconn)

	itemChipView := FindPinnedFileChip(name)
	unpinMenuItem := FindContextMenuItem("Unpin")

	if err := uiauto.Combine("Unpin holding space item via context menu",
		uia.WaitUntilExists(itemChipView),
		uia.RightClick(itemChipView),
		uia.WaitUntilExists(unpinMenuItem),
		uia.LeftClick(unpinMenuItem),
	)(ctx); err != nil {
		errors.Errorf("failed to unpin item from holding space: %s", err)
	}

	// If the tray doesn't exist, then we can return, since nothing is pinned.
	if err := uia.EnsureGoneFor(FindTray(), time.Second)(ctx); err != nil {
		return nil
	}

	if err := OpenBubble(ctx, tconn); err != nil {
		return err
	}

	if err := uia.EnsureGoneFor(FindPinnedFileChip(name), 3*time.Second)(ctx); err != nil {
		return errors.Errorf("Unpinning file %s failed: %s", name, err)
	}

	return nil
}

// PinViaFileContextMenu pins a given file visible in the given `FilesApp` to the
// holding space via the context menu.
func PinViaFileContextMenu(ctx context.Context, tconn *chrome.TestConn,
	fsapp *filesapp.FilesApp, newFileName string) error {

	// Pin the file to the shelf by using the context menu in the files app.
	if err := fsapp.ClickContextMenuItem(newFileName, "Pin to shelf")(ctx); err != nil {
		return errors.Errorf("Pinning file %s failed: %s", newFileName, err)
	}

	// Make sure the file got pinned.
	if err := OpenBubble(ctx, tconn); err != nil {
		return errors.Errorf("failed to open holding space bubble: %s", err)
	}

	uia := uiauto.New(tconn)
	if err := uia.WaitUntilExists(FindPinnedFileChip(newFileName))(ctx); err != nil {
		return errors.Errorf("failed to find pinned item %s: %s", newFileName, err)
	}

	return nil
}

// OpenBubble opens the holding space bubble.
func OpenBubble(ctx context.Context, tconn *chrome.TestConn) error {
	uia := uiauto.New(tconn)

	pinnedFilesBubble := nodewith.ClassName("PinnedFilesBubble")

	// If we find the bubble, it's already open. Return early.
	if uia.Exists(pinnedFilesBubble)(ctx) == nil {
		info, err := uia.Info(ctx, pinnedFilesBubble)
		if err == nil && !info.State[state.Invisible] {
			return nil
		}
	}

	holdingSpaceTray := FindTray()

	if err := uiauto.Combine("Open holding space bubble",
		uia.WaitUntilExists(holdingSpaceTray),
		uia.LeftClick(holdingSpaceTray),
		uia.WaitUntilExists(pinnedFilesBubble),
	)(ctx); err != nil {
		return err
	}

	return nil
}

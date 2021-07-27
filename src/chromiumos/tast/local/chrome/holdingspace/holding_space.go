// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package holdingspace contains a library to interface with holdingspace in ash
package holdingspace

import (
	"context"
	"io/ioutil"
	"path/filepath"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/filesapp"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/chrome/uiauto/state"
	"chromiumos/tast/local/coords"
)

func openBubble(ctx context.Context, tconn *chrome.TestConn) error {
	uia := uiauto.New(tconn)

	pinnedFilesBubble := nodewith.ClassName("PinnedFilesBubble")

	// If we find the bubble, it's already open. Return early.
	if uia.Exists(pinnedFilesBubble)(ctx) == nil {
		info, err := uia.Info(ctx, pinnedFilesBubble)
		if err == nil && !info.State[state.Invisible] {
			return nil
		}
	}

	holdingSpaceTray := nodewith.ClassName("HoldingSpaceTray")

	if err := uiauto.Combine("Open holding space bubble",
		uia.WithInterval(100*time.Millisecond).WaitUntilExists(holdingSpaceTray),
		uia.LeftClick(holdingSpaceTray),
		uia.WaitUntilExists(pinnedFilesBubble),
	)(ctx); err != nil {
		return err
	}

	return nil
}

// FileIsPinned confirms that an item is pinned to Holding Space.
func FileIsPinned(ctx context.Context, tconn *chrome.TestConn, name string) error {
	if err := openBubble(ctx, tconn); err != nil {
		return err
	}

	uia := uiauto.New(tconn)
	if err := uia.WaitUntilExists(nodewith.ClassName("HoldingSpaceItemChipView").
		Name(name).Ancestor(nodewith.ClassName("PinnedFilesSection")))(ctx); err != nil {
		return errors.Errorf("failed to find pinned item %s: %s", name, err)
	}
	return nil
}

// WaitUntilNotPinned waits for the node of an item pinned in Holding Space to
// be gone.
func WaitUntilNotPinned(ctx context.Context, tconn *chrome.TestConn, name string) error {
	uia := uiauto.New(tconn)
	// If there is no holding space tray, it means nothing is pinned. We can return.
	if err := uia.WithTimeout(2 * time.Second).
		WaitUntilExists(nodewith.ClassName("HoldingSpaceTray")); err != nil {
		return nil
	}

	if err := openBubble(ctx, tconn); err != nil {
		return err
	}

	if err := uia.EnsureGoneFor(nodewith.ClassName("HoldingSpaceItemChipView").
		Name(name), 3*time.Second)(ctx); err != nil {
		return errors.Errorf("Unpinning file %s failed: %s", name, err)
	}

	return nil
}

// UnpinItemViaContextMenu unpins an item from Holding Space.
func UnpinItemViaContextMenu(ctx context.Context, tconn *chrome.TestConn,
	name string) error {
	uia := uiauto.New(tconn)

	itemChipView := nodewith.ClassName("HoldingSpaceItemChipView").Name(name).
		Ancestor(nodewith.ClassName("PinnedFilesSection"))
	unpinMenuItem := nodewith.Role(role.MenuItem).Name("Unpin")

	if err := uiauto.Combine("Unpin holding space item via context menu",
		uia.WithInterval(200*time.Millisecond).RightClickUntil(itemChipView,
			uia.Exists(unpinMenuItem)),
		uia.LeftClick(unpinMenuItem),
	)(ctx); err != nil {
		errors.Wrap(err, "failed to unpin item from holding space")
	}

	return WaitUntilNotPinned(ctx, tconn, name)
}

// GetHoldingSpaceTrayLocation gets the ui.Node for the Holding Space tray.
func GetHoldingSpaceTrayLocation(ctx context.Context, tconn *chrome.TestConn) (*coords.Rect,
	error) {
	uia := uiauto.New(tconn)
	tray := nodewith.ClassName("HoldingSpaceTray")

	uia.WaitUntilExists(tray)
	location, err := uia.Location(ctx, tray)
	if err != nil {
		return nil, errors.Errorf("failed to find holding space tray location: %s", err)
	}
	return location, nil
}

// CreateFile creates a new nonsense text file in My Files. Returns the file's
// path. Caller is responsible for deleting the file.
func CreateFile(ctx context.Context, tconn *chrome.TestConn,
	newFileName string) (string, error) {
	// Create our file, with appropriate permissions so we can delete later.
	newFilePath := filepath.Join(filesapp.MyFilesPath, newFileName)
	if err := ioutil.WriteFile(newFilePath, []byte("Per aspera ad astra"), 0644); err != nil {
		return "", errors.Errorf("Creating file %s failed: %s", newFilePath, err)
	}
	return newFilePath, nil
}

// PinViaFileContextMenu pins a given file visible in the given FilesApp to the
// holding space via the context menu.
func PinViaFileContextMenu(ctx context.Context, tconn *chrome.TestConn,
	fsapp *filesapp.FilesApp, newFileName string) error {

	// Pin the file to the shelf by using the context menu in the files app.
	if err := fsapp.ClickContextMenuItem(newFileName, "Pin to shelf")(ctx); err != nil {
		return errors.Errorf("Pinning file %s failed: %s", newFileName, err)
	}

	// Make sure the file got pinned.
	if err := FileIsPinned(ctx, tconn, newFileName); err != nil {
		return errors.Errorf("failed to find item %s: %s", newFileName, err)
	}
	return nil
}

// ForceShowHoldingSpaceTrayIcon uses a junk file to cause the
// HoldingSpaceTrayIcon to be shown, since it is hidden on a clean Chrome OS
// instance. A defered call to os.Remove is recommended to get rid of the junk
// file, to ensure the tray icon stays around until the test is done, but
// doesn't leak files.
func ForceShowHoldingSpaceTrayIcon(ctx context.Context, tconn *chrome.TestConn,
	fsapp *filesapp.FilesApp) (string, error) {
	const junkFile = "junk.txt"
	junkFilePath, err := CreateFile(ctx, tconn, junkFile)

	if err != nil {
		return "", errors.Errorf("failed to create file: %s", err)
	}

	if err := PinViaFileContextMenu(ctx, tconn, fsapp, junkFile); err != nil {
		return "", errors.Errorf("failed to pin file: %s", err)
	}

	return junkFilePath, nil
}

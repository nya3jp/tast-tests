// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package holdingspace contains a library to interface with holdingspace in ash
package holdingspace

import (
	"context"
	"io/ioutil"
	"os"
	"path/filepath"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ui"
	"chromiumos/tast/local/chrome/uiauto/filesapp"
	"chromiumos/tast/local/chrome/uiauto/role"
)

// ConfirmHoldingSpaceBubbleOpen opens the holding space bubble above the tray, which
// contains pinned file nodes, if it is not already open.
func ConfirmHoldingSpaceBubbleOpen(ctx context.Context, tconn *chrome.TestConn) error {
	pinnedFilesBubbleParams := ui.FindParams{ClassName: "PinnedFilesBubble"}

	// If we find the bubble, it's already open. Return early.
	_, err := ui.Find(ctx, tconn, pinnedFilesBubbleParams)
	if err == nil {
		return nil
	}

	// Wait for tray node to exist, in case we just pinned/downloaded an item and it
	// hasn't shown up yet.
	holdingSpaceParams := ui.FindParams{ClassName: "HoldingSpaceTray"}
	if err := ui.WaitUntilExists(ctx, tconn, holdingSpaceParams, 10*time.Second); err != nil {
		return errors.Errorf("Time expired waiting for holding space: %s", err)
	}

	holdingSpaceTray, err := ui.Find(ctx, tconn, holdingSpaceParams)
	if err != nil {
		return errors.Errorf("failed to get holding space tray node: %s", err)
	}

	holdingSpaceTray.LeftClick(ctx)
	if err := ui.WaitUntilExists(ctx, tconn, pinnedFilesBubbleParams, 10*time.Second); err != nil {
		return errors.Errorf("Holding space bubble failed to open: %s", err)
	}
	return nil
}

// GetPinnedItem gets the Node for an item pinned to Holding Space.
func GetPinnedItem(ctx context.Context, tconn *chrome.TestConn, name string) (*ui.Node,
	error) {
	if err := ConfirmHoldingSpaceBubbleOpen(ctx, tconn); err != nil {
		return nil, err
	}

	pinnedItemParams := ui.FindParams{ClassName: "HoldingSpaceItemChipView", Name: name}
	if err := ui.WaitUntilExists(ctx, tconn, pinnedItemParams, 10*time.Second); err != nil {
		return nil, errors.Errorf("failed to find pinned item: %s", err)
	}

	return ui.Find(ctx, tconn, pinnedItemParams)
}

// WaitUntilNotPinned waits for the node of an item pinned in Holding Space to be gone.
func WaitUntilNotPinned(ctx context.Context, tconn *chrome.TestConn, name string) error {
	if err := ConfirmHoldingSpaceBubbleOpen(ctx, tconn); err != nil {
		return err
	}

	pinnedItemParams := ui.FindParams{ClassName: "HoldingSpaceItemChipView", Name: name}
	if err := ui.WaitUntilGone(ctx, tconn, pinnedItemParams, 10*time.Second); err != nil {
		return errors.Errorf("Unpinning file %s failed: %s", name, err)
	}

	return nil
}

// UnpinItem unpins an item from Holding Space.
func UnpinItem(ctx context.Context, tconn *chrome.TestConn, name string) error {
	item, err := GetPinnedItem(ctx, tconn, name)
	if err != nil {
		return err
	}

	item.RightClick(ctx)
	unpinParams := ui.FindParams{Name: "Unpin", Role: ui.RoleType(role.MenuItem)}
	unpin, err := ui.Find(ctx, tconn, unpinParams)
	if err != nil {
		return err
	}
	unpin.LeftClick(ctx)

	return WaitUntilNotPinned(ctx, tconn, name)
}

// GetHoldingSpaceTrayNode gets the ui.Node for the Holding Space tray.
func GetHoldingSpaceTrayNode(ctx context.Context, tconn *chrome.TestConn) (*ui.Node,
	error) {
	holdingSpaceParams := ui.FindParams{ClassName: "HoldingSpaceTray"}
	if err := ui.WaitUntilExists(ctx, tconn, holdingSpaceParams, 10*time.Second); err != nil {
		return nil, errors.Errorf("failed to find holding space: %s", err)
	}
	holdingSpace, err := ui.Find(ctx, tconn, holdingSpaceParams)
	if err != nil {
		return nil, errors.Errorf("failed to get holding space: %s", err)
	}

	return holdingSpace, nil
}

// CreateAndPinNewfile creates a new nonsense text file in My Files and pins it to
// the holding space. Returns the file's path. Caller is responsible for deleting the
// file.
func CreateAndPinNewfile(ctx context.Context, tconn *chrome.TestConn,
	fsapp *filesapp.FilesApp, newFileName string) (string, error) {
	// Create our file, with appropriate permissions so we can delete later.
	newFilePath := filepath.Join(filesapp.MyFilesPath, newFileName)
	if err := ioutil.WriteFile(newFilePath, []byte("blahblah"), 0644); err != nil {
		return "", errors.Errorf("Creating file %s failed: %s", newFilePath, err)
	}

	// Pin the file to the shelf by using the context menu in the files app.
	if err := fsapp.ClickContextMenuItem(newFileName, "Pin to shelf")(ctx); err != nil {
		os.Remove(newFilePath)
		return "", errors.Errorf("Pinning file %s failed: %s", newFilePath, err)
	}

	// Make sure the file got pinned.
	_, err := GetPinnedItem(ctx, tconn, newFileName)
	if err != nil {
		os.Remove(newFilePath)
		return "", errors.Errorf("failed to find item %s: %s", newFileName, err)
	}

	return newFilePath, nil
}

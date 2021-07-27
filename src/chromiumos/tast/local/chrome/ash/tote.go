// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ash

import (
	"context"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ui"
	"chromiumos/tast/local/chrome/uiauto/role"
)

// ConfirmToteBubbleOpen the Tote bubble, containing pinned file nodes, if it is not already open
func ConfirmToteBubbleOpen(ctx context.Context, tconn *chrome.TestConn) error {
	pinnedFilesBubbleParams := ui.FindParams{ClassName: "PinnedFilesBubble"}

	// If we find the bubble, it's already open. Return early.
	_, err := ui.Find(ctx, tconn, pinnedFilesBubbleParams)
	if err == nil {
		return nil
	}

	// Wait for tray icon to exist, in case we just pinned/downloaded an item and it hasn't shown up yet
	toteIconParams := ui.FindParams{ClassName: "HoldingSpaceTrayIcon"}
	if err := ui.WaitUntilExists(ctx, tconn, toteIconParams, 10*time.Second); err != nil {
		return errors.Errorf("Time expired waiting for tote icon: %s", err)
	}

	toteIcon, err := ui.Find(ctx, tconn, toteIconParams)
	if err != nil {
		return errors.Errorf("failed to get tote tray icon node: %s", err)
	}

	toteIcon.LeftClick(ctx)

	if err := ui.WaitUntilExists(ctx, tconn, pinnedFilesBubbleParams, 10*time.Second); err != nil {
		return errors.Errorf("Tote bubble failed to open: %s", err)
	}
	return nil
}

// GetPinnedItem gets the Node for an item pinned to Tote
func GetPinnedItem(ctx context.Context, tconn *chrome.TestConn, name string) (*ui.Node, error) {
	if err := ConfirmToteBubbleOpen(ctx, tconn); err != nil {
		return nil, err
	}

	pinnedItemParams := ui.FindParams{ClassName: "HoldingSpaceItemChipView", Name: name}
	if err := ui.WaitUntilExists(ctx, tconn, pinnedItemParams, 10*time.Second); err != nil {
		return nil, errors.Errorf("failed to find pinned item: %s", err)
	}

	return ui.Find(ctx, tconn, pinnedItemParams)
}

// WaitUntilNotPinned waits for the node of an item pinned in tote to be gone
func WaitUntilNotPinned(ctx context.Context, tconn *chrome.TestConn, name string) error {
	pinnedItemParams := ui.FindParams{ClassName: "HoldingSpaceItemChipView", Name: name}
	if err := ui.WaitUntilGone(ctx, tconn, pinnedItemParams, 10*time.Second); err != nil {
		return errors.Errorf("Unpinning file %s failed: %s", name, err)
	}

	return nil
}

// UnpinItem unpins an item from Tote
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

// GetToteTrayIconNode gets the ui.Node for the Tote tray icon
func GetToteTrayIconNode(ctx context.Context, tconn *chrome.TestConn) (*ui.Node, error) {
	toteparams := ui.FindParams{ClassName: "HoldingSpaceTrayIcon"}
	if err := ui.WaitUntilExists(ctx, tconn, toteparams, 10*time.Second); err != nil {
		return nil, errors.Errorf("failed to find holding space: %s", err)
	}
	tote, err := ui.Find(ctx, tconn, toteparams)
	if err != nil {
		return nil, errors.Errorf("failed to get holding space: %s", err)
	}

	return tote, nil
}

// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package holdingspace

import (
	"context"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
)

// FindChip returns a finder which locates a holding space chip node.
func FindChip() *nodewith.Finder {
	return nodewith.HasClass(holdingSpaceItemChipViewClassName)
}

// FindContextMenu returns a finder which locates a holding space context
// menu node.
func FindContextMenu() *nodewith.Finder {
	return nodewith.HasClass(optionMenuClassName).Role(role.Menu)
}

// FindContextMenuItem returns a finder which locates a holding space context
// menu item node.
func FindContextMenuItem() *nodewith.Finder {
	return nodewith.HasClass(menuItemViewClassName)
}

// FindDownloadChip returns a finder which locates a holding space download chip
// node.
func FindDownloadChip() *nodewith.Finder {
	return nodewith.Ancestor(nodewith.HasClass(downloadsSectionClassName)).
		HasClass(holdingSpaceItemChipViewClassName)
}

// FindPinnedFileChip returns a finder which locates a holding space pinned file
// chip node.
func FindPinnedFileChip() *nodewith.Finder {
	return nodewith.Ancestor(nodewith.HasClass(pinnedFilesSectionClassName)).
		HasClass(holdingSpaceItemChipViewClassName)
}

// FindScreenCaptureView returns a finder which locates a holding space screen
// capture view node.
func FindScreenCaptureView() *nodewith.Finder {
	return nodewith.Ancestor(nodewith.HasClass(screenCapturesSectionClassName)).
		HasClass(holdingSpaceItemScreenCaptureViewClassName)
}

// FindTray returns a finder which locates the holding space tray node.
func FindTray() *nodewith.Finder {
	return nodewith.HasClass(holdingSpaceTrayClassName)
}

// FindRootFinder returns a finder which represents the holding space view.
func FindRootFinder() *nodewith.Finder {
	return nodewith.Name(rootFinderName).HasClass("Widget").Role(role.Dialog)
}

// HoldingSpace represents an instance of the holding space.
type HoldingSpace struct {
	ui *uiauto.Context
}

// New returns HoldingSpace object.
func New(tconn *chrome.TestConn) *HoldingSpace {
	return &HoldingSpace{
		ui: uiauto.New(tconn),
	}
}

// shown checks if holding space exists in the UI.
func shown(ctx context.Context, tconn *chrome.TestConn) (bool, error) {
	return uiauto.New(tconn).IsNodeFound(ctx, FindRootFinder())
}

// Expand clicks tray bubble button to display holding space.
// If holding space is already open, it does nothing.
func (t *HoldingSpace) Expand(tconn *chrome.TestConn) uiauto.Action {
	return func(ctx context.Context) error {
		if isShown, err := shown(ctx, tconn); err != nil {
			return errors.Wrap(err, "failed to check holding space visibility")
		} else if isShown {
			return nil
		}

		return uiauto.Combine("show the holding space",
			t.ui.LeftClick(FindTray()),
			t.ui.WaitUntilExists(FindRootFinder()),
		)(ctx)
	}
}

// Collapse clicks tray bubble button to hide holding space.
// If holding space is already closed, it does nothing.
func (t *HoldingSpace) Collapse(tconn *chrome.TestConn) uiauto.Action {
	return func(ctx context.Context) error {
		if isShown, err := shown(ctx, tconn); err != nil {
			return errors.Wrap(err, "failed to check holding space visibility")
		} else if !isShown {
			return nil
		}

		return uiauto.Combine("hide the holding space",
			t.ui.LeftClick(FindTray()),
			t.ui.WaitUntilGone(FindRootFinder()),
		)(ctx)
	}
}

// ShowOptionMenu right clicks the item to make the option menu show.
func (t *HoldingSpace) ShowOptionMenu(item *nodewith.Finder) uiauto.Action {
	return uiauto.Combine("show the option menu",
		t.ui.RightClick(item),
		t.ui.WaitUntilExists(FindContextMenu()),
	)
}

// ClickMenuOption clicks the option from the option menu under holding space.
func (t *HoldingSpace) ClickMenuOption(option MenuOptions) uiauto.Action {
	return t.ui.LeftClick(FindContextMenuItem().Name(string(option)))
}

// PinItem pins the item from holding space.
// It calls ShowOptionMenu and then ClickMenuOption.
// The caller need to verify the item is pinned in the pinned section if necessary.
func (t *HoldingSpace) PinItem(item *nodewith.Finder) uiauto.Action {
	return uiauto.Combine("pin an item from holding space",
		t.ShowOptionMenu(item),
		t.ClickMenuOption(Pin),
	)
}

// UnpinItem unpins the item from holding space.
// It calls ShowOptionMenu and then ClickMenuOption.
// The caller need to verify the item is unpinned from the pinned section if necessary.
func (t *HoldingSpace) UnpinItem(item *nodewith.Finder) uiauto.Action {
	return uiauto.Combine("unpin an item from holding space",
		t.ShowOptionMenu(item),
		t.ClickMenuOption(Unpin),
	)
}

// RemoveItem removes the item from holding space.
// It calls ShowOptionMenu and then ClickMenuOption.
// The caller need to verify the item is removed from the holding space if necessary.
func (t *HoldingSpace) RemoveItem(item *nodewith.Finder) uiauto.Action {
	return uiauto.Combine("remove an item from holding space",
		t.ShowOptionMenu(item),
		t.ClickMenuOption(Remove),
	)
}

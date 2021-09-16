// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package holdingspace

import (
	"context"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
)

// FindChip returns a finder which locates a holding space chip node.
func FindChip() *nodewith.Finder {
	return nodewith.HasClass(holdingSpaceItemChipViewClassName)
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

// FindPinnedFilesSectionFilesAppChip returns a finder which locates the holding
// space pinned files section Files app chip node.
func FindPinnedFilesSectionFilesAppChip() *nodewith.Finder {
	return nodewith.Ancestor(nodewith.HasClass(buttonClassName).Ancestor(
		nodewith.HasClass(pinnedFilesSectionClassName))).Name("Open Files")
}

// FindPinnedFilesSectionFilesAppPrompt returns a finder which locates the
// holding space pinned files section Files app prompt node.
func FindPinnedFilesSectionFilesAppPrompt() *nodewith.Finder {
	return nodewith.Ancestor(nodewith.HasClass(pinnedFilesSectionClassName)).
		Name("You can pin your important files here. Open Files app to get started.")
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

// ResetHoldingSpaceOptions is defined in autotest_private.idl.
type ResetHoldingSpaceOptions struct {
	MarkTimeOfFirstAdd bool `json:"markTimeOfFirstAdd"`
}

// ResetHoldingSpace calls autotestPrivate to remove all items in the holding space model
// and resets all holding space prefs.
func ResetHoldingSpace(ctx context.Context, tconn *chrome.TestConn,
	options ResetHoldingSpaceOptions) error {
	if err := tconn.Call(ctx, nil,
		"tast.promisify(chrome.autotestPrivate.resetHoldingSpace)", options); err != nil {
		return errors.Wrap(err, "failed to reset holding space")
	}
	return nil
}

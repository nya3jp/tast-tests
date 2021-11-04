// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package holdingspace

import (
	"chromiumos/tast/local/chrome/uiauto/nodewith"
)

// Class names.
const downloadsSectionClassName = "DownloadsSection"
const holdingSpaceItemChipViewClassName = "HoldingSpaceItemChipView"
const holdingSpaceItemScreenCaptureViewClassName = "HoldingSpaceItemScreenCaptureView"
const holdingSpaceTrayClassName = "HoldingSpaceTray"
const menuItemViewClassName = "MenuItemView"
const pinnedFilesSectionClassName = "PinnedFilesSection"
const screenCapturesSectionClassName = "ScreenCapturesSection"

// FindChip returns a finder which locates a holding space chip node.
func FindChip() *nodewith.Finder {
	return nodewith.ClassName(holdingSpaceItemChipViewClassName)
}

// FindContextMenuItem returns a finder which locates a holding space context
// menu item node.
func FindContextMenuItem() *nodewith.Finder {
	return nodewith.ClassName(menuItemViewClassName)
}

// FindDownloadChip returns a finder which locates a holding space download chip
// node.
func FindDownloadChip() *nodewith.Finder {
	return nodewith.Ancestor(nodewith.ClassName(downloadsSectionClassName)).
		ClassName(holdingSpaceItemChipViewClassName)
}

// FindPinnedFileChip returns a finder which locates a holding space pinned file
// chip node.
func FindPinnedFileChip() *nodewith.Finder {
	return nodewith.Ancestor(nodewith.ClassName(pinnedFilesSectionClassName)).
		ClassName(holdingSpaceItemChipViewClassName)
}

// FindScreenCaptureView returns a finder which locates a holding space screen
// capture view node.
func FindScreenCaptureView() *nodewith.Finder {
	return nodewith.Ancestor(nodewith.ClassName(screenCapturesSectionClassName)).
		ClassName(holdingSpaceItemScreenCaptureViewClassName)
}

// FindTray returns a finder which locates the holding space tray node.
func FindTray() *nodewith.Finder {
	return nodewith.ClassName(holdingSpaceTrayClassName)
}

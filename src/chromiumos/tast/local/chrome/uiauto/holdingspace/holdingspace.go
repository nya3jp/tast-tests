// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package holdingspace

import (
	"chromiumos/tast/local/chrome/uiauto/nodewith"
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

// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package holdingspace

// Node names and class names.
const (
	holdingSpaceItemChipViewClassName          = "HoldingSpaceItemChipView"
	holdingSpaceItemScreenCaptureViewClassName = "HoldingSpaceItemScreenCaptureView"
	holdingSpaceTrayClassName                  = "HoldingSpaceTray"
	menuItemViewClassName                      = "MenuItemView"
	downloadsSectionClassName                  = "DownloadsSection"
	pinnedFilesSectionClassName                = "PinnedFilesSection"
	screenCapturesSectionClassName             = "ScreenCapturesSection"
	optionMenuClassName                        = "SubmenuView"
	rootFinderName                             = "Tote: recent screen captures, downloads, and pinned files"
)

// MenuOptions represents the options under the OptionMenu.
type MenuOptions string

// The block are the options in the option menu from holding space.
const (
	ShowInFolder MenuOptions = "Show in folder"
	CopyImage    MenuOptions = "Copy image"
	Pin          MenuOptions = "Pin"
	Unpin        MenuOptions = "Unpin"
	Remove       MenuOptions = "Remove"
)

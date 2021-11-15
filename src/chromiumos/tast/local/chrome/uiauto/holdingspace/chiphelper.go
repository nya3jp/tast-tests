// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package holdingspace

import (
	"fmt"
	"time"

	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
)

// ChipType indicates the type of a chip.
type ChipType int

const (
	// Done indicates the chip is done.
	Done ChipType = iota
	// Downloading indicates the chip is downloading.
	Downloading
	// Paused indicates the chip is paused.
	Paused
)

// ChipHelper is a helper that can easier interact with chips in the HoldingSpace downloaded and
// pinned chips.
type ChipHelper struct {
	chipFinder *nodewith.Finder
	chipType   ChipType
}

// DownloadChipHelper returns the helper to interact with the chips under download section in holdingspace.
func DownloadChipHelper(chipType ChipType) *ChipHelper {
	return &ChipHelper{
		chipFinder: FindDownloadChip(),
		chipType:   chipType,
	}
}

// PinnedChipHelper returns the helper to interact with the chips under pinned file section in holdingspace.
func PinnedChipHelper() *ChipHelper {
	return &ChipHelper{
		chipFinder: FindPinnedFileChip(),
		chipType:   Done,
	}
}

// Finder returns the finder of the chip.
func (chip *ChipHelper) Finder(fileName string) *nodewith.Finder {
	return chip.chipFinder.Name(chip.name(fileName))
}

// name returns the name of the chip.
func (chip *ChipHelper) name(fileName string) string {
	switch chip.chipType {
	case Downloading:
		return fmt.Sprintf("Downloading %s", fileName)
	case Paused:
		return fmt.Sprintf("Download paused %s", fileName)
	default:
		return fileName
	}
}

// WaitUntilAllRemoved waits until all chips with specified file names are removed.
func (chip *ChipHelper) WaitUntilAllRemoved(tconn *chrome.TestConn, files []string) uiauto.Action {
	ui := uiauto.New(tconn)
	actions := make([]uiauto.Action, 0, len(files))
	for _, file := range files {
		actions = append(actions,
			ui.WaitUntilGone(chip.Finder(file)),
			ui.EnsureGoneFor(chip.Finder(file), 5*time.Second),
		)
	}
	return uiauto.Combine("wait for all chips are removed", actions...)
}

// WaitUntilAllExist waits until all chips with specified file names are exist.
func (chip *ChipHelper) WaitUntilAllExist(tconn *chrome.TestConn, files []string) uiauto.Action {
	ui := uiauto.New(tconn)
	actions := make([]uiauto.Action, 0, len(files))
	for _, file := range files {
		actions = append(actions, ui.WaitUntilExists(chip.Finder(file)))
	}
	return uiauto.Combine("wait for all chips exist", actions...)
}

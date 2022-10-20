// Copyright 2021 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package holdingspace

import (
	"context"
	"path/filepath"
	"reflect"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

// Class names.
const buttonClassName = "Button"
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

// FindPinnedFilesSectionFilesAppChip returns a finder which locates the holding
// space pinned files section Files app chip node.
func FindPinnedFilesSectionFilesAppChip() *nodewith.Finder {
	return nodewith.Ancestor(nodewith.ClassName(buttonClassName).Ancestor(
		nodewith.ClassName(pinnedFilesSectionClassName))).Name("Open Files")
}

// FindPinnedFilesSectionFilesAppPrompt returns a finder which locates the
// holding space pinned files section Files app prompt node.
func FindPinnedFilesSectionFilesAppPrompt() *nodewith.Finder {
	return nodewith.Ancestor(nodewith.ClassName(pinnedFilesSectionClassName)).
		Name("You can pin your important files here. Open Files app to get started.")
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

// GetScreenshots returns the locations of screenshot files present in the user's
// downloads directory. Screenshot files are assumed to match a specific pattern.
func GetScreenshots(downloadsPath string) (map[string]struct{}, error) {
	result := make(map[string]struct{})
	screenshots, err := filepath.Glob(filepath.Join(
		downloadsPath, "Screenshot*.png"))
	if err == nil {
		for i := range screenshots {
			result[screenshots[i]] = struct{}{}
		}
	}
	return result, err
}

// TakeScreenshot captures a fullscreen screenshot using the virtual keyboard
// and returns the location of the screenshot file in the user's downloads
// directory. This should behave consistently across device form factors.
func TakeScreenshot(ctx context.Context, downloadsPath string) (string, error) {
	var result string

	// Cache existing screenshots.
	screenshots, err := GetScreenshots(downloadsPath)
	if err != nil {
		return result, err
	}

	// Create virtual keyboard.
	keyboard, err := input.VirtualKeyboard(ctx)
	if err != nil {
		return result, err
	}
	defer keyboard.Close()

	// Take a screenshot.
	if err := keyboard.Accel(ctx, "Ctrl+F5"); err != nil {
		return result, err
	}

	// Wait for screenshot.
	err = testing.Poll(ctx, func(ctx context.Context) error {
		newScreenshots, err := GetScreenshots(downloadsPath)
		if err != nil {
			return testing.PollBreak(err)
		}
		if reflect.DeepEqual(screenshots, newScreenshots) {
			return errors.New("waiting for screenshot")
		}
		for newScreenshot := range newScreenshots {
			if _, exists := screenshots[newScreenshot]; !exists {
				result = newScreenshot
				return nil
			}
		}
		return nil
	}, nil)

	return result, err
}

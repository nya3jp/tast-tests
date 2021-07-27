// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

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
	"chromiumos/tast/local/chrome/uiauto/state"
	"chromiumos/tast/local/coords"
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

// FileIsPinned confirms that an item is pinned to holding space.
func FileIsPinned(ctx context.Context, tconn *chrome.TestConn, name string) error {
	if err := openHoldingSpaceBubble(ctx, tconn); err != nil {
		return err
	}

	uia := uiauto.New(tconn)
	if err := uia.WaitUntilExists(FindPinnedFileChip(name))(ctx); err != nil {
		return errors.Errorf("failed to find pinned item %s: %s", name, err)
	}
	return nil
}

// WaitUntilNotPinned waits for the node of an item pinned in holding space to
// be gone.
func WaitUntilNotPinned(ctx context.Context, tconn *chrome.TestConn, name string) error {
	uia := uiauto.New(tconn)

	// If there is no holding space tray, it means nothing is pinned. We can return.
	if err := uia.WithTimeout(2 * time.Second).
		WaitUntilExists(FindTray()); err != nil {
		return nil
	}

	if err := openHoldingSpaceBubble(ctx, tconn); err != nil {
		return err
	}

	if err := uia.EnsureGoneFor(FindPinnedFileChip(name), 3*time.Second)(ctx); err != nil {
		return errors.Errorf("Unpinning file %s failed: %s", name, err)
	}

	return nil
}

// CreateFile creates a new nonsense text file in My Files. Returns the file's
// path. Caller is responsible for deleting the file.
func CreateFile(ctx context.Context, tconn *chrome.TestConn,
	newFileName string) (string, error) {
	newFilePath := filepath.Join(filesapp.MyFilesPath, newFileName)
	// Create our file, with appropriate permissions so we can delete later.
	if err := ioutil.WriteFile(newFilePath, []byte("Per aspera, ad astra"), 0644); err != nil {
		return "", errors.Errorf("Creating file %s failed: %s", newFilePath, err)
	}
	return newFilePath, nil
}

// UnpinItemViaContextMenu unpins an item from holding space.
func UnpinItemViaContextMenu(ctx context.Context, tconn *chrome.TestConn,
	name string) error {
	uia := uiauto.New(tconn)

	itemChipView := FindPinnedFileChip(name)
	unpinMenuItem := FindContextMenuItem("Unpin")

	if err := uiauto.Combine("Unpin holding space item via context menu",
		uia.WithInterval(200*time.Millisecond).RightClickUntil(itemChipView,
			uia.Exists(unpinMenuItem)),
		uia.LeftClick(unpinMenuItem),
	)(ctx); err != nil {
		errors.Wrap(err, "failed to unpin item from holding space")
	}

	return WaitUntilNotPinned(ctx, tconn, name)
}

// GetHoldingSpaceTrayLocation gets the location of the holding space tray.
func GetHoldingSpaceTrayLocation(ctx context.Context, tconn *chrome.TestConn) (*coords.Rect,
	error) {
	uia := uiauto.New(tconn)
	tray := FindTray()

	uia.WaitUntilExists(tray)
	location, err := uia.Location(ctx, tray)
	if err != nil {
		return nil, errors.Errorf("failed to find holding space tray location: %s", err)
	}
	return location, nil
}

// PinViaFileContextMenu pins a given file visible in the given `FilesApp` to the
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
// instance. A deferred call to os.Remove is recommended to get rid of the junk
// file, to ensure the tray icon stays around until the test is done, but
// doesn't leak files.
func ForceShowHoldingSpaceTrayIcon(ctx context.Context, tconn *chrome.TestConn,
	fsapp *filesapp.FilesApp) (string, error) {
	const junkFileName = "junk.txt"
	junkFilePath, err := CreateFile(ctx, tconn, junkFileName)

	if err != nil {
		return "", errors.Errorf("failed to create file: %s", err)
	}

	if err := PinViaFileContextMenu(ctx, tconn, fsapp, junkFileName); err != nil {
		return "", errors.Errorf("failed to pin file: %s", err)
	}

	return junkFilePath, nil
}

func openHoldingSpaceBubble(ctx context.Context, tconn *chrome.TestConn) error {
	uia := uiauto.New(tconn)

	pinnedFilesBubble := nodewith.ClassName("PinnedFilesBubble")

	// If we find the bubble, it's already open. Return early.
	if uia.Exists(pinnedFilesBubble)(ctx) == nil {
		info, err := uia.Info(ctx, pinnedFilesBubble)
		if err == nil && !info.State[state.Invisible] {
			return nil
		}
	}

	holdingSpaceTray := FindTray()

	if err := uiauto.Combine("Open holding space bubble",
		uia.WaitUntilExists(holdingSpaceTray),
		uia.LeftClick(holdingSpaceTray),
		uia.WaitUntilExists(pinnedFilesBubble),
	)(ctx); err != nil {
		return err
	}

	return nil
}

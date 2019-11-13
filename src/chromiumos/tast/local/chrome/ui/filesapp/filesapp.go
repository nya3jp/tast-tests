// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package filesapp

import (
	"context"
	"time"

	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ui"
	"chromiumos/tast/local/input"
)

// DownloadPath is the location of Downloads for the test user.
const DownloadPath = "/home/chronos/user/Downloads/"

var rootFindParams ui.FindParams = ui.FindParams{
	Name:      "Files",
	Role:      "window",
	ClassName: "RootView",
}

// FilesApp represents an instance of the Files App.
type FilesApp struct {
	tconn *chrome.Conn
	Root  *ui.Node
}

// Launch launches the Files App and returns it.
// An error is returned if the app fails to launch.
func Launch(ctx context.Context, tconn *chrome.Conn) (*FilesApp, error) {
	// Launch the Files App.
	if err := apps.Launch(ctx, tconn, apps.Files.ID); err != nil {
		return nil, err
	}

	// Get UI root.
	root, err := ui.Root(ctx, tconn)
	if err != nil {
		return nil, err
	}
	defer root.Release(ctx)

	// Get Files App root node.
	app, err := root.GetDescendantWithTimeout(ctx, rootFindParams, time.Minute)
	if err != nil {
		return nil, err
	}
	return &FilesApp{tconn: tconn, Root: app}, nil
}

// Close closes the Files App.
func (f *FilesApp) Close(ctx context.Context) {
	defer f.Root.Release(ctx)

	// Click Close button.
	params := ui.FindParams{
		Name:      "Close",
		Role:      "button",
		ClassName: "FrameCaptionButton",
	}
	close, err := f.Root.GetDescendant(ctx, params)
	if err != nil {
		return
	}
	defer close.Release(ctx)
	if err := close.LeftClick(ctx); err != nil {
		return
	}

	// Get UI root.
	root, err := ui.Root(ctx, f.tconn)
	if err != nil {
		return
	}
	defer root.Release(ctx)

	//Wait for window to close
	root.WaitForDescendantRemoved(ctx, rootFindParams, time.Minute)
}

// OpenDownloads opens the Downloads folder in the Files App.
// An error is returned if Downloads is not found or does not open.
func (f *FilesApp) OpenDownloads(ctx context.Context) error {
	// Click Downloads to open the folder.
	params := ui.FindParams{
		Name: "Downloads",
		Role: "treeItem",
	}
	downloads, err := f.Root.GetDescendantWithTimeout(ctx, params, 15*time.Second)
	if err != nil {
		return err
	}
	defer downloads.Release(ctx)
	if err := downloads.LeftClick(ctx); err != nil {
		return err
	}

	// Ensure the Files App has switched to the Downloads folder.
	params = ui.FindParams{
		Name: "Files - Downloads",
		Role: "rootWebArea",
	}
	return f.Root.WaitForDescendantAdded(ctx, params, 15*time.Second)
}

// WaitForFile waits for a file to be visible.
// An error is returned if the timeout is hit.
func (f *FilesApp) WaitForFile(ctx context.Context, filename string, timeout time.Duration) error {
	// Get the Files App listBox.
	params := ui.FindParams{
		Role: "listBox",
	}
	filesBox, err := f.Root.GetDescendant(ctx, params)
	if err != nil {
		return err
	}
	defer filesBox.Release(ctx)

	// Wait for the file.
	params = ui.FindParams{
		Name: filename,
		Role: "staticText",
	}
	return filesBox.WaitForDescendantAdded(ctx, params, 15*time.Second)
}

// SelectFile selects a file by clicking on it.
func (f *FilesApp) SelectFile(ctx context.Context, filename string) error {
	// Get the Files App listBox.
	params := ui.FindParams{
		Role: "listBox",
	}
	filesBox, err := f.Root.GetDescendant(ctx, params)
	if err != nil {
		return err
	}
	defer filesBox.Release(ctx)

	// Click for the file.
	params = ui.FindParams{
		Name: filename,
		Role: "staticText",
	}
	file, err := filesBox.GetDescendant(ctx, params)
	if err != nil {
		return err
	}
	defer file.Release(ctx)
	return file.LeftClick(ctx)
}

// OpenQuickView opens the QuickView menu for a file.
func (f *FilesApp) OpenQuickView(ctx context.Context, filename string) error {
	if err := f.SelectFile(ctx, filename); err != nil {
		return err
	}
	// TODO(bhansknecht@): Figure out why chrome.automation can't click items in the Files App menus.
	// Hopefully the keyboard can be removed from this
	kb, err := input.Keyboard(ctx)
	if err != nil {
		return err
	}
	defer kb.Close()
	return kb.Accel(ctx, "Space")
}

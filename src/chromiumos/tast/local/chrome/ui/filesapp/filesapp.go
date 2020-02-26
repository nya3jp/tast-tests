// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package filesapp supports controlling the Files App on Chrome OS.
package filesapp

import (
	"context"
	"time"

	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ui"
)

// DownloadPath is the location of Downloads for the user.
const DownloadPath = "/home/chronos/user/Downloads/"

// TODO(crbug/1046853): Look for way to not rely on names being in English.
var rootFindParams ui.FindParams = ui.FindParams{
	Name:      "Files",
	Role:      ui.RoleTypeWindow,
	ClassName: "RootView",
}

// FilesApp represents an instance of the Files App.
type FilesApp struct {
	tconn *chrome.TestConn
	Root  *ui.Node
}

// Launch launches the Files App and returns it.
// An error is returned if the app fails to launch.
func Launch(ctx context.Context, tconn *chrome.TestConn) (*FilesApp, error) {
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
	app, err := root.DescendantWithTimeout(ctx, rootFindParams, time.Minute)
	if err != nil {
		return nil, err
	}
	return &FilesApp{tconn: tconn, Root: app}, nil
}

// Close closes the Files App.
func (f *FilesApp) Close(ctx context.Context) error {
	f.Root.Release(ctx)

	// Close the Files App.
	if err := apps.Close(ctx, f.tconn, apps.Files.ID); err != nil {
		return err
	}

	// Get UI root.
	root, err := ui.Root(ctx, f.tconn)
	if err != nil {
		return err
	}
	defer root.Release(ctx)

	// Wait for window to close.
	return root.WaitForDescendant(ctx, rootFindParams, false, time.Minute)
}

// OpenDownloads opens the Downloads folder in the Files App.
// An error is returned if Downloads is not found or does not open.
func (f *FilesApp) OpenDownloads(ctx context.Context) error {
	// Click Downloads to open the folder.
	params := ui.FindParams{
		Name: "Downloads",
		Role: ui.RoleTypeTreeItem,
	}
	downloads, err := f.Root.DescendantWithTimeout(ctx, params, 15*time.Second)
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
		Role: ui.RoleTypeRootWebArea,
	}
	return f.Root.WaitForDescendant(ctx, params, true, 15*time.Second)
}

// file returns a ui.Node that references the specified file.
// An error is returned if the timeout is hit.
func (f *FilesApp) file(ctx context.Context, filename string, timeout time.Duration) (*ui.Node, error) {
	// Limit overall timeout for function.
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// Get the Files App listBox.
	filesBox, err := f.Root.DescendantWithTimeout(ctx, ui.FindParams{Role: ui.RoleTypeListBox}, timeout)
	if err != nil {
		return nil, err
	}
	defer filesBox.Release(ctx)

	// Wait for the file.
	params := ui.FindParams{
		Name: filename,
		Role: ui.RoleTypeStaticText,
	}
	return filesBox.DescendantWithTimeout(ctx, params, timeout)
}

// WaitForFile waits for a file to be visible.
// An error is returned if the timeout is hit.
func (f *FilesApp) WaitForFile(ctx context.Context, filename string, timeout time.Duration) error {
	file, err := f.file(ctx, filename, timeout)
	if err != nil {
		return err
	}
	defer file.Release(ctx)
	return nil
}

// SelectFile selects a file by clicking on it.
func (f *FilesApp) SelectFile(ctx context.Context, filename string) error {
	file, err := f.file(ctx, filename, 15*time.Second)
	if err != nil {
		return err
	}
	defer file.Release(ctx)
	return file.LeftClick(ctx)
}

// OpenQuickView opens the QuickView menu for a file.
func (f *FilesApp) OpenQuickView(ctx context.Context, filename string) error {
	file, err := f.file(ctx, filename, 15*time.Second)
	if err != nil {
		return err
	}
	defer file.Release(ctx)
	if err := file.RightClick(ctx); err != nil {
		return err
	}

	// Left click Get info menuItem.
	params := ui.FindParams{
		Name: "Get info",
		Role: ui.RoleTypeMenuItem,
	}
	getInfo, err := f.Root.DescendantWithTimeout(ctx, params, 15*time.Second)
	if err != nil {
		return err
	}
	defer getInfo.Release(ctx)
	return getInfo.LeftClick(ctx)
}

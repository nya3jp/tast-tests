// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package filesapp supports controlling the Files App on Chrome OS.
package filesapp

import (
	"context"
	"strings"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ui"
	"chromiumos/tast/local/input"
)

// DownloadPath is the location of Downloads for the user.
const DownloadPath = "/home/chronos/user/Downloads/"

const uiTimeout = 15 * time.Second

// Context menu items for a file.
const (
	Open         = "Open"
	OpenWith     = "Open with..."
	Cut          = "Cut"
	Copy         = "Copy"
	Paste        = "Paste"
	GetInfo      = "Get info"
	Rename       = "Rename"
	Delete       = "Delete"
	ZipSelection = "Zip select"
	NewFolder    = "New folder"
)

// Directory names.
const (
	Downloads = "Downloads"
)

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

	// Get Files App root node.
	app, err := ui.FindWithTimeout(ctx, tconn, rootFindParams, time.Minute)
	if err != nil {
		return nil, err
	}

	// The child folders of My Files in the navigation tree are loaded in
	// asynchronously meaning any clicks in the navigation tree at startup
	// may encounter race issues. As Downloads is a fixed child folder of
	// MyFiles, and these folders appear at the same time, wait for the
	// Downloads folder to load to indicate that the tree's ui has settled.
	params := ui.FindParams{
		Name: Downloads,
		Role: ui.RoleTypeTreeItem,
	}
	if err := app.WaitUntilDescendantExists(ctx, params, uiTimeout); err != nil {
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

	// Wait for window to close.
	return ui.WaitUntilGone(ctx, f.tconn, rootFindParams, time.Minute)
}

// OpenDir opens one of the directories shown in the navigation tree.
// An error is returned if dir is not found or does not open.
func (f *FilesApp) OpenDir(ctx context.Context, dirName, expectedTitle string) error {
	// Select dirName in the directory tree.
	params := ui.FindParams{
		Name: dirName,
		Role: ui.RoleTypeTreeItem,
	}
	dir, err := f.Root.DescendantWithTimeout(ctx, params, uiTimeout)
	if err != nil {
		return err
	}
	defer dir.Release(ctx)

	// Within the subtree, click the row to navigate to the location.
	params = ui.FindParams{
		Name: dirName,
		Role: ui.RoleTypeStaticText,
	}

	dirRow, err := dir.DescendantWithTimeout(ctx, params, uiTimeout)
	if err != nil {
		return err
	}

	if err := dirRow.LeftClick(ctx); err != nil {
		return err
	}

	// Ensure the Files App has switched to the folder.
	params = ui.FindParams{
		Name: expectedTitle,
		Role: ui.RoleTypeRootWebArea,
	}
	return f.Root.WaitUntilDescendantExists(ctx, params, uiTimeout)
}

// OpenDownloads opens the Downloads folder in the Files App.
// An error is returned if Downloads is not found or does not open.
func (f *FilesApp) OpenDownloads(ctx context.Context) error {
	return f.OpenDir(ctx, Downloads, "Files - "+Downloads)
}

// OpenDrive opens the Google Drive folder in the Files App.
// An error is returned if Drive is not found or does not open.
func (f *FilesApp) OpenDrive(ctx context.Context) error {
	return f.OpenDir(ctx, "Google Drive", "Files - My Drive")
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
	file, err := f.file(ctx, filename, uiTimeout)
	if err != nil {
		return err
	}
	defer file.Release(ctx)
	return file.LeftClick(ctx)
}

// OpenFile executes double click on a file to open it with default app.
func (f *FilesApp) OpenFile(ctx context.Context, filename string) error {
	file, err := f.file(ctx, filename, uiTimeout)
	if err != nil {
		return err
	}
	defer file.Release(ctx)
	return file.DoubleClick(ctx)
}

// OpenQuickView opens the QuickView menu for a file.
func (f *FilesApp) OpenQuickView(ctx context.Context, filename string) error {
	file, err := f.file(ctx, filename, uiTimeout)
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
	getInfo, err := f.Root.DescendantWithTimeout(ctx, params, uiTimeout)
	if err != nil {
		return err
	}
	defer getInfo.Release(ctx)
	return getInfo.LeftClick(ctx)
}

// ClickMoreMenuItem opens More menu then clicks on sub menu items.
// An error is returned if one of the menu items can't be found.
func (f *FilesApp) ClickMoreMenuItem(ctx context.Context, menuItems []string) error {
	// Open the More Options menu.
	params := ui.FindParams{
		Name: "More…",
		Role: ui.RoleTypePopUpButton,
	}
	more, err := f.Root.DescendantWithTimeout(ctx, params, uiTimeout)
	if err != nil {
		return errors.Wrap(err, "failed finding the More menu item")
	}
	defer more.Release(ctx)

	if err := more.LeftClick(ctx); err != nil {
		return errors.Wrap(err, "failed clicking More menu item")
	}

	// Iterate over the menu items in string and click them
	for _, menuItem := range menuItems {
		params = ui.FindParams{
			Name: menuItem,
			Role: ui.RoleTypeMenuItem,
		}
		menuItemNode, err := f.Root.DescendantWithTimeout(ctx, params, uiTimeout)
		if err != nil {
			return errors.Wrapf(err, "failed finding menu item: %s", menuItem)
		}
		defer menuItemNode.Release(ctx)

		if err := menuItemNode.LeftClick(ctx); err != nil {
			return errors.Wrapf(err, "failed clicking menu item: %s", menuItem)
		}
	}

	return nil
}

// SelectContextMenu right clicks and selects a context menu for a file.
func (f *FilesApp) SelectContextMenu(ctx context.Context, fileName string, menuNames ...string) error {
	file, err := f.file(ctx, fileName, 15*time.Second)
	if err != nil {
		return errors.Wrapf(err, "failed to find %s", fileName)
	}
	defer file.Release(ctx)
	if err := file.RightClick(ctx); err != nil {
		return errors.Wrapf(err, "failed to right click on %s", fileName)
	}

	for _, menuName := range menuNames {
		// Wait location.
		if err := ui.WaitForLocationChangeCompleted(ctx, f.tconn); err != nil {
			return errors.Wrap(err, "failed to wait for animation finished")
		}

		// Left click menuItem.
		if err := f.LeftClickItem(ctx, menuName, ui.RoleTypeMenuItem); err != nil {
			return errors.Wrapf(err, "failed to click %s in context menu", menuName)
		}
	}
	return nil
}

// OpenPath opens a folder.
// Parameter path should be a path to the folder, e.g, Downloads > testfolder1 > subfolder > ...
func (f *FilesApp) OpenPath(ctx context.Context, title string, path ...string) error {
	if len(path) < 1 {
		return errors.New("failed to verify the folder, should contain at least one folder, got 0")
	}
	// Open the directory in the navigation tree.
	if err := f.OpenDir(ctx, path[0], title); err != nil {
		return errors.Wrap(err, "failed to open Linux files")
	}

	// Open folders in the path.
	for _, folder := range path[1:] {
		if err := f.OpenFile(ctx, folder); err != nil {
			return errors.Wrapf(err, "failed to open folder %s", folder)
		}
	}
	return nil
}

// LeftClickItem left clicks a target item.
// An error is returned if the target item can't be found.
func (f *FilesApp) LeftClickItem(ctx context.Context, itemName string, role ui.RoleType) error {
	params := ui.FindParams{
		Name: itemName,
		Role: role,
	}
	item, err := f.Root.DescendantWithTimeout(ctx, params, uiTimeout)
	if err != nil {
		return errors.Wrapf(err, "failed to left click %s", itemName)
	}
	defer item.Release(ctx)
	return item.LeftClick(ctx)
}

// DeleteFileOrFolder deletes a file or folder through selecting Delete in context menu.
func (f *FilesApp) DeleteFileOrFolder(ctx context.Context, fileName string) error {
	// Select Delete from context menu of the file / folder.
	if err := f.SelectContextMenu(ctx, fileName, Delete); err != nil {
		return errors.Wrapf(err, "failed to right click on %s", fileName)
	}

	if err := ui.WaitForLocationChangeCompleted(ctx, f.tconn); err != nil {
		return errors.Wrap(err, "failed to wait for animation finished")
	}

	params := ui.FindParams{
		ClassName: "cr-dialog-ok",
		Name:      Delete,
		Role:      ui.RoleTypeButton,
	}
	deleteButton, err := f.Root.DescendantWithTimeout(ctx, params, 15*time.Second)
	if err != nil {
		return errors.Wrapf(err, "failed to find button Delete after selecting Delet in context menu of %s", fileName)
	}
	defer deleteButton.Release(ctx)

	// Click button "Delete".
	if err := deleteButton.LeftClick(ctx); err != nil {
		return errors.Wrapf(err, "failed to click button Delete on file %s ", fileName)
	}

	if err := f.Root.Update(ctx); err != nil {
		return errors.Wrap(err, "failed to update the Files app content")
	}

	if err = f.Root.WaitUntilDescendantGone(ctx, params, 15*time.Second); err != nil {
		return errors.Wrapf(err, "the deleted file/folder %s is still listed in Files app", fileName)
	}
	return nil
}

// CheckFileDoesNotExist checks a file does not exist in a path.
// Parameter path should be a path to the file, e.g, Downloads > testfolder1 > subfolder > ...
// Return error if any occurs or the file exists in Linux files.
func (f *FilesApp) CheckFileDoesNotExist(ctx context.Context, title, fileName string, path ...string) error {
	// Open the directory in the navigation tree.
	if err := f.OpenPath(ctx, title, path...); err != nil {
		return errors.Wrapf(err, "failed to open %s", strings.Join(path, ">"))
	}

	// Click Refresh.
	if err := f.LeftClickItem(ctx, "Refresh", ui.RoleTypeButton); err != nil {
		return errors.Wrapf(err, "failed to click button Refresh on Files app %s ", fileName)
	}

	// Check the file has gone.
	params := ui.FindParams{
		Name: fileName,
		Role: ui.RoleTypeStaticText,
	}
	if err := f.Root.WaitUntilDescendantGone(ctx, params, uiTimeout); err != nil {
		return errors.Wrapf(err, "file %s still exists", fileName)
	}
	return nil
}

// RenameFile renames a file in a path.
// Parameter path should be a path to the file, e.g, Downloads > testfolder1 > subfolder > ...
func (f *FilesApp) RenameFile(ctx context.Context, keyboard *input.KeyboardEventWriter, title, oldName, newName string, path ...string) error {
	// Open the directory in the navigation tree.
	if err := f.OpenPath(ctx, title, path...); err != nil {
		return errors.Wrapf(err, "failed to open %s", strings.Join(path, ">"))
	}

	// Right click and select rename.
	if err := f.SelectContextMenu(ctx, oldName, Rename); err != nil {
		return errors.Wrapf(err, "failed to select Rename in context menu for file %s in Linux files", oldName)
	}

	// Wait for rename text field.
	params := ui.FindParams{
		Role:  ui.RoleTypeTextField,
		State: map[ui.StateType]bool{ui.StateTypeEditable: true, ui.StateTypeFocusable: true, ui.StateTypeFocused: true},
	}
	if err := f.Root.WaitUntilDescendantExists(ctx, params, uiTimeout); err != nil {
		return errors.Wrap(err, "failed finding rename input text field")
	}

	// Type the new name.
	if err := keyboard.Type(ctx, newName); err != nil {
		return errors.Wrapf(err, "failed to rename the file %s", oldName)
	}

	// Press Enter.
	if err := keyboard.Accel(ctx, "Enter"); err != nil {
		return errors.Wrapf(err, "failed validating the new name of file %s: ", newName)
	}
	return nil
}

// SelectDirectoryContextMenuItem right clicks the specified directory in the navigation tree and selects the specified context menu item.
// An error is returned if dir is not found or right click fails.
func (f *FilesApp) SelectDirectoryContextMenuItem(ctx context.Context, dirName, menuItem string) error {
	// Select dirName in the directory tree.
	params := ui.FindParams{
		Name: dirName,
		Role: ui.RoleTypeTreeItem,
	}
	dir, err := f.Root.DescendantWithTimeout(ctx, params, uiTimeout)
	if err != nil {
		return errors.Wrapf(err, "failed to find %s in the navigation tree", dirName)
	}
	defer dir.Release(ctx)

	// Within the subtree, click the row to navigate to the location.
	params = ui.FindParams{
		Name: dirName,
		Role: ui.RoleTypeStaticText,
	}

	dirRow, err := dir.DescendantWithTimeout(ctx, params, uiTimeout)
	if err != nil {
		return errors.Wrapf(err, "failed to find text %s in the navigation tree", dirName)
	}
	defer dirRow.Release(ctx)

	if err := dirRow.RightClick(ctx); err != nil {
		return errors.Wrapf(err, "failed to right click %s", dirName)
	}

	// Wait location.
	if err := ui.WaitForLocationChangeCompleted(ctx, f.tconn); err != nil {
		return errors.Wrap(err, "failed to wait for animation finished")
	}

	// Left click menuItem.
	if err := f.LeftClickItem(ctx, menuItem, ui.RoleTypeMenuItem); err != nil {
		return errors.Wrapf(err, "failed to click %s in context menu", menuItem)
	}

	return nil
}

// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package filesapp supports controlling the Files App on Chrome OS.
package filesapp

import (
	"context"
	"fmt"
	"regexp"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ui/mouse"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/coords"
	"chromiumos/tast/local/input"
)

// DownloadPath is the location of Downloads for the user.
const DownloadPath = "/home/chronos/user/Downloads/"

// MyFilesPath is the location of My files for the user.
const MyFilesPath = "/home/chronos/user/MyFiles"

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
	Downloads   = "Downloads"
	GoogleDrive = "Google Drive"
	MyDrive     = "My Drive"
	Playfiles   = "Play files"
)

// WindowFinder is the finder for the FilesApp window.
var WindowFinder *nodewith.Finder = nodewith.NameStartingWith("Files").ClassName("RootView").Role(role.Window)

// FilesApp represents an instance of the Files App.
type FilesApp struct {
	ui    *uiauto.Context
	tconn *chrome.TestConn
}

// Launch launches the Files App and returns it.
// An error is returned if the app fails to launch.
func Launch(ctx context.Context, tconn *chrome.TestConn) (*FilesApp, error) {
	// Launch the Files App.
	if err := apps.Launch(ctx, tconn, apps.Files.ID); err != nil {
		return nil, err
	}

	// Create a uiauto.Context with default timeout.
	ui := uiauto.New(tconn)

	// The child folders of My Files in the navigation tree are loaded in
	// asynchronously meaning any clicks in the navigation tree at startup
	// may encounter race issues. As Downloads is a fixed child folder of
	// MyFiles, and these folders appear at the same time, wait for the
	// Downloads folder to load to indicate that the tree's ui has settled.
	downloads := nodewith.Name(Downloads).Role(role.TreeItem).Ancestor(WindowFinder)
	if err := ui.WithTimeout(time.Minute).WaitUntilExists(downloads)(ctx); err != nil {
		return nil, err
	}

	return &FilesApp{tconn: tconn, ui: ui}, nil
}

// Close closes the Files App.
// This is automatically done when chrome resets and is not necessary to call.
func (f *FilesApp) Close(ctx context.Context) error {
	// Close the Files App.
	if err := apps.Close(ctx, f.tconn, apps.Files.ID); err != nil {
		return err
	}

	// Wait for window to close.
	return f.ui.WithTimeout(time.Minute).WaitUntilGone(WindowFinder)(ctx)
}

// OpenDir returns a function that opens one of the directories shown in the navigation tree.
// An error is returned if dir is not found or does not open.
func (f *FilesApp) OpenDir(dirName, expectedTitle string) uiauto.Action {
	dir := nodewith.Name(dirName).Role(role.TreeItem)
	return uiauto.Combine("OpenDir",
		f.LeftClick(nodewith.Name(dirName).Role(role.StaticText).Ancestor(dir)),
		f.WaitUntilExists(nodewith.Name(expectedTitle).Role(role.RootWebArea)),
	)
}

// OpenDownloads returns a function that opens the Downloads folder in the Files App.
// An error is returned if Downloads is not found or does not open.
func (f *FilesApp) OpenDownloads() uiauto.Action {
	return f.OpenDir(Downloads, "Files - "+Downloads)
}

// OpenDrive returns a function that opens the Google Drive folder in the Files App.
// An error is returned if Drive is not found or does not open.
func (f *FilesApp) OpenDrive() uiauto.Action {
	return f.OpenDir(GoogleDrive, "Files - "+MyDrive)
}

// OpenLinuxFiles returns a function that opens the Linux files folder in the Files App.
// An error is returned if Linux files is not found or does not open.
func (f *FilesApp) OpenLinuxFiles() uiauto.Action {
	return f.OpenDir("Linux files", "Files - Linux files")
}

// file returns a nodewith.Finder for a file with the specified name.
func file(fileName string) *nodewith.Finder {
	filesBox := nodewith.Role(role.ListBox)
	return nodewith.Name(fileName).Role(role.StaticText).Ancestor(filesBox)
}

// WaitForFile returns a function that waits for a file to exist.
func (f *FilesApp) WaitForFile(fileName string) uiauto.Action {
	return f.WaitUntilExists(file(fileName))
}

// WaitUntilFileGone returns a function that waits for a file to no longer exist.
func (f *FilesApp) WaitUntilFileGone(fileName string) uiauto.Action {
	return f.WaitUntilGone(file(fileName))
}

// FileExists calls ui.Exists to check whether a folder or a file exists in the Files App.
func (f *FilesApp) FileExists(fileName string) uiauto.Action {
	return f.ui.Exists(file(fileName))
}

// SelectFile returns a function that selects a file by clicking on it.
func (f *FilesApp) SelectFile(fileName string) uiauto.Action {
	return f.LeftClick(file(fileName))
}

// OpenFile returns a function that executes double click on a file to open it with default app.
func (f *FilesApp) OpenFile(fileName string) uiauto.Action {
	return f.DoubleClick(file(fileName))
}

// OpenQuickView returns a function that opens the QuickView menu for a file.
func (f *FilesApp) OpenQuickView(fileName string) uiauto.Action {
	return f.ClickContextMenuItem(fileName, GetInfo)
}

// ClickMoreMenuItem returns a function that opens More menu then clicks on sub menu items.
func (f *FilesApp) ClickMoreMenuItem(menuItems ...string) uiauto.Action {
	var steps []uiauto.Action
	// Open More menu.
	steps = append(steps, f.LeftClick(nodewith.Name("More…").Role(role.PopUpButton)))
	// Iterate over the menu items and click them.
	for _, menuItem := range menuItems {
		steps = append(steps, f.LeftClick(nodewith.Name(menuItem).Role(role.MenuItem)))
	}
	return uiauto.Combine(fmt.Sprintf("ClickMoreMenu(%s)", menuItems), steps...)
}

// ClickContextMenuItem returns a function that right clicks a file to open the context menu and then clicks on sub menu items.
// This method will not select context menu for items in the navigation tree.
func (f *FilesApp) ClickContextMenuItem(fileName string, menuItems ...string) uiauto.Action {
	var steps []uiauto.Action
	// Open Context menu.
	steps = append(steps, f.RightClick(file(fileName)))
	// Iterate over the menu items and click them.
	for _, menuItem := range menuItems {
		steps = append(steps, f.LeftClick(nodewith.Name(menuItem).Role(role.MenuItem)))
	}
	return uiauto.Combine(fmt.Sprintf("ClickContextMenuItem(%s, %s)", fileName, menuItems), steps...)
}

// ClickDirectoryContextMenuItem returns a function that right clicks a directory in the navigation tree to open the context menu and then clicks on sub menu items.
// An error is returned if dir is not found or right click fails.
func (f *FilesApp) ClickDirectoryContextMenuItem(dirName string, menuItems ...string) uiauto.Action {
	var steps []uiauto.Action
	// Open Context menu.
	dir := nodewith.Name(dirName).Role(role.TreeItem)
	steps = append(steps, f.RightClick(nodewith.Name(dirName).Role(role.StaticText).Ancestor(dir)))
	// Iterate over the menu items and click them.
	for _, menuItem := range menuItems {
		steps = append(steps, f.LeftClick(nodewith.Name(menuItem).Role(role.MenuItem)))
	}
	return uiauto.Combine(fmt.Sprintf("ClickDirectoryContextMenuItem(%s, %s)", dirName, menuItems), steps...)
}

// SelectMultipleFiles returns a function that selects multiple items in the Files app listBox while pressing 'Ctrl'.
func (f *FilesApp) SelectMultipleFiles(kb *input.KeyboardEventWriter, fileList ...string) uiauto.Action {
	return func(ctx context.Context) error {
		// Hold Ctrl during multi selection.
		if err := kb.AccelPress(ctx, "Ctrl"); err != nil {
			return errors.Wrap(err, "failed to press Ctrl")
		}
		defer kb.AccelRelease(ctx, "Ctrl")

		for _, fileName := range fileList {
			if err := f.SelectFile(fileName)(ctx); err != nil {
				return errors.Wrapf(err, "failed to select %s", fileName)
			}
		}
		// Ensure the correct number of items are selected.
		var selectionLabelRE = regexp.MustCompile(fmt.Sprintf("%d (files|items|folders) selected", len(fileList)))
		if len(fileList) == 1 {
			selectionLabelRE = regexp.MustCompile("1 (file|item|folder) selected")
		}
		return f.WaitUntilExists(nodewith.Role(role.StaticText).NameRegex(selectionLabelRE))(ctx)
	}
}

// CreateFolder returns a function that creates a new folder named dirName in the current directory.
func (f *FilesApp) CreateFolder(kb *input.KeyboardEventWriter, dirName string) uiauto.Action {
	return uiauto.Combine(fmt.Sprintf("CreateFolder(%s)", dirName),
		f.FocusAndWait(nodewith.Role(role.ListBox)),
		func(ctx context.Context) error {
			// Press Ctrl+E to create a new folder.
			return kb.Accel(ctx, "Ctrl+E")
		},
		// Wait for rename text field.
		f.WaitUntilExists(nodewith.Role(role.TextField).Editable().Focusable().Focused()),
		func(ctx context.Context) error {
			return kb.Type(ctx, dirName)
		},
		func(ctx context.Context) error {
			return kb.Accel(ctx, "Enter")
		},
		f.WaitForFile(dirName),
	)
}

// OpenPath returns a function that opens a folder.
// Parameter path should be a path to the folder, e.g, Downloads > testfolder1 > subfolder > ...
func (f *FilesApp) OpenPath(expectedTitle, dirName string, path ...string) uiauto.Action {
	var steps []uiauto.Action
	// Open the directory in the navigation tree.
	steps = append(steps, f.OpenDir(dirName, expectedTitle))
	// Open folders in the path.
	for _, folder := range path {
		steps = append(steps, f.OpenFile(folder))
	}
	return uiauto.Combine(fmt.Sprintf("OpenPath(%s, %s, %s)", expectedTitle, dirName, path), steps...)
}

// DeleteFileOrFolder returns a function that deletes a file or folder.
// The parent folder must currently be open for this to work.
// Consider using OpenPath to do this.
func (f *FilesApp) DeleteFileOrFolder(kb *input.KeyboardEventWriter, fileName string) uiauto.Action {
	return uiauto.Combine(fmt.Sprintf("DeleteFileOrFolder(%s)", fileName),
		f.SelectFile(fileName),
		func(ctx context.Context) error {
			return kb.Accel(ctx, "Alt+Backspace")
		},
		f.LeftClick(nodewith.Name(Delete).ClassName("cr-dialog-ok").Role(role.Button)),
		f.WaitUntilFileGone(fileName),
	)
}

// RenameFile renames a file that is currently visible.
// To rename a file in a specific directory, first open the path, then rename the file.
func (f *FilesApp) RenameFile(kb *input.KeyboardEventWriter, oldName, newName string) uiauto.Action {
	return uiauto.Combine(fmt.Sprintf("RenameFile(%s, %s)", oldName, newName),
		f.SelectFile(oldName),
		func(ctx context.Context) error {
			// Use Ctrl+Enter enter file rename mode.
			return kb.Accel(ctx, "Ctrl+Enter")
		},
		func(ctx context.Context) error {
			// Select the entire file name including extension.
			return kb.Accel(ctx, "Ctrl+A")
		},
		func(ctx context.Context) error {
			return kb.Type(ctx, newName)
		},
		func(ctx context.Context) error {
			return kb.Accel(ctx, "Enter")
		},
		f.WaitForFile(newName),
	)
}

// Search clicks the search button, enters search text and presses enter.
// The search occurs within the currently visible directory root e.g. Downloads.
func (f *FilesApp) Search(kb *input.KeyboardEventWriter, searchTerms string) uiauto.Action {
	return uiauto.Combine(fmt.Sprintf("Search(%s)", searchTerms),
		f.LeftClick(nodewith.Name("Search").Role(role.Button)),
		f.WaitUntilExists(nodewith.Name("Search").Role(role.SearchBox)),
		func(ctx context.Context) error {
			return kb.Type(ctx, searchTerms)
		},
		func(ctx context.Context) error {
			return kb.Accel(ctx, "Enter")
		},
		// TODO(b/178020071): Check if waiting for the listbox to stabilize is still required.
		// It may be possible to ignore this do to always waiting for stability within queries of the new library.
	)
}

// DragAndDropFile selects the specified file and does a drag and drop to the specified point.
func (f *FilesApp) DragAndDropFile(fileName string, dropPoint coords.Point, kb *input.KeyboardEventWriter) uiauto.Action {
	return func(ctx context.Context) error {
		// Clicking on a file is not enough as the clicks can be too quick for FileInfo
		// to be added to the drop event, this leads to an empty event. Clicking the
		// file and checking the Action Bar we can guarantee FileInfo exists on the
		// drop event.
		if err := f.SelectMultipleFiles(kb, fileName)(ctx); err != nil {
			return errors.Wrap(err, "failed to select the file for drag and drop")
		}

		srcPoint, err := f.ui.Location(ctx, file(fileName))
		if err != nil {
			return errors.Wrap(err, "failed to find the location for the file")
		}

		return mouse.Drag(ctx, f.tconn, srcPoint.CenterPoint(), dropPoint, time.Second)
	}
}

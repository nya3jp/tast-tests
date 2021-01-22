// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package filesapp supports controlling the Files App on Chrome OS.
package filesapp

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ui"
	"chromiumos/tast/local/chrome/ui/mouse"
	"chromiumos/tast/local/coords"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

// DownloadPath is the location of Downloads for the user.
const DownloadPath = "/home/chronos/user/Downloads/"

// MyFilesPath is the location of My files for the user.
const MyFilesPath = "/home/chronos/user/MyFiles"

const uiTimeout = 15 * time.Second

var defaultStablePollOpts = testing.PollOptions{Interval: 100 * time.Millisecond, Timeout: 5 * time.Second}

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
	Share        = "Share"
)

// Directory names.
const (
	Downloads   = "Downloads"
	GoogleDrive = "Google Drive"
	MyDrive     = "My Drive"
	Playfiles   = "Play files"
)

// TODO(crbug/1046853): Look for way to not rely on names being in English.
var rootFindParams ui.FindParams = ui.FindParams{
	Name:      "Files",
	Role:      ui.RoleTypeWindow,
	ClassName: "RootView",
}

// FilesApp represents an instance of the Files App.
type FilesApp struct {
	tconn          *chrome.TestConn
	Root           *ui.Node
	stablePollOpts *testing.PollOptions
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

	return &FilesApp{tconn: tconn, Root: app, stablePollOpts: &defaultStablePollOpts}, nil
}

// Release releases the root node held by the Files App.
// This method is better for screenshots at the end of a test than Close.
func (f *FilesApp) Release(ctx context.Context) {
	f.Root.Release(ctx)
}

// Close closes the Files App and releases the root node.
// Release can be used instead of Close if the goal is just to clean up at the end of a test.
func (f *FilesApp) Close(ctx context.Context) error {
	f.Release(ctx)

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
	// Wait location.
	if err := ui.WaitForLocationChangeCompleted(ctx, f.tconn); err != nil {
		return errors.Wrap(err, "failed to wait for animation finished")
	}

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

	if err := dirRow.StableLeftClick(ctx, f.stablePollOpts); err != nil {
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
	return f.OpenDir(ctx, GoogleDrive, "Files - "+MyDrive)
}

// OpenLinuxFiles opens the Linux files folder in the Files App.
// An error is returned if Linux files is not found or does not open.
func (f *FilesApp) OpenLinuxFiles(ctx context.Context) error {
	return f.OpenDir(ctx, "Linux files", "Files - Linux files")
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
	return file.StableLeftClick(ctx, f.stablePollOpts)
}

// SelectMultipleFiles selects multiple items in the Files app listBox while pressing 'Ctrl'.
func (f *FilesApp) SelectMultipleFiles(ctx context.Context, fileList []string) error {
	// Define keyboard to press 'Ctrl'.
	ew, err := input.Keyboard(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to create keyboard")
	}
	defer ew.Close()

	// Hold Ctrl during multi selection.
	if err := ew.AccelPress(ctx, "Ctrl"); err != nil {
		return errors.Wrap(err, "failed to press Ctrl")
	}
	defer ew.AccelRelease(ctx, "Ctrl")

	// Select files.
	for _, fileName := range fileList {
		if err := f.SelectFile(ctx, fileName); err != nil {
			return errors.Wrapf(err, "failed to select %s", fileName)
		}
	}

	// Define the label associated to the number of files we are selecting.
	var selectionLabelRE = regexp.MustCompile(fmt.Sprintf("%d (files|items|folders) selected", len(fileList)))

	params := ui.FindParams{
		Role: ui.RoleTypeStaticText,
		Attributes: map[string]interface{}{
			"name": selectionLabelRE,
		},
	}

	if err := f.Root.WaitUntilDescendantExists(ctx, params, 5*time.Second); err != nil {
		return errors.Wrap(err, "failed to find expected selection label")
	}
	return nil
}

// OpenFile executes double click on a file to open it with default app.
func (f *FilesApp) OpenFile(ctx context.Context, filename string) error {
	file, err := f.file(ctx, filename, uiTimeout)
	if err != nil {
		return err
	}
	defer file.Release(ctx)
	return file.StableDoubleClick(ctx, f.stablePollOpts)
}

// OpenQuickView opens the QuickView menu for a file.
func (f *FilesApp) OpenQuickView(ctx context.Context, filename string) error {
	file, err := f.file(ctx, filename, uiTimeout)
	if err != nil {
		return err
	}
	defer file.Release(ctx)
	if err := file.StableRightClick(ctx, f.stablePollOpts); err != nil {
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
	return getInfo.StableLeftClick(ctx, f.stablePollOpts)
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

	if err := more.StableLeftClick(ctx, f.stablePollOpts); err != nil {
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

		if err := menuItemNode.StableLeftClick(ctx, f.stablePollOpts); err != nil {
			return errors.Wrapf(err, "failed clicking menu item: %s", menuItem)
		}
	}

	return nil
}

// SelectContextMenu right clicks and selects a context menu for a file in the file list.
// This method will not select context menu for items in the navigation tree.
func (f *FilesApp) SelectContextMenu(ctx context.Context, fileName string, menuNames ...string) error {
	params := ui.FindParams{
		Name: fileName,
		Role: ui.RoleTypeListBoxOption,
	}

	opts := testing.PollOptions{Timeout: 10 * time.Second, Interval: time.Second}
	if err := ui.StableFindAndRightClick(ctx, f.tconn, params, &opts); err != nil {
		return errors.Wrapf(err, "failed to find and right click %s", fileName)
	}

	// Wait location.
	if err := ui.WaitForLocationChangeCompleted(ctx, f.tconn); err != nil {
		return errors.Wrap(err, "failed to wait for animation finished")
	}

	for _, menuName := range menuNames {
		// Left click menuItem.
		if err := f.LeftClickItem(ctx, menuName, ui.RoleTypeMenuItem); err != nil {
			return errors.Wrapf(err, "failed to click %s in context menu", menuName)
		}
	}
	return nil
}

// CreateFolder creates a new folder named |dirName| in the current directory.
func (f *FilesApp) CreateFolder(ctx context.Context, dirName string) error {
	listBox, err := f.Root.DescendantWithTimeout(ctx, ui.FindParams{Role: ui.RoleTypeListBox}, uiTimeout)
	defer listBox.Release(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to find listbox")
	}
	if err := listBox.FocusAndWait(ctx, uiTimeout); err != nil {
		return errors.Wrap(err, "failed to focus on listbox")
	}

	keyboard, err := input.Keyboard(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to create keyboard")
	}
	defer keyboard.Close()

	// Press Ctrl+E to create a new folder.
	if err := keyboard.Accel(ctx, "Ctrl+E"); err != nil {
		return errors.Wrap(err, "failed to press Ctrl+E")
	}

	// Wait for rename text field.
	params := ui.FindParams{
		Role:  ui.RoleTypeTextField,
		State: map[ui.StateType]bool{ui.StateTypeEditable: true, ui.StateTypeFocusable: true, ui.StateTypeFocused: true},
	}
	if err := f.Root.WaitUntilDescendantExists(ctx, params, uiTimeout); err != nil {
		return errors.Wrap(err, "failed finding input text field")
	}

	// Type the new name.
	if err := keyboard.Type(ctx, dirName); err != nil {
		return errors.Wrapf(err, "failed to type the file name %s", dirName)
	}

	// Press Enter.
	if err := keyboard.Accel(ctx, "Enter"); err != nil {
		return errors.Wrapf(err, "failed validating the name of file %s", dirName)
	}

	if err := f.WaitForFile(ctx, dirName, uiTimeout); err != nil {
		return errors.Wrapf(err, "failed to find the newly created folder %s", dirName)
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
	return item.StableLeftClick(ctx, f.stablePollOpts)
}

// DeleteFileOrFolder deletes a file or folder through selecting Delete in context menu.
func (f *FilesApp) DeleteFileOrFolder(ctx context.Context, fileName string) error {
	if err := f.SelectFile(ctx, fileName); err != nil {
		return errors.Wrapf(err, "failed to select %s", fileName)
	}

	keyboard, err := input.Keyboard(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to create keyboard")
	}
	defer keyboard.Close()

	// Press Alt+Backspace to delete.
	if err := keyboard.Accel(ctx, "Alt+Backspace"); err != nil {
		return errors.Wrap(err, "failed to press Alt+Backspace")
	}

	params := ui.FindParams{
		ClassName: "cr-dialog-ok",
		Name:      Delete,
		Role:      ui.RoleTypeButton,
	}
	deleteButton, err := f.Root.DescendantWithTimeout(ctx, params, 15*time.Second)
	if err != nil {
		return errors.Wrapf(err, "failed to find button Delete after pressing Alt+Backspace on %s", fileName)
	}
	defer deleteButton.Release(ctx)

	// Click button "Delete".
	if err := deleteButton.StableLeftClick(ctx, f.stablePollOpts); err != nil {
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
func (f *FilesApp) RenameFile(ctx context.Context, title, oldName, newName string, path ...string) error {
	// Open the directory in the navigation tree.
	if err := f.OpenPath(ctx, title, path...); err != nil {
		return errors.Wrapf(err, "failed to open %s", strings.Join(path, ">"))
	}

	params := ui.FindParams{
		Name: oldName,
		Role: ui.RoleTypeStaticText,
	}

	opts := testing.PollOptions{Timeout: 5 * time.Second, Interval: 500 * time.Millisecond}
	if err := ui.StableFindAndClick(ctx, f.tconn, params, &opts); err != nil {
		return errors.Wrapf(err, "failed to find and click %s", oldName)
	}

	keyboard, err := input.Keyboard(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to create keyboard")
	}
	defer keyboard.Close()

	// Press Ctrl+Enter.
	if err := keyboard.Accel(ctx, "Ctrl+Enter"); err != nil {
		return errors.Wrap(err, "failed press Ctrl+Enter")
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
	// Wait location.
	if err := ui.WaitForLocationChangeCompleted(ctx, f.tconn); err != nil {
		return errors.Wrap(err, "failed to wait for animation finished")
	}

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

	if err := dirRow.StableRightClick(ctx, f.stablePollOpts); err != nil {
		return errors.Wrapf(err, "failed to right click %s", dirName)
	}

	// Wait location.
	if err := ui.WaitForLocationChangeCompleted(ctx, f.tconn); err != nil {
		return errors.Wrap(err, "failed to wait for animation finished")
	}

	item, err := f.Root.DescendantWithTimeout(ctx, ui.FindParams{Name: menuItem, Role: ui.RoleTypeMenuItem}, 2*time.Minute)
	if err != nil {
		return errors.Wrapf(err, "failed to find %s", menuItem)
	}
	defer item.Release(ctx)
	return item.StableLeftClick(ctx, f.stablePollOpts)
}

// SetStablePollOpts sets the polling options for ensuring that a nodes location is stable before clicking.
func (f *FilesApp) SetStablePollOpts(opts *testing.PollOptions) {
	if opts == nil {
		f.stablePollOpts = &defaultStablePollOpts
	} else {
		f.stablePollOpts = opts
	}
}

// Search clicks the search button, enters search text and presses enter.
// The search occurs within the currently visible directory root e.g. Downloads.
func (f *FilesApp) Search(ctx context.Context, searchTerms string) error {
	// Find and left click the search icon.
	params := ui.FindParams{
		Name: "Search",
		Role: ui.RoleTypeButton,
	}
	searchIcon, err := f.Root.DescendantWithTimeout(ctx, params, uiTimeout)
	if err != nil {
		return errors.Wrap(err, "failed to find the search icon")
	}
	defer searchIcon.Release(ctx)

	if err := searchIcon.LeftClick(ctx); err != nil {
		return errors.Wrap(err, "failed to click the search icon")
	}

	// Wait for the search box to appear within view.
	params = ui.FindParams{
		Name: "Search",
		Role: ui.RoleTypeSearchBox,
	}
	if err := f.Root.WaitUntilDescendantExists(ctx, params, uiTimeout); err != nil {
		return errors.Wrap(err, "failed waiting for search box to appear")
	}

	// Get a keyboard handle to type into search box.
	kb, err := input.Keyboard(ctx)
	if err != nil {
		return errors.Wrap(err, "failed trying to get the keyboard handle")
	}
	defer kb.Close()

	// Search box gets focus so type terms into box and press enter.
	if err := kb.Type(ctx, searchTerms); err != nil {
		return errors.Wrap(err, "failed typing the supplied terms")
	}
	if err := kb.Accel(ctx, "enter"); err != nil {
		return errors.Wrap(err, "failed typing the supplied terms")
	}

	return f.waitForListBox(ctx)
}

// waitForListBox waits for the files app listbox to stabilize.
func (f *FilesApp) waitForListBox(ctx context.Context) error {
	// Get the listbox which has the list of files.
	listBox, err := f.Root.DescendantWithTimeout(ctx, ui.FindParams{Role: ui.RoleTypeListBox}, uiTimeout)
	if err != nil {
		return errors.Wrap(err, "failed to find listbox")
	}
	defer listBox.Release(ctx)

	// Setup a watcher to wait for list box to stabilize.
	ew, err := ui.NewWatcher(ctx, listBox, ui.EventTypeActiveDescendantChanged)
	if err != nil {
		return errors.Wrap(err, "failed getting a watcher for the files listbox")
	}
	defer ew.Release(ctx)

	// Check the listbox for any Activedescendantchanged events occurring in a 2 second interval.
	// If any events are found continue polling until uiTimeout is reached.
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		return ew.EnsureNoEvents(ctx, 2*time.Second)
	}, &testing.PollOptions{Timeout: uiTimeout}); err != nil {
		return errors.Wrapf(err, "failed waiting %v for listbox to stabilize", uiTimeout)
	}

	return nil
}

// tickCheckboxForFile clicks the checkbox on a file and waits for selected label.
func (f *FilesApp) tickCheckboxForFile(ctx context.Context, fileName string) (coords.Point, error) {
	ew, err := input.Keyboard(ctx)
	if err != nil {
		return coords.Point{}, errors.Wrap(err, "failed to create keyboard")
	}
	defer ew.Close()

	// Hold Ctrl during selection.
	if err := ew.AccelPress(ctx, "Ctrl"); err != nil {
		return coords.Point{}, errors.Wrap(err, "failed to press Ctrl")
	}
	defer ew.AccelRelease(ctx, "Ctrl")

	// Wait for the file.
	params := ui.FindParams{
		Name: fileName,
		Role: ui.RoleTypeStaticText,
	}
	file, err := f.Root.DescendantWithTimeout(ctx, params, 15*time.Second)
	if err != nil {
		return coords.Point{}, errors.Wrapf(err, "failed finding file %q: %v", fileName, err)
	}
	defer file.Release(ctx)

	if err := file.LeftClick(ctx); err != nil {
		return coords.Point{}, errors.Wrap(err, "failed to left click file")
	}

	params = ui.FindParams{
		Name: "1 file selected",
		Role: ui.RoleTypeStaticText,
	}
	if err := f.Root.WaitUntilDescendantExists(ctx, params, 5*time.Second); err != nil {
		return coords.Point{}, errors.Wrap(err, "failed to find expected selection label")
	}

	return file.Location.CenterPoint(), nil
}

// DragAndDropFile selects the specified file and does a drag and drop to the specified point.
func (f *FilesApp) DragAndDropFile(ctx context.Context, fileName string, dropPoint coords.Point) error {
	// Clicking on a file is not enough as the clicks can be too quick for FileInfo
	// to be added to the drop event, this leads to an empty event. Clicking the
	// file and checking the Action Bar we can guarantee FileInfo exists on the
	// drop event.
	srcPoint, err := f.tickCheckboxForFile(ctx, fileName)
	if err != nil {
		return errors.Wrap(err, "failed selecting file for drag and drop")
	}

	if err := f.waitForListBox(ctx); err != nil {
		return errors.Wrap(err, "failed waiting for drag and drop file selection")
	}

	if err := mouse.Drag(ctx, f.tconn, srcPoint, dropPoint, time.Second); err != nil {
		return errors.Wrap(err, "failed mouse drag")
	}

	return f.waitForListBox(ctx)
}

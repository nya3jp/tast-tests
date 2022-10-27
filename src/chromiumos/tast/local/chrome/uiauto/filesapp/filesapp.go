// Copyright 2021 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package filesapp supports controlling the Files App on ChromeOS.
package filesapp

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"chromiumos/tast/caller"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/filepicker/vars"
	"chromiumos/tast/local/chrome/uiauto/mouse"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/coords"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

// FilesTitlePrefix is the prefix of the Ash window title.
const FilesTitlePrefix = "Files - "

// Context menu items for a file, values are the a11y name.
const (
	Open         = "Open"
	OpenWith     = "Open with…"
	Cut          = "Cut Ctrl+X"
	Copy         = "Copy Ctrl+C"
	Paste        = "Paste Ctrl+V"
	GetInfo      = "Get info Space" // Space is the key shortcut.
	Rename       = "Rename Ctrl+Enter"
	Delete       = "Delete Alt+Backspace"
	ZipSelection = "Zip select"
	NewFolder    = "New folder Ctrl+E"
	Share        = "Share"
)

// Directory names.
const (
	Downloads   = "Downloads"
	GoogleDrive = "Google Drive"
	MyDrive     = "My Drive"
	MyFiles     = "My files"
	Playfiles   = "Play files"
	Recent      = "Recent"
	Images      = "Images"
	Trash       = "Trash"
	USBDrive    = "USB Drive"
)

// FilesApp represents an instance of the Files App.
type FilesApp struct {
	ui    *uiauto.Context
	tconn *chrome.TestConn
	appID string
}

// WindowFinder finds the window based on the Files app type running.
func WindowFinder(appID string) *nodewith.Finder {
	if appID == apps.FilesSWA.ID {
		return nodewith.NameStartingWith("Files").Role(role.Window).HasClass("BrowserFrame").First()
	}
	if appID == vars.FilePickerPseudoAppID {
		return nodewith.Name("Select a file to open").Role(role.Window).ClassName("WebDialogView")
	}
	return nodewith.NameStartingWith("Files").Role(role.Window).ClassName("RootView")
}

// LaunchChromeApp launches the Files Chrome app and returns it.
// An error is returned if the app fails to launch.
func LaunchChromeApp(ctx context.Context, tconn *chrome.TestConn) (*FilesApp, error) {
	// Launch the Files App.
	if err := apps.Launch(ctx, tconn, apps.Files.ID); err != nil {
		return nil, err
	}

	return App(ctx, tconn, apps.Files.ID)
}

// Launch launches the Files app SWA variant and returns it.
// An error is returned if the app fails to launch.
func Launch(ctx context.Context, tconn *chrome.TestConn) (*FilesApp, error) {
	// Launch the Files App.
	if err := apps.LaunchSystemWebApp(ctx, tconn, "File Manager", "chrome://file-manager"); err != nil {
		return nil, err
	}

	return App(ctx, tconn, apps.FilesSWA.ID)
}

// LaunchSWAToPath launches the Files app directly to the supplied path.
// This avoids navigating the Files app when you want to just access the folder directly.
func LaunchSWAToPath(ctx context.Context, tconn *chrome.TestConn, path string) (*FilesApp, error) {
	if !filepath.IsAbs(path) {
		return nil, errors.New("failed as supplied path is not absolute")
	}
	// Wait for the supplied path to exist and to be a directory before launching
	// to it.
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		if info, err := os.Stat(path); os.IsNotExist(err) {
			return errors.New("failed as file does not yet exist")
		} else if err != nil || !info.IsDir() {
			return testing.PollBreak(errors.Wrapf(err, "failed to wait for path %q to exist or path is not a directory", path))
		}
		return nil
	}, &testing.PollOptions{Timeout: 10 * time.Second}); err != nil {
		return nil, err
	}
	if err := tconn.Call(ctx, nil, `tast.promisify(chrome.autotestPrivate.launchFilesAppToPath)`, path); err != nil {
		return nil, errors.Wrapf(err, "failed to launch Files app to path: %q", path)
	}

	return App(ctx, tconn, apps.FilesSWA.ID)
}

// Relaunch closes the existing Files app first then launch the Files app again.
func Relaunch(ctx context.Context, tconn *chrome.TestConn, filesApp *FilesApp) (*FilesApp, error) {
	if err := filesApp.Close(ctx); err != nil {
		return nil, errors.Wrap(err, "failed to close Files app")
	}
	filesApp, err := Launch(ctx, tconn)
	if err != nil {
		return nil, errors.Wrap(err, "failed to launch Files app again")
	}
	return filesApp, nil
}

// App returns an existing instance of the Files app.
// An error is returned if the app cannot be found.
func App(ctx context.Context, tconn *chrome.TestConn, appID string) (*FilesApp, error) {
	if appID == vars.FilePickerPseudoAppID {
		// Only allow uiauto.filepicker to call this function with vars.FilePickerPseudoAppID.
		caller.Check(2, []string{"chromiumos/tast/local/chrome/uiauto/filepicker"})
	}

	// Create a uiauto.Context with default timeout.
	ui := uiauto.New(tconn).WithInterval(500 * time.Millisecond)

	// The child folders of My Files in the navigation tree are loaded in
	// asynchronously meaning any clicks in the navigation tree at startup
	// may encounter race issues. As Downloads is a fixed child folder of
	// MyFiles, and these folders appear at the same time, wait for the
	// Downloads folder to load to indicate that the tree's ui has settled.
	downloads := nodewith.Name(Downloads).Role(role.TreeItem).Ancestor(WindowFinder(appID))
	if err := ui.WithTimeout(time.Minute).WaitUntilExists(downloads)(ctx); err != nil {
		return nil, err
	}

	return &FilesApp{tconn: tconn, ui: ui, appID: appID}, nil
}

// Close closes the Files App.
// This is automatically done when chrome resets and is not necessary to call.
func (f *FilesApp) Close(ctx context.Context) error {
	// Close the Files App.
	if err := apps.Close(ctx, f.tconn, f.appID); err != nil {
		return err
	}

	// Wait for window to close.
	return f.ui.WithTimeout(time.Minute).WaitUntilGone(WindowFinder(f.appID))(ctx)
}

// OpenDir returns a function that opens one of the directories shown in the navigation tree.
// An error is returned if dir is not found or does not open.
func (f *FilesApp) OpenDir(dirName, expectedTitle string) uiauto.Action {
	dir := nodewith.Name(dirName).Role(role.TreeItem)
	roleType := role.RootWebArea
	if f.appID == apps.FilesSWA.ID {
		roleType = role.Window
	}
	if f.appID == vars.FilePickerPseudoAppID {
		// For the picker, we check that the button in the header exists.
		roleType = role.Button
	}
	return uiauto.Combine("OpenDir",
		f.LeftClick(nodewith.Name(dirName).Role(role.StaticText).Ancestor(dir)),
		f.WaitUntilExists(nodewith.Name(expectedTitle).Role(roleType).First()),
	)
}

// FormatDevice returns a function that formats USB drive with the default options.
func (f *FilesApp) FormatDevice() uiauto.Action {
	return uiauto.Combine("FormatDevice",
		f.LeftClick(nodewith.Name("Format device").Role(role.Button)),
		f.WaitUntilExists(nodewith.Name("Erase and Format").Role(role.Button)),
		f.LeftClick(nodewith.Name("Erase and Format").Role(role.Button)),
	)
}

// OpenDownloads returns a function that opens the Downloads folder in the Files App.
// An error is returned if Downloads is not found or does not open.
func (f *FilesApp) OpenDownloads() uiauto.Action {
	return f.OpenDir(Downloads, FilesTitlePrefix+Downloads)
}

// OpenPlayfiles returns a function that opens the "Play files" folder in the Files App.
// An error is returned if "Play files"" is not found or does not open.
func (f *FilesApp) OpenPlayfiles() uiauto.Action {
	return f.OpenDir(Playfiles, FilesTitlePrefix+Playfiles)
}

// OpenDrive returns a function that opens the Google Drive folder in the Files App.
// An error is returned if Drive is not found or does not open.
func (f *FilesApp) OpenDrive() uiauto.Action {
	return f.OpenDir(GoogleDrive, FilesTitlePrefix+MyDrive)
}

// OpenLinuxFiles returns a function that opens the Linux files folder in the Files App.
// An error is returned if Linux files is not found or does not open.
func (f *FilesApp) OpenLinuxFiles() uiauto.Action {
	return f.OpenDir("Linux files", FilesTitlePrefix+"Linux files")
}

// OpenTrash returns a function that opens the Trash folder in the Files App.
// An error is returned if Trash directory is not found or does not open.
func (f *FilesApp) OpenTrash() uiauto.Action {
	return f.OpenDir(Trash, FilesTitlePrefix+Trash)
}

// OpenUSBDrive returns a function that opens default unformatted USB drive in the Files App.
func (f *FilesApp) OpenUSBDrive() uiauto.Action {
	return f.OpenDir(USBDrive, FilesTitlePrefix+USBDrive)
}

// OpenUSBDriveWithName returns a function that opens USB drive with drive in the Files App.
func (f *FilesApp) OpenUSBDriveWithName(driveName string) uiauto.Action {
	return f.OpenDir(driveName, FilesTitlePrefix+driveName)
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

// EnsureFileGone returns a function that ensures that the file doesn't appear for the duration.
func (f *FilesApp) EnsureFileGone(fileName string, duration time.Duration) uiauto.Action {
	return f.EnsureGoneFor(file(fileName), duration)
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

// OpenContextMenu returns a function that selects a file, then executes right click to open its context menu.
func (f *FilesApp) OpenContextMenu(fileName string) uiauto.Action {
	// Select file to ensure context menu items are calculated and menu is
	// sized before it is shown and positioned.
	return uiauto.Combine(fmt.Sprintf("OpenContextMenu(%s)", fileName),
		f.SelectFile(fileName),
		f.RightClick(file(fileName)),
	)
}

// OpenQuickView returns a function that opens the QuickView menu for a file.
func (f *FilesApp) OpenQuickView(fileName string) uiauto.Action {
	return f.ClickContextMenuItem(fileName, GetInfo)
}

// CopyFileToClipboard returns a function that copies the file to the clipboard.
func (f *FilesApp) CopyFileToClipboard(fileName string) uiauto.Action {
	return f.ClickContextMenuItem(fileName, Copy)
}

// PasteFileFromClipboard returns a function that pastes a file from the clipboard to the current directory.
func (f *FilesApp) PasteFileFromClipboard(kb *input.KeyboardEventWriter) uiauto.Action {
	return uiauto.Combine("PasteFromClipboard()",
		kb.AccelAction("Ctrl+V"), // Press Ctrl+V to paste the file.
	)
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
	steps = append(steps, f.OpenContextMenu(fileName))
	// Iterate over the menu items and click them.
	for _, menuItem := range menuItems {
		steps = append(steps, f.LeftClick(nodewith.Name(menuItem).Role(role.MenuItem)))
	}
	return uiauto.Combine(fmt.Sprintf("ClickContextMenuItem(%s, %s)", fileName, menuItems), steps...)
}

// ClickContextMenuItemRegex returns a function that right clicks a file to
// open the context menu and then clicks on sub menu items using regexp.
// This method will not select context menu for items in the navigation tree.
func (f *FilesApp) ClickContextMenuItemRegex(fileName string, menuItems ...string) uiauto.Action {
	var steps []uiauto.Action
	// Open Context menu.
	steps = append(steps, f.OpenContextMenu(fileName))
	// Iterate over the menu items and click them.
	for _, menuItem := range menuItems {
		steps = append(steps, f.LeftClick(nodewith.NameRegex(regexp.MustCompile(menuItem)).Role(role.MenuItem)))
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
		// First press Esc to clear any selection.
		if err := kb.Accel(ctx, "Esc"); err != nil {
			return errors.Wrap(err, "failed to clear selection")
		}
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
		var selectionLabelRE = regexp.MustCompile(fmt.Sprintf("%d (file|item|folder)s? selected", len(fileList)))
		return f.WaitUntilExists(nodewith.Role(role.StaticText).NameRegex(selectionLabelRE))(ctx)
	}
}

// CreateFolder returns a function that creates a new folder named dirName in the current directory.
func (f *FilesApp) CreateFolder(kb *input.KeyboardEventWriter, dirName string) uiauto.Action {
	return uiauto.Combine(fmt.Sprintf("CreateFolder(%s)", dirName),
		f.FocusAndWait(nodewith.Role(role.ListBox)),
		kb.AccelAction("Ctrl+E"), // Press Ctrl+E to create a new folder.
		// Wait for rename text field.
		f.WaitUntilExists(nodewith.Role(role.TextField).Editable().Focusable().Focused()),
		kb.TypeAction(dirName),
		kb.AccelAction("Enter"),
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
		kb.AccelAction("Alt+Backspace"),
		f.WaitUntilFileGone(fileName),
	)
}

// DeleteMultipleFilesOrFolders returns a function that deletes multiple files or folders.
// The parent folder must currently be open for this to work.
// Consider using OpenPath to do this.
func (f *FilesApp) DeleteMultipleFilesOrFolders(kb *input.KeyboardEventWriter, targets ...string) uiauto.Action {
	actions := make([]uiauto.Action, len(targets))
	for i, files := range targets {
		actions[i] = f.WaitUntilFileGone(files)
	}
	waitUnitAllTargetsGone := uiauto.Combine("verify all targets are gone", actions...)

	return uiauto.Combine(fmt.Sprintf("DeleteMultipleFilesOrFolders(%s)", targets),
		f.SelectMultipleFiles(kb, targets...),
		kb.AccelAction("Alt+Backspace"),
		waitUnitAllTargetsGone,
	)
}

// TrashFileOrFolder returns a function that trashes a file or folder.
// The parent folder must currently be open for this to work.
// NOTE: The FilesTrash feature must be enabled for this to properly trash, if
// the feature is not enabled, please use DeleteFileOrFolder which correctly
// clicks on the confirmation dialog that appears on deletion.
func (f *FilesApp) TrashFileOrFolder(kb *input.KeyboardEventWriter, fileName string) uiauto.Action {
	return uiauto.Combine(fmt.Sprintf("TrashFileOrFolder(%s)", fileName),
		f.SelectFile(fileName),
		kb.AccelAction("Alt+Backspace"),
		f.WaitUntilFileGone(fileName),
	)
}

// RenameFile renames a file that is currently visible.
// To rename a file in a specific directory, first open the path, then rename the file.
func (f *FilesApp) RenameFile(kb *input.KeyboardEventWriter, oldName, newName string) uiauto.Action {
	return uiauto.Combine(fmt.Sprintf("RenameFile(%s, %s)", oldName, newName),
		f.SelectFile(oldName),
		kb.AccelAction("Ctrl+Enter"), // Use Ctrl+Enter enter file rename mode.
		kb.AccelAction("Ctrl+A"),     // Select the entire file name including extension.
		kb.TypeAction(newName),
		kb.AccelAction("Enter"),
		f.WaitForFile(newName),
	)
}

// Search clicks the search button, enters search text and presses enter.
// The search occurs within the currently visible directory root e.g. Downloads.
func (f *FilesApp) Search(kb *input.KeyboardEventWriter, searchTerms string) uiauto.Action {
	return uiauto.Combine(fmt.Sprintf("Search(%s)", searchTerms),
		f.LeftClick(nodewith.Name("Search").Role(role.Button)),
		f.WaitUntilExists(nodewith.Name("Search").Role(role.SearchBox)),
		kb.TypeAction(searchTerms),
		kb.AccelAction("Enter"),
		// TODO(b/178020071): Check if waiting for the listbox to stabilize is still required.
		// It may be possible to ignore this do to always waiting for stability within queries of the new library.
	)
}

// ClearSearch clicks the clear button to clear the search results and leave search mode.
func (f *FilesApp) ClearSearch() uiauto.Action {
	clear := nodewith.Role(role.Button).ClassName("clear").Name("Clear")
	return uiauto.Combine("clear search box",
		uiauto.IfSuccessThen(
			f.WithTimeout(5*time.Second).WaitUntilExists(clear),
			f.LeftClick(clear),
		),
		f.EnsureFocused(nodewith.Role(role.ListBox)),
	)
}

// ToggleAvailableOfflineForFile selects the specified file and toggles the Available Offline switch.
func (f *FilesApp) ToggleAvailableOfflineForFile(fileName string) uiauto.Action {
	toggleOfflineErrorOkButton := nodewith.Name("OK").Role(role.Button)
	// Just after startup there's a period of time where making Docs/Sheets/Slides files available offline errors out
	// as DriveFS has not established communication with the Docs Offline extension, so retry if the error appears.
	return f.ui.RetryUntil(
		uiauto.Combine(fmt.Sprintf("Try toggle Available offline for %q", fileName),
			f.SelectFile(fileName),
			f.LeftClick(nodewith.Name("Available offline").Role(role.ToggleButton)),
		),
		// If the error appears, dismiss it and return an error so we will retry.
		uiauto.IfSuccessThen(f.WithTimeout(time.Second).WaitUntilExists(toggleOfflineErrorOkButton),
			func(ctx context.Context) error {
				if err := f.LeftClick(toggleOfflineErrorOkButton)(ctx); err != nil {
					return errors.Wrap(err, "failed to dismiss the error dialog")
				}
				return errors.Errorf("toggling Available offline for %q returned an error", fileName)
			},
		),
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
		// Focus back to FilesApp after drop.
		defer f.LeftClick(nodewith.Role(role.ListBox))(ctx)

		srcPoint, err := f.ui.Location(ctx, file(fileName))
		if err != nil {
			return errors.Wrap(err, "failed to find the location for the file")
		}

		return mouse.Drag(f.tconn, srcPoint.CenterPoint(), dropPoint, time.Second)(ctx)
	}
}

// PerformActionAndRetryMaximizedOnFail attempts an action and if it fails, maximizes the Files app and tries again.
// TODO(crbug/1189914): Remove once the underlying race condition causing the listbox to not populate is fixed.
func (f *FilesApp) PerformActionAndRetryMaximizedOnFail(action uiauto.Action) uiauto.Action {
	return func(ctx context.Context) error {
		err := action(ctx)
		if err == nil {
			return nil
		}
		testing.ContextLog(ctx, "Supplied action failed, resizing window and trying again: ", err)

		window, err := ash.FindWindow(ctx, f.tconn, func(w *ash.Window) bool {
			return strings.HasPrefix(w.Title, FilesTitlePrefix)
		})
		if err != nil {
			return err
		}

		if err := ash.SetWindowStateAndWait(ctx, f.tconn, window.ID, ash.WindowStateMaximized); err != nil {
			return err
		}

		return action(ctx)
	}
}

// ExpandOpenDropdown waits for the Open button (in the toolbar) to be a dropdown and click to display the dropdown menu.
func (f *FilesApp) ExpandOpenDropdown() uiauto.Action {
	openButton := nodewith.Role(role.Button).Collapsed().Name(Open)
	dropdownMenu := nodewith.Role(role.Menu).Name(OpenWith)
	return uiauto.Combine("ExpandOpenDropdown",
		f.WaitUntilExists(openButton),
		f.LeftClick(openButton),
		f.WaitUntilExists(dropdownMenu),
	)
}

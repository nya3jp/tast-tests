// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package settings provides support for the Linux settings on the Settings app.
package settings

import (
	"context"
	"regexp"
	"strconv"
	"strings"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/ossettings"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/crostini/faillog"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

const (
	// SizeB is a multiplier to convert bytes to bytes.
	SizeB = 1
	// SizeKB is a multiplier to convert bytes to kilobytes.
	SizeKB = 1024
	// SizeMB is a multiplier to convert bytes to megabytes.
	SizeMB = 1024 * 1024
	// SizeGB is a multiplier to convert bytes to gigabytes.
	SizeGB = 1024 * 1024 * 1024
	// SizeTB is a multiplier to convert bytes to terabytes.
	SizeTB = 1024 * 1024 * 1024 * 1024
)

const uiTimeout = 10 * time.Second

// Sub settings name.
const (
	ManageSharedFolders = "Manage shared folders"
)

// Window names for different settings page.
const (
	PageNameLinux = "Settings - Linux development environment"
	PageNameMSF   = "Settings - " + ManageSharedFolders
)

// find params for fixed items.
var (
	PageLinux             = nodewith.NameStartingWith(PageNameLinux).First()
	DevelopersButton      = nodewith.NameRegex(regexp.MustCompile("Developers|Linux.*")).Role(role.Button).Ancestor(ossettings.WindowFinder)
	nextButton            = nodewith.Name("Next").Role(role.Button)
	settingsHead          = nodewith.Name("Settings").Role(role.Heading)
	emptySharedFoldersMsg = nodewith.Name("Shared folders will appear here").Role(role.StaticText)
	sharedFoldersList     = nodewith.Name("Shared folders").Role(role.List)
	unshareFailDlg        = nodewith.Name("Unshare failed").Role(role.StaticText).Ancestor(nodewith.Role(role.Dialog))
	tryAgainButton        = nodewith.Name("Try again").Role(role.Button)
	removeLinuxButton     = nodewith.NameRegex(regexp.MustCompile(`Remove.*`)).Role(role.Button)
	removeLinuxDialog     = nodewith.NameRegex(regexp.MustCompile("Remove|Delete")).Role(role.Dialog).First()
	resizeButton          = nodewith.Name("Change disk size").Role(role.Button)
	RemoveLinuxAlert      = nodewith.Name("Remove Linux development environment").Role(role.AlertDialog).ClassName("Widget")
	BackupButton          = nodewith.NameStartingWith("Backup Linux").Role(role.Button).Ancestor(ossettings.WindowFinder)
	RestoreButton         = nodewith.NameStartingWith("Replace").Role(role.Button).Ancestor(ossettings.WindowFinder)
	BackupFileWindow      = nodewith.Name("Backup").Role(role.Window).ClassName("ExtensionViewViews")
	BackupSave            = nodewith.Name("Save").Role(role.Button).Ancestor(BackupFileWindow)
	BackupNotification    = nodewith.NameStartingWith("Backup complete").Role(role.AlertDialog).ClassName("MessagePopupView")
	RestoreNotification   = nodewith.NameStartingWith("Restore complete").Role(role.AlertDialog).ClassName("MessagePopupView")
	RestoreConfirmButton  = nodewith.Name("Restore").Role(role.Button).ClassName("action-button")
	RestoreFileWindow     = nodewith.Name("Restore").Role(role.Window).ClassName("ExtensionViewViews")
	RestoreTiniFile       = nodewith.NameContaining(".tini").Role(role.StaticText).Ancestor(RestoreFileWindow)
	RestoreOpen           = nodewith.Name("Open").Role(role.Button).Ancestor(RestoreFileWindow)
)

// Settings represents an instance of the Linux settings in Settings App.
type Settings struct {
	ui    *uiauto.Context
	tconn *chrome.TestConn
}

// OpenLinuxSubpage opens Linux subpage on Settings page. If Linux is not installed, it opens the installer.
func OpenLinuxSubpage(ctx context.Context, tconn *chrome.TestConn, cr *chrome.Chrome) (*Settings, error) {
	// Open Settings app.
	if err := ash.CloseNotifications(ctx, tconn); err != nil {
		return nil, errors.Wrap(err, "failed to close all notifications in OpenLinuxSubpage()")
	}

	ui := uiauto.New(tconn)
	if _, err := ossettings.LaunchAtPageURL(ctx, tconn, cr, "crostini", ui.WaitUntilExists(DevelopersButton)); err != nil {
		return nil, errors.Wrap(err, "failed to launch settings app")
	}
	if err := ui.LeftClick(DevelopersButton)(ctx); err != nil {
		return nil, errors.Wrap(err, "failed to go to linux subpage")
	}

	return &Settings{tconn: tconn, ui: ui}, nil
}

// OpenLinuxSettings opens Settings app and navigate to Linux Settings and its sub settings if any.
// Returns a Settings instance.
func OpenLinuxSettings(ctx context.Context, tconn *chrome.TestConn, cr *chrome.Chrome, subSettings ...string) (*Settings, error) {
	s, err := OpenLinuxSubpage(ctx, tconn, cr)
	if err != nil {
		return nil, errors.Wrap(err, "failed to open linux subpage on Settings app")
	}

	// Open the sub Settings.
	for _, setting := range subSettings {
		if err := uiauto.New(tconn).LeftClick(nodewith.Name(setting).Role(role.Link).Ancestor(ossettings.WindowFinder))(ctx); err != nil {
			return nil, errors.Wrapf(err, "failed to open sub setting %s", setting)
		}
	}

	return s, nil
}

// FindSettingsPage finds a pre-opened Settings page with a window name.
func FindSettingsPage(ctx context.Context, tconn *chrome.TestConn, windowName string) (s *Settings, err error) {
	// Create a uiauto.Context with default timeout.
	ui := uiauto.New(tconn)

	// Check Settings app is opened to the specific page.
	if err := ui.WaitUntilExists(nodewith.NameRegex(regexp.MustCompile(".*" + windowName + ".*")).First())(ctx); err != nil {
		return nil, errors.Wrapf(err, "failed to find window %s", windowName)
	}

	return &Settings{tconn: tconn, ui: ui}, nil
}

// OpenInstaller clicks the "Turn on" Linux button to open the Crostini installer.
//
// It also clicks next to skip the information screen.  An ui.Installer
// page object can be constructed after calling OpenInstaller to adjust the settings and to complete the installation.
func OpenInstaller(ctx context.Context, tconn *chrome.TestConn, cr *chrome.Chrome) (retErr error) {
	s, err := OpenLinuxSubpage(ctx, tconn, cr)
	if err != nil {
		return errors.Wrap(err, "failed to open linux subpage on Settings app")
	}
	defer s.Close(ctx)
	defer func() { faillog.DumpUITreeAndScreenshot(ctx, tconn, "crostini_installer", retErr) }()
	return s.ui.LeftClick(nextButton)(ctx)
}

// Close closes the Settings App.
func (s *Settings) Close(ctx context.Context) error {
	// Close the Settings App.
	if err := apps.Close(ctx, s.tconn, apps.Settings.ID); err != nil {
		return errors.Wrap(err, "failed to close Settings app")
	}

	// Wait for the window to close.
	return s.ui.WaitUntilGone(settingsHead)(ctx)
}

// GetSharedFolders returns a list of shared folders.
// Settings must be open at the Linux Manage Shared Folders page.
func (s *Settings) GetSharedFolders(ctx context.Context) ([]string, error) {
	// Polling here works around a couple of different races:
	// - On first load, both the empty shared folders message and the shared folders heading
	//   are present. Normally only one of these is present.
	// - When folders are shared or unshared (deleted), the UI will asynchronously update.
	//   If this update occurs between textErr/listErr/sharedFolders being evaluated, it
	//   can cause the page to appear to be in an inconsistent state.

	var listOfFolders []string
	err := testing.Poll(ctx, func(ctx context.Context) error {
		// Find "Shared folders will appear here". It will be displayed if no folder is shared.
		textErr := s.ui.WithTimeout(2 * time.Second).WaitUntilExists(emptySharedFoldersMsg)(ctx)

		// Find "Shared folders" list. It will be displayed if any folder is shared.
		listErr := s.ui.WithTimeout(2 * time.Second).WaitUntilExists(sharedFoldersList)(ctx)

		if textErr == nil && listErr == nil {
			return errors.New("page appears to be in an inconsistent state, it may be still initialising")
		} else if textErr != nil && listErr != nil {
			return errors.Wrap(listErr, "page appears to be in an inconsistent state")
		} else if textErr == nil && listErr != nil {
			// No shared folders
			return nil
		} else {
			// Found shared folders
			sharedFolderButtons := nodewith.Role(role.Button).Ancestor(sharedFoldersList)
			sharedFolders, err := s.ui.NodesInfo(ctx, sharedFolderButtons)
			if err != nil {
				return errors.Wrap(err, "page appears to be in an inconsistent state")
			}
			for _, folder := range sharedFolders {
				listOfFolders = append(listOfFolders, folder.Name)
			}
			return nil
		}
	}, &testing.PollOptions{Timeout: 10 * time.Second})

	if err != nil {
		return nil, err
	}
	return listOfFolders, nil
}

// UnshareFolder deletes a shared folder from Settings app.
// Settings must be open at the Linux Manage Shared Folders page.
func (s *Settings) UnshareFolder(ctx context.Context, folder string) error {
	folderButton := nodewith.Name(folder).Role(role.Button).Ancestor(sharedFoldersList)

	// Unsharing can fail and show a modal dialog if a file is detected as in use by
	// the guest. This can happen if a file was recently accessed, so try a couple
	// of times to unshare and only return an error if that fails.
	retryUnshare := uiauto.Combine("retry unsharing",
		s.ui.LeftClick(tryAgainButton),
		s.ui.WaitUntilGone(unshareFailDlg),
		s.ui.EnsureGoneFor(unshareFailDlg, 5*time.Second))
	retryIfFailed := s.ui.IfSuccessThen(
		s.ui.WithTimeout(5*time.Second).WaitUntilExists(unshareFailDlg),
		s.ui.WithPollOpts(testing.PollOptions{Interval: 5 * time.Second}).Retry(4, retryUnshare))

	return uiauto.Combine("unshare folder "+folder,
		s.ui.LeftClick(folderButton),
		retryIfFailed,
		s.ui.IfSuccessThen(s.ui.Exists(sharedFoldersList), s.ui.WaitUntilGone(folderButton)),
	)(ctx)
}

type removeConfirmDialogStruct struct {
	Self   *nodewith.Finder
	Delete *nodewith.Finder
	Cancel *nodewith.Finder
}

// RemoveConfirmDialog represents an instance of the confirm dialog of removing Crostini.
var RemoveConfirmDialog = removeConfirmDialogStruct{
	Self:   removeLinuxDialog,
	Delete: nodewith.Name("Delete").Role(role.Button).Ancestor(removeLinuxDialog),
	Cancel: nodewith.Name("Cancel").Role(role.Button).Ancestor(removeLinuxDialog),
}

// ClickRemove clicks Remove to launch the delete.
func (s *Settings) ClickRemove() uiauto.Action {
	return uiauto.Combine("to click button Remove to launch delete dialog",
		s.ui.LeftClick(removeLinuxButton),
		s.ui.WaitUntilExists(RemoveConfirmDialog.Self))
}

// Remove removes Crostini.
func (s *Settings) Remove() uiauto.Action {
	return uiauto.Combine("remove Linux",
		s.ClickRemove(),
		s.ui.LeftClick(RemoveConfirmDialog.Delete),
		s.ui.WaitUntilExists(RemoveLinuxAlert),
		s.ui.WaitUntilGone(RemoveLinuxAlert),
		s.ui.WaitUntilExists(DevelopersButton))
}

type resizeDiskDialogStruct struct {
	Self   *nodewith.Finder
	Slider *nodewith.Finder
	Resize *nodewith.Finder
	Cancel *nodewith.Finder
}

// ResizeDiskDialog represents an instance of the Resize Linux disk dialog.
var ResizeDiskDialog = resizeDiskDialogStruct{
	Self:   nodewith.Name("Resize Linux disk").Role(role.GenericContainer),
	Slider: nodewith.Role(role.Slider),
	Resize: nodewith.Name("Resize").Role(role.Button),
	Cancel: nodewith.Name("Cancel").Role(role.Button).ClassName("cancel-button"),
}

// ClickChange clicks Change to launch the resize dialog.
func (s *Settings) ClickChange() uiauto.Action {
	return uiauto.Combine("click button resize and wait for resize dialog and slider",
		s.ui.LeftClick(resizeButton),
		s.ui.WaitUntilExists(ResizeDiskDialog.Self),
		s.ui.WithTimeout(time.Minute).WaitUntilExists(ResizeDiskDialog.Slider))
}

// GetDiskSize returns the disk size on the Settings app.
func (s *Settings) GetDiskSize(ctx context.Context) (string, error) {
	nodeInfo, err := s.ui.Info(ctx, nodewith.NameRegex(regexp.MustCompile(`[0-9]+.[0-9]+ GB`)).Role(role.StaticText))
	if err != nil {
		return "", errors.Wrap(err, "failed to find disk size information on the Settings app")
	}
	return nodeInfo.Name, nil
}

// ResizeDisk resizes the VM disk to approximately targetSize via the settings app.
// If growing the VM disk, set increase to true, otherwise set it to false.
func (s *Settings) ResizeDisk(ctx context.Context, kb *input.KeyboardEventWriter, targetSize uint64, increase bool) error {
	if err := uiauto.Combine("open resize dialog and focus on slider",
		s.ui.LeftClick(resizeButton),
		s.ui.FocusAndWait(ResizeDiskDialog.Slider))(ctx); err != nil {
		return err
	}

	if _, err := ChangeDiskSize(ctx, s.tconn, kb, ResizeDiskDialog.Slider, increase, targetSize); err != nil {
		return errors.Wrap(err, "failed to resize disk")
	}

	return uiauto.Combine("click button Resize and wait resize dialog gone",
		s.ui.LeftClick(ResizeDiskDialog.Resize),
		s.ui.WaitUntilGone(ResizeDiskDialog.Self),
	)(ctx)
}

// GetDiskSize returns the current size based on the disk size slider text.
func GetDiskSize(ctx context.Context, tconn *chrome.TestConn, slider *nodewith.Finder) (uint64, error) {
	nodeInfo, err := uiauto.New(tconn).Info(ctx, nodewith.Role(role.StaticText).Ancestor(slider))
	if err != nil {
		return 0, errors.Wrap(err, "error getting disk size setting")
	}
	return ParseDiskSize(nodeInfo.Name)
}

// ParseDiskSize parses disk size from a string like "xx.x GB" to a uint64 value in bytes.
func ParseDiskSize(sizeString string) (uint64, error) {
	parts := strings.Split(sizeString, " ")
	if len(parts) != 2 {
		return 0, errors.Errorf("failed to parse disk size from %s: does not have exactly 2 space separated parts", sizeString)
	}
	num, err := strconv.ParseFloat(parts[0], 64)
	if err != nil {
		return 0, errors.Wrapf(err, "failed to parse disk size from %s", sizeString)
	}
	unitMap := map[string]float64{
		"B":  SizeB,
		"KB": SizeKB,
		"MB": SizeMB,
		"GB": SizeGB,
		"TB": SizeTB,
	}
	units, ok := unitMap[parts[1]]
	if !ok {
		return 0, errors.Errorf("failed to parse disk size from %s: does not have a recognized units string", sizeString)
	}
	return uint64(num * units), nil
}

// ChangeDiskSize changes the disk size to targetDiskSize through moving the slider.
// If the target disk size is bigger, set increase to true, otherwise set it to false.
// The method will return if it reaches the target or the end of the slider.
// The real size might not be exactly equal to the target because the increment changes depending on the range.
// FocusAndWait(slider) should be called before calling this method.
func ChangeDiskSize(ctx context.Context, tconn *chrome.TestConn, kb *input.KeyboardEventWriter, slider *nodewith.Finder, increase bool, targetDiskSize uint64) (uint64, error) {
	direction := "right"
	if !increase {
		direction = "left"
	}

	for {
		size, err := GetDiskSize(ctx, tconn, slider)
		if err != nil {
			return 0, errors.Wrap(err, "failed to get disk size")
		}
		// Check whether it has reached the target.
		if (increase && size >= targetDiskSize) || (!increase && size <= targetDiskSize) {
			return size, nil
		}

		// Move slider.
		if err := kb.Accel(ctx, direction); err != nil {
			return 0, errors.Wrapf(err, "failed to move slider to %s", direction)
		}

		// Check whether it has reached the end.
		newSize, err := GetDiskSize(ctx, tconn, slider)
		if err != nil {
			return 0, errors.Wrap(err, "failed to get disk size")
		}
		if size == newSize {
			return size, nil
		}
	}
}

// GetCurAndTargetDiskSize gets the current disk size and calculates a target disk size to resize.
func (s *Settings) GetCurAndTargetDiskSize(ctx context.Context, keyboard *input.KeyboardEventWriter) (curSize, targetSize uint64, err error) {
	if err := uiauto.Combine("launch resize dialog and focus on the slider",
		s.ClickChange(),
		s.ui.FocusAndWait(ResizeDiskDialog.Slider))(ctx); err != nil {
		return 0, 0, err
	}

	// Get current size.
	curSize, err = GetDiskSize(ctx, s.tconn, ResizeDiskDialog.Slider)
	if err != nil {
		return 0, 0, errors.Wrap(err, "failed to get initial disk size")
	}

	// Get the minimum size.
	minSize, err := ChangeDiskSize(ctx, s.tconn, keyboard, ResizeDiskDialog.Slider, false, 0)
	if err != nil {
		return 0, 0, errors.Wrap(err, "failed to resize to the minimum disk size")
	}
	// Get the maximum size.
	maxSize, err := ChangeDiskSize(ctx, s.tconn, keyboard, ResizeDiskDialog.Slider, true, 500*SizeGB)
	if err != nil {
		return 0, 0, errors.Wrap(err, "failed to resize to the maximum disk size")
	}

	targetSize = minSize + (maxSize-minSize)/2
	if targetSize == curSize {
		targetSize = minSize + (maxSize-minSize)/3
	}

	if err := uiauto.Combine("click button Cancel and wait resize dialog gone",
		s.ui.LeftClick(ResizeDiskDialog.Cancel),
		s.ui.WaitUntilGone(ResizeDiskDialog.Self))(ctx); err != nil {
		return 0, 0, err
	}

	return curSize, targetSize, nil
}

// Resize changes the disk size to the target size.
// It returns the size on the slider as string and the result size as uint64.
func (s *Settings) Resize(ctx context.Context, keyboard *input.KeyboardEventWriter, curSize, targetSize uint64) (string, uint64, error) {
	if err := uiauto.Combine("launch resize dialog and focus on the slider",
		s.ClickChange(),
		s.ui.FocusAndWait(ResizeDiskDialog.Slider))(ctx); err != nil {
		return "", 0, err
	}

	// Resize to the target size.
	size, err := ChangeDiskSize(ctx, s.tconn, keyboard, ResizeDiskDialog.Slider, targetSize > curSize, targetSize)
	if err != nil {
		return "", 0, errors.Wrapf(err, "failed to resize to %d: ", targetSize)
	}

	// Record the new size on the slider.
	nodeInfo, err := s.ui.Info(ctx, nodewith.Role(role.StaticText).Ancestor(ResizeDiskDialog.Slider))
	if err != nil {
		return "", 0, errors.Wrap(err, "failed to read the disk size from slider after resizing")
	}
	sizeOnSlider := nodeInfo.Name

	if err := uiauto.Combine("to click button Resize on Resize Linux disk dialog",
		s.ui.LeftClick(ResizeDiskDialog.Resize),
		s.ui.WaitUntilGone(ResizeDiskDialog.Self))(ctx); err != nil {
		return "", 0, errors.Wrap(err, "failed to resize")
	}

	return sizeOnSlider, size, nil
}

// LeftClickUI performs a left click action on the given Finder
func (s *Settings) LeftClickUI(findParams *nodewith.Finder) uiauto.Action {
	return uiauto.Combine("focus and left click UI",
		s.ui.FocusAndWait(findParams),
		s.ui.LeftClick(findParams))
}

// WaitForUI waits until findParams exists
func (s *Settings) WaitForUI(findParams *nodewith.Finder) uiauto.Action {
	return s.ui.WaitUntilExists(findParams)
}

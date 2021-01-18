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
	"chromiumos/tast/local/chrome/ui"
	"chromiumos/tast/local/chrome/uig"
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
	developersButton      = ui.FindParams{Attributes: map[string]interface{}{"name": regexp.MustCompile(`Developers|Linux.*`)}, Role: ui.RoleTypeButton}
	advancedButton        = ui.FindParams{Name: "Advanced", Role: ui.RoleTypeButton}
	nextButton            = ui.FindParams{Name: "Next", Role: ui.RoleTypeButton}
	settingsHeading       = ui.FindParams{Name: "Settings", Role: ui.RoleTypeHeading}
	settingsWindow        = ui.FindParams{Name: "Settings", Role: ui.RoleTypeWindow}
	linuxSettings         = ui.FindParams{Name: PageNameLinux, Role: ui.RoleTypeRootWebArea}
	emptySharedFoldersMsg = ui.FindParams{Name: "Shared folders will appear here", Role: ui.RoleTypeStaticText}
	sharedFoldersList     = ui.FindParams{Name: "Shared folders", Role: ui.RoleTypeList}
	unshareFailDlg        = ui.FindParams{Name: "Unshare failed", Role: ui.RoleTypeDialog}
	okButton              = ui.FindParams{Name: "OK", Role: ui.RoleTypeButton}
	removeLinuxButton     = ui.FindParams{Attributes: map[string]interface{}{"name": regexp.MustCompile(`Remove.*`)}, Role: ui.RoleTypeButton}
	resizeButton          = ui.FindParams{Name: "Change disk size", Role: ui.RoleTypeButton}
)

// Settings represents an instance of the Linux settings in Settings App.
type Settings struct {
	tconn *chrome.TestConn
}

// Open finds or launches Settings app and returns a Settings instance.
func Open(ctx context.Context, tconn *chrome.TestConn) (*Settings, error) {
	// Open Settings app.
	if err := ash.HideVisibleNotifications(ctx, tconn); err != nil {
		return nil, errors.Wrap(err, "failed to hide all notifications in OpenSettings()")
	}
	s := &Settings{tconn}
	if err := s.ensureOpen(ctx); err != nil {
		return nil, errors.Wrap(err, "error in OpenSettings()")
	}
	return s, nil
}

func navigateToDevelopers(ctx context.Context, tconn *chrome.TestConn) error {
	err := uig.Do(ctx, tconn, uig.FindWithTimeout(settingsWindow, uiTimeout).
		FindWithTimeout(advancedButton, uiTimeout).
		FocusAndWait(uiTimeout).
		LeftClick().WithNamef("clickAdvanced()"))
	if err != nil {
		return errors.Wrap(err, "failed to click Advanced")
	}
	// Navigate to Developers page or Linux settings page.
	return uig.Do(ctx, tconn, uig.Retry(2,
		uig.FindWithTimeout(settingsWindow, uiTimeout).
			FindWithTimeout(developersButton, uiTimeout).
			FocusAndWait(uiTimeout).
			LeftClick()).WithNamef("navigateToDevelopers()"))
}

// OpenLinuxSettings open finds or launches Settings app and navigate to Linux Settings and its sub settings if any.
// Returns a Settings instance.
func OpenLinuxSettings(ctx context.Context, tconn *chrome.TestConn, subSettings ...string) (s *Settings, err error) {
	s, err = Open(ctx, tconn)
	if err != nil {
		return nil, errors.Wrap(err, "failed to open the Settings app")
	}
	defer func() {
		if err != nil && s != nil {
			s.Close(ctx)
		}
	}()

	// Navigate to Developers settings page.
	if err = navigateToDevelopers(ctx, tconn); err != nil {
		return nil, errors.Wrap(err, "failed to open Linux settings")
	}

	// Find the sub Settings.
	for _, setting := range subSettings {
		if err := uig.Do(ctx, tconn, uig.FindWithTimeout(ui.FindParams{Name: setting, Role: ui.RoleTypeLink}, uiTimeout).LeftClick()); err != nil {
			return nil, errors.Wrapf(err, "failed to open sub setting %s", setting)
		}
	}

	return s, nil
}

// FindSettingsPage finds a pre-opened Settings page with a window name.
func FindSettingsPage(ctx context.Context, tconn *chrome.TestConn, windowName string) (s *Settings, err error) {
	// Check Settings app is opened to the specific page.
	if _, err := ui.FindWithTimeout(ctx, tconn, ui.FindParams{Name: windowName, Role: ui.RoleTypeWindow}, uiTimeout); err != nil {
		return nil, errors.Wrapf(err, "failed to find window %s", windowName)
	}

	return &Settings{tconn: tconn}, nil
}

// ensureOpen checks if the settings app is open, and opens it if it is not.
func (s *Settings) ensureOpen(ctx context.Context) error {
	shown, err := ash.AppShown(ctx, s.tconn, apps.Settings.ID)
	if err != nil {
		return err
	}
	if shown {
		return nil
	}
	if err := apps.Launch(ctx, s.tconn, apps.Settings.ID); err != nil {
		return errors.Wrap(err, "failed to launch settings app")
	}
	if err := ash.WaitForApp(ctx, s.tconn, apps.Settings.ID); err != nil {
		return errors.Wrap(err, "Settings app did not appear in the shelf")
	}
	return nil
}

// OpenInstaller clicks the "Turn on" Linux button to open the Crostini installer.
//
// It also clicks next to skip the information screen.  An ui.Installer
// page object can be constructed after calling OpenInstaller to adjust the settings and to complete the installation.
func (s *Settings) OpenInstaller(ctx context.Context) error {
	if err := s.ensureOpen(ctx); err != nil {
		return errors.Wrap(err, "error in OpenInstaller()")
	}
	if err := navigateToDevelopers(ctx, s.tconn); err != nil {
		return errors.Wrap(err, "error in OpenInstaller()")
	}
	return uig.Do(ctx, s.tconn,
		uig.Steps(
			uig.FindWithTimeout(nextButton, uiTimeout).LeftClick()).WithNamef("OpenInstaller()"))
}

// Close closes the Settings App.
func (s *Settings) Close(ctx context.Context) error {
	// Close the Settings App.
	if err := apps.Close(ctx, s.tconn, apps.Settings.ID); err != nil {
		return errors.Wrap(err, "failed to close Settings app")
	}

	// Wait for the window to close.
	return ui.WaitUntilGone(ctx, s.tconn, settingsHeading, time.Minute)
}

// GetSharedFolders returns a list of shared folders.
// Settings must be open at the Linux Manage Shared Folders page.
func (s *Settings) GetSharedFolders(ctx context.Context) (listOffolders []string, err error) {
	if err := ui.WaitForLocationChangeCompleted(ctx, s.tconn); err != nil {
		return nil, errors.Wrap(err, "failed to wait for location on Settings app")
	}

	// Find "Shared folders will appear here". It will be displayed if no folder is shared.
	msg, textErr := ui.FindWithTimeout(ctx, s.tconn, emptySharedFoldersMsg, 1*time.Second)
	if msg != nil {
		defer msg.Release(ctx)
	}

	// Find "Shared folders" list. It will be displayed if any folder is shared.
	list, listErr := ui.FindWithTimeout(ctx, s.tconn, sharedFoldersList, 1*time.Second)
	if list != nil {
		defer list.Release(ctx)
	}

	// Method to get shared folders list.
	getList := func() ([]string, error) {
		sharedFolders, err := list.Descendants(ctx, ui.FindParams{Role: ui.RoleTypeButton})
		if err != nil {
			return nil, errors.Wrap(err, "failed to find list of shared folders")
		}
		for _, folder := range sharedFolders {
			listOffolders = append(listOffolders, folder.Name)
		}
		return listOffolders, nil
	}

	if textErr != nil && listErr != nil {
		// Did not find "Shared folders will appear here" or "Shared folders" list.
		return nil, errors.Wrap(err, "failed to find list of 'Shared folders' or 'Shared folders will appear here'")
	} else if textErr != nil && listErr == nil {
		// Found "Shared folders".
		return getList()
	} else if listErr == nil {
		// Unexpectedly found shared folder list.
		listOffolders, err = getList()
		return nil, errors.Wrap(err, "unexpectedly found Shared folders list")
	}

	// No shared folder.
	return nil, nil
}

// UnshareFolder deletes a shared folder from Settings app.
// Settings must be open at the Linux Manage Shared Folders page.
func (s *Settings) UnshareFolder(ctx context.Context, folder string) error {
	list := uig.FindWithTimeout(sharedFoldersList, 2*time.Second)
	folderParam := ui.FindParams{Role: ui.RoleTypeButton, Name: folder}
	if err := uig.Do(ctx, s.tconn, list); err != nil {
		return errors.Wrap(err, "failed to find shared folder list")
	}

	// Click X button on the target folder.
	if err := uig.Do(ctx, s.tconn, list.FindWithTimeout(folderParam, uiTimeout).LeftClick()); err != nil {
		return errors.Wrapf(err, "failed to click X button on %s", folder)
	}

	// There might be an unshare dialog. Click OK on it.
	unshareDialog := uig.FindWithTimeout(unshareFailDlg, 2*time.Second)
	if err := uig.Do(ctx, s.tconn, unshareDialog); err == nil {
		if err := uig.Do(ctx, s.tconn, unshareDialog.FindWithTimeout(okButton, uiTimeout).LeftClick()); err != nil {
			return errors.Wrap(err, "failed to click OK on Unshare failed dialog")
		}
	}

	if err := uig.Do(ctx, s.tconn, list); err == nil {
		if err := uig.Do(ctx, s.tconn, list.WaitUntilDescendantGone(folderParam, uiTimeout)); err != nil {
			return errors.Wrapf(err, "%s failed to disappear after deleting", folder)
		}
	}

	return nil
}

// RemoveConfirmDialog represents the confirm dialog of removing Crostini.
type RemoveConfirmDialog struct {
	Self   *uig.Action `nameRegex:"Remove|Delete" role:"dialog"`
	Delete *uig.Action `name:"Delete" role:"button"`
	Cancel *uig.Action `name:"Cancel" role:"button"`
}

// ClickRemove clicks Remove to delete Linux and returns an instance of RemoveConfirmDialog.
func (s *Settings) ClickRemove(ctx context.Context, tconn *chrome.TestConn) (*RemoveConfirmDialog, error) {
	dialog := &RemoveConfirmDialog{}
	uig.PageObject(dialog)

	if err := uig.Do(ctx, tconn,
		uig.FindWithTimeout(removeLinuxButton, uiTimeout).LeftClick().WaitForLocationChangeCompleted(),
		dialog.Self); err != nil {
		return nil, errors.Wrap(err, "failed to find the delete dialog")
	}

	return dialog, nil
}

// ResizeDiskDialog represents the Resize Linux disk dialog.
type ResizeDiskDialog struct {
	Self   *uig.Action `name:"Resize Linux disk" role:"genericContainer"`
	Slider *uig.Action `role:"slider"`
	Resize *uig.Action `name:"Resize" role:"button"`
	Cancel *uig.Action `name:"Cancel" role:"button"`
}

// ClickChange clicks Change to resize disk and returns an instance of ResizeDiskDialog.
func (s *Settings) ClickChange(ctx context.Context) (*ResizeDiskDialog, error) {
	dialog := &ResizeDiskDialog{}
	uig.PageObject(dialog)

	if err := uig.Do(ctx, s.tconn,
		uig.FindWithTimeout(resizeButton, uiTimeout).LeftClick().WaitForLocationChangeCompleted(),
		dialog.Self); err != nil {
		return nil, errors.Wrap(err, "failed to find the resize dialog")
	}

	// It takes some time for the data to load on the slider, especially when Crostini is shutdown.
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		return uig.Do(ctx, s.tconn, dialog.Slider)
	}, &testing.PollOptions{Timeout: time.Minute}); err != nil {
		return nil, errors.Wrap(err, "failed to find slider in resize dialog")
	}
	return dialog, nil
}

// GetDiskSize returns the disk size on the Settings app.
func (s *Settings) GetDiskSize(ctx context.Context) (string, error) {
	diskSizeFindParams := ui.FindParams{
		Role:       ui.RoleTypeStaticText,
		Attributes: map[string]interface{}{"name": regexp.MustCompile(`[0-9]+.[0-9]+ GB`)},
	}

	node, err := ui.FindWithTimeout(ctx, s.tconn, diskSizeFindParams, uiTimeout)
	if err != nil {
		return "", errors.Wrap(err, "failed to find disk size information on the Settings app")
	}
	defer node.Release(ctx)
	return node.Name, nil
}

// ResizeDisk resizes the VM disk to approximately targetSize via the settings app.
// If growing the VM disk, set increase to true, otherwise set it to false.
func (s *Settings) ResizeDisk(ctx context.Context, kb *input.KeyboardEventWriter, targetSize uint64, increase bool) error {
	dialog := &ResizeDiskDialog{}
	uig.PageObject(dialog)

	if err := uig.Do(ctx, s.tconn,
		uig.FindWithTimeout(resizeButton, uiTimeout).LeftClick().WaitForLocationChangeCompleted(),
		dialog.Slider.FocusAndWait(15*time.Second)); err != nil {
		return errors.Wrap(err, "failed to open resize slider")
	}

	if _, err := ChangeDiskSize(ctx, s.tconn, kb, dialog.Slider, increase, targetSize); err != nil {
		return errors.Wrap(err, "failed to resize disk")
	}

	if err := uig.Do(ctx, s.tconn,
		dialog.Resize.LeftClick(),
		uig.WaitUntilDescendantGone(ui.FindParams{Name: "Resize Linux disk", Role: "genericContainer"}, uiTimeout)); err != nil {
		return errors.Wrap(err, "failed to click \"Resize\"")
	}

	return nil
}

// GetDiskSize returns the current size based on the disk size slider text.
func GetDiskSize(ctx context.Context, tconn *chrome.TestConn, slider *uig.Action) (uint64, error) {
	node, err := uig.GetNode(ctx, tconn, slider.FindWithTimeout(ui.FindParams{Role: ui.RoleTypeStaticText}, uiTimeout))
	if err != nil {
		return 0, errors.Wrap(err, "error getting disk size setting")
	}
	defer node.Release(ctx)
	return ParseDiskSize(node.Name)
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
// slider.FocusAndWait(uiTimeout) should be called before calling this method.
func ChangeDiskSize(ctx context.Context, tconn *chrome.TestConn, kb *input.KeyboardEventWriter, slider *uig.Action, increase bool, targetDiskSize uint64) (uint64, error) {
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
	resizeDlg, dialog, err := s.getResizeDialog(ctx)
	if err != nil {
		return 0, 0, errors.Wrap(err, "failed to launch Resize Linux disk dialog")
	}
	defer dialog.Release(ctx)

	// Focus on the slider.
	if err := uig.Do(ctx, s.tconn, uig.WaitForLocationChangeCompleted(), resizeDlg.Slider.FocusAndWait(15*time.Second)); err != nil {
		return 0, 0, errors.Wrap(err, "failed to focus on the slider on the Resize Linux disk dialog")
	}

	// Get current size.
	curSize, err = GetDiskSize(ctx, s.tconn, resizeDlg.Slider)
	if err != nil {
		return 0, 0, errors.Wrap(err, "failed to get initial disk size")
	}

	// Get the minimum size.
	minSize, err := ChangeDiskSize(ctx, s.tconn, keyboard, resizeDlg.Slider, false, 0)
	if err != nil {
		return 0, 0, errors.Wrap(err, "failed to resize to the minimum disk size")
	}
	// Get the maximum size.
	maxSize, err := ChangeDiskSize(ctx, s.tconn, keyboard, resizeDlg.Slider, true, 500*SizeGB)
	if err != nil {
		return 0, 0, errors.Wrap(err, "failed to resize to the maximum disk size")
	}

	targetSize = minSize + (maxSize-minSize)/2
	if targetSize == curSize {
		targetSize = minSize + (maxSize-minSize)/3
	}

	if err := uig.Do(ctx, s.tconn, uig.WaitForLocationChangeCompleted(), resizeDlg.Cancel.LeftClick()); err != nil {
		return 0, 0, errors.Wrap(err, "failed to click button Cancel on Resize Linux disk dialog")
	}

	// Wait the resize dialog gone.
	if err := ui.WaitUntilGone(ctx, s.tconn, ui.FindParams{Role: dialog.Role, Name: dialog.Name}, 15*time.Second); err != nil {
		return 0, 0, errors.Wrap(err, "failed to close the Resize Linux disk dialog")
	}
	return curSize, targetSize, nil
}

// Resize changes the disk size to the target size.
// It returns the size on the slider as string and the result size as uint64.
func (s *Settings) Resize(ctx context.Context, keyboard *input.KeyboardEventWriter, curSize, targetSize uint64) (string, uint64, error) {
	resizeDlg, dialog, err := s.getResizeDialog(ctx)
	if err != nil {
		return "", 0, errors.Wrap(err, "failed to launch Resize Linux disk dialog")
	}
	defer dialog.Release(ctx)

	// Focus on the slider.
	if err := uig.Do(ctx, s.tconn, uig.WaitForLocationChangeCompleted(), resizeDlg.Slider.FocusAndWait(15*time.Second)); err != nil {
		return "", 0, errors.Wrap(err, "failed to focus on the slider on the Resize Linux disk dialog")
	}

	// Resize to the target size.
	size, err := ChangeDiskSize(ctx, s.tconn, keyboard, resizeDlg.Slider, targetSize > curSize, targetSize)
	if err != nil {
		return "", 0, errors.Wrapf(err, "failed to resize to %d: ", targetSize)
	}

	// Record the new size on the slider.
	node, err := uig.GetNode(ctx, s.tconn, resizeDlg.Slider.FindWithTimeout(ui.FindParams{Role: ui.RoleTypeStaticText}, 15*time.Second))
	if err != nil {
		return "", 0, errors.Wrap(err, "failed to read the disk size from slider after resizing")
	}
	defer node.Release(ctx)
	sizeOnSlider := node.Name

	if err := uig.Do(ctx, s.tconn, uig.WaitForLocationChangeCompleted(), resizeDlg.Resize.LeftClick()); err != nil {
		return "", 0, errors.Wrap(err, "failed to click button Resize on Resize Linux disk dialog")
	}

	// Wait the resize dialog gone.
	if err := ui.WaitUntilGone(ctx, s.tconn, ui.FindParams{Role: dialog.Role, Name: dialog.Name}, 15*time.Second); err != nil {
		return "", 0, errors.Wrap(err, "failed to close the Resize Linux disk dialog")
	}

	return sizeOnSlider, size, nil
}

func (s *Settings) getResizeDialog(ctx context.Context) (*ResizeDiskDialog, *ui.Node, error) {
	// Click Change on Linux settings page.
	resizeDlg, err := s.ClickChange(ctx)
	if err != nil {
		return nil, nil, errors.Wrap(err, "failed to click button Change on Linux settings page")
	}

	// Get the dialog node and params.
	dialog, err := uig.GetNode(ctx, s.tconn, resizeDlg.Self)
	if err != nil {
		return nil, nil, errors.New("failed to get the node of the Resize Linux disk dialog")
	}

	return resizeDlg, dialog, nil
}

// Copyright 2020 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package settings provides support for the Linux settings on the Settings app.
package settings

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/ossettings"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/chrome/webutil"
	"chromiumos/tast/local/crostini/faillog"
	"chromiumos/tast/local/vm"
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

const uiTimeout = 15 * time.Second
const shortUITimeout = 5 * time.Second

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
	DebianUpgradeText = nodewith.NameStartingWith("An upgrade to Debian").First().Onscreen()
	PageLinux         = nodewith.NameStartingWith(PageNameLinux).First().Onscreen()
	// We may need to update this if more 'Turn on' buttons are added to Settings, but there isn't a good way to make this more specific yet.
	TurnOnButton          = nodewith.Name("Turn on").Role(role.Button).Ancestor(ossettings.WindowFinder).Onscreen()
	DevelopersButton      = nodewith.Name("Developers").Role(role.Button).Ancestor(ossettings.WindowFinder).Onscreen()
	LinuxText             = nodewith.Name("Linux development environment").Role(role.StaticText).Ancestor(ossettings.WindowFinder).Onscreen()
	nextButton            = nodewith.Name("Next").Role(role.Button).Onscreen()
	settingsHead          = nodewith.Name("Settings").Role(role.Heading).Onscreen()
	emptySharedFoldersMsg = nodewith.Name("Shared folders will appear here").Role(role.StaticText).Onscreen()
	sharedFoldersList     = nodewith.Name("Shared folders").Role(role.List).Onscreen()
	unshareFailDlg        = nodewith.Name("Unshare failed").Role(role.StaticText).Ancestor(nodewith.Role(role.Dialog)).Onscreen()
	tryAgainButton        = nodewith.Name("Try again").Role(role.Button).Onscreen()
	removeLinuxButton     = nodewith.NameRegex(regexp.MustCompile(`Remove.*`)).Role(role.Button).Onscreen()
	removeLinuxDialog     = nodewith.NameRegex(regexp.MustCompile("Remove|Delete")).Role(role.Dialog).First().Onscreen()
	resizeButton          = nodewith.Name("Change disk size").Role(role.Button).Onscreen()
	RemoveLinuxAlert      = nodewith.Name("Remove Linux development environment").Role(role.AlertDialog).ClassName("Widget").Onscreen()
	BackupButton          = nodewith.NameStartingWith("Backup Linux").Role(role.Button).Ancestor(ossettings.WindowFinder).Onscreen()
	RestoreButton         = nodewith.NameStartingWith("Replace").Role(role.Button).Ancestor(ossettings.WindowFinder).Onscreen()
	BackupFileWindow      = nodewith.Name("Backup").Role(role.Window).ClassName("WebDialogView").Onscreen()
	BackupSave            = nodewith.Name("Save").Role(role.Button).Ancestor(BackupFileWindow).Onscreen()
	BackupNotification    = nodewith.NameStartingWith("Backup complete").Role(role.AlertDialog).ClassName("MessagePopupView").Onscreen()
	RestoreNotification   = nodewith.NameStartingWith("Restore complete").Role(role.AlertDialog).ClassName("MessagePopupView").Onscreen()
	RestoreConfirmButton  = nodewith.Name("Restore").Role(role.Button).ClassName("action-button").Onscreen()
	RestoreFileWindow     = nodewith.Name("Restore").Role(role.Window).ClassName("WebDialogView").Onscreen()
	RestoreTiniFile       = nodewith.NameContaining(".tini").Role(role.StaticText).Ancestor(RestoreFileWindow).Onscreen()
	RestoreOpen           = nodewith.Name("Open").Role(role.Button).Ancestor(RestoreFileWindow).Onscreen()
)

// Settings represents an instance of the Linux settings in Settings App.
type Settings struct {
	ui    *uiauto.Context
	tconn *chrome.TestConn
	cr    *chrome.Chrome
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

	// The remove button is initially unstable in the a11y tree on low-end
	// devices. Add a sleep for the node to be stable.
	testing.Sleep(ctx, time.Second)

	return &Settings{tconn: tconn, ui: ui, cr: cr}, nil
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

// OpenLinuxManagedSharedFoldersSetting opens the Manage Shared Folders sub-settings page.
// This is implemented a two-stage process as a single invocation off OpenLinuxSettings
// with the ManageSharedFolders subsettings param fails on a few boards.
func OpenLinuxManagedSharedFoldersSetting(ctx context.Context, tconn *chrome.TestConn, cr *chrome.Chrome) (*Settings, error) {
	// Open linux settings.
	s, err := OpenLinuxSettings(ctx, tconn, cr)
	if err != nil {
		return nil, errors.Wrap(err, "failed to open linux subpage on Settings app")
	}

	// Open linux settings with the ManageSharedFolders param passed.
	s, err = OpenLinuxSettings(ctx, tconn, cr, ManageSharedFolders)
	if err != nil {
		return nil, errors.Wrap(err, "failed to open linux Manage Shared Folder sub-settings page")
	}

	return s, err
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

// OpenLinuxInstaller opens the Linux subpage on Settings page and clicks on the Turn on button.
func OpenLinuxInstaller(ctx context.Context, tconn *chrome.TestConn, cr *chrome.Chrome) (*Settings, error) {
	// Open Settings app.
	if err := ash.CloseNotifications(ctx, tconn); err != nil {
		return nil, errors.Wrap(err, "failed to close all notifications in OpenLinuxInstaller()")
	}

	ui := uiauto.New(tconn)
	if _, err := ossettings.LaunchAtPageURL(ctx, tconn, cr, "crostini", ui.WaitUntilExists(TurnOnButton)); err != nil {
		return nil, errors.Wrap(err, "failed to launch settings app")
	}
	if err := ui.LeftClick(TurnOnButton)(ctx); err != nil {
		return nil, errors.Wrap(err, "failed to open Linux installer")
	}

	return &Settings{tconn: tconn, ui: ui}, nil
}

// OpenLinuxInstallerAndClickNext clicks the "Turn on" Linux button to open the Crostini installer.
//
// It also clicks next to skip the information screen.  An ui.Installer
// page object can be constructed after calling OpenLinuxInstallerAndClickNext to adjust the settings and to complete the installation.
func OpenLinuxInstallerAndClickNext(ctx context.Context, tconn *chrome.TestConn, cr *chrome.Chrome) (retErr error) {
	// Open Settings app.
	if err := ash.CloseNotifications(ctx, tconn); err != nil {
		return errors.Wrap(err, "failed to close all notifications in OpenLinuxInstaller()")
	}

	ui := uiauto.New(tconn).WithInterval(500 * time.Millisecond)
	if _, err := ossettings.LaunchAtPageURL(ctx, tconn, cr, "crostini", ui.WaitUntilExists(LinuxText)); err != nil {
		return errors.Wrap(err, "failed to launch settings app")
	}

	s := &Settings{tconn: tconn, ui: ui}
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()
	defer s.Close(cleanupCtx)
	defer func(ctx context.Context) { faillog.DumpUITreeAndScreenshot(ctx, tconn, "crostini_installer", retErr) }(cleanupCtx)

	if err := ui.WaitUntilExists(DevelopersButton)(ctx); err == nil {
		// Linux has been installed already, uninstall it.
		if err := ui.LeftClickUntil(DevelopersButton, ui.WithTimeout(shortUITimeout).WaitUntilExists(removeLinuxButton))(ctx); err != nil {
			return errors.Wrap(err, "failed to go to linux subpage")
		}

		if errRemove := s.Remove()(ctx); errRemove != nil {
			return errors.Wrap(errRemove, "failed to uninstall Linux before installation")
		}
	}

	installButton := nodewith.Name("Install").Role(role.Button)
	if err := uiauto.Combine("open Install and click button Next",
		ui.LeftClickUntil(TurnOnButton, ui.WithTimeout(shortUITimeout).WaitUntilExists(nextButton)),
		ui.LeftClickUntil(nextButton, ui.WithTimeout(shortUITimeout).WaitUntilExists(installButton)))(ctx); err != nil {
		return errors.Wrap(err, "failed to click button Next on the installer")
	}
	return nil
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
	retryIfFailed := uiauto.IfSuccessThen(
		s.ui.WithTimeout(5*time.Second).WaitUntilExists(unshareFailDlg),
		s.ui.WithPollOpts(testing.PollOptions{Interval: 5 * time.Second}).Retry(4, retryUnshare))

	return uiauto.Combine("unshare folder "+folder,
		s.ui.LeftClick(folderButton),
		retryIfFailed,
		uiauto.IfSuccessThen(s.ui.Exists(sharedFoldersList), s.ui.WaitUntilGone(folderButton)),
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
	return s.ui.LeftClickUntil(removeLinuxButton, s.ui.WithTimeout(shortUITimeout).WaitUntilExists(RemoveConfirmDialog.Self))
}

// Remove removes Crostini.
func (s *Settings) Remove() uiauto.Action {
	return uiauto.Combine("remove Linux",
		s.ClickRemove(),
		s.ui.LeftClickUntil(RemoveConfirmDialog.Delete, s.ui.WithTimeout(shortUITimeout).WaitUntilGone(RemoveConfirmDialog.Self)),
		s.ui.WithTimeout(time.Minute).WaitUntilExists(TurnOnButton))
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
		s.tconn.ResetAutomation,
		s.ui.LeftClick(resizeButton),
		s.ui.WithTimeout(time.Minute).WaitUntilExists(ResizeDiskDialog.Slider))
}

// GetDiskSize returns the disk size on the Settings app.
func (s *Settings) GetDiskSize(ctx context.Context) (string, error) {
	if err := s.tconn.ResetAutomation(ctx); err != nil {
		return "", errors.Wrap(err, "failed to call ResetAutomation")
	}
	size := nodewith.NameRegex(regexp.MustCompile(`[0-9]+.[0-9]+ GB$`)).Role(role.StaticText)
	if err := s.ui.WithInterval(500 * time.Millisecond).WaitForLocation(size)(ctx); err != nil {
		return "", errors.Wrap(err, "failed to wait for location for the disk size")
	}
	nodeInfo, err := s.ui.Info(ctx, size)
	if err != nil {
		return "", errors.Wrap(err, "failed to find disk size information on the Settings app")
	}
	return nodeInfo.Name, nil
}

// SliderDiskSizes returns the current size, maximum size and minimal size on the slider.
func SliderDiskSizes(ctx context.Context, tconn *chrome.TestConn, slider *nodewith.Finder) (curSize, maxSize, minSize uint64, err error) {
	sliderNode, err := uiauto.New(tconn).Info(ctx, ResizeDiskDialog.Slider)
	if err != nil {
		return 0, 0, 0, errors.Wrap(err, "error getting slider node info")
	}

	// Get the current size.
	curSize, err = ParseDiskSize(sliderNode.HTMLAttributes["aria-valuenow"])
	if err != nil {
		return 0, 0, 0, errors.Wrap(err, "failed to retrieve the current disk size")
	}

	// Get the maximum size.
	maxSize, err = ParseDiskSize(sliderNode.HTMLAttributes["aria-valuemax"])
	if err != nil {
		return 0, 0, 0, errors.Wrap(err, "failed to retrieve the maximum disk size")
	}

	// Get the minimum size.
	minSize, err = ParseDiskSize(sliderNode.HTMLAttributes["aria-valuemin"])
	if err != nil {
		return 0, 0, 0, errors.Wrap(err, "failed to retrieve the minimum disk size")
	}
	return curSize, maxSize, minSize, nil
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

// UpdateDiskSizeSliderWithJS uses JS function to change the disk size slider
// value. sliderContainerName is the name of a JSObject that has field
// "diskSizeTicks_" and shadowRoot that contains the slider.
// E.g., sliderContainerName is "settings-crostini-disk-resize-dialog" if
// resizing in the crostini settings page, and "crostini-installer-app" if
// resizing in the crostini installer.
func UpdateDiskSizeSliderWithJS(ctx context.Context, conn *chrome.Conn, sliderContainerName string, targetDiskSize uint64, isSoftExtremum bool) (uint64, error) {
	const query = `
			(sliderContainer, targetDiskSize, IsSoftExtremum=false) => {
				const diskSizeTicks = sliderContainer.diskSizeTicks_;
				const slider = sliderContainer.shadowRoot?.querySelector('#diskSlider');

				if (!(slider&&diskSizeTicks)) {
					throw 'Cannot find diskSizeTicks or slider with the given query.'
				}

				const minSize = Number(diskSizeTicks[0].value);
				const maxSize = Number(diskSizeTicks[diskSizeTicks.length-1].value);

				if ((targetDiskSize > maxSize || targetDiskSize < minSize) && !IsSoftExtremum) {
				    throw 'Target size '+ targetDiskSize +' is outside the valid range ['+ minSize +','+ maxSize +'], consider set IsSoftExtremum=true to use the posible extremum.';
				}

				targetDiskSize = Math.max(Math.min(targetDiskSize,maxSize),minSize);

				let targetSliderValue = 0;
				while (diskSizeTicks[targetSliderValue].value < targetDiskSize) {
					targetSliderValue++;
				}
				
				slider.value = targetSliderValue;
				// Number is needed for Tast to parse Bigint that has 'n' suffix.
				// It may lose a little precision if the value is bigger than 9007199254740991,
				// which is approximately 8388608 GB. 
				return Number(diskSizeTicks[slider.value].value);
			}`

	var sizeOnSlider uint64
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		sliderContainer := &chrome.JSObject{}
		if err := webutil.EvalWithShadowPiercer(ctx, conn, fmt.Sprintf(`shadowPiercingQuery('%s')`, sliderContainerName), sliderContainer); err != nil {
			return errors.Wrap(err, "failed to find the slider container object")
		}
		defer sliderContainer.Release(ctx)
		return conn.Call(ctx, &sizeOnSlider, query, sliderContainer, targetDiskSize, isSoftExtremum)
	}, &testing.PollOptions{Timeout: 15 * time.Second}); err != nil {
		return 0, errors.Wrap(err, "failed to set disk size via JS")
	}

	return sizeOnSlider, nil
}

// ChangeDiskSize changes the disk size to targetDiskSize through moving the slider.
// If the target disk size is bigger, set increase to true, otherwise set it to false.
// The method will return if it reaches the target or the end of the slider.
// The real size might not be exactly equal to the target because the increment changes depending on the range.
// FocusAndWait(slider) should be called before calling this method.
func ChangeDiskSize(ctx context.Context, cr *chrome.Chrome, tconn *chrome.TestConn, targetDiskSize uint64) (uint64, error) {
	const crostiniSettingsURL = "chrome://os-settings/crostini"
	const diskResizeDialogName = "settings-crostini-disk-resize-dialog"
	targetMatcher := func(t *chrome.Target) bool { return strings.HasPrefix(t.URL, crostiniSettingsURL) }
	conn, err := cr.NewConnForTarget(ctx, targetMatcher)
	if err != nil {
		return 0, errors.Wrapf(err, "failed to connect to installer page %s", crostiniSettingsURL)
	}
	defer conn.Close()

	curSize, err := UpdateDiskSizeSliderWithJS(ctx, conn, diskResizeDialogName, targetDiskSize, false)
	if err != nil {
		return 0, err
	}
	if tconn.ResetAutomation(ctx); err != nil {
		return 0, errors.Wrap(err, "failed to call ResetAutomation to refresh the accessibility tree")
	}
	return curSize, nil
}

// GetCurAndTargetDiskSize gets the current disk size and calculates a target disk size to resize.
func (s *Settings) GetCurAndTargetDiskSize(ctx context.Context) (curSize, targetSize uint64, err error) {
	if err := uiauto.Combine("launch resize dialog and focus on the slider",
		s.ClickChange(),
		s.ui.FocusAndWait(ResizeDiskDialog.Slider))(ctx); err != nil {
		return 0, 0, err
	}

	// Get current size.
	curSize, maxSize, minSize, err := SliderDiskSizes(ctx, s.tconn, ResizeDiskDialog.Slider)
	if err != nil {
		return 0, 0, errors.Wrap(err, "failed to get initial disk sizes")
	}

	testing.ContextLogf(ctx, "The cursize, maxSize and minSize are: %d, %d, %d", curSize, maxSize, minSize)

	targetSize = minSize + (maxSize-minSize)/2
	if targetSize == curSize {
		targetSize = minSize + (maxSize-minSize)/3
	}
	testing.ContextLogf(ctx, "The target size is: %d", targetSize)

	if err := uiauto.Combine("click button Cancel and wait resize dialog gone",
		s.ui.LeftClick(ResizeDiskDialog.Cancel),
		// TODO (crbug/1232877): remove this line when crbug/1232877 is resolved.
		s.tconn.ResetAutomation,
		s.ui.WaitUntilGone(ResizeDiskDialog.Self))(ctx); err != nil {
		return 0, 0, err
	}

	return curSize, targetSize, nil
}

// Resize changes the disk size to the target size.
// It returns the size on the slider as string and the result size as uint64.
func (s *Settings) Resize(ctx context.Context, targetSize uint64) (string, uint64, error) {
	if err := uiauto.Combine("launch resize dialog and focus on the slider",
		s.ClickChange(),
		// TODO (crbug/1232877): remove this line when crbug/1232877 is resolved.
		s.tconn.ResetAutomation,
		s.ui.FocusAndWait(ResizeDiskDialog.Slider))(ctx); err != nil {
		return "", 0, err
	}

	// Resize to the target size.
	size, err := ChangeDiskSize(ctx, s.cr, s.tconn, targetSize)
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
		// TODO (crbug/1232877): remove this line when crbug/1232877 is resolved.
		s.tconn.ResetAutomation,
		s.ui.WaitUntilGone(ResizeDiskDialog.Self))(ctx); err != nil {
		return "", 0, errors.Wrap(err, "failed to resize")
	}
	// TODO (crbug/1232877): remove this line when crbug/1232877 is resolved.
	testing.Sleep(ctx, time.Second)
	return sizeOnSlider, size, nil
}

// VerifyResizeResults verifies the disk after resizing, both on the Settings page and container.
func (s *Settings) VerifyResizeResults(ctx context.Context, cont *vm.Container, sizeOnSlider string, size uint64) error {
	// Check the disk size on the Settings app.
	sizeOnSettings, err := s.GetDiskSize(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to get the disk size from the Settings app after resizing")
	}
	if sizeOnSlider != sizeOnSettings {
		return errors.Wrapf(err, "failed to verify the disk size on the Settings app, got %s, want %s", sizeOnSettings, sizeOnSlider)
	}
	// Check the disk size of the container.
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		disk, err := cont.VM.Concierge.GetVMDiskInfo(ctx, vm.DefaultVMName)
		if err != nil {
			return errors.Wrap(err, "failed to get VM disk info")
		}
		contSize := disk.GetSize()

		// Allow some gap.
		var diff uint64
		if size > contSize {
			diff = size - contSize
		} else {
			diff = contSize - size
		}
		if diff > SizeMB {
			return errors.Errorf("failed to verify disk size after resizing, got %d, want approximately %d", contSize, size)
		}
		return nil
	}, &testing.PollOptions{Timeout: 5 * time.Second}); err != nil {
		return errors.Wrap(err, "failed to verify the disk size of the container after resizing")
	}

	return nil
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

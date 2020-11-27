// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"os"
	"path/filepath"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/fsutil"
	arcui "chromiumos/tast/local/android/ui"
	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/arc/optin"
	"chromiumos/tast/local/arc/playstore"
	"chromiumos/tast/local/chrome"
	chromeui "chromiumos/tast/local/chrome/ui"
	"chromiumos/tast/local/chrome/ui/faillog"
	"chromiumos/tast/local/chrome/ui/filesapp"
	"chromiumos/tast/local/power"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: Sharesheet,
		Desc: "Install ARC++ app and share to app via Sharesheet",
		Contacts: []string{
			"benreich@chromium.org",
			"melzhang@chromium.org",
			"chromeos-apps-foundation-team@google.com",
		},
		Attr: []string{"group:mainline", "informational"},
		Data: []string{"capybara.jpg"},
		Params: []testing.Param{{
			ExtraSoftwareDeps: []string{"android_p", "chrome"},
		}, {
			Name:              "vm",
			ExtraSoftwareDeps: []string{"android_vm", "chrome"},
		}},
		Vars: []string{
			"arc.Sharesheet.username",
			"arc.Sharesheet.password",
		},
	})
}

const (
	arcSharesheetUITimeout    = 15 * time.Second
	arcSharesheetPollInterval = 2 * time.Second
)

func Sharesheet(ctx context.Context, s *testing.State) {
	const (
		appPkgName    = "com.iudesk.android.photo.editor"
		appShareLabel = "Photo Editor"
		imageFileName = "capybara.jpg"
	)

	username := s.RequiredVar("arc.Sharesheet.username")
	password := s.RequiredVar("arc.Sharesheet.password")
	pollOpts := testing.PollOptions{Interval: arcSharesheetPollInterval, Timeout: arcSharesheetUITimeout}

	// Setup Chrome.
	cr, err := chrome.New(ctx, chrome.GAIALogin(), chrome.Auth(username, password, "gaia-id"), chrome.ARCSupported(),
		chrome.ExtraArgs(arc.DisableSyncFlags()...))
	if err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}
	defer cr.Close(ctx)

	// Transfer the test image to share into Downloads.
	imageFileLocation := filepath.Join(filesapp.DownloadPath, imageFileName)
	if err := fsutil.CopyFile(s.DataPath(imageFileName), imageFileLocation); err != nil {
		s.Fatalf("Failed to copy the test image to %s: %s", imageFileLocation, err)
	}
	defer os.Remove(imageFileLocation)

	// Setup Test API Connection.
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create test API connection: ", err)
	}
	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)

	// Setup Play Store, ARC and the UI Automator.
	a, d, err := setupARC(ctx, s, cr, tconn)
	if err != nil {
		s.Fatal("Failed setting up ARC: ", err)
	}
	defer a.Close()
	defer d.Close(ctx)

	// TODO(crbug.com/1153218): Replace this app with a Tast Android app.
	if err := installARCApp(ctx, tconn, a, d, appPkgName); err != nil {
		s.Fatalf("Failed trying to install %q: %v", appPkgName, err)
	}

	// Open the Files App.
	files, err := filesapp.Launch(ctx, tconn)
	if err != nil {
		s.Fatal("Failed launching the Files app: ", err)
	}
	defer files.Release(ctx)

	if err := files.OpenDownloads(ctx); err != nil {
		s.Fatal("Failed navigating to Downloads directory: ", err)
	}

	if err := files.SelectFile(ctx, imageFileName); err != nil {
		s.Fatalf("Failed selecting file %q: %v", imageFileName, err)
	}

	if err := waitForStableActionbar(ctx, tconn, files, &pollOpts); err != nil {
		s.Fatal("Failed waiting for the Share button to appear: ", err)
	}

	if err := clickShareButtonWaitForApp(ctx, files, &pollOpts, tconn, appShareLabel); err != nil {
		s.Fatal("Failed clicking share button and waiting for app: ", err)
	}

	if err := allowARCAppFileAccess(ctx, d); err != nil {
		s.Fatalf("Failed granting %q file access: %v", appShareLabel, err)
	}

	// Wait for the file name to be available in the top left of the ARC app.
	imageTextView := d.Object(arcui.ClassName("android.widget.TextView"), arcui.Text(imageFileName))
	if err := imageTextView.WaitForExists(ctx, arcSharesheetUITimeout); err != nil {
		s.Fatalf("Failed waiting for file name %q to appear in ARC window: %v", imageFileName, err)
	}
}

// waitForStableActionbar makes sure the Action bar is stable as items are loaded asynchronously.
func waitForStableActionbar(ctx context.Context, tconn *chrome.TestConn, f *filesapp.FilesApp, pollOpts *testing.PollOptions) error {
	// Get the Action bar which contains the Share button.
	params := chromeui.FindParams{
		Role: chromeui.RoleTypeContentInfo,
	}
	actionBar, err := f.Root.DescendantWithTimeout(ctx, params, arcSharesheetUITimeout)
	if err != nil {
		return errors.Wrap(err, "failed to find Action bar")
	}
	defer actionBar.Release(ctx)

	// Setup a watcher to wait for the Share button to show.
	ew, err := chromeui.NewWatcher(ctx, actionBar, chromeui.EventTypeActiveDescendantChanged)
	if err != nil {
		return errors.Wrap(err, "failed getting a watcher for the files Action bar")
	}
	defer ew.Release(ctx)

	// Check the Action bar for any Activedescendantchanged events occurring in a 2 second interval.
	// If any events are found continue polling until 10s is reached.
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		return ew.EnsureNoEvents(ctx, arcSharesheetPollInterval)
	}, pollOpts); err != nil {
		return errors.Wrapf(err, "failed waiting %v for action bar to stabilize", pollOpts.Timeout)
	}

	return nil
}

// clickShareButtonWaitForApp opens up the Sharesheet and waits for animations to finish.
func clickShareButtonWaitForApp(ctx context.Context, f *filesapp.FilesApp, pollOpts *testing.PollOptions, tconn *chrome.TestConn, appShareLabel string) error {
	// Get the Share button.
	params := chromeui.FindParams{
		Role: chromeui.RoleTypeButton,
		Name: "Share",
	}
	shareButton, err := f.Root.DescendantWithTimeout(ctx, params, arcSharesheetUITimeout)
	if err != nil {
		return errors.Wrap(err, "failed to find Share button")
	}
	defer shareButton.Release(ctx)

	if err := shareButton.StableLeftClick(ctx, pollOpts); err != nil {
		return errors.Wrap(err, "failed to click Share button")
	}

	quickEditButton, err := waitForAppOnStableSharesheet(ctx, f, tconn, appShareLabel, pollOpts)
	if err != nil {
		return errors.Wrap(err, "failed waiting for Sharesheet window to stabilize")
	}
	defer quickEditButton.Release(ctx)

	return quickEditButton.StableLeftClick(ctx, pollOpts)
}

// waitForAppOnStableSharesheet waits for the Sharesheet to stabilize and returns the ARC apps node.
func waitForAppOnStableSharesheet(ctx context.Context, f *filesapp.FilesApp, tconn *chrome.TestConn, appName string, pollOpts *testing.PollOptions) (*chromeui.Node, error) {
	// Get the Sharesheet View popup window.
	params := chromeui.FindParams{
		ClassName: "View",
		Name:      "Share",
	}
	sharesheetWindow, err := chromeui.FindWithTimeout(ctx, tconn, params, arcSharesheetUITimeout)
	if err != nil {
		return nil, errors.Wrap(err, "failed to find Sharesheet window")
	}
	defer sharesheetWindow.Release(ctx)

	// Setup a watcher to wait for the apps list in Sharesheet to stabilize.
	ew, err := chromeui.NewWatcher(ctx, sharesheetWindow, chromeui.EventTypeActiveDescendantChanged)
	if err != nil {
		return nil, errors.Wrap(err, "failed getting a watcher for the Sharesheet views window")
	}
	defer ew.Release(ctx)

	// Check the Sharesheet window for any Activedescendantchanged events occurring in a 2 second interval.
	// If any events are found continue polling until 10s is reached.
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		return ew.EnsureNoEvents(ctx, arcSharesheetPollInterval)
	}, pollOpts); err != nil {
		return nil, errors.Wrapf(err, "failed waiting %v for Sharesheet window to stabilize", pollOpts.Timeout)
	}

	// Get the app button to click.
	params = chromeui.FindParams{
		Role: chromeui.RoleTypeButton,
		Name: appName,
	}
	appButton, err := sharesheetWindow.DescendantWithTimeout(ctx, params, arcSharesheetUITimeout)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to find app %q on Sharesheet window", appName)
	}

	return appButton, nil
}

// setupARC opts in to Play Store, starts an ARC device and start UI automator.
func setupARC(ctx context.Context, s *testing.State, cr *chrome.Chrome, tconn *chrome.TestConn) (*arc.ARC, *arcui.Device, error) {
	// Optin to Play Store.
	if err := optin.Perform(ctx, cr, tconn); err != nil {
		return nil, nil, errors.Wrap(err, "failed to optin to the Play Store")
	}
	if err := optin.WaitForPlayStoreShown(ctx, tconn); err != nil {
		return nil, nil, errors.Wrap(err, "failed to wait for Play Store to show")
	}

	// Setup ARC device.
	a, err := arc.New(ctx, s.OutDir())
	if err != nil {
		return nil, nil, errors.Wrap(err, "failed to start ARC")
	}

	// Start up UI automator.
	d, err := a.NewUIDevice(ctx)
	if err != nil {
		return nil, nil, errors.Wrap(err, "failed initializing UI automator")
	}

	return a, d, nil
}

// installARCApp installs the supplied ARC app and closes Play Store upon installation.
func installARCApp(ctx context.Context, tconn *chrome.TestConn, a *arc.ARC, d *arcui.Device, appPkgName string) error {
	// TODO(b/166637700): Remove this if a proper solution is found that doesn't require the display to be on.
	if err := power.TurnOnDisplay(ctx); err != nil {
		return errors.Wrap(err, "failed to ensure the display is on")
	}
	if err := playstore.InstallApp(ctx, a, d, appPkgName, 3); err != nil {
		return errors.Wrapf(err, "failed installing app %q", appPkgName)
	}

	return apps.Close(ctx, tconn, apps.PlayStore.ID)
}

// allowARCAppFileAccess clicks the Allow file permission button.
func allowARCAppFileAccess(ctx context.Context, d *arcui.Device) error {
	const permissionButtonResourceID = "com.android.packageinstaller:id/permission_allow_button"

	// Click on ALLOW button for edit file access.
	allowButton := d.Object(arcui.ResourceIDMatches(permissionButtonResourceID), arcui.Text("ALLOW"))
	if err := allowButton.WaitForExists(ctx, arcSharesheetUITimeout); err != nil {
		return errors.Wrap(err, "failed as allow file permissions button was not present")
	} else if err := allowButton.Click(ctx); err != nil {
		return errors.Wrap(err, "failed clicking on allow file permissions button")
	}

	return nil
}

// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"io/ioutil"
	"os"
	"path/filepath"
	"time"

	"chromiumos/tast/errors"
	arcui "chromiumos/tast/local/android/ui"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/chrome"
	chromeui "chromiumos/tast/local/chrome/ui"
	"chromiumos/tast/local/chrome/ui/faillog"
	"chromiumos/tast/local/chrome/ui/filesapp"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: Sharesheet,
		Desc: "Install ARC app and share to app via Sharesheet",
		Contacts: []string{
			"benreich@chromium.org",
			"melzhang@chromium.org",
			"chromeos-apps-foundation-team@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Timeout:      7 * time.Minute,
		Params: []testing.Param{{
			ExtraSoftwareDeps: []string{"android_p"},
		}, {
			Name:              "vm",
			ExtraSoftwareDeps: []string{"android_vm"},
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
		appShareLabel        = "ARC Chrome Sharesheet Test"
		expectedFileName     = "test.txt"
		expectedFileContents = "test file contents"
		fileContentsID       = "org.chromium.arc.testapp.chromesharesheet:id/file_content"
	)

	username := s.RequiredVar("arc.Sharesheet.username")
	password := s.RequiredVar("arc.Sharesheet.password")
	pollOpts := testing.PollOptions{Interval: arcSharesheetPollInterval, Timeout: arcSharesheetUITimeout}

	// Setup Chrome.
	cr, err := chrome.New(ctx, chrome.GAIALogin(), chrome.Auth(username, password, "gaia-id"), chrome.ARCEnabled(),
		chrome.ExtraArgs(arc.DisableSyncFlags()...))
	if err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}
	defer cr.Close(ctx)

	// Setup the test file.
	testFileLocation := filepath.Join(filesapp.DownloadPath, expectedFileName)
	if err := ioutil.WriteFile(testFileLocation, []byte(expectedFileContents), 0644); err != nil {
		s.Fatalf("Failed to create file %q: %s", testFileLocation, err)
	}
	defer os.Remove(testFileLocation)

	// Setup Test API Connection.
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create test API connection: ", err)
	}
	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)

	// Setup ARC device and UI Automator.
	arcDevice, uiAutomator, err := setUpARC(ctx, cr, s.OutDir())
	if err != nil {
		s.Fatal("Failed setting up ARC: ", err)
	}
	defer arcDevice.Close()
	defer uiAutomator.Close(ctx)

	if err := arcDevice.Install(ctx, arc.APKPath("ArcChromeSharesheetTest.apk")); err != nil {
		s.Fatal("Failed to install the APK: ", err)
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

	if err := files.SelectFile(ctx, expectedFileName); err != nil {
		s.Fatalf("Failed selecting file %q: %v", expectedFileName, err)
	}

	if err := waitForActionBarStabilized(ctx, tconn, files, &pollOpts); err != nil {
		s.Fatal("Failed waiting for the Share button to appear: ", err)
	}

	if err := clickShareButton(ctx, files, &pollOpts); err != nil {
		s.Fatal("Failed clicking the share sheet button: ", err)
	}

	if err := clickAppOnStableSharesheet(ctx, tconn, files, &pollOpts, appShareLabel); err != nil {
		s.Fatal("Failed waiting for app to appear on sharesheet: ", err)
	}

	// Wait for the file contents to show in the Android test app.
	fileContentField := uiAutomator.Object(arcui.ID(fileContentsID), arcui.Text(expectedFileContents))
	if err := fileContentField.WaitForExists(ctx, arcSharesheetUITimeout); err != nil {
		s.Fatalf("Failed waiting for file contents %q to appear in ARC window: %v", expectedFileContents, err)
	}
}

// waitForActionBarStabilized makes sure the Action bar is stable as items are loaded asynchronously.
func waitForActionBarStabilized(ctx context.Context, tconn *chrome.TestConn, f *filesapp.FilesApp, pollOpts *testing.PollOptions) error {
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

// clickShareButton clicks the Share button in the Action bar on the Files app.
func clickShareButton(ctx context.Context, f *filesapp.FilesApp, pollOpts *testing.PollOptions) error {
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

	return shareButton.StableLeftClick(ctx, pollOpts)
}

// clickAppOnStableSharesheet clicks the requested app on the sharesheet.
// The app must be available in the first 8 apps.
func clickAppOnStableSharesheet(ctx context.Context, tconn *chrome.TestConn, f *filesapp.FilesApp, pollOpts *testing.PollOptions, appShareLabel string) error {
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

// setUpARC starts an ARC device and starts UI automator.
func setUpARC(ctx context.Context, cr *chrome.Chrome, outDir string) (*arc.ARC, *arcui.Device, error) {
	// Setup ARC device.
	arcDevice, err := arc.New(ctx, outDir)
	if err != nil {
		return nil, nil, errors.Wrap(err, "failed to start ARC")
	}

	// Start up UI automator.
	uiAutomator, err := arcDevice.NewUIDevice(ctx)
	if err != nil {
		if err := arcDevice.Close(); err != nil {
			testing.ContextLog(ctx, "Failed closing UI automator: ", err)
		}
		return nil, nil, errors.Wrap(err, "failed initializing UI automator")
	}

	if err := arcDevice.WaitIntentHelper(ctx); err != nil {
		return nil, nil, errors.Wrap(err, "failed waiting for intent helper")
	}

	return arcDevice, uiAutomator, nil
}

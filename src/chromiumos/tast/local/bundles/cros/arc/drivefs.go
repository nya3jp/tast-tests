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

	"chromiumos/tast/local/arc"
	arcUI "chromiumos/tast/local/arc/ui"
	"chromiumos/tast/local/bundles/cros/platform"
	"chromiumos/tast/local/bundles/cros/ui/faillog"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ui"
	"chromiumos/tast/local/chrome/ui/filesapp"
	"chromiumos/tast/testing"
)

const (
	// Test app to read the Drive FS file
	apkName = "ArcFileReaderTest.apk"
	// Filename of the test file
	testFile = "drive_fs_test.txt"
	// Timeout to wait for UI item to appear
	uiTimeout = 10 * time.Second
	// Text labels in the test app and their expected values
	actionID            = "org.chromium.arc.testapp.filereader:id/action"
	expectedAction      = "android.intent.action.VIEW"
	uriID               = "org.chromium.arc.testapp.filereader:id/uri"
	expectedURI         = "content://org.chromium.arc.file_system.fileprovider/download/drive_fs_test.txt"
	fileContentID       = "org.chromium.arc.testapp.filereader:id/file_content"
	expectedFileContent = "this is a test"
)

var drivefsGaia = &arc.GaiaVars{
	UserVar: "arc.username",
	PassVar: "arc.password",
}

// LoggedInAndBooted is a precondition similar to arc.Booted(). The only difference from arc.Booted() is
// that it will GAIA login with the app compat credentials.
var LoggedInAndBooted = arc.NewPrecondition("loggedin_booted", false, drivefsGaia)

func init() {
	testing.AddTest(&testing.Test{
		Func: Drivefs,
		Desc: "Drive FS support for ARC++/ARCVM",
		Contacts: []string{
			"cherieccy@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"android_p", "chrome", "cups"},
		Pre:          LoggedInAndBooted,
		Vars:         []string{"arc.username", "arc.password"},
	})
}

func Drivefs(ctx context.Context, s *testing.State) {
	a := s.PreValue().(arc.PreData).ARC
	cr := s.PreValue().(arc.PreData).Chrome

	//directory := filesapp.DownloadPath
	directory := platform.SetupDrivefs(ctx, s, cr)
	testing.ContextLog(ctx, "Performing this test on: "+directory)

	installReaderApp(ctx, s, a)
	setupTestFile(ctx, s, directory, testFile)
	openWithReaderApp(ctx, s, cr)
	validateResult(ctx, s, a)
	cleanUpTestFile(s, directory, testFile)
}

// installReaderApp installs ArcFileReaderTest app for opening file.
func installReaderApp(ctx context.Context, s *testing.State, a *arc.ARC) {
	testing.ContextLog(ctx, "Installing ArcFileReaderTest app")
	if err := a.Install(ctx, arc.APKPath(apkName)); err != nil {
		s.Fatal("Failed to install ArcFileReaderTest app: ", err)
	}
}

// setupTestFile creates a test file.
func setupTestFile(ctx context.Context, s *testing.State, directory string, filename string) {
	testing.ContextLog(ctx, "Setting up a test file")
	testFileLocation := filepath.Join(directory, filename)
	if err := ioutil.WriteFile(testFileLocation, []byte(expectedFileContent), 0666); err != nil {
		s.Fatalf("Failed to create test file %s: %s", testFileLocation, err)
	}
}

// openWithReaderApp launches FilesApp and opens the test file with ArcFileReaderTest.
func openWithReaderApp(ctx context.Context, s *testing.State, cr *chrome.Chrome) {
	testing.ContextLog(ctx, "Opening the test file with ArcFileReaderTest")
	// Open the test API.
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Creating test API connection failed: ", err)
	}
	defer faillog.DumpUITreeOnError(ctx, s, tconn)

	// Open the Files App.
	files, err := filesapp.Launch(ctx, tconn)
	if err != nil {
		s.Fatal("Launching the Files App failed: ", err)
	}
	// Instead of closing the Files App, just release the memory reference.
	// Otherwise, when this test fails, the screenshot will be of an empty desktop/closing app.
	defer files.Root.Release(ctx)

	// Open the Downloads folder.
	//if err := files.OpenDownloads(ctx); err != nil {
	//	s.Fatal("Opening Downloads folder failed: ", err)
	//}

	// Open the Drive FS folder.
	if err := files.OpenDrive(ctx); err != nil {
		s.Fatal("Could not open Google Drive folder: ", err)
	}

	// Wait for and click the test file.
	if err := files.WaitForFile(ctx, testFile, uiTimeout); err != nil {
		s.Fatalf("Waiting for the test file %s failed: %s", testFile, err)
	}
	if err := files.SelectFile(ctx, testFile); err != nil {
		s.Fatalf("Waiting to select the test file %s failed: %s", testFile, err)
	}

	// Wait for the Open menu button in the top bar.
	params := ui.FindParams{
		Name: "Open",
		Role: ui.RoleTypeButton,
	}
	open, err := files.Root.DescendantWithTimeout(ctx, params, uiTimeout)
	if err != nil {
		s.Fatal("Failed to find the Open menu button: ", err)
	}
	defer open.Release(ctx)

	// Click the Open menu button.
	if err := open.LeftClick(ctx); err != nil {
		s.Fatal("Clicking the Open menu button failed: ", err)
	}

	// Wait for 'Open with ArcFileReaderTest' to appear.
	params = ui.FindParams{
		Name: "ARC File Reader Test",
		Role: ui.RoleTypeStaticText,
	}
	app, err := files.Root.DescendantWithTimeout(ctx, params, uiTimeout)
	if err != nil {
		s.Fatal("Waiting for 'Open with ArcFileReaderTest' failed: ", err)
	}
	defer app.Release(ctx)

	// Click 'Open with ArcFileReaderTest'.
	if err := app.LeftClick(ctx); err != nil {
		s.Fatal("Clicking 'Open with ArcFileReaderTest' failed: ", err)
	}
}

// validateResult validates the results read from ArcFileReaderTest app.
func validateResult(ctx context.Context, s *testing.State, a *arc.ARC) {
	d, err := arcUI.NewDevice(ctx, a)
	if err != nil {
		s.Fatal("Failed initializing UI Automator: ", err)
	}
	defer d.Close()

	validateLabel(ctx, s, d, actionID, expectedAction)
	//validateLabel(ctx, s, d, uriID, expectedURI)
	validateLabel(ctx, s, d, fileContentID, expectedFileContent)
}

// validateLabel is a helper function to load label text and compare with expected result.
func validateLabel(ctx context.Context, s *testing.State, d *arcUI.Device, labelID string, expectedContent string) {
	contentText := d.Object(arcUI.ID(labelID))
	if err := contentText.WaitForExists(ctx, uiTimeout); err != nil {
		s.Fatalf("Failed to find the label %s: %s", labelID, err)
	}

	actualContent, err := contentText.GetText(ctx)
	if err != nil {
		s.Fatalf("Failed to get text from the label %s: %s", labelID, err)
	}

	if actualContent != expectedContent {
		s.Fatalf("Label content mismatch for %s. Actual = %s. Expected = %s", labelID, actualContent, expectedContent)
	}

	testing.ContextLogf(ctx, "Label content for %s = %s", labelID, actualContent)
}

func cleanUpTestFile(s *testing.State, directory string, filename string) {
	s.Log("Removing the test file")
	testFileLocation := filepath.Join(directory, filename)
	os.Remove(testFileLocation)
}

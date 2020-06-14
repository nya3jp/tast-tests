// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package storage

import (
	"context"
	"io/ioutil"
	"os"
	"path/filepath"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	arcUI "chromiumos/tast/local/arc/ui"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ui"
	"chromiumos/tast/local/chrome/ui/filesapp"
	"chromiumos/tast/testing"
)

const (
	// Test app for reading the test file.
	apkName = "ArcFileReaderTest.apk"
	// Filename of the test file.
	testFile = "storage.txt"
	// Timeout to wait for UI item to appear.
	uiTimeout = 10 * time.Second

	// Labels to appear in the test app and their expected values.

	// ActionID is the id of the action label.
	ActionID = "org.chromium.arc.testapp.filereader:id/action"
	// ExpectedAction should be VIEW intent.
	ExpectedAction = "android.intent.action.VIEW"
	// URIID is the id of the uri label.
	URIID = "org.chromium.arc.testapp.filereader:id/uri"
	// FileContentID is the id of the file content label.
	FileContentID = "org.chromium.arc.testapp.filereader:id/file_content"
	// ExpectedFileContent in the test file.
	ExpectedFileContent = "this is a test"
)

// AppLabel is used for validating app label contents.
type AppLabel struct {
	ID            string
	ExpectedValue string
}

// Directory represents a FilesApp directory, e.g. Drive FS, Downloads.
type Directory struct {
	Path  string
	Name  string
	Title string
}

// TestOpenWithAndroidApp performs OpenWith operation on the test file in the specified directory.
func TestOpenWithAndroidApp(ctx context.Context, s *testing.State, a *arc.ARC, cr *chrome.Chrome, dir Directory, appLabels []AppLabel) {
	testing.ContextLogf(ctx, "Performing TestOpenWithAndroidApp on: %s", dir.Path)

	testing.ContextLog(ctx, "Installing ArcFileReaderTest app")
	if err := a.Install(ctx, arc.APKPath(apkName)); err != nil {
		s.Fatal("Failed to install ArcFileReaderTest app: ", err)
	}

	testing.ContextLog(ctx, "Setting up a test file")
	testFileLocation := filepath.Join(dir.Path, testFile)
	if err := ioutil.WriteFile(testFileLocation, []byte(ExpectedFileContent), 0666); err != nil {
		s.Fatalf("Failed to create test file %s: %s", testFileLocation, err)
	}

	if err := openWithReaderApp(ctx, cr, dir); err != nil {
		s.Fatal("Could not open file with ArcFileReaderTest: ", err)
	}

	if err := validateResult(ctx, a, appLabels); err != nil {
		s.Fatal("ArcFileReaderTest's data is invalid: ", err)
	}

	testing.ContextLog(ctx, "Removing the test file")
	os.Remove(testFileLocation)
}

// openWithReaderApp launches FilesApp and opens the test file with ArcFileReaderTest.
func openWithReaderApp(ctx context.Context, cr *chrome.Chrome, dir Directory) error {
	testing.ContextLog(ctx, "Opening the test file with ArcFileReaderTest")

	// Open the test API.
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		return errors.Wrap(err, "creating test API connection failed")
	}

	// Open the Files App.
	files, err := filesapp.Launch(ctx, tconn)
	if err != nil {
		return errors.Wrap(err, "launching the Files App failed")
	}
	// Instead of closing the Files App, just release the memory reference.
	// Otherwise, when this test fails, the screenshot will be of an empty desktop/closing app.
	defer files.Root.Release(ctx)

	// Open the directory under testing.
	if err := files.OpenDir(ctx, dir.Name, dir.Title); err != nil {
		return errors.Wrapf(err, "could not open %s folder", dir.Name)
	}

	// Wait for and click the test file.
	if err := files.WaitForFile(ctx, testFile, uiTimeout); err != nil {
		return errors.Wrapf(err, "waiting for the test file %s failed", testFile)
	}
	if err := files.SelectFile(ctx, testFile); err != nil {
		return errors.Wrapf(err, "selecting the test file %s failed", testFile)
	}

	// Wait for the Open menu button in the top bar (with extra UI timeout).
	params := ui.FindParams{
		Name: "Open",
		Role: ui.RoleTypeButton,
	}
	open, err := files.Root.DescendantWithTimeout(ctx, params, 2*uiTimeout)
	if err != nil {
		return errors.Wrap(err, "failed to find the Open menu button")
	}
	defer open.Release(ctx)

	// Click the Open menu button.
	if err := open.LeftClick(ctx); err != nil {
		return errors.Wrap(err, "clicking the Open menu button failed")
	}

	// Wait for 'Open with ArcFileReaderTest' to appear (with extra UI timeout).
	params = ui.FindParams{
		Name: "ARC File Reader Test",
		Role: ui.RoleTypeStaticText,
	}
	app, err := files.Root.DescendantWithTimeout(ctx, params, 2*uiTimeout)
	if err != nil {
		return errors.Wrap(err, "waiting for 'Open with ArcFileReaderTest' failed")
	}
	defer app.Release(ctx)

	// Click 'Open with ArcFileReaderTest'.
	if err := app.LeftClick(ctx); err != nil {
		return errors.Wrap(err, "clicking 'Open with ArcFileReaderTest' failed")
	}

	return nil
}

// validateResult validates the data read from ArcFileReaderTest app.
func validateResult(ctx context.Context, a *arc.ARC, appLabels []AppLabel) error {
	d, err := arcUI.NewDevice(ctx, a)
	if err != nil {
		return errors.Wrap(err, "failed initializing UI Automator")
	}
	defer d.Close()

	for _, appLabel := range appLabels {
		if err := validateLabel(ctx, d, appLabel); err != nil {
			return err
		}
	}

	return nil
}

// validateLabel is a helper function to load appLabel text and compare it with expected value.
func validateLabel(ctx context.Context, d *arcUI.Device, appLabel AppLabel) error {
	content := d.Object(arcUI.ID(appLabel.ID))
	if err := content.WaitForExists(ctx, uiTimeout); err != nil {
		return errors.Wrapf(err, "failed to find the appLabel %s", appLabel.ID)
	}

	actualValue, err := content.GetText(ctx)
	if err != nil {
		return errors.Wrapf(err, "failed to get text from the appLabel %s", appLabel.ID)
	}

	if actualValue != appLabel.ExpectedValue {
		return errors.Errorf("label content mismatch for %s. Actual = %s. Expected = %s", appLabel.ID, actualValue, appLabel.ExpectedValue)
	}

	testing.ContextLogf(ctx, "Label content of %s = %s", appLabel.ID, actualValue)
	return nil
}

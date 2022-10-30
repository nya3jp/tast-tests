// Copyright 2020 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package storage

import (
	"context"
	"io/ioutil"
	"os"
	"path/filepath"
	"time"

	androidui "chromiumos/tast/common/android/ui"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/filesapp"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

const (
	// Timeout to wait for UI item to appear.
	uiTimeout = 10 * time.Second
	// Test app's name displayed in the context menu of the Files app.
	testAppName = "ARC File Reader Test"

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

// Expectation is used for validating app label contents.
type Expectation struct {
	LabelID string
	// Predicate returns whether the actual value is valid. It can be nil, in
	// which case the implied validation condition is "actual == Value".
	Predicate func(actual string) bool
	// Value is the expected value (when Predicate is nil). This field is
	// ignored if Predicate is non-nil.
	Value string
}

// TestConfig stores the details of the directory under test and misc test configurations.
type TestConfig struct {
	// Name of the directory.
	DirName string
	// Title of the directory.
	DirTitle string
	// If specified, open the sub-directories under "DirName".
	SubDirectories []string
	// Actual path of the directory on the file system.
	DirPath string
	// If set to true, create the test file at "DirPath". For directory where there is no actual
	// file path, set this to false and create the file before running the test (e.g. MTP).
	CreateTestFile bool
	// Optional: If set to true, wait for file type to appear before opening the file.
	// Currently used by DriveFS to ensure metadata has arrived.
	CheckFileType bool
	// Name of the test file to be used in the test.
	FileName string
	// If set to true, retain the test file created early in "DirPath".
	// Currently used by DriveFS to avoid creating and deleting files.
	KeepFile bool
}

// TestOpenWithAndroidApp opens a test file in the specified directory, e.g. Google Drive,
// Downloads, MyFiles etc, using the test android app, ArcFileReaderTest. The app will display
// the respective Action, URI and FileContent on its UI, to be validated against our
// expected values.
func TestOpenWithAndroidApp(ctx context.Context, s *testing.State, a *arc.ARC, cr *chrome.Chrome, d *androidui.Device, config TestConfig, expectations []Expectation) {
	testing.ContextLogf(ctx, "Performing TestOpenWithAndroidApp on: %s", config.DirName)

	testing.ContextLog(ctx, "Installing ArcFileReaderTest app")
	if err := a.Install(ctx, arc.APKPath("ArcFileReaderTest.apk")); err != nil {
		s.Fatal("Failed to install ArcFileReaderTest app: ", err)
	}

	if config.CreateTestFile {
		testFileLocation := filepath.Join(config.DirPath, config.FileName)
		_, err := os.Stat(testFileLocation)
		fileNotExist := errors.Is(err, os.ErrNotExist)

		if fileNotExist {
			testing.ContextLog(ctx, "Setting up a test file")
			if err := ioutil.WriteFile(testFileLocation, []byte(ExpectedFileContent), 0666); err != nil {
				s.Fatalf("Failed to create test file %s: %s", testFileLocation, err)
			}
		}
		if !config.KeepFile {
			defer os.Remove(testFileLocation)
		}
	}

	if err := a.WaitIntentHelper(ctx); err != nil {
		s.Fatal("Failed to wait for ARC Intent Helper: ", err)
	}

	files, err := openFilesApp(ctx, cr)
	if err != nil {
		s.Fatal("Failed to open Files App: ", err)
	}
	defer files.Close(ctx)

	if err := openWithReaderApp(ctx, files, config); err != nil {
		s.Fatal("Could not open file with ArcFileReaderTest: ", err)
	}

	if err := validateResult(ctx, d, expectations); err != nil {
		s.Fatal("ArcFileReaderTest's data is invalid: ", err)
	}
}

// openFilesApp opens the Files App and returns a pointer to it.
func openFilesApp(ctx context.Context, cr *chrome.Chrome) (*filesapp.FilesApp, error) {
	testing.ContextLog(ctx, "Opening Files App")

	// Open the test API.
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "creating test API connection failed")
	}

	// Open the Files App with default timeouts.
	files, err := filesapp.Launch(ctx, tconn)
	if err != nil {
		return nil, errors.Wrap(err, "launching the Files App failed")
	}

	return files, nil
}

// openWithReaderApp opens the test file with ArcFileReaderTest.
func openWithReaderApp(ctx context.Context, files *filesapp.FilesApp, config TestConfig) error {
	testing.ContextLog(ctx, "Opening the test file with ArcFileReaderTest")

	return uiauto.Combine("open the test file with ArcFileReaderTest",
		files.OpenPath(config.DirTitle, config.DirName, config.SubDirectories...),
		// Note: due to the banner loading, this may still be flaky.
		// If that is the case, we may want to increase the interval and timeout for this next call.
		files.SelectFile(config.FileName),
		func(ctx context.Context) error {
			if config.CheckFileType {
				if err := waitForFileType(ctx, files); err != nil {
					return errors.Wrap(err, "waiting for file type failed")
				}
				if err := files.SelectFile(config.FileName)(ctx); err != nil {
					return errors.Wrapf(err, "selecting the test file %s failed", config.FileName)
				}
			}
			return nil
		},
		files.ClickContextMenuItem(config.FileName, filesapp.OpenWith, testAppName),
	)(ctx)
}

// waitForFileType waits for file type (mime type) to be populated. This is an
// indication that the backend metadata is ready.
func waitForFileType(ctx context.Context, files *filesapp.FilesApp) error {
	// Get the keyboard.
	keyboard, err := input.Keyboard(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to get keyboard")
	}
	defer keyboard.Close()

	// Press 'Space' to open/close the QuickView and check the file type. Repeat this
	// until 'text/plain' is shown. Retries up to 6 times (~ 30 seconds).
	times := 6
	if err := uiauto.Retry(times,
		uiauto.Combine("Checking file type",
			keyboard.AccelAction("Space"),
			files.WithTimeout(5*time.Second).WaitUntilExists(nodewith.Name("text/plain").Role(role.StaticText)),
			keyboard.AccelAction("Space"),
		),
	)(ctx); err != nil {
		return errors.Wrapf(err, "failed to wait for file type after %d retries", times)
	}
	return nil
}

// validateResult validates the data read from ArcFileReaderTest app.
func validateResult(ctx context.Context, d *androidui.Device, expectations []Expectation) error {
	testing.ContextLog(ctx, "Validating result in ArcFileReaderTest")

	for _, e := range expectations {
		if err := validateLabel(ctx, d, e); err != nil {
			return err
		}
	}

	return nil
}

// validateLabel is a helper function to load app label texts and compare it with expectation.
func validateLabel(ctx context.Context, d *androidui.Device, expectation Expectation) error {
	uiObj := d.Object(androidui.ID(expectation.LabelID))
	if err := uiObj.WaitForExists(ctx, uiTimeout); err != nil {
		return errors.Wrapf(err, "failed to find the label id %s", expectation.LabelID)
	}

	actual, err := uiObj.GetText(ctx)
	if err != nil {
		return errors.Wrapf(err, "failed to get text from the label id %s", expectation.LabelID)
	}

	if expectation.Predicate != nil {
		if !expectation.Predicate(actual) {
			return errors.Errorf("unexpected value in label %s: got %q", expectation.LabelID, actual)
		}
	} else {
		if actual != expectation.Value {
			return errors.Errorf("unexpected value in label %s: got %q, want %q", expectation.LabelID, actual, expectation.Value)
		}
	}

	testing.ContextLogf(ctx, "Label content of %s = %s", expectation.LabelID, actual)
	return nil
}

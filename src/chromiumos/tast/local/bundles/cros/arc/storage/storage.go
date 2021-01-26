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
	androidui "chromiumos/tast/local/android/ui"
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

// Expectation is used for validating app label contents.
type Expectation struct {
	LabelID string
	Value   string
}

// Directory represents a FilesApp directory, e.g. Drive FS, Downloads.
type Directory struct {
	Path  string
	Name  string
	Title string
	// Optional: If CheckFileType is true, wait for file type to appear before opening the file.
	CheckFileType bool
}

// TestOpenWithAndroidApp performs OpenWith operation on the test file in the specified directory dir.
// dir needs to be one of the top folders in FilesApp, e.g. Google Drive, Downloads.
func TestOpenWithAndroidApp(ctx context.Context, s *testing.State, a *arc.ARC, cr *chrome.Chrome, dir Directory, expectations []Expectation) {
	testing.ContextLogf(ctx, "Performing TestOpenWithAndroidApp on: %s", dir.Path)

	testing.ContextLog(ctx, "Installing ArcFileReaderTest app")
	if err := a.Install(ctx, arc.APKPath("ArcFileReaderTest.apk")); err != nil {
		s.Fatal("Failed to install ArcFileReaderTest app: ", err)
	}

	testing.ContextLog(ctx, "Setting up a test file")
	testFileLocation := filepath.Join(dir.Path, testFile)
	if err := ioutil.WriteFile(testFileLocation, []byte(ExpectedFileContent), 0666); err != nil {
		s.Fatalf("Failed to create test file %s: %s", testFileLocation, err)
	}
	defer os.Remove(testFileLocation)

	if err := a.WaitIntentHelper(ctx); err != nil {
		s.Fatal("Failed to wait for ARC Intent Helper: ", err)
	}

	files, err := openFilesApp(ctx, cr)
	if err != nil {
		s.Fatal("Failed to open Files App: ", err)
	}

	if err := openWithReaderApp(ctx, files, dir); err != nil {
		s.Fatal("Could not open file with ArcFileReaderTest: ", err)
	}

	if err := validateResult(ctx, a, expectations); err != nil {
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

	// Open the Files App.
	ui := uiauto.New(tconn).WithTimeout(uiTimeout)
	files, err := filesapp.Launch(ctx, tconn, ui)
	if err != nil {
		return nil, errors.Wrap(err, "launching the Files App failed")
	}

	return files, nil
}

// openWithReaderApp opens the test file with ArcFileReaderTest.
func openWithReaderApp(ctx context.Context, files *filesapp.FilesApp, dir Directory) error {
	testing.ContextLog(ctx, "Opening the test file with ArcFileReaderTest")

	return uiauto.Run(ctx,
		files.OpenDir(dir.Name, dir.Title),
		// Note: due to the banner loading, this may still be flaky.
		// If that is the case, we may want to increase the interval and timeout for this next call.
		files.SelectFile(testFile),
		func(ctx context.Context) error {
			if dir.CheckFileType {
				if err := waitForFileType(ctx, files); err != nil {
					return errors.Wrap(err, "waiting for file type failed")
				}
				if err := files.SelectFile(testFile)(ctx); err != nil {
					return errors.Wrapf(err, "selecting the test file %s failed", testFile)
				}
			}
			return nil
		},
		files.UI.LeftClick(nodewith.Name("Open").Role(role.Button).Ancestor(filesapp.WindowFinder)),
		files.UI.LeftClick(nodewith.Name("ARC File Reader Test").Role(role.StaticText).Ancestor(filesapp.WindowFinder)),
	)
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
	// until 'text/plain' is shown.
	const pollTimeout = 30 * time.Second
	if err := testing.Poll(ctx, func(ctx context.Context) (e error) {
		// Open QuickView.
		if err := keyboard.Accel(ctx, "Space"); err != nil {
			return errors.Wrap(err, "failed to press Space")
		}
		defer func() {
			// Close QuickView.
			if err := keyboard.Accel(ctx, "Space"); err != nil {
				e = errors.Wrap(err, "failed to press Space")
			}
		}()

		if err := files.UI.WaitUntilExists(nodewith.Name("text/plain").Role(role.StaticText).Ancestor(filesapp.WindowFinder))(ctx); err != nil {
			return errors.Wrap(err, "file type was not found")
		}
		return nil
	}, &testing.PollOptions{Timeout: pollTimeout}); err != nil {
		return errors.Wrapf(err, "timeout after %s", pollTimeout)
	}
	return nil
}

// validateResult validates the data read from ArcFileReaderTest app.
func validateResult(ctx context.Context, a *arc.ARC, expectations []Expectation) error {
	testing.ContextLog(ctx, "Validating result in ArcFileReaderTest")

	d, err := a.NewUIDevice(ctx)
	if err != nil {
		return errors.Wrap(err, "failed initializing UI Automator")
	}
	defer d.Close(ctx)

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

	if actual != expectation.Value {
		return errors.Errorf("unexpected value in label %s: got %q, want %q", expectation.LabelID, actual, expectation.Value)
	}

	testing.ContextLogf(ctx, "Label content of %s = %s", expectation.LabelID, actual)
	return nil
}

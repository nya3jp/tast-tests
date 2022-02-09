// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package storage

import (
	"bytes"
	"context"
	"io/ioutil"
	"os"
	"path/filepath"
	"time"

	androidui "chromiumos/tast/common/android/ui"
	"chromiumos/tast/ctxutil"
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
func TestOpenWithAndroidApp(ctx context.Context, a *arc.ARC, cr *chrome.Chrome, d *androidui.Device, config TestConfig, expectations []Expectation) error {
	testing.ContextLogf(ctx, "Performing TestOpenWithAndroidApp on: %s", config.DirName)

	testing.ContextLog(ctx, "Installing ArcFileReaderTest app")
	if err := a.Install(ctx, arc.APKPath("ArcFileReaderTest.apk")); err != nil {
		return errors.Wrap(err, "failed to install ArcFileReaderTest app")
	}

	testFileLocation := filepath.Join(config.DirPath, config.FileName)
	_, err := os.Stat(testFileLocation)
	fileNotExist := errors.Is(err, os.ErrNotExist)

	if config.CreateTestFile {
		if fileNotExist {
			testing.ContextLog(ctx, "Setting up a test file")
			if err := ioutil.WriteFile(testFileLocation, []byte(ExpectedFileContent), 0666); err != nil {
				return errors.Wrapf(err, "failed to create test file %s", testFileLocation)
			}
		}
		if !config.KeepFile {
			defer os.Remove(testFileLocation)
		}
	}

	if err := a.WaitIntentHelper(ctx); err != nil {
		return errors.Wrap(err, "failed to wait for ARC Intent Helper")
	}

	files, err := openFilesApp(ctx, cr)
	if err != nil {
		return errors.Wrap(err, "failed to open Files app")
	}

	if err := openWithReaderApp(ctx, files, config); err != nil {
		return errors.Wrap(err, "failed to open file with ArcFileReaderTest app")
	}

	if err := validateResult(ctx, d, expectations); err != nil {
		return errors.Wrap(err, "ArcFileReaderTest app's data is invalid")
	}

	return nil
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
		files.LeftClick(nodewith.Name("Open").Role(role.Button)),
		files.LeftClick(nodewith.Name("ARC File Reader Test").Role(role.StaticText)),
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

	if actual != expectation.Value {
		return errors.Errorf("unexpected value in label %s: got %q, want %q", expectation.LabelID, actual, expectation.Value)
	}

	testing.ContextLogf(ctx, "Label content of %s = %s", expectation.LabelID, actual)
	return nil
}

// TestVolumeSharing tests whether a storage volume is properly shared with ARC
// by checking whether 1) a file created on the Android side can be read from
// the Chrome OS side, and 2) a file created on the Chrome OS side can be read
// by Android apps via the Chrome OS Files app.
func TestVolumeSharing(ctx context.Context, a *arc.ARC, cr *chrome.Chrome, d *androidui.Device, dirPath, dirName, uuid, fileName, dataPath string) error {
	testing.ContextLog(ctx, "Testing Android -> CrOS")
	if err := testVolumeSharingARCToCros(ctx, a, dirPath, uuid, fileName, dataPath); err != nil {
		return errors.Wrap(err, "Android -> CrOS failed")
	}

	testing.ContextLog(ctx, "Testing CrOS -> Android")
	if err := testVolumeSharingCrosToARC(ctx, a, cr, d, dirPath, dirName, uuid); err != nil {
		return errors.Wrap(err, "CrOS -> Android failed")
	}

	return nil
}

// testVolumeSharingARCToCros checks whether a file created in a volume on the
// Android side can be read from the Chrome OS side.
func testVolumeSharingARCToCros(ctx context.Context, a *arc.ARC, crosDir, uuid, fileName, dataPath string) error {
	androidPath := filepath.Join("/storage", uuid, fileName)
	crosPath := filepath.Join(crosDir, fileName)

	return testPushToARCAndReadFromCros(ctx, a, androidPath, crosPath, dataPath)
}

// testPushToARCAndReadFromCros pushes the content of dataPath (in Chrome OS)
// to androidPath (in Android) using adb, and then checks whether the file can
// be accessed under crosPath (in Chrome OS).
func testPushToARCAndReadFromCros(ctx context.Context, a *arc.ARC, androidPath, crosPath, dataPath string) (retErr error) {
	// Shorten the context to make room for cleanup jobs.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	expected, err := ioutil.ReadFile(dataPath)
	if err != nil {
		return errors.Wrapf(err, "failed to read from %s in Chrome OS", dataPath)
	}

	if err := a.WriteFile(ctx, androidPath, expected); err != nil {
		return errors.Wrapf(err, "failed to write to %s in Android", androidPath)
	}
	defer func(ctx context.Context) {
		if err := a.RemoveAll(ctx, androidPath); err != nil {
			if retErr == nil {
				retErr = errors.Wrapf(err, "failed to remove %s in Android", androidPath)
			} else {
				testing.ContextLogf(ctx, "Failed to remove %s in Android: %v", androidPath, err)
			}
		}
	}(cleanupCtx)

	actual, err := ioutil.ReadFile(crosPath)
	if err != nil {
		return errors.Wrapf(err, "failed to read from %s in Chrome OS", crosPath)
	}
	if !bytes.Equal(actual, expected) {
		return errors.Errorf("content mismatch between %s in Android and %s in Chrome OS", androidPath, crosPath)
	}

	return nil
}

// testVolumeSharingCrosToARC checks whether a file created in a volume on the
// Chrome OS side can be read by Android apps via the Chrome OS Files app.
func testVolumeSharingCrosToARC(ctx context.Context, a *arc.ARC, cr *chrome.Chrome, d *androidui.Device, dirPath, dirName, uuid string) error {
	config := TestConfig{DirPath: dirPath, DirName: dirName, DirTitle: "Files - " + dirName,
		CreateTestFile: true, FileName: "storage.txt"}
	testFileURI := arc.VolumeProviderContentURIPrefix + filepath.Join(uuid, config.FileName)

	expectations := []Expectation{
		{LabelID: ActionID, Value: ExpectedAction},
		{LabelID: URIID, Value: testFileURI},
		{LabelID: FileContentID, Value: ExpectedFileContent}}

	return TestOpenWithAndroidApp(ctx, a, cr, d, config, expectations)
}

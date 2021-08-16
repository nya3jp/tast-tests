// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package storage

import (
	"bytes"
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"time"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/ctxutil"
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
	// Timeout to wait for UI item to appear.
	uiTimeout = 10 * time.Second

	// VolumeProviderContentURIPrefix is the prefix of VolumeProvider content URIs.
	VolumeProviderContentURIPrefix = "content://org.chromium.arc.volumeprovider/"

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
	// If set to true, create the test file at "DirPath". The content of the
	// created file is set to "ExpectedFileContent". For a directory where there is
	// no actual file path, set this to false and create the file before running
	// the test (e.g. MTP).
	CreateTestFile bool
	// Optional: If set to true, write the content of the test file with the
	// ArcFileWriterTest app (regardless of whether "CreateTestFile" is true).
	WriteFileContentWithApp bool
	// Optional: If set to true, wait for file type to appear before opening the file.
	// Currently used by DriveFS to ensure metadata has arrived.
	CheckFileType bool
	// Name of the test file to be used in the test.
	FileName string
}

// TestOpenWithAndroidApp opens a test file in the specified directory, e.g. Google Drive,
// Downloads, MyFiles etc, using the test android app, ArcFileReaderTest. The app will display
// the respective Action, URI and FileContent on its UI, to be validated against our
// expected values.
func TestOpenWithAndroidApp(ctx context.Context, s *testing.State, a *arc.ARC, cr *chrome.Chrome, config TestConfig, expectations []Expectation) {
	testing.ContextLogf(ctx, "Performing TestOpenWithAndroidApp on: %s", config.DirName)

	testing.ContextLog(ctx, "Installing ArcFileReaderTest app")
	if err := a.Install(ctx, arc.APKPath("ArcFileReaderTest.apk")); err != nil {
		s.Fatal("Failed to install ArcFileReaderTest app: ", err)
	}

	if config.CreateTestFile {
		testing.ContextLog(ctx, "Setting up a test file")
		content := ExpectedFileContent
		if config.WriteFileContentWithApp {
			content = ""
		}
		testFileLocation := filepath.Join(config.DirPath, config.FileName)
		if err := ioutil.WriteFile(testFileLocation, []byte(content), 0666); err != nil {
			s.Fatalf("Failed to create test file %s: %s", testFileLocation, err)
		}
		defer os.Remove(testFileLocation)
	}

	if err := a.WaitIntentHelper(ctx); err != nil {
		s.Fatal("Failed to wait for ARC Intent Helper: ", err)
	}

	files, err := openFilesApp(ctx, cr)
	if err != nil {
		s.Fatal("Failed to open Files App: ", err)
	}

	if config.WriteFileContentWithApp {
		if err := writeFileContentWithApp(ctx, a, files, config); err != nil {
			s.Fatal("Failed to write the test file content with ArcFileWriterTest: ", err)
		}
	}

	if err := openWithApp(ctx, files, config, "ArcFileReaderTest"); err != nil {
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

	// Open the Files App with default timeouts.
	files, err := filesapp.Launch(ctx, tconn)
	if err != nil {
		return nil, errors.Wrap(err, "launching the Files App failed")
	}

	return files, nil
}

// writeFileContentWithApp writes the test file content with ArcFileWriterTest.
func writeFileContentWithApp(ctx context.Context, a *arc.ARC, files *filesapp.FilesApp, config TestConfig) error {
	testFileLocation := filepath.Join(config.DirPath, config.FileName)
	// The mode parameter specified in ioutil.WriteFile is affected by umask.
	// Hence we apply chmod here so that the test app can write to the file.
	if err := os.Chmod(testFileLocation, 0666); err != nil {
		return errors.Wrapf(err, "failed to chmod %s", testFileLocation)
	}

	testing.ContextLog(ctx, "Installing ArcFileWriterTest app")
	if err := a.Install(ctx, arc.APKPath("ArcFileWriterTest.apk")); err != nil {
		return errors.Wrap(err, "failed to install ArcFileWriterTest")
	}

	if err := openWithApp(ctx, files, config, "ArcFileWriterTest"); err != nil {
		return errors.Wrap(err, "failed to open the test file with ArcFileWriterTest")
	}

	d, err := a.NewUIDevice(ctx)
	if err != nil {
		return errors.Wrap(err, "failed initializing UI Automator")
	}
	defer d.Close(ctx)

	return validateLabel(ctx, d, "org.chromium.arc.testapp.filewriter:id/result", "Success")
}

// openWithApp opens the test file with the specified app.
func openWithApp(ctx context.Context, files *filesapp.FilesApp, config TestConfig, appName string) error {
	testing.ContextLogf(ctx, "Opening the test file with %s", appName)

	return uiauto.Combine(fmt.Sprintf("open the test file with %s", appName),
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
		files.LeftClick(nodewith.Name(appName).Role(role.StaticText)),
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
func validateResult(ctx context.Context, a *arc.ARC, expectations []Expectation) error {
	testing.ContextLog(ctx, "Validating result in ArcFileReaderTest")

	d, err := a.NewUIDevice(ctx)
	if err != nil {
		return errors.Wrap(err, "failed initializing UI Automator")
	}
	defer d.Close(ctx)

	for _, e := range expectations {
		if err := validateLabel(ctx, d, e.LabelID, e.Value); err != nil {
			return err
		}
	}

	return nil
}

// validateLabel is a helper function to load app label texts and compare it with expectation.
func validateLabel(ctx context.Context, d *androidui.Device, label, expected string) error {
	uiObj := d.Object(androidui.ID(label))
	if err := uiObj.WaitForExists(ctx, uiTimeout); err != nil {
		return errors.Wrapf(err, "failed to find the label id %s", label)
	}

	actual, err := uiObj.GetText(ctx)
	if err != nil {
		return errors.Wrapf(err, "failed to get text from the label id %s", label)
	}

	if actual != expected {
		return errors.Errorf("unexpected value in label %s: got %q, want %q", label, actual, expected)
	}

	testing.ContextLogf(ctx, "Label content of %s = %s", label, actual)
	return nil
}

// TestPushToARCAndReadFromCros pushes the content of sourcePath (in Chrome OS)
// to androidPath (in Android) using adb, and then checks whether the file can
// be accessed under crosPath (in Chrome OS).
func TestPushToARCAndReadFromCros(ctx context.Context, a *arc.ARC, sourcePath, androidPath, crosPath string) (retErr error) {
	// Shorten the context to make room for cleanup jobs.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	expected, err := ioutil.ReadFile(sourcePath)
	if err != nil {
		return errors.Wrapf(err, "failed to read from %s in Chrome OS", sourcePath)
	}

	if err := a.WriteFile(ctx, androidPath, expected); err != nil {
		return errors.Wrapf(err, "failed to write to %s in Android", androidPath)
	}
	defer func(ctx context.Context) {
		if err := a.RemoveAll(ctx, androidPath); err != nil {
			if retErr == nil {
				retErr = errors.Wrapf(err, "failed remove %s in Android", androidPath)
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

// WaitForARCVolumeMount waits for the volume to be mounted in ARC using the sm command.
// Just checking mountinfo is not sufficient here since it takes some
// time for the FUSE layer in Android R+ to be ready after /storage/<UUID> has
// become a mountpoint.
func WaitForARCVolumeMount(ctx context.Context, a *arc.ARC, uuid string) error {
	// Regular expression that matches the output line for the mounted
	// volume. Each output line of the sm command is of the form:
	// <volume id><space(s)><mount status><space(s)><volume UUID>.
	// Examples:
	//   1821167369 mounted 00000000000000000000000000000000DEADBEEF
	//   stub:18446744073709551614 mounted 0000000000000000000000000000CAFEF00D2019
	re := regexp.MustCompile(`^(stub:)?[0-9]+\s+mounted\s+` + uuid + `$`)

	testing.ContextLog(ctx, "Waiting for the volume to be mounted in ARC")

	return testing.Poll(ctx, func(ctx context.Context) error {
		out, err := a.Command(ctx, "sm", "list-volumes").Output(testexec.DumpLogOnError)
		if err != nil {
			return testing.PollBreak(errors.Wrap(err, "sm command failed"))
		}
		lines := bytes.Split(out, []byte("\n"))
		for _, line := range lines {
			if re.Find(bytes.TrimSpace(line)) != nil {
				return nil
			}
		}
		return errors.New("the volume is not yet mounted")
	}, &testing.PollOptions{Timeout: 30 * time.Second})
}

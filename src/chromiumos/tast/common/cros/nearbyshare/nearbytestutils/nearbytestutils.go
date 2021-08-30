// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package nearbytestutils provides utility functions for Nearby Share tests.
package nearbytestutils

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"io/ioutil"
	"math"
	"math/rand"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
	"chromiumos/tast/fsutil"
	"chromiumos/tast/local/chrome/nearbyshare/nearbysnippet"
	"chromiumos/tast/local/syslog"
	"chromiumos/tast/testing"
)

// TestData contains the values for parameterized tests, such as:
// - File name of the archive containing files to be shared
// - File transfer timeout (varies depending on file size)
// - Total test timeout (transfer timeout + time required for sender and receiver to detect each other)
// - MIME type of shared files (only required when sending from Android)
type TestData struct {
	Filename        string
	TransferTimeout time.Duration
	TestTimeout     time.Duration
	MimeType        nearbysnippet.MimeType
}

// DownloadPath is the downloads directory on CrOS.
const DownloadPath = "/home/chronos/user/Downloads/"

// SendDir is the staging directory for test files when sending from CrOS.
const SendDir = DownloadPath + "nearby_test_files"

// UnzipTestFiles extracts test data files to a temporary directory. Returns an array of base filenames and the name of the temporary dir.
// The extracted files can then be pushed to the Android device or copied to a user-accessible directory on CrOS, depending on which device is the sender.
// The data files supplied for file transfer tests should be contained in a .zip file regardless of how many files are being transferred.
func UnzipTestFiles(ctx context.Context, zipPath string) (filenames []string, tempDir string, err error) {
	tempDir, err = ioutil.TempDir("", "nearby-test-files")
	if err != nil {
		return filenames, tempDir, errors.Wrap(err, "failed to create temp dir")
	}
	if err := testexec.CommandContext(ctx, "unzip", zipPath, "-d", tempDir).Run(testexec.DumpLogOnError); err != nil {
		return filenames, tempDir, errors.Wrapf(err, "failed to unzip test data from %v", zipPath)
	}

	files, err := ioutil.ReadDir(tempDir)
	if err != nil {
		return filenames, tempDir, errors.Wrap(err, "failed to read tempDir's contents")
	}
	for _, f := range files {
		filenames = append(filenames, f.Name())
	}
	return filenames, tempDir, nil
}

// ExtractCrosTestFiles prepares test files to send from a CrOS device. The test files will be staged in SendDir,
// which is a subdirectory of the current user's download directory. Callers should defer removing the test files to clean up after tests.
func ExtractCrosTestFiles(ctx context.Context, zipPath string) ([]string, error) {
	filenames, tempDir, err := UnzipTestFiles(ctx, zipPath)
	if err != nil {
		return filenames, err
	}
	defer os.RemoveAll(tempDir)

	targetPath := SendDir

	// Delete and remake the target directory to ensure there are no existing files.
	if err := os.RemoveAll(targetPath); err != nil {
		return nil, errors.Wrap(err, "failed to delete the target path")
	}
	// Ensure the subdirectory has the same mode as user-created ones in /home/chronos/user/Downloads.
	if err := os.Mkdir(targetPath, os.FileMode(int(0711))); err != nil {
		return filenames, errors.Wrap(err, "failed to create subdirectory in Downloads folder")
	}

	for _, f := range filenames {
		src := filepath.Join(tempDir, f)
		dst := filepath.Join(targetPath, f)
		if err := fsutil.CopyFile(src, dst); err != nil {
			return nil, errors.Wrapf(err, "failed to copy test file %v to %v", src, dst)
		}
		// Sharing may fail depending on file permissions, so set the file permissions to the Download folder's default.
		if err := os.Chmod(dst, os.FileMode(int(0644))); err != nil {
			return nil, errors.Wrapf(err, "failed to change mode of %v", dst)
		}
	}
	return filenames, nil
}

// ExtractAndroidTestFile prepares a test file to send from an Android device.
func ExtractAndroidTestFile(ctx context.Context, zipPath string, a *nearbysnippet.AndroidNearbyDevice) (string, error) {
	filenames, tempDir, err := UnzipTestFiles(ctx, zipPath)
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(tempDir)

	// Expecting just 1 file for Android, which is a limitation of the Nearby Snippet.
	if len(filenames) != 1 {
		return "", errors.Errorf("expected exactly 1 file, got %v", len(filenames))
	}
	if err := a.StageFile(ctx, filepath.Join(tempDir, filenames[0])); err != nil {
		return "", err
	}
	return filenames[0], nil
}

// RandomDeviceName appends a randomly generated integer (up to 6 digits) to the base device name to avoid conflicts
// when nearby devices in the lab may be running the same test at the same time.
func RandomDeviceName(basename string) string {
	const maxDigits = 6
	rand.Seed(time.Now().UnixNano())
	num := rand.Intn(int(math.Pow10(maxDigits) - 1))
	return basename + strconv.Itoa(num)
}

// FileHashComparison compares file hashes on CrOS and Android after a share has been completed.
func FileHashComparison(ctx context.Context, filenames []string, crosFileDir, androidFileDir string, androidDevice *nearbysnippet.AndroidNearbyDevice) error {
	var mismatched []string
	for _, f := range filenames {
		// Get the hash on the CrOS side.
		crosHash, err := HashFile(ctx, crosFileDir, f)
		if err != nil {
			return err
		}

		// Get the hash on the Android side.
		androidHash, err := androidDevice.SHA256Sum(ctx, filepath.Join(androidFileDir, f))
		if err != nil {
			return errors.Wrapf(err, "failed to get test file's (%v) sha256sum on Android", f)
		}

		if crosHash != androidHash {
			mismatched = append(mismatched, f)
		}
	}

	if len(mismatched) != 0 {
		return errors.Errorf("CrOS and Android hashes did not match for files %v", mismatched)
	}
	return nil
}

// HashFile returns the hash for a single file.
func HashFile(ctx context.Context, filePath, fileName string) (string, error) {
	filePath = filepath.Join(filePath, fileName)
	r, err := os.Open(filePath)
	if err != nil {
		return "", errors.Wrapf(err, "failed to open test file %v on CrOS", filePath)
	}
	defer r.Close()
	h := sha256.New()
	if _, err := io.Copy(h, r); err != nil {
		return "", errors.Wrapf(err, "failed to copy %v file contents to the hasher", filePath)
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// HashFiles takes in a list of filenames and returns a list of their hashes.
func HashFiles(ctx context.Context, filenames []string, fileDir string) ([]string, error) {
	var hashes []string
	for _, f := range filenames {
		crosHash, err := HashFile(ctx, fileDir, f)
		if err != nil {
			return nil, err
		}
		hashes = append(hashes, crosHash)
	}
	return hashes, nil
}

// StartLogging starts collecting logs from the specified log file, such as /var/log/chrome/chrome or /var/log/messages.
// Only log lines that appear after StartLogging is called will be collected, so logs for
// individual tests can be extracted if tests are running consecutively on a shared fixture or precondition.
// Callers should defer calling Save with the returned *syslog.LineReader to save the logs and free associated resources.
func StartLogging(ctx context.Context, path string) (*syslog.LineReader, error) {
	// Poll for a couple of secs only so that service code calling into this doesn't hang.
	reader, err := syslog.NewLineReader(ctx, path, false, &testing.PollOptions{Timeout: 2 * time.Second})
	if err != nil {
		return nil, errors.Wrap(err, "failed to create LineReader")
	}
	return reader, nil
}

// SaveLogs saves the logs that have appeared since StartLogging was called, and then closes the individual line readers.
func SaveLogs(ctx context.Context, reader *syslog.LineReader, path string) error {
	// Ensure the LineReader is closed.
	defer reader.Close()

	log, err := os.Create(path)
	if err != nil {
		return errors.Wrapf(err, "failed to create %v", path)
	}
	defer log.Close()
	for {
		line, err := reader.ReadLine()
		if err == io.EOF {
			break
		} else if err != nil {
			return errors.Wrap(err, "failed to read log")
		}
		log.WriteString(line)
	}

	return nil
}

// ClearCrOSDownloads clears the Downloads folder (where incoming shares are received).
func ClearCrOSDownloads(ctx context.Context) error {
	files, err := ioutil.ReadDir(DownloadPath)
	if err != nil {
		return errors.Wrap(err, "failed to retrieve Downloads folder contents")
	}

	for _, f := range files {
		if err := os.RemoveAll(filepath.Join(DownloadPath, f.Name())); err != nil {
			return errors.Wrapf(err, "failed to remove %v from Downloads", f.Name())
		}
	}
	return nil
}

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

	"chromiumos/tast/errors"
	"chromiumos/tast/fsutil"
	"chromiumos/tast/local/bundles/cros/nearbyshare/nearbysnippet"
	"chromiumos/tast/local/chrome/nearbyshare"
	"chromiumos/tast/local/testexec"
)

// TestData contains the values for parameterized tests, such as the file name and the total test timeout
// which can vary depending on file size. The timeout for the test is actually set in the testing.Params,
// Android Nearby tests need to know the total timeout, which is used as a parameter to several RPCs.
type TestData struct {
	Filename string
	Timeout  time.Duration
}

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

// ExtractCrosTestFiles prepares test files to send from a CrOS device. The test files will be staged in nearbyshare.SendDir,
// which is a subdirectory of the current user's download directory. Callers should defer removing the test files to clean up after tests.
func ExtractCrosTestFiles(ctx context.Context, zipPath string) ([]string, error) {
	filenames, tempDir, err := UnzipTestFiles(ctx, zipPath)
	if err != nil {
		return filenames, err
	}
	defer os.RemoveAll(tempDir)

	targetPath := nearbyshare.SendDir

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
		crosPath := filepath.Join(crosFileDir, f)
		r, err := os.Open(crosPath)
		if err != nil {
			return errors.Wrapf(err, "failed to open test file %v on CrOS", crosPath)
		}
		defer r.Close()
		h := sha256.New()
		if _, err := io.Copy(h, r); err != nil {
			return errors.Wrapf(err, "failed to copy %v file contents to the hasher", crosPath)
		}
		crosHash := hex.EncodeToString(h.Sum(nil))

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

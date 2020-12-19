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
	"chromiumos/tast/local/chrome/ui/filesapp"
	"chromiumos/tast/local/testexec"
)

// UnzipTestFiles extracts test data files to a temporary directory and copies them
// to the user's download directory, which will be the source of the share.
// The data files supplied for file transfer tests should be contained in a .zip file
// regardless of how many files are being transferred. Callers should defer deleting
// the returned files or clearing the downloads directory.
func UnzipTestFiles(ctx context.Context, zipPath string) ([]string, error) {
	// Extract the data zip file to a temporary directory.
	tempDir, err := ioutil.TempDir("", "nearby-test-files")
	if err != nil {
		return nil, errors.Wrap(err, "failed to create temp dir")
	}
	defer os.RemoveAll(tempDir)
	if err := testexec.CommandContext(ctx, "unzip", zipPath, "-d", tempDir).Run(testexec.DumpLogOnError); err != nil {
		return nil, errors.Wrapf(err, "failed to unzip test data from %v", zipPath)
	}

	files, err := ioutil.ReadDir(tempDir)
	if err != nil {
		return nil, errors.Wrap(err, "failed to read tempDir's contents")
	}

	var testFiles []string
	for _, f := range files {
		tempPath := filepath.Join(tempDir, f.Name())
		downloadsPath := filepath.Join(filesapp.DownloadPath, f.Name())
		testFiles = append(testFiles, downloadsPath)
		if err := fsutil.CopyFile(tempPath, downloadsPath); err != nil {
			return nil, errors.Wrapf(err, "failed to copy test file %v to %v", tempPath, downloadsPath)
		}
		// Sharing may fail depending on file permissions, so set the file permissions to the Download folder's default.
		if err := os.Chmod(downloadsPath, os.FileMode(int(0644))); err != nil {
			return nil, errors.Wrapf(err, "failed to chmod %v", downloadsPath)
		}
	}
	return testFiles, nil
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
func FileHashComparison(ctx context.Context, crosFileDir string, androidDevice *nearbysnippet.AndroidNearbyDevice) error {
	files, err := ioutil.ReadDir(crosFileDir)
	if err != nil {
		return errors.Wrap(err, "failed to read crosFileDir's contents")
	}
	var mismatched []string
	for _, f := range files {
		// Get the hash on the CrOS side.
		crosPath := filepath.Join(crosFileDir, f.Name())
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
		androidHash, err := androidDevice.HashFile(ctx, f.Name())
		if err != nil {
			return errors.Wrapf(err, "failed to get test file's (%v) sha256sum on Android", f.Name())
		}

		if crosHash != androidHash {
			mismatched = append(mismatched, f.Name())
		}
	}

	if len(mismatched) != 0 {
		return errors.Errorf("CrOS and Android hashes did not match for files %v", mismatched)
	}
	return nil
}

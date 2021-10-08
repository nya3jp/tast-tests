// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package files

import (
	"context"
	"fmt"

	"chromiumos/tast/common/hwsec"
	"chromiumos/tast/errors"
)

const (
	// Below is a list of files that we want to test in the user's home directory.
	testfile1_1 = "/home/user/%s/testfile1"
	testfile1_2 = "/home/.shadow/%s/mount/user/testfile1"
	testfile1_3 = "/home/chronos/u-%s/testfile1"
	testfile2_1 = "/home/user/%s/Downloads/testfile2"
	testfile2_2 = "/home/.shadow/%s/mount/user/MyFiles/Downloads/testfile2"
	testfile3_1 = "/home/chronos/u-%s/MyFiles/Downloads/testfile3"
	testfile3_2 = "/home/.shadow/%s/mount/user/MyFiles/Downloads/testfile3"
	testfile4_1 = "/run/daemon-store/chaps/%s/testfile4"
	testfile4_2 = "/home/.shadow/%s/mount/root/chaps/testfile4"
	testfile4_3 = "/home/root/%s/chaps/testfile4"
	testfile5_1 = "/run/daemon-store/u2f/%s/testfile5"
	testfile5_2 = "/home/.shadow/%s/mount/root/u2f/testfile5"
	testfile5_3 = "/home/root/%s/u2f/testfile5"
)

// HomedirFiles stores the test file related to a user's home directory.
type HomedirFiles struct {
	// username is the username of the user whose home directory we're test.
	username string

	// fileInfos is an array of FileInfo struct, each is a test file.
	fileInfos []*FileInfo

	// testFiles is an array of array of paths. Each secondary level array refers to
	// all paths that refers to the same file. Each file can have different path because
	// of bind mount.
	testFiles [][]string
}

// getTestFiles returns the array of array of test file paths, given the user's
// obfuscated username.
// Each secondary level array represents the same file that is bind mount to
// different locations.
func getTestFiles(sanitizedUsername string) [][]string {
	t := [][]string{
		[]string{testfile1_1, testfile1_2, testfile1_3},
		[]string{testfile2_1, testfile2_2},
		[]string{testfile3_1, testfile3_2},
		[]string{testfile4_1, testfile4_2, testfile4_3},
		[]string{testfile5_1, testfile5_2, testfile5_3},
	}
	var result [][]string
	for _, a := range t {
		var currentSet []string
		for _, s := range a {
			currentSet = append(currentSet, fmt.Sprintf(s, sanitizedUsername))
		}
		result = append(result, currentSet)
	}
	return result
}

// NewHomedirFiles creates a new HomedirFiles for testing the files in the
// given user's home directory.
// Note that calling this method only initializes the data structures and doesn't
// touch anything on disk. To reset the on disk state, call Clear().
// Thus, this can be called before the home is mounted.
func NewHomedirFiles(ctx context.Context, util *hwsec.CryptohomeClient, runner hwsec.CmdRunner, username string) (*HomedirFiles, error) {
	sanitizedUsername, err := util.GetSanitizedUsername(ctx, username, true /* useDBus */)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to get sanitized username for %q", username)
	}

	f := getTestFiles(sanitizedUsername)
	var fileInfos []*FileInfo
	for _, p := range f {
		fi, err := NewFileInfo(ctx, p[0], runner)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to initialize FileInfo for %q", p[0])
		}
		fileInfos = append(fileInfos, fi)
	}

	return &HomedirFiles{
		username:  username,
		fileInfos: fileInfos,
		testFiles: f,
	}, nil
}

// Clear resets the files and their corresponding states in data structure.
func (h *HomedirFiles) Clear(ctx context.Context) error {
	for _, f := range h.fileInfos {
		if err := f.Clear(ctx); err != nil {
			return errors.Wrapf(err, "failed to Clear() FileInfo %q", f.Path())
		}
	}
	return nil
}

// Step appends data to all test files in the home directory.
// It'll only use the first path for each file.
func (h *HomedirFiles) Step(ctx context.Context) error {
	for _, f := range h.fileInfos {
		if err := f.Step(ctx); err != nil {
			return errors.Wrapf(err, "failed to Step() FileInfo %q", f.Path())
		}
	}
	return nil
}

// Verify verifies all test files in the user's home directory.
// It'll only verify through the first path for each file.
func (h *HomedirFiles) Verify(ctx context.Context) error {
	for _, f := range h.fileInfos {
		if err := f.Verify(ctx); err != nil {
			return errors.Wrapf(err, "failed to Verify() FileInfo %q", f.Path())
		}
	}
	return nil
}

// StepAll appends to data to all test files in the home directory.
// However, unlike Step(), it'll use all available paths to the same file and
// Step once for each available paths.
func (h *HomedirFiles) StepAll(ctx context.Context) error {
	for idx, f := range h.fileInfos {
		for _, p := range h.testFiles[idx] {
			if err := f.StepOverridePath(ctx, p); err != nil {
				return errors.Wrapf(err, "failed to StepOverridePath() FileInfo %q with path %q", f.Path(), p)
			}
		}
	}
	return nil
}

// VerifyAll verifies all test files in the user's home directory.
// Unlike Verify(), it'll use all available paths to the same file and verifies
// all of them are correct.
func (h *HomedirFiles) VerifyAll(ctx context.Context) error {
	for idx, f := range h.fileInfos {
		for _, p := range h.testFiles[idx] {
			if err := f.VerifyOverridePath(ctx, p); err != nil {
				return errors.Wrapf(err, "failed to VerifyOverridePath() FileInfo %q through %q", f.Path(), p)
			}
		}
	}
	return nil
}

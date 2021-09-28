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
	testfile1 = "/home/user/%s/testfile1"
	testfile2 = "/home/user/%s/Downloads/testfile2"
	testfile3 = "/home/chronos/u-%s/MyFiles/Downloads/testfile3"
	testfile4 = "/run/daemon-store/chaps/%s/testfile4"
	testfile5 = "/run/daemon-store/u2f/%s/testfile5"
)

// HomedirFiles stores the test file related to a user's home directory.
type HomedirFiles struct {
	// username is the username of the user whose home directory we're test.
	username string

	// fileInfos is an array of FileInfo struct, each is a test file.
	fileInfos []*FileInfo
}

// getTestFiles returns the array of test file paths, given the user's
// obfuscated username.
func getTestFiles(sanitizedUsername string) []string {
	t := []string{testfile1, testfile2, testfile3, testfile4, testfile5}
	var result []string
	for _, s := range t {
		result = append(result, fmt.Sprintf(s, sanitizedUsername))
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
		fi, err := NewFileInfo(ctx, p, runner)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to initialize FileInfo for %q", p)
		}
		fileInfos = append(fileInfos, fi)
	}

	return &HomedirFiles{
		username:  username,
		fileInfos: fileInfos,
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
func (h *HomedirFiles) Step(ctx context.Context) error {
	for _, f := range h.fileInfos {
		if err := f.Step(ctx); err != nil {
			return errors.Wrapf(err, "failed to Step() FileInfo %q", f.Path())
		}
	}
	return nil
}

// Verify verifies all test files in the user's home directory.
func (h *HomedirFiles) Verify(ctx context.Context) error {
	for _, f := range h.fileInfos {
		if err := f.Verify(ctx); err != nil {
			return errors.Wrapf(err, "failed to Verify() FileInfo %q", f.Path())
		}
	}
	return nil
}

// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package selinux

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	"chromiumos/tast/testing"

	"github.com/opencontainers/selinux/go-selinux"
)

// FileLabelCheckFilter takes a string of file path, and a os.FileInfo, and
// returns a (bool, error) indicating the files should be skipped or not.
// If the file should be skipped, true bool should be returned, otherwise
// false. Any error during the filter can be returned in error, will cause
// the test to fail.
// fi might be nil if the test failed to Lstat this file, and the filter
// should handle this otherwise the test will fail.
// FileLabelCheckFilter only filters label check. It won't affect the recursive
// logic. For example, if filtering all sub-directory is necessary, path should
// be checked with prefix-check instead of equal.
type FileLabelCheckFilter func(path string, fi os.FileInfo) (skip bool, err error)

// IgnorePath returns a FileLabelCheckFilter which allows the test to skip
// files matching pathToIgnore.
func IgnorePath(pathToIgnore string) FileLabelCheckFilter {
	return func(p string, _ os.FileInfo) (bool, error) {
		return p == pathToIgnore, nil
	}
}

// SkipNonExist is a FileLabelCheckFilter, which takes the path and FileInfo,
// returns true bool if the file doesn't exist, false otherwise. error could
// be returned if other error occurred during Lstat of the file.
func SkipNonExist(p string, fi os.FileInfo) (bool, error) {
	if fi != nil {
		return false, nil
	}
	_, err := os.Lstat(p)
	if os.IsNotExist(err) {
		return true, nil
	}
	return false, err
}

// InvertFilter takes one filter and return a FileLabelCheckFilter which
// reverses the boolean value, and preserves the error.
func InvertFilter(filter FileLabelCheckFilter) FileLabelCheckFilter {
	return func(p string, fi os.FileInfo) (bool, error) {
		ret, err := filter(p, fi)
		return !ret, err
	}
}

// CheckFileContext takes a path and a expected context, and return an error
// if the context mismatch or unable to check context.
func CheckFileContext(path string, expected string) error {
	actual, err := selinux.FileLabel(path)
	if err != nil {
		return fmt.Errorf("Failed to get file context: %v", err)
	}
	if actual != expected {
		return fmt.Errorf(
			"File context mismatch for file %s, got %q; want %q",
			path,
			actual,
			expected)
	}
	return nil
}

// CheckFileContextRecursively checks all files in dir, except files where
// filter returns true, to have selinux label equals to expect.
// If filter is nil, all files are checked without any filter.
// Errors are passed through s.
func CheckFileContextRecursively(s *testing.State, dir string, expect string, filter FileLabelCheckFilter) {
	fis, err := ioutil.ReadDir(dir)
	if err != nil {
		s.Errorf("Failed to list directory %s: %s", dir, err)
		return
	}
	for _, fi := range fis {
		path := filepath.Join(dir, fi.Name())
		skip := false
		if filter != nil {
			if skip, err = filter(path, fi); err != nil {
				s.Errorf("Failed to filter %v: %v", path, err)
			}
		}
		if !skip {
			err := CheckFileContext(path, expect)
			if err != nil {
				s.Errorf("Failed file context check for %v: %v", path, err)
			}
		}
		if fi.IsDir() {
			CheckFileContextRecursively(s, path, expect, filter)
		}
	}
}

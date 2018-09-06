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

// FileLabelCheckFilter returns true if the file described by path
// and fi should be skipped. An error may also be returned, in which
// case a test error will be reported. fi may be nil if the file does
// not exist.
type FileLabelCheckFilter func(path string, fi os.FileInfo) (skip bool, err error)

// IgnorePath returns a FileLabelCheckFilter which allows the test to skip
// files matching pathToIgnore.
func IgnorePath(pathToIgnore string) FileLabelCheckFilter {
	return func(p string, _ os.FileInfo) (bool, error) {
		return p == pathToIgnore, nil
	}
}

// SkipNonExists is a FileLabelCheckFilter that returns true if
// path p doesn't exist. An error is returned if Lstat fails for some
// other reason.
func SkipNonExist(p string, fi os.FileInfo) (bool, error) {
	return fi == nil, nil
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
		return fmt.Errorf("failed to get file context: %v", err)
	}
	if actual != expected {
		return fmt.Errorf(
			"got %q; want %q",
			actual,
			expected)
	}
	return nil
}

// CheckFileContextRecursively checks all files in dir, except files where
// filter returns true, to have selinux label equal to expect.
// If filter is nil, all files are checked without any filter.
// Errors are passed through s.
// This function will be called recursively for every subdirectory
// within dir, even if filter indicates that the subdir itself should
// not be checked.
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
			if err := CheckFileContext(path, expect); err != nil {
				s.Errorf("Failed file context check for %v: %v", path, err)
			}
		}
		if fi.IsDir() {
			CheckFileContextRecursively(s, path, expect, filter)
		}
	}
}

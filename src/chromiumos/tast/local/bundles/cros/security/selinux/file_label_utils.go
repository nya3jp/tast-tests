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
	return func(p string, _ os.FileInfo) (bool, error) { return p == pathToIgnore, nil }
}

// CheckAll returns (false, nil) to let the test to check all files
func CheckAll(_ string, _ os.FileInfo) (bool, error) { return false, nil }

// SkipNonExists is a FileLabelCheckFilter that returns true if
// path p doesn't exist. An error is returned if Lstat fails for some
// other reason.
func SkipNonExist(p string, fi os.FileInfo) (bool, error) { return fi == nil, nil }

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
		return fmt.Errorf("got %q; want %q", actual, expected)
	}
	return nil
}

// CheckFileContext checks path, optionally recursively, except files where
// filter returns true, to have selinux label equal to expected.
// Errors are passed through s.
// If recursive is true. this function will be called recursively for every
// subdirectory within path, even if filter indicates that the subdir itself
// should not be checked.
func CheckContext(s *testing.State, path string, expected string, recursive bool, filter FileLabelCheckFilter) {
	fi, err := os.Lstat(path)
	if err != nil && !os.IsNotExist(err) {
		s.Errorf("Failed to stat %v: %v", path, err)
		return
	}

	skip := false
	if skip, err = filter(path, fi); err != nil {
		s.Errorf("Failed to filter %v: %v", path, err)
	}
	if !skip {
		if err = CheckFileContext(path, expected); err != nil {
			s.Errorf("Failed file context check for %v: %v", path, err)
		}
	}

	if recursive && fi.IsDir() {
		fis, err := ioutil.ReadDir(path)
		if err != nil {
			s.Errorf("Failed to list directory %s: %s", path, err)
			return
		}
		for _, fi := range fis {
			subpath := filepath.Join(path, fi.Name())
			CheckContext(s, subpath, expected, recursive, filter)
		}
	}
}

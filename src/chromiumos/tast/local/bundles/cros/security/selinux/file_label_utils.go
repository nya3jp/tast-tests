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

const Skip bool = true
const Check bool = false

// FileLabelCheckFilter returns true if the file described by path
// and fi should be skipped. fi may be nil if the file does not exist.
type FileLabelCheckFilter func(path string, fi os.FileInfo) (skipFile, skipSubdir bool)

// IgnorePath returns a FileLabelCheckFilter which allows the test to skip
// files matching pathToIgnore, but not its subdirectory.
func IgnorePathItself(pathToIgnore string) FileLabelCheckFilter {
	return func(p string, _ os.FileInfo) (bool, bool) { return p == pathToIgnore, Check }
}

// CheckAll returns (Check, Check) to let the test to check all files
func CheckAll(_ string, _ os.FileInfo) (bool, bool) { return Check, Check }

// SkipNonExists is a FileLabelCheckFilter that returns (Skip, Skip) if
// path p doesn't exist.
func SkipNonExist(p string, fi os.FileInfo) (bool, bool) { return fi == nil, fi == nil }

// InvertFilterSkipFile takes one filter and return a FileLabelCheckFilter which
// reverses the boolean value for skipFile.
func InvertFilterSkipFile(filter FileLabelCheckFilter) FileLabelCheckFilter {
	return func(p string, fi os.FileInfo) (bool, bool) {
		skipFile, skipSubdir := filter(p, fi)
		return !skipFile, skipSubdir
	}
}

// checkFileContext takes a path and a expected context, and return an error
// if the context mismatch or unable to check context.
func checkFileContext(path string, expected string) error {
	actual, err := selinux.FileLabel(path)
	if err != nil {
		return fmt.Errorf("failed to get file context: %v", err)
	}
	if actual != expected {
		return fmt.Errorf("got %q; want %q", actual, expected)
	}
	return nil
}

// CheckContext checks path, optionally recursively, except files where
// filter returns true, to have selinux label equal to expected.
// Errors are passed through s.
// If recursive is true, this function will be called recursively for every
// subdirectory within path, unless the filter indicates the subdir should
// be skipped.
func CheckContext(s *testing.State, path string, expected string, recursive bool, filter FileLabelCheckFilter) {
	fi, err := os.Lstat(path)
	if err != nil && !os.IsNotExist(err) {
		s.Errorf("Failed to stat %v: %v", path, err)
		return
	}

	skipThis, skipSubdir := filter(path, fi)

	if !skipThis {
		if err = checkFileContext(path, expected); err != nil {
			s.Errorf("Failed file context check for %v: %v", path, err)
		}
	}

	if recursive && !skipSubdir && fi.IsDir() {
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

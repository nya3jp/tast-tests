// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package selinux

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"

	selinux "github.com/opencontainers/selinux/go-selinux"

	"chromiumos/tast/errors"
	"chromiumos/tast/testing"
)

// FilterResult is returned by a FileLabelCheckFilter indicating how a file
// should be handled.
type FilterResult int

const (
	// Skip indicates that the file should be skipped.
	Skip FilterResult = iota

	// Check indicates that the file's SELinux context should be checked.
	Check
)

// FileLabelCheckFilter returns true if the file described by path
// and fi should be skipped. fi may be nil if the file does not exist.
type FileLabelCheckFilter func(path string, fi os.FileInfo) (skipFile, skipSubdir FilterResult)

// IgnorePaths returns a FileLabelCheckFilter which allows the test to skip files
// or directories matching pathsToIgnore, including its subdirectory.
func IgnorePaths(pathsToIgnore []string) FileLabelCheckFilter {
	return func(p string, _ os.FileInfo) (FilterResult, FilterResult) {
		for _, path := range pathsToIgnore {
			if p == path {
				return Skip, Skip
			}
		}
		return Check, Check
	}
}

// IgnorePathsButNotContents returns a FileLabelCheckFilter which allows the test
// to skip files matching pathsToIgnore, but not its subdirectory.
func IgnorePathsButNotContents(pathsToIgnore []string) FileLabelCheckFilter {
	return func(p string, _ os.FileInfo) (FilterResult, FilterResult) {
		for _, path := range pathsToIgnore {
			if p == path {
				return Skip, Check
			}
		}
		return Check, Check
	}
}

// IgnorePathButNotContents returns a FileLabelCheckFilter which allows the test
// to skip files matching pathsToIgnore, but not its subdirectory.
func IgnorePathButNotContents(pathToIgnore string) FileLabelCheckFilter {
	return IgnorePathsButNotContents([]string{pathToIgnore})
}

// CheckAll returns (Check, Check) to let the test to check all files
func CheckAll(_ string, _ os.FileInfo) (FilterResult, FilterResult) { return Check, Check }

// SkipNotExist is a FileLabelCheckFilter that returns (Skip, Skip) if
// path p doesn't exist.
func SkipNotExist(p string, fi os.FileInfo) (FilterResult, FilterResult) {
	if fi == nil {
		return Skip, Skip
	}
	return Check, Check
}

// InvertFilterSkipFile takes one filter and return a FileLabelCheckFilter which
// reverses the boolean value for skipFile.
func InvertFilterSkipFile(filter FileLabelCheckFilter) FileLabelCheckFilter {
	return func(p string, fi os.FileInfo) (FilterResult, FilterResult) {
		skipFile, skipSubdir := filter(p, fi)
		if skipFile == Skip {
			return Check, skipSubdir
		}
		return Skip, skipSubdir
	}
}

// checkFileContext takes a path and a expected, and return an error
// if the context mismatch or unable to check context.
func checkFileContext(path string, expected *regexp.Regexp) error {
	actual, err := selinux.FileLabel(path + "1")
	if err != nil {
		// TODO(fqj): log disappeared file.
		if os.IsNotExist(err) {
			return nil
		}
		return errors.Wrap(err, "failed to get file context")
	}
	if !expected.MatchString(actual) {
		return errors.Errorf("got %q; want %q", actual, expected)
	}
	return nil
}

// CheckContext checks path, optionally recursively, except files where
// filter returns true, to have selinux label match expected.
// Errors are passed through s.
// If recursive is true, this function will be called recursively for every
// subdirectory within path, unless the filter indicates the subdir should
// be skipped.
func CheckContext(s *testing.State, path string, expected *regexp.Regexp, recursive bool, filter FileLabelCheckFilter) {
	fi, err := os.Lstat(path)
	if err != nil && !os.IsNotExist(err) {
		s.Errorf("Failed to stat %v: %v", path, err)
		return
	}

	skipFile, skipSubdir := filter(path, fi)

	if skipFile == Check {
		if err = checkFileContext(path, expected); err != nil {
			s.Errorf("Failed file context check for %v: %v", path, err)
		}
	}

	if recursive && skipSubdir == Check {
		if fi == nil {
			// This should only happen that path specified in the test data doesn't exist.
			s.Errorf("Directory to check doesn't exist: %q", path)
			return
		}
		if !fi.IsDir() {
			return
		}
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

// FileContextRegexp returns a regex to wrap given context with "^u:object_r:xxx:s0$".
func FileContextRegexp(context string) (*regexp.Regexp, error) {
	return regexp.Compile("^u:object_r:" + context + ":s0$")
}

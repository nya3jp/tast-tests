// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package selinux

import (
	"io/ioutil"
	"os"
	"path"

	"chromiumos/tast/testing"

	"github.com/opencontainers/selinux/go-selinux"
)

// Common Filters for file label check
type SELinuxFileLabelCheckFilter func(*testing.State, string, os.FileInfo) bool

func IgnorePath(pathToIgnore string) SELinuxFileLabelCheckFilter {
	return func(_ *testing.State, p string, _ os.FileInfo) bool {
		return p == pathToIgnore
	}
}

func SkipNonExist(_ *testing.State, p string, f os.FileInfo) bool {
	if f != nil {
		return false
	}
	_, err := os.Lstat(p)
	if os.IsNotExist(err) {
		return true
	}
	return false
}

func FilterAny(filters ...SELinuxFileLabelCheckFilter) SELinuxFileLabelCheckFilter {
	return func(s *testing.State, p string, f os.FileInfo) bool {
		for _, filter := range filters {
			if filter(s, p, f) {
				return true
			}
		}
		return false
	}
}

func FilterReverse(filter SELinuxFileLabelCheckFilter) SELinuxFileLabelCheckFilter {
	return func(s *testing.State, p string, f os.FileInfo) bool {
		return !filter(s, p, f)
	}
}
func getFileLabel(path string) (string, error) {
	return selinux.FileLabel(path)
}

func AssertSELinuxFileContext(s *testing.State, path string, expected string) {
	actual, err := getFileLabel(path)
	if err != nil {
		s.Errorf("Fail to get file context for %s: %s", path, err)
		return
	}
	if actual != expected {
		s.Errorf(
			"File context mismatch for file %s, expect %q, actual %q",
			path,
			expected,
			actual)
	}

}

func CheckSELinuxFileContextRecursively(s *testing.State, filePath string, expect string, filter SELinuxFileLabelCheckFilter) {
	files, err := ioutil.ReadDir(filePath)
	if err != nil {
		s.Errorf("Fail to list directory %s: %s", filePath, err)
		return
	}
	for _, file := range files {
		subFilePath := path.Join(filePath, file.Name())
		if filter == nil || !filter(s, subFilePath, file) {
			AssertSELinuxFileContext(s, subFilePath, expect)
		}
		if file.IsDir() {
			CheckSELinuxFileContextRecursively(s, subFilePath, expect, filter)
		}
	}
}

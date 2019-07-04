// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package binsetup is used to perform setup before running Chrome video test binaries.
package binsetup

import (
	"io/ioutil"
	"os"
	"path/filepath"

	"chromiumos/tast/fsutil"
	"chromiumos/tast/testing"
)

// CreateTempDataDir creates a world-readable temporary directory using the supplied prefix
// and copies the supplied data file basenames into it. The directory's path is returned.
// A fatal test error is reported using s if an error is encountered.
func CreateTempDataDir(s *testing.State, prefix string, dataFiles []string) string {
	td, err := ioutil.TempDir("", prefix)
	if err != nil {
		s.Fatal("Failed to create temp dir: ", err)
	}
	if err := os.Chmod(td, 0755); err != nil {
		os.RemoveAll(td)
		s.Fatalf("Failed to chmod %v: %v", td, err)
	}

	for _, fn := range dataFiles {
		src := s.DataPath(fn)
		dst := filepath.Join(td, fn)
		if err := fsutil.CopyFile(src, dst); err != nil {
			os.RemoveAll(td)
			s.Fatalf("Failed to copy test file %s to %s: %v", src, dst, err)
		}
		if err := os.Chmod(dst, 0644); err != nil {
			os.RemoveAll(td)
			s.Fatalf("Failed to chmod %v: %v", dst, err)
		}
	}

	return td
}

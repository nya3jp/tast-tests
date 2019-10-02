// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package binsetup is used to perform setup before running Chrome video test binaries.
package binsetup

import (
	"io/ioutil"
	"os"
	"path/filepath"

	"chromiumos/tast/errors"
	"chromiumos/tast/fsutil"
)

// CreateTempDataDir creates a world-readable temporary directory using the supplied prefix
// and copies basenames of the supplied data file into it.
// The directory's path and error are returned.
func CreateTempDataDir(prefix string, srcs []string) (string, error) {
	td, err := ioutil.TempDir("", prefix)
	if err != nil {
		return "", errors.Wrap(err, "failed to create temp dir")
	}
	if err := os.Chmod(td, 0755); err != nil {
		os.RemoveAll(td)
		return "", errors.Wrapf(err, "failed to chmod %v", td)
	}

	for _, src := range srcs {
		dst := filepath.Join(td, filepath.Base(src))
		if err := fsutil.CopyFile(src, dst); err != nil {
			os.RemoveAll(td)
			return "", errors.Wrapf(err, "failed to copy test file %s to %s", src, dst)
		}
		if err := os.Chmod(dst, 0644); err != nil {
			os.RemoveAll(td)
			return "", errors.Wrapf(err, "failed to chmod %v", dst)
		}
	}

	return td, nil
}

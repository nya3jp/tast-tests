// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package cleanupfolder provides funcs to cleanup folders in ChromeOS.
package cleanupfolder

import (
	"io/ioutil"
	"os"
	"path/filepath"

	"chromiumos/tast/errors"
)

// RemoveAllFilesInDirectory removes all files in a directory but leaves the directory itself intact.
func RemoveAllFilesInDirectory(directory string) error {
	files, err := ioutil.ReadDir(directory)
	if err != nil {
		return errors.Wrapf(err, "failed to read files in %s", directory)
	}
	for _, f := range files {
		path := filepath.Join(directory, f.Name())
		if err := os.RemoveAll(path); err != nil {
			return errors.Wrapf(err, "failed to RemoveAll(%q)", path)
		}
	}
	return nil
}

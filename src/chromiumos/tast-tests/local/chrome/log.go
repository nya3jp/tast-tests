// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package chrome

import (
	"os"
	"path/filepath"

	"chromiumos/tast/errors"
)

// CurrentLogFile returns the real path name of the current run of Chrome at that
// time.
func CurrentLogFile() (string, error) {
	const baseLogPath = "/var/log/chrome/chrome"

	// Chrome's base log path is a symbolic link. Readlink to find out the actual
	// filename.
	filename, err := os.Readlink(baseLogPath)
	if err != nil {
		return "", errors.Wrap(err, "failed to read link of chrome log path")
	}
	// Filename is relative to baseLogPath.
	return filepath.Clean(filepath.Join(filepath.Dir(baseLogPath), filename)), nil
}

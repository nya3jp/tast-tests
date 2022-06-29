// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package screenshot

import (
	"os"
	"path/filepath"
	"regexp"

	"chromiumos/tast/errors"
)

// screenshotPaths returns list of screenshot paths in Download folder.
func screenshotPaths(path string) ([]string, error) {
	if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
		// If Download folder does not exist, then there are no screenshots.
		return nil, nil
	}

	re := regexp.MustCompile(`Screenshot.*png`)
	var paths []string

	if err := filepath.Walk(path, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return errors.Wrap(err, "failed to walk through files in Downloads folder")
		}
		if re.FindString(info.Name()) != "" {
			paths = append(paths, path)
		}
		return nil
	}); err != nil {
		return nil, err
	}

	return paths, nil
}

// HasScreenshots returns whether `path` has screenshots.
func HasScreenshots(path string) (bool, error) {
	paths, err := screenshotPaths(path)
	if err != nil {
		return false, err
	}
	return len(paths) > 0, nil
}

// RemoveScreenshots removes screenshots from supplied `path`.
func RemoveScreenshots(path string) error {
	paths, err := screenshotPaths(path)
	if err != nil {
		return errors.Wrap(err, "failed to get list of screenshots")
	}

	for _, path := range paths {
		if err := os.Remove(path); err != nil {
			return errors.Wrapf(err, "failed to remove %q file", path)
		}
	}

	return nil
}

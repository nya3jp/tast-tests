// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package crash

import (
	"io/ioutil"
	"os"

	"chromiumos/tast/errors"
)

// enableCrashFiltering enables crash filtering by writing to the specified file.
func enableCrashFiltering(filterFile, s string) error {
	if err := ioutil.WriteFile(filterFile, []byte(s), 0644); err != nil {
		return errors.Wrapf(err, "failed writing %q to filter in file %s", s, filterFile)
	}
	return nil
}

// EnableCrashFiltering enables crash filtering with the specified process.
func EnableCrashFiltering(s string) error {
	return enableCrashFiltering(FilterInPath, s)
}

// disableCrashFiltering removes the filter_in file using the specified path.
func disableCrashFiltering(filterFile string) error {
	if err := os.Remove(filterFile); err != nil && !os.IsNotExist(err) {
		return errors.Wrapf(err, "failed removing filter in file %s", filterFile)
	}
	return nil
}

// DisableCrashFiltering removes the filter_in file using the default path.
// Next time the crash reporter is invoked, it will not filter crashes.
func DisableCrashFiltering() error {
	return disableCrashFiltering(FilterInPath)
}

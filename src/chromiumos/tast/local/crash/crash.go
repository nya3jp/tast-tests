// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package crash contains utilties common to tests that use crash_reporter and
// crash_sender. TODO(https://crbug.com/995143) Move this to the common local &
// remote repository when supported.
package crash

import (
	"os"
	"path/filepath"

	"chromiumos/tast/errors"
)

const (
	crashTestInProgressDir = "/run/crash_reporter"
	// crashTestInProgressFile is a special control file that tells crash_reporter
	// to act normally during a crash test. Usually, crash_reporter is being told
	// (by /mnt/stateful_partition/etc/collect_chrome_crashes) to be more
	// aggressive about gathering crash data so that we can debug other, non-
	// crash_reporter tests more easily.
	crashTestInProgressFile = "crash-test-in-progress"
)

// StartCrashTest indicates that we are running a test that involves the crash
// reporting system (crash_reporter, crash_sender, or anomaly_detector). The
// test should "defer FinishCrashTest()" after calling this.
func StartCrashTest() error {
	return startCrashTestWithDirectory(crashTestInProgressDir)
}

// startCrashTestWithDirectory is a helper function for StartCrashTest. We need
// this as a separate function for testing.
func startCrashTestWithDirectory(dir string) error {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return errors.Wrapf(err, "could not make directory %v", dir)
	}

	filePath := filepath.Join(dir, crashTestInProgressFile)
	f, err := os.Create(filePath)
	if err != nil {
		if os.IsExist(err) {
			// Leftovers from a previous test; don't abort the current test.
			return nil
		}
		return errors.Wrapf(err, "could not create %v", filePath)
	}
	f.Close()
	return nil
}

// FinishCrashTest undoes the work of StartCrashTest.
func FinishCrashTest() error {
	return finishCrashTestWithDirectory(crashTestInProgressDir)
}

// finishCrashTestWithDirectory is a helper function for FinishCrashTest. We need
// this as a separate function for testing.
func finishCrashTestWithDirectory(dir string) error {
	filePath := filepath.Join(dir, crashTestInProgressFile)
	if err := os.Remove(filePath); err != nil {
		if os.IsNotExist(err) {
			// Something else already removed the file. Well, whatever, we're in the
			// correct state now (the file is gone).
			return nil
		}
		return errors.Wrapf(err, "could not remove %v", filePath)
	}
	return nil
}

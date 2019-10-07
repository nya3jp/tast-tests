// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package crash contains utilties common to tests that use crash_reporter and
// crash_sender.
package crash

import (
	"context"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"chromiumos/tast/crash"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/set"
	"chromiumos/tast/local/syslog"
	"chromiumos/tast/local/upstart"
	"chromiumos/tast/testing"
)

const (
	crashTestInProgressDir = "/run/crash_reporter"
	// crashTestInProgressFile is a special control file that tells crash_reporter
	// to act normally during a crash test. Usually, crash_reporter is being told
	// (by /mnt/stateful_partition/etc/collect_chrome_crashes) to be more
	// aggressive about gathering crash data so that we can debug other, non-
	// crash_reporter tests more easily.
	crashTestInProgressFile = "crash-test-in-progress"
	// SystemCrashDir is the directory where system crash reports go.
	SystemCrashDir = "/var/spool/crash"
)

// RestartAnomalyDetector restarts the anomaly detector and waits for it to open the journal.
// This is useful for tests that need to clear its cache of previously seen hashes
// and ensure that the anomaly detector runs for an artificially-induced crash.
func RestartAnomalyDetector(ctx context.Context) error {
	w, err := syslog.NewWatcher(syslog.MessageFile)
	if err != nil {
		return errors.Wrapf(err, "couldn't create watcher for %s", syslog.MessageFile)
	}
	defer w.Close()

	// Restart anomaly detector to clear its cache of recently seen service
	// failures and ensure this one is logged.
	if err := upstart.RestartJob(ctx, "anomaly-detector"); err != nil {
		return errors.Wrap(err, "upstart couldn't restart anomaly-detector")
	}

	// Wait for anomaly detector to indicate that it's ready. Otherwise, it'll miss the warning.
	if err := w.WaitForMessage(ctx, "Opened journal and sought to end"); err != nil {
		return errors.Wrap(err, "failed to wait for anomaly detector to start")
	}
	return nil
}

// WaitForCrashFiles waits for each regex in regexes to match a file in dir that is not also in oldFiles.
// One might use it by
// 1. Getting a list of already-extant files in dir.
// 2. Doing some operation that will create new files in dir (e.g. inducing a crash).
// 3. Calling this method to wait for the expected files to appear.
// On success, WaitForCrashFiles returns a list of the files that matched the regexes.
func WaitForCrashFiles(ctx context.Context, dir string, oldFiles []string, regexes []string) ([]string, error) {
	var files []string
	err := testing.Poll(ctx, func(c context.Context) error {
		newFiles, err := crash.GetCrashes(dir)
		if err != nil {
			return testing.PollBreak(errors.Wrap(err, "failed to get new crashes"))
		}
		diffFiles := set.DiffStringSlice(newFiles, oldFiles)

		var missing []string
		files = nil
		for _, re := range regexes {
			match := false
			for _, f := range diffFiles {
				match, err = regexp.MatchString(re, f)
				if err != nil {
					return testing.PollBreak(errors.Wrapf(err, "invalid regexp %s", re))
				}
				if match {
					files = append(files, f)
					break
				}
			}
			if !match {
				missing = append(missing, re)
			}
		}
		if len(missing) != 0 {
			return errors.Errorf("no file matched %s (found %s)", strings.Join(missing, ", "), strings.Join(diffFiles, ", "))
		}
		return nil
	}, &testing.PollOptions{Timeout: 15 * time.Second})
	if err != nil {
		return nil, err
	}
	return files, nil
}

// SetUpCrashTest indicates that we are running a test that involves the crash
// reporting system (crash_reporter, crash_sender, or anomaly_detector). The
// test should "defer TearDownCrashTest()" after calling this.
func SetUpCrashTest() error {
	return setUpCrashTestWithDirectory(crashTestInProgressDir)
}

// setUpCrashTestWithDirectory is a helper function for SetUpCrashTest. We need
// this as a separate function for testing.
func setUpCrashTestWithDirectory(dir string) error {
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
	if err = f.Close(); err != nil {
		return errors.Wrap(err, "error during close")
	}
	return nil
}

// TearDownCrashTest undoes the work of SetUpCrashTest.
func TearDownCrashTest() error {
	return tearDownCrashTestWithDirectory(crashTestInProgressDir)
}

// tearDownCrashTestWithDirectory is a helper function for TearDownCrashTest. We need
// this as a separate function for testing.
func tearDownCrashTestWithDirectory(dir string) error {
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

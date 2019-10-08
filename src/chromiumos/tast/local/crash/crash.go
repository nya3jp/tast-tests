// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package crash contains utilties common to tests that use crash_reporter and
// crash_sender.
package crash

import (
	"context"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"chromiumos/tast/crash"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/set"
	"chromiumos/tast/local/syslog"
	"chromiumos/tast/local/sysutil"
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
	// systemCrashStash is a directory to stash pre-existing system crashes during crash tests.
	systemCrashStash = "/var/spool/crash.real"
	// LocalCrashDir is the directory where user crash reports go.
	LocalCrashDir = "/home/chronos/crash"
	// localCrashStash is a directory to stash pre-existing user crashes during crash tests.
	localCrashStash = "/home/chronos/crash.real"
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

// moveAllCrashesTo moves crashes from |source| to |target|. This allows us to
// start crash tests with an empty spool directory, reducing risk of flakes if
// the dir is already full when the test starts.
func moveAllCrashesTo(source, target string) error {
	if err := os.MkdirAll(target, 0755); err != nil {
		return errors.Wrapf(err, "couldn't make stash crash dir %s", target)
	}
	files, err := ioutil.ReadDir(source)
	if err != nil {
		// Bubble this up so caller can check whether IsNotExist and behave accordingly.
		return err
	}
	for _, f := range files {
		if err := os.Rename(filepath.Join(source, f.Name()), filepath.Join(target, f.Name())); err != nil {
			return errors.Wrapf(err, "couldn't move file: %v", f.Name())
		}
	}
	return nil
}

// SetUpCrashTest indicates that we are running a test that involves the crash
// reporting system (crash_reporter, crash_sender, or anomaly_detector). The
// test should "defer TearDownCrashTest()" after calling this.
func SetUpCrashTest() error {
	return setUpCrashTestWithDirectories(crashTestInProgressDir, SystemCrashDir, systemCrashStash,
		LocalCrashDir, localCrashStash)
}

// setUpCrashTestWithDirectories is a helper function for SetUpCrashTest. We need
// this as a separate function for testing.
func setUpCrashTestWithDirectories(inProgDir, sysCrashDir, sysCrashStash, userCrashDir, userCrashStash string) (retErr error) {
	// Move all crashes into stash directory so a full directory won't stop
	// us from saving a new crash report
	if err := moveAllCrashesTo(sysCrashDir, sysCrashStash); err != nil && !os.IsNotExist(err) {
		return err
	}
	defer func() {
		if retErr != nil {
			cleanUpStashDir(sysCrashStash, sysCrashDir)
		}
	}()

	if err := moveAllCrashesTo(userCrashDir, userCrashStash); err != nil && !os.IsNotExist(err) {
		return err
	}
	defer func() {
		if retErr != nil {
			cleanUpStashDir(userCrashStash, userCrashDir)
		}
	}()

	if err := os.MkdirAll(inProgDir, 0755); err != nil {
		return errors.Wrapf(err, "could not make directory %v", inProgDir)
	}

	filePath := filepath.Join(inProgDir, crashTestInProgressFile)
	if err := ioutil.WriteFile(filePath, nil, 0644); err != nil {
		return errors.Wrapf(err, "could not create %v", filePath)
	}
	return nil
}

func cleanUpStashDir(stashDir, realDir string) error {
	// Stash dir should exist, so error if it doesn't.
	if err := moveAllCrashesTo(stashDir, realDir); err != nil {
		return err
	}
	if err := os.Remove(stashDir); err != nil {
		if !os.IsNotExist(err) {
			return errors.Wrapf(err, "couldn't remove stash dir: %v", stashDir)
		}
	}
	return nil
}

// TearDownCrashTest undoes the work of SetUpCrashTest.
func TearDownCrashTest() error {
	if err := tearDownCrashTestWithDirectories(crashTestInProgressDir, SystemCrashDir, systemCrashStash,
		LocalCrashDir, localCrashStash); err != nil {
		return err
	}
	// The user crash directory should always be owned by chronos not root. The
	// unit tests don't run as root and can't chown, so skip this in tests.
	if err := os.Chown(LocalCrashDir, int(sysutil.ChronosUID), int(sysutil.ChronosGID)); err != nil {
		return errors.Wrapf(err, "couldn't chown %s", LocalCrashDir)
	}
	return nil
}

// tearDownCrashTestWithDirectories is a helper function for TearDownCrashTest. We need
// this as a separate function for testing.
func tearDownCrashTestWithDirectories(inProgDir, sysCrashDir, sysCrashStash, userCrashDir, userCrashStash string) error {
	var firstErr error
	if err := cleanUpStashDir(sysCrashStash, sysCrashDir); err != nil {
		firstErr = err
	}
	if err := cleanUpStashDir(userCrashStash, userCrashDir); err != nil && firstErr == nil {
		firstErr = err
	}

	filePath := filepath.Join(inProgDir, crashTestInProgressFile)
	if err := os.Remove(filePath); err != nil && firstErr == nil {
		if os.IsNotExist(err) {
			// Something else already removed the file. Well, whatever, we're in the
			// correct state now (the file is gone).
			return nil
		}
		return errors.Wrapf(err, "could not remove %v", filePath)
	}
	return firstErr
}

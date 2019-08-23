// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package crash

import (
	"context"
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

// SystemCrashDir is the directory where system crash reports go.
const SystemCrashDir = "/var/spool/crash"

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

// WaitForFiles waits for each regex in |regexes| to match a file in |dir| that is not also in |oldFiles|.
// One might use it by
// 1. Getting a list of already-extant files in |dir|.
// 2. Doing some operation that will create new files in |dir| (e.g. inducing a crash).
// 3. Calling this method to wait for the expected files to appear.
// On success, WaitForFiles returns a list of the files that matched the regexes.
func WaitForFiles(ctx context.Context, dir string, oldFiles []string, regexes []string) ([]string, error) {
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
					return testing.PollBreak(errors.Errorf("invalid regexp %s (err: %v)", re, err))
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

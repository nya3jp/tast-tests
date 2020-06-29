// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package crash

import (
	"context"
	"io/ioutil"
	"os"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/testing"
)

// enableCrashFiltering enables crash filtering by writing to the specified file
// and waiting for crash_reporter to finish.
func enableCrashFiltering(ctx context.Context, filterFile, filter string) error {
	if err := ioutil.WriteFile(filterFile, []byte(filter), 0644); err != nil {
		return errors.Wrapf(err, "failed writing %q to filter in file %s", filter, filterFile)
	}

	// Wait for crash_reporter to stop running so that after this method
	// returns, we can guarantee that any newly-written crashes match the
	// filter. (If we did not wait, we'd have a race: a crash_reporter
	// instance that began execution before this file was written wouldn't
	// use it.)
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		running, err := processRunning("crash_reporter")
		if err != nil {
			return testing.PollBreak(err)
		}
		if running {
			return errors.New("crash_reporter is still running")
		}
		return nil
	}, &testing.PollOptions{Timeout: 10 * time.Second}); err != nil {
		return errors.Wrap(err, "failed to wait for crash_reporter to finish")
	}

	return nil
}

// EnableCrashFiltering enables crash filtering with the specified command-line
// filter..
func EnableCrashFiltering(ctx context.Context, filter string) error {
	return enableCrashFiltering(ctx, FilterInPath, filter)
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

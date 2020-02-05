// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package crash

import (
	"context"
	"os"
	"path/filepath"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/upstart"
	"chromiumos/tast/testing"
)

// SetUpAnomalyDetector restarts the anomaly detector and waits for it to open the journal.
// This is useful for tests that need to clear its cache of previously seen hashes
// and ensure that the anomaly detector runs for an artificially-induced crash.
func SetUpAnomalyDetector(ctx context.Context) error {
	return SetUpAnomalyDetectorWithSendAll(ctx, false)
}

// SetUpAnomalyDetectorWithSendAll restarts anomaly detector, setting the
// "--testonly-send-all" flag to the value specified by sendAll.
func SetUpAnomalyDetectorWithSendAll(ctx context.Context, sendAll bool) error {
	return RestartAnomalyDetectorWithArgs(ctx, sendAll, true)
}

// TearDownAnomalyDetector restarts anomaly detector, resetting args back to normal.
func TearDownAnomalyDetector(ctx context.Context) error {
	return RestartAnomalyDetectorWithArgs(ctx, false, false)
}

func restartAnomalyDetectorWithArgs(ctx context.Context, sendAll, ignoreConsent bool) error {
	if err := upstart.StopJob(ctx, "anomaly-detector"); err != nil {
		return errors.Wrap(err, "upstart couldn't stop anomaly-detector")
	}

	// Delete the "ready" file so we can easily tell when it is ready.
	if err := os.Remove(filepath.Join(crashTestInProgressDir, anomalyDetectorReadyFile)); err != nil {
		if !os.IsNotExist(err) {
			return errors.Wrap(err, "couldn't remove anomalyDetectorReadyFile")
		}
		// Otherwise, we're good - the file already doesn't exist.
	}

	// And now start it...
	extraFlags := "EXTRA_FLAGS='"
	if sendAll {
		extraFlags += "--testonly_send_all"
	}
	if ignoreConsent {
		extraFlags += " --testonly_ignore_consent"
	}
	extraFlags += "'"

	if err := upstart.StartJob(ctx, "anomaly-detector", extraFlags); err != nil {
		return errors.Wrap(err, "upstart couldn't start anomaly-detector")
	}

	// and wait for it to indicate that it's ready. Otherwise, it'll miss the anomaly the test creates.
	err := testing.Poll(ctx, func(ctx context.Context) error {
		if _, err := os.Stat(filepath.Join(crashTestInProgressDir, anomalyDetectorReadyFile)); os.IsNotExist(err) {
			return err
		} else if err != nil {
			return testing.PollBreak(errors.Wrap(err, "failed to stat"))
		}
		return nil

	}, &testing.PollOptions{Timeout: 15 * time.Second})
	if err != nil {
		return errors.Wrap(err, "failed to wait for anomaly detector to start")
	}
	return nil
}

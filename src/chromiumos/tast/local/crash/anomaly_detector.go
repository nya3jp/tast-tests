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

// RestartAnomalyDetector restarts the anomaly detector and waits for it to open the journal.
// This is useful for tests that need to clear its cache of previously seen hashes
// and ensure that the anomaly detector runs for an artificially-induced crash.
func RestartAnomalyDetector(ctx context.Context) error {
	return RestartAnomalyDetectorWithSendAll(ctx, false)
}

// RestartAnomalyDetectorWithSendAll restarts anomaly detector, setting the
// "--testonly-send-all" flag to the value specified by sendAll.
func RestartAnomalyDetectorWithSendAll(ctx context.Context, sendAll bool) error {
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
	var err error
	if sendAll {
		err = upstart.StartJob(ctx, "anomaly-detector", upstart.WithArg("TESTONLY_SEND_ALL", "--testonly_send_all"))
	} else {
		err = upstart.StartJob(ctx, "anomaly-detector")
	}
	if err != nil {
		return errors.Wrap(err, "upstart couldn't start anomaly-detector")
	}

	// and wait for it to indicate that it's ready. Otherwise, it'll miss the anomaly the test creates.
	err = testing.Poll(ctx, func(ctx context.Context) error {
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

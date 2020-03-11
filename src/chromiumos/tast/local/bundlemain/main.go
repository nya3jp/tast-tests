// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package bundlemain provides a main function implementation for a bundle
// to share it from various local bundle executables.
// The most of the frame implementation is in chromiumos/tast/bundle package,
// but some utilities, which lives in support libraries for maintenance,
// need to be injected.
package bundlemain

import (
	"context"
	"io"
	"os"
	"path/filepath"

	"chromiumos/tast/bundle"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/faillog"
	"chromiumos/tast/local/ready"
	"chromiumos/tast/testing"
)

const varLogMessages = "/var/log/messages"

func copyLogs(ctx context.Context, oldInfo os.FileInfo, outDir string) error {
	dp := filepath.Join(outDir, "messages")

	df, err := os.Create(dp)
	if err != nil {
		return errors.Wrapf(err, "failed to write log: failed to create %s", dp)
	}
	defer df.Close()

	sf, err := os.Open(varLogMessages)
	if err != nil {
		return errors.Wrapf(err, "failed to read log: failed to open %s", varLogMessages)
	}
	defer sf.Close()

	info, err := sf.Stat()
	if err != nil {
		return errors.Wrapf(err, "failed reading log position: failed to stat %s", varLogMessages)
	}

	if os.SameFile(info, oldInfo) {
		// If the file has not rotated just copy everything since the test started.
		if _, err = sf.Seek(oldInfo.Size(), 0); err != nil {
			return errors.Wrapf(err, "failed to read log: failed to seek %s", varLogMessages)
		}

		if _, err = io.Copy(df, sf); err != nil {
			return errors.Wrapf(err, "failed to write log: failed to copy %s", varLogMessages)
		}
	} else {
		// If the log has rotated copy the old file from where the test started and then copy the entire new file.
		// We assume that the log does not rotate twice during one test.
		// If we fail to open the older log, we still copy the newer one.
		previousLog := varLogMessages + ".1"

		sfp, err := os.Open(previousLog)
		if err != nil {
			_, _ = io.Copy(df, sf)

			return errors.Wrapf(err, "failed to read log: failed to open %s", previousLog)
		}
		defer sfp.Close()

		if _, err = sfp.Seek(oldInfo.Size(), 0); err != nil {
			_, _ = io.Copy(df, sf)

			return errors.Wrapf(err, "failed to read log: failed to seek %s", previousLog)
		}

		// Copy previous log
		if _, err = io.Copy(df, sfp); err != nil {
			_, _ = io.Copy(df, sf)

			return errors.Wrapf(err, "failed to write log: failed to copy previous %s", previousLog)
		}

		// Copy current log
		if _, err = io.Copy(df, sf); err != nil {
			return errors.Wrapf(err, "failed to write log: failed to copy current %s", previousLog)
		}
	}

	return nil
}

func preTestRun(ctx context.Context, s *testing.State) func(ctx context.Context, s *testing.State) {
	// Store the current log state
	oldInfo, err := os.Stat(varLogMessages)
	if err != nil {
		s.Logf("Saving log position: failed to stat %s: %v", varLogMessages, err)

		// Call faillog even if saving the log position failed
		return func(ctx context.Context, s *testing.State) {
			if s.HasError() {
				faillog.Save(ctx)
			}
		}
	}

	return func(ctx context.Context, s *testing.State) {
		if s.HasError() {
			faillog.Save(ctx)
		}

		if err := copyLogs(ctx, oldInfo, s.OutDir()); err != nil {
			s.Log("Failed to copy logs: ", err)
			return
		}
	}
}

// Main is an entry point function for bundles.
func Main() {
	os.Exit(bundle.LocalDefault(bundle.LocalDelegate{
		Ready:      ready.Wait,
		PreTestRun: preTestRun,
	}))
}

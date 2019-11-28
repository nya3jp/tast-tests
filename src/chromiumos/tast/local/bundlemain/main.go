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
	"chromiumos/tast/local/faillog"
	"chromiumos/tast/local/ready"
	"chromiumos/tast/testing"
)

const varLogMessages = "/var/log/messages"

func preTestRun(ctx context.Context, s *testing.State) func(ctx context.Context, s *testing.State) {
	// Store the current log state
	oldInfo, err := os.Stat(varLogMessages)
	if err != nil {
		s.Logf("Saving log position: Failed to stat %s: %v", varLogMessages, err)

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

		info, err := os.Stat(varLogMessages)
		if err != nil {
			s.Logf("Reading log position: Failed to stat %s: %v", varLogMessages, err)
			return
		}

		sf, err := os.Open(varLogMessages)
		if err != nil {
			s.Logf("Failed to read log: Failed to open %s: %v", varLogMessages, err)
			return
		}
		defer sf.Close()

		dp := filepath.Join(s.OutDir(), varLogMessages)
		if err = os.MkdirAll(filepath.Dir(dp), 0755); err != nil {
			s.Logf("Failed to write log: Failed to create %s dir: %v", dp, err)
			return
		}

		df, err := os.Create(dp)
		if err != nil {
			s.Logf("Failed to write log: Failed to create %s: %v", dp, err)
			return
		}
		defer df.Close()

		if os.SameFile(info, oldInfo) {
			// If the file has not rotated just copy everything since the test started.
			if _, err = sf.Seek(oldInfo.Size(), 0); err != nil {
				s.Logf("Failed to read log: Failed to seek %s: %v", varLogMessages, err)
				return
			}
		} else {
			// If the log has rotated copy the old file from where the test started and then copy the entire new file.
			// We assume that the log does not rotate twice during one test.
			// If we fail to open the older log, we still copy the newer one.
			sfp, err := os.Open(varLogMessages + ".1")
			if err != nil {
				s.Logf("Failed to read log: Failed to open %s: %v", varLogMessages+".1", err)
				goto copyLog
			}
			defer sfp.Close()

			if _, err = sfp.Seek(oldInfo.Size(), 0); err != nil {
				s.Logf("Failed to read log: Failed to seek %s: %v", varLogMessages, err)
				goto copyLog
			}

			if _, err = io.Copy(df, sfp); err != nil {
				s.Logf("Failed to write log: Failed to copy %s: %v", varLogMessages, err)
				goto copyLog
			}
		}

	copyLog:
		if _, err = io.Copy(df, sf); err != nil {
			s.Logf("Failed to write log: Failed to copy %s: %v", varLogMessages, err)
			return
		}
	}
}

// Main is an entry point function for bundles.
func Main() {
	os.Exit(bundle.Local(os.Args[1:], os.Stdin, os.Stdout, os.Stderr, bundle.LocalDelegate{
		Ready:      ready.Wait,
		PreTestRun: preTestRun,
	}))
}

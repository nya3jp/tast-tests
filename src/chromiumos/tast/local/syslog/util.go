// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package syslog

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"regexp"

	"chromiumos/tast/errors"
)

// CollectSyslog collects shards of system log between timing of calling this
// function and each call to the returned function.
func CollectSyslog() (func(context.Context, string) error, error) {
	// Store the current log state.
	oldInfo, err := os.Stat(MessageFile)
	if err != nil {
		return nil, errors.Wrapf(err, "filed to stat %s: %v", MessageFile, err)
	}
	return func(ctx context.Context, outDir string) error {
		dp := filepath.Join(outDir, filepath.Base(MessageFile))

		df, err := os.Create(dp)
		if err != nil {
			return errors.Wrapf(err, "failed to write log: failed to create %s", dp)
		}
		defer df.Close()

		sf, err := os.Open(MessageFile)
		if err != nil {
			return errors.Wrapf(err, "failed to read log: failed to open %s", MessageFile)
		}
		defer sf.Close()

		info, err := sf.Stat()
		if err != nil {
			return errors.Wrapf(err, "failed reading log position: failed to stat %s", MessageFile)
		}

		if os.SameFile(info, oldInfo) {
			// If the file has not rotated just copy everything since the test started.
			if _, err = sf.Seek(oldInfo.Size(), 0); err != nil {
				return errors.Wrapf(err, "failed to read log: failed to seek %s", MessageFile)
			}

			if _, err = io.Copy(df, sf); err != nil {
				return errors.Wrapf(err, "failed to write log: failed to copy %s", MessageFile)
			}
		} else {
			// If the log has rotated copy the old file from where the test started and then copy the entire new file.
			// We assume that the log does not rotate twice during one test.
			// If we fail to open the older log, we still copy the newer one.
			previousLog := MessageFile + ".1"

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

			// Copy previous log.
			if _, err = io.Copy(df, sfp); err != nil {
				_, _ = io.Copy(df, sf)

				return errors.Wrapf(err, "failed to write log: failed to copy previous %s", previousLog)
			}

			// Copy current log.
			if _, err = io.Copy(df, sf); err != nil {
				return errors.Wrapf(err, "failed to write log: failed to copy current %s", previousLog)
			}
		}

		return nil
	}, nil
}

// ExtractFileName extracts source file name from Entry.
// If there are multiple file names, it extracts the last one.
func ExtractFileName(entry Entry) string {
	r := regexp.MustCompile(`^.*!?\[(?P<filename>\S+)\([-]?\d+\)\].*$`)
	m := r.FindStringSubmatch(entry.Content)
	if len(m) < 2 {
		return ""
	}
	return m[1]
}

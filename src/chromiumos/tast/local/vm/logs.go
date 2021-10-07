// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package vm

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/cryptohome"
	"chromiumos/tast/local/syslog"
	"chromiumos/tast/testing"
)

// LogReader keeps a persistent view of the log files created by a VM, and
// can be used to save them to an output directory for tast.
type LogReader struct {
	vmName  string
	ownerID string
	reader  *syslog.LineReader
}

// NewLogReaderForVM creates a new LogReader which can be used to save the
// daemon-store logs from a running VM.
func NewLogReaderForVM(ctx context.Context, vmName, user string) (*LogReader, error) {
	ownerID, err := cryptohome.UserHash(ctx, user)
	if err != nil {
		return nil, err
	}
	path := "/run/daemon-store/crosvm/" + ownerID + "/log/" + GetEncodedName(vmName) + ".log"

	// Only wait 1 second for the log file to exist, don't want to hang until
	// timeout if it doesn't exist, instead we continue.
	reader, err := syslog.NewLineReader(ctx, path, false,
		&testing.PollOptions{Timeout: 1 * time.Second})
	if err != nil {
		return nil, err
	}
	return &LogReader{vmName, ownerID, reader}, nil
}

// Close closes the underlying syslog.LineReader
func (r *LogReader) Close() error {
	return r.reader.Close()
}

// TrySaveLogs attempts to save the VM logs to the given directory. The logs
// will be saved in a file called "<vm name>_logs.txt".
func (r *LogReader) TrySaveLogs(ctx context.Context, dir string) error {
	path := filepath.Join(dir, r.vmName+"_logs.txt")
	f, err := os.Create(path)
	if err != nil {
		return errors.Wrapf(err, "failed creating log file at %v", path)
	}
	defer f.Close()

	for {
		line, err := r.reader.ReadLine()
		if err != nil {
			if err != io.EOF {
				return errors.Wrapf(err, "failed to read file %v", path)
			}
			break
		}
		_, err = f.WriteString(line)
		if err != nil {
			return errors.Wrapf(err, "failed to write %q to file %v", line, path)
		}
	}
	return nil
}

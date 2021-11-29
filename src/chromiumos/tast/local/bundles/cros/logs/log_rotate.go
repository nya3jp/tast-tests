// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package logs

import (
	"bytes"
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"time"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         LogRotate,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Tests to run log_rotator",
		Contacts:     []string{"yoshiki@chromium.org"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
	})
}

func LogRotate(ctx context.Context, s *testing.State) {
	const (
		logRotatorExecutable = "/usr/sbin/log_rotator"
		// Days we would keep the old file.
		// Log file is rotated daily, so that "7" means 7 log files are kept in maximum.
		// This value must match with "DAYS_TO_PRESERVE_LOGS" in //croslog/log_rotate/log_rotate.cc
		daysToPreserveLogs = 7
	)

	// Prepares log files to rotate.
	err := createLogFiles(s, daysToPreserveLogs)
	if err != nil {
		s.Error("Creating log files failed: ", err)
	}

	// Removes the old log file which should not exist.
	tooOldPath := getLogFilePathWithIndex(daysToPreserveLogs + 1)
	if _, err := os.Stat(tooOldPath); !os.IsNotExist(err) {
		err = os.Remove(tooOldPath)
		if err != nil {
			s.Errorf("Removing file failed %q: %v", tooOldPath, err)
		}
	}

	// Calculates the hashes for the files to check the identity of files
	var hashes [daysToPreserveLogs][]byte
	for i := 0; i < daysToPreserveLogs; i++ {
		path := getLogFilePathWithIndex(i)
		hashes[i], _ = calculateHashOfFile(path)
	}

	// Runs log_rotater command.
	s.Log("Running log_rotator")
	out, err := testexec.CommandContext(ctx, logRotatorExecutable).Output()
	if err != nil {
		s.Errorf("Executing log_rotator failed with output %q: %v", out, err)
	}

	basePath := getLogFilePathWithIndex(0)

	// Ensures the base log file is newly created by the rotation.
	fileinfo, err := os.Stat(basePath)
	if err != nil {
		if os.IsNotExist(err) {
			s.Errorf("A new log file doesn't exit %q", basePath)
		} else {
			s.Errorf("Stat of the new base file failed %q: %v", basePath, err)
		}
	}
	if fileinfo.Size() != 0 {
		s.Errorf("A new log file should be empty just after the rotation %q", basePath)
	}

	// Compares the hashes to check if the rotation is done correctly.
	for i := 1; i <= daysToPreserveLogs; i++ {
		newPath := getLogFilePathWithIndex(i)

		newHash, err := calculateHashOfFile(newPath)
		if err != nil {
			s.Errorf("Rotation failed. The calculating hash of the log file %q failed: %v", newPath, err)
		}
		if !bytes.Equal(hashes[i-1], newHash) {
			s.Errorf("Rotation failed. The hash doesn't match (new path: %q)", newPath)
		}

		// Cleans up the log file.
		err = os.Remove(newPath)
		if err != nil {
			s.Errorf("Removing file failed %q: %v", newPath, err)
		}
	}

	// Ensures the too-old log is not created, since it's not within |daysToPreserveLogs|.
	if _, err := os.Stat(tooOldPath); !os.IsNotExist(err) {
		s.Errorf("The log file should not exist %q: %v", tooOldPath, err)
	}
}

func calculateHashOfFile(path string) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return []byte{}, errors.Wrap(err, "failed to open log files")
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return []byte{}, errors.Wrap(err, "failed to calculate the hash of the log file")
	}

	return h.Sum(nil), nil
}

func createLogsIfNotExist(path string, s *testing.State) error {
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		return nil
	}

	s.Logf("Creating a log file %q", path)
	contents := "THIS FILE IS FOR TESTING " + time.Now().Format("2006-01-02 15:04:05.000000000")

	err := ioutil.WriteFile(path, []byte(contents), 0644)
	if err != nil {
		return errors.Wrapf(err, "failed to create log files %q", path)
	}
	return nil
}

// createLogFiles prepares (daysToPreserveLogs + 1) files.
func createLogFiles(s *testing.State, daysToPreserveLogs int) error {
	for i := 0; i <= daysToPreserveLogs; i++ {
		path := getLogFilePathWithIndex(i)
		err := createLogsIfNotExist(path, s)
		if err != nil {
			return errors.Wrapf(err, "failed to create a log file %q", path)
		}
	}
	return nil
}

// getLogFilePathWithIndex returns log file path with the specified index.
func getLogFilePathWithIndex(index int) string {
	if index == 0 {
		return "/var/log/temporary_log_file_for_testing.log"
	}

	return fmt.Sprintf("/var/log/temporary_log_file_for_testing.%d.log", index)
}

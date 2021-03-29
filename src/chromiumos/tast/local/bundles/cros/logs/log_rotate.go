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

	"chromiumos/tast/errors"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         LogRotate,
		Desc:         "Tests related to bootid-logger",
		Contacts:     []string{"yoshiki@chromium.org"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
	})
}

func LogRotate(ctx context.Context, s *testing.State) {
	const (
		bootidLoggerExecutable = "/usr/sbin/log_rotator"
	)

	// Prepares log files to rotate.
	err := createLogFiles(s)
	if err != nil {
		s.Error("Creating log files failed: ", err)
	}

	// Calculates the hashes for the files to check the identity of files
	var hashes [7][]byte
	for i := 0; i < 7; i++ {
		path := getLogFilePathWithIndex(i)
		hashes[i], _ = calculateHashOfFile(path)
	}

	// Runs log_rotater command.
	s.Log("Running log_rotator")
	out, err := testexec.CommandContext(ctx, bootidLoggerExecutable).Output()
	if err != nil {
		s.Errorf("Executing log_rotator failed with output %q: %v", out, err)
	}

	// Ensures the base log file is gone by the rotation.
	basePath := getLogFilePathWithIndex(0)
	if _, err := os.Stat(basePath); !os.IsNotExist(err) {
		s.Errorf("The log file should not exist %q: %v", basePath, err)
	}

	// Compares the hashes to check if the rotation is done correctly.
	for i := 1; i < 7; i++ {
		newPath := getLogFilePathWithIndex(i)

		newHash, _ := calculateHashOfFile(newPath)
		if !bytes.Equal(hashes[i-1], newHash) {
			s.Errorf("Rotation failed. The hash doesn't match (new path: %q): %v", newPath, err)
		}

		// Cleans up the log file.
		err := os.Remove(newPath)
		if err != nil {
			s.Errorf("Removing file failed %q: %v", newPath, err)
		}
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
	str := "THIS FILE IS FOR TESTING " + time.Now().Format("2006-01-02 15:04:05.05.000000000")

	d1 := []byte(str)
	err := ioutil.WriteFile(path, d1, 0644)
	if err != nil {
		return errors.Wrap(err, "failed to create log files")
	}
	return nil
}

func createLogFiles(s *testing.State) error {
	for i := 0; i < 7; i++ {
		path := getLogFilePathWithIndex(i)
		err := createLogsIfNotExist(path, s)
		if err != nil {
			return errors.Wrap(err, "failed to create a log file")
		}
	}
	return nil
}

func getLogFilePathWithIndex(index int) string {
	if index == 0 {
		return "/tmp/temporary_log_file_for_testing"
	}

	path := fmt.Sprintf("/tmp/temporary_log_file_for_testing.%d", index)
	return path
}

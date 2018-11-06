// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package decode provides common code to run Chrome binary tests for encoding.
package decode

import (
	"context"
	"os"
	"path/filepath"

	"chromiumos/tast/local/bundles/cros/video/lib/chrometest"
	"chromiumos/tast/local/bundles/cros/video/lib/logging"
	"chromiumos/tast/testing"
)

// RunAccelJPEGTest runs the jpeg_encode_accelerator_unittest binary. the
// 'testFiles' parameter should contain the list of all files required by the
// binary test.
func RunAccelJPEGTest(ctx context.Context, s *testing.State, name string, testFiles []string) {
	vl, err := logging.NewVideoLogger()
	if err != nil {
		s.Fatal("Failed to set values for verbose logging: ", err)
	}
	defer vl.Close()

	// The jpeg decode test doesn't operate on a single file, we need to pass a
	// directory where all test files are located. But we can only get the path
	// to individual test files, so the system can ensure that each file used in
	// the test is actually present in 'external_data.conf'. This means we need
	// to copy all files to a temporary directory before running the test.
	tempDir, err := chrometest.CreateWritableTempDir(name)
	if err != nil {
		s.Fatalf("Failed to create temp dir %s: %v", name, err)
	}
	defer os.RemoveAll(tempDir)

	// Copy all test files to temporary directory.
	for _, f := range testFiles {
		testfile := s.DataPath(f)
		tempfile := filepath.Join(tempDir, f)
		if err := chrometest.CopyFile(testfile, tempfile); err != nil {
			s.Errorf("Failed to copy test file %s to temp file %s: %v", testfile, tempfile, err)
		}
	}

	// Execute the test binary.
	args := []string{
		logging.ChromeVmoduleFlag(), "--test_data_path=" + tempDir + "/"}
	if err := chrometest.Run(ctx, s.OutDir(),
		"jpeg_decode_accelerator_unittest", args); err != nil {
		s.Fatal(err)
	}
}

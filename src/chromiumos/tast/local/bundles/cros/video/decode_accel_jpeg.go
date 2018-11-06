// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package video

import (
	"context"
	"os"
	"path/filepath"

	"chromiumos/tast/local/bundles/cros/video/lib/caps"
	"chromiumos/tast/local/bundles/cros/video/lib/chrometest"
	"chromiumos/tast/local/bundles/cros/video/lib/logging"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         DecodeAccelJPEG,
		Desc:         "Run Chrome jpeg_decode_accelerator_unittest",
		Attr:         []string{"informational"},
		SoftwareDeps: []string{caps.HWDecodeJPEG},
		Data:         jpegTestFiles,
	})
}

// jpegTestFiles lists the files required by the JPEG decode accelerator test.
var jpegTestFiles = []string{
	"peach_pi-1280x720.jpg",
	"peach_pi-40x23.jpg",
	"peach_pi-41x22.jpg",
	"peach_pi-41x23.jpg",
	"pixel-1280x720.jpg",
}

// DecodeAccelJPEG runs a set of HW JPEG decode tests, defined in
// jpeg_decode_accelerator_unittest.
func DecodeAccelJPEG(ctx context.Context, s *testing.State) {
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
	const name string = "DecodeAccelJPEG"
	tempDir, err := chrometest.CreateWritableTempDir(name)
	if err != nil {
		s.Fatalf("Failed to create temp dir %s: %v", name, err)
	}
	defer os.RemoveAll(tempDir)

	// Copy all test files to temporary directory.
	for _, f := range jpegTestFiles {
		testfile := s.DataPath(f)
		tempfile := filepath.Join(tempDir, f)
		if err := chrometest.CopyFile(testfile, tempfile); err != nil {
			s.Fatalf("Failed to copy test file %s to temp file %s: %v", testfile, tempfile, err)
		}
	}

	// Execute the test binary.
	args := []string{
		logging.ChromeVmoduleFlag(), "--test_data_path=" + tempDir + "/"}
	if err := chrometest.Run(ctx, s.OutDir(),
		"jpeg_decode_accelerator_unittest", args); err != nil {
		s.Fatal("Failed to run jpeg_decode_accelerator_unittest: ", err)
	}
}
